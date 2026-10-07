package claude

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"agent-sfx/internal/events"
)

// ClaudeHookPayload mirrors the authoritative stdin payload emitted by Claude Code 2.1.162.
type ClaudeHookPayload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path,omitempty"`
	Cwd            string `json:"cwd,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	HookEventName  string `json:"hook_event_name"`

	// Event-specific fields
	Prompt               string          `json:"prompt,omitempty"`
	StopHookActive       bool            `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string          `json:"last_assistant_message,omitempty"`
	ToolName             string          `json:"tool_name,omitempty"`
	ToolUseID            string          `json:"tool_use_id,omitempty"`
	ToolInput            json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse         json.RawMessage `json:"tool_response,omitempty"`
	Error                any             `json:"error,omitempty"`
	ErrorDetails         string          `json:"error_details,omitempty"`
	IsInterrupt          bool            `json:"is_interrupt,omitempty"`
	DurationMS           float64         `json:"duration_ms,omitempty"`
	Timestamp            string          `json:"timestamp,omitempty"`
}

// AskUserQuestionOption represents a selectable option in AskUserQuestion.
type AskUserQuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"`
}

// AskUserQuestionItem represents an individual question item in AskUserQuestion.
type AskUserQuestionItem struct {
	Question    string                  `json:"question"`
	Header      string                  `json:"header,omitempty"`
	Options     []AskUserQuestionOption `json:"options,omitempty"`
	MultiSelect bool                    `json:"multiSelect,omitempty"`
}

// AskUserQuestionInput represents the tool input object for AskUserQuestion.
type AskUserQuestionInput struct {
	Questions []AskUserQuestionItem `json:"questions"`
}

// validateClaudeAskUserPayload validates the tool_input against Claude Code's AskUserQuestion schema.
func validateClaudeAskUserPayload(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	var input AskUserQuestionInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return false
	}

	// Must have between 1 and 4 questions
	if len(input.Questions) < 1 || len(input.Questions) > 4 {
		return false
	}

	for _, q := range input.Questions {
		if strings.TrimSpace(q.Question) == "" {
			return false
		}
		// If options are provided, each option must have a non-empty label
		if len(q.Options) > 0 {
			for _, opt := range q.Options {
				if strings.TrimSpace(opt.Label) == "" {
					return false
				}
			}
		}
	}

	return true
}

// verifiedStopFailureErrors defines the exact error classification strings emitted by Claude Code 2.1.162
// in the StopFailure hook payload (defined by the internal jS8 schema enum).
// These are distinct from internal Anthropic provider/SDK error names (e.g. rate_limit_error vs rate_limit).
var verifiedStopFailureErrors = map[string]bool{
	"authentication_failed": true,
	"oauth_org_not_allowed": true,
	"billing_error":         true,
	"rate_limit":            true,
	"overloaded":            true,
	"invalid_request":       true,
	"model_not_found":       true,
	"server_error":          true,
	"unknown":               true,
	"max_output_tokens":     true,
}

// parseStopFailureError checks whether the error field matches a verified classification string.
// Object-shaped error payloads, assistant prose, refusal phrases, or substring matching are rejected.
func parseStopFailureError(raw any) (string, bool) {
	str, ok := raw.(string)
	if !ok {
		// Object-shaped or non-string error payloads are unsupported
		return "", false
	}
	str = strings.TrimSpace(str)
	if verifiedStopFailureErrors[str] {
		return str, true
	}
	return "", false
}

// hasNonEmptyToolError verifies that tool failure error content is non-empty.
func hasNonEmptyToolError(raw any) bool {
	if raw == nil {
		return false
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]any:
		if msg, ok := v["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return true
		}
		if typ, ok := v["type"].(string); ok && strings.TrimSpace(typ) != "" {
			return true
		}
	}
	return false
}

// Normalize parses Claude Code hook payload JSON into canonical events.
// It returns an empty slice for unsupported events, malformed question schemas,
// suppressed question-permission collisions, or tool interruptions.
func Normalize(payload []byte) ([]events.Event, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var p ClaudeHookPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("malformed claude hook payload: %w", err)
	}

	observedAt := time.Now()
	if p.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, p.Timestamp); err == nil {
			observedAt = t
		}
	}

	switch p.HookEventName {
	case "UserPromptSubmit":
		// User submitted prompt turn -> task_started
		dedupeKey := ""
		if p.SessionID != "" {
			dedupeKey = fmt.Sprintf("claude:%s:task_started", p.SessionID)
		}
		return []events.Event{
			{
				Kind:       events.EventTaskStarted,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
			},
		}, nil

	case "Stop":
		// Model response turn completed -> task_finished (turn completion only, not overall task success)
		dedupeKey := ""
		if p.SessionID != "" {
			dedupeKey = fmt.Sprintf("claude:%s:task_finished", p.SessionID)
		}
		return []events.Event{
			{
				Kind:       events.EventTaskFinished,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
				ReasonCode: "turn_completed",
			},
		}, nil

	case "PermissionRequest":
		// Direct permission prompt request
		// Collision avoidance: suppress when tool_name is AskUserQuestion so only waiting_for_user plays
		if strings.TrimSpace(p.ToolName) == "AskUserQuestion" {
			return nil, nil
		}

		dedupeKey := ""
		if p.SessionID != "" {
			if p.ToolUseID != "" {
				dedupeKey = fmt.Sprintf("claude:%s:permission_requested:%s", p.SessionID, p.ToolUseID)
			} else {
				dedupeKey = fmt.Sprintf("claude:%s:permission_requested:%s", p.SessionID, p.ToolName)
			}
		}
		return []events.Event{
			{
				Kind:       events.EventPermissionRequested,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
				ReasonCode: "permission_prompt",
			},
		}, nil

	case "PreToolUse":
		// Explicit AskUserQuestion dialog request with exact name anchoring
		if strings.TrimSpace(p.ToolName) != "AskUserQuestion" {
			return nil, nil
		}

		// Validate question payload against Claude Code's schema
		if !validateClaudeAskUserPayload(p.ToolInput) {
			return nil, nil
		}

		dedupeKey := ""
		if p.SessionID != "" {
			if p.ToolUseID != "" {
				dedupeKey = fmt.Sprintf("claude:%s:waiting_for_user:%s", p.SessionID, p.ToolUseID)
			} else {
				dedupeKey = fmt.Sprintf("claude:%s:waiting_for_user", p.SessionID)
			}
		}
		return []events.Event{
			{
				Kind:       events.EventWaitingForUser,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
				ReasonCode: "ask_user_question",
			},
		}, nil

	case "PostToolUseFailure":
		// Tool execution failure
		// User cancellation / interruption (is_interrupt: true) is explicitly excluded
		if p.IsInterrupt {
			return nil, nil
		}
		// Validate required tool_name
		if strings.TrimSpace(p.ToolName) == "" {
			return nil, nil
		}
		// Validate error field is present and non-empty
		if !hasNonEmptyToolError(p.Error) {
			return nil, nil
		}

		dedupeKey := ""
		if p.SessionID != "" {
			if p.ToolUseID != "" {
				dedupeKey = fmt.Sprintf("claude:%s:error:%s", p.SessionID, p.ToolUseID)
			} else {
				dedupeKey = fmt.Sprintf("claude:%s:error", p.SessionID)
			}
		}
		return []events.Event{
			{
				Kind:       events.EventError,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
				ReasonCode: "tool_failure",
			},
		}, nil

	case "StopFailure":
		// Turn stopped due to API, rate-limit, overload, or provider failure
		errStr, isAPIError := parseStopFailureError(p.Error)
		if !isAPIError {
			return nil, nil
		}

		dedupeKey := ""
		if p.SessionID != "" {
			dedupeKey = fmt.Sprintf("claude:%s:error:stop_failure", p.SessionID)
		}
		_ = errStr
		return []events.Event{
			{
				Kind:       events.EventError,
				Agent:      "claude",
				SessionID:  p.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  dedupeKey,
				ReasonCode: "claude_stop_failure",
			},
		}, nil

	default:
		// Unsupported hook events produce zero events and neutral behavior
		// (PostToolUse, SessionStart, SessionEnd, PreCompact, PostCompact, CwdChanged, FileChanged, Notification, etc.)
		return nil, nil
	}
}
