package gemini

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agent-sfx/internal/events"
)

// GeminiBaseEnvelope represents the common fields present in all Gemini CLI 0.62.0 hook inputs.
type GeminiBaseEnvelope struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	HookEventName  string `json:"hook_event_name"`
	Timestamp      string `json:"timestamp"`
}

// NotificationPayload represents the Notification hook input.
type NotificationPayload struct {
	GeminiBaseEnvelope
	NotificationType string          `json:"notification_type"`
	Message          string          `json:"message"`
	Details          json.RawMessage `json:"details"`
}

// ToolResponse represents the execution response structure inside AfterTool.
type ToolResponse struct {
	LLMContent    json.RawMessage `json:"llmContent"`
	ReturnDisplay json.RawMessage `json:"returnDisplay"`
	Error         json.RawMessage `json:"error,omitempty"`
}

// AfterToolPayload represents the AfterTool hook input.
type AfterToolPayload struct {
	GeminiBaseEnvelope
	ToolName     string          `json:"tool_name"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse *ToolResponse   `json:"tool_response"`
}

// parseToolErrorObject inspects raw error JSON and identifies verified error shapes.
// Returns (hasError, reasonCode).
// - Recognizes verified {message, type} and {message} shapes.
// - Recognizes non-empty strings.
// - Treats null, empty objects, empty strings, and unsupported shapes as non-errors.
func parseToolErrorObject(raw json.RawMessage) (bool, string) {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 || trimmed == "null" {
		return false, ""
	}

	// 1. Plain string error (e.g. from custom/test fixtures)
	if strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			if strings.TrimSpace(str) != "" {
				return true, "tool_failure"
			}
		}
		return false, ""
	}

	// 2. Verified error object shape {message, type}
	if strings.HasPrefix(trimmed, `{`) && strings.HasSuffix(trimmed, `}`) {
		var obj struct {
			Message *string `json:"message"`
			Type    *string `json:"type"`
		}
		if err := json.Unmarshal(raw, &obj); err == nil {
			hasMessage := obj.Message != nil && strings.TrimSpace(*obj.Message) != ""
			hasType := obj.Type != nil && strings.TrimSpace(*obj.Type) != ""
			if hasMessage || hasType {
				return true, "tool_failure"
			}
		}
		// Empty object or unparseable shape -> non-error
		return false, ""
	}

	// Unsupported shapes (arrays, numbers, booleans) -> non-error
	return false, ""
}

// parseShellExitCode scans the trailing metadata block of Gemini CLI 0.62.0's ShellTool llmContent.
// In ShellTool, Exit Code is ONLY appended when exitCode != 0, positioned in the trailing
// metadata block immediately before Signal, Background PIDs, or Process Group PGID.
// It stops scanning if any stdout/stderr line is encountered to avoid matching command output.
func parseShellExitCode(llmContentRaw json.RawMessage) (bool, int) {
	if len(llmContentRaw) == 0 {
		return false, 0
	}

	var content string
	if err := json.Unmarshal(llmContentRaw, &content); err != nil {
		content = string(llmContentRaw)
	}

	// Strip <untrusted_context> wrapper tags if present
	content = strings.TrimPrefix(content, "<untrusted_context>")
	content = strings.TrimSuffix(content, "</untrusted_context>")
	content = strings.TrimSpace(content)

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return false, 0
	}

	// Scan from the bottom up through the trailing metadata block
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "Process Group PGID: ") ||
			strings.HasPrefix(line, "Background PIDs: ") ||
			strings.HasPrefix(line, "Signal: ") {
			// Recognized trailing metadata line; continue searching upward
			continue
		}

		if strings.HasPrefix(line, "Exit Code: ") {
			codeStr := strings.TrimPrefix(line, "Exit Code: ")
			codeStr = strings.TrimSpace(codeStr)
			if code, err := strconv.Atoi(codeStr); err == nil {
				if code != 0 {
					return true, code
				}
				return false, 0
			}
			return false, 0
		}

		// Encountered any line not part of trailing metadata (e.g. Output: ... or command stdout/stderr)
		// Stop immediately to ensure we never match arbitrary command text
		break
	}

	return false, 0
}

// Normalize parses raw Gemini hook stdin JSON and maps it to a canonical Event.
// Returns nil, nil if the event is unsupported or should produce no audio.
// Unknown shapes or missing required fields are ignored without guessing.
func Normalize(payload []byte) (*events.Event, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var base GeminiBaseEnvelope
	if err := json.Unmarshal(payload, &base); err != nil {
		return nil, fmt.Errorf("failed to parse base hook envelope: %w", err)
	}

	observedAt := time.Now()
	if base.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, base.Timestamp); err == nil {
			observedAt = t
		}
	}

	switch base.HookEventName {
	case "BeforeAgent":
		return &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      "gemini",
			SessionID:  base.SessionID,
			ObservedAt: observedAt,
			DedupeKey:  fmt.Sprintf("before_agent:%s", base.Timestamp),
		}, nil

	case "AfterAgent":
		return &events.Event{
			Kind:       events.EventTaskFinished,
			Agent:      "gemini",
			SessionID:  base.SessionID,
			ObservedAt: observedAt,
			DedupeKey:  fmt.Sprintf("after_agent:%s", base.Timestamp),
		}, nil

	case "Notification":
		var notif NotificationPayload
		if err := json.Unmarshal(payload, &notif); err != nil {
			return nil, fmt.Errorf("failed to parse Notification payload: %w", err)
		}
		if notif.NotificationType == "ToolPermission" {
			return &events.Event{
				Kind:       events.EventPermissionRequested,
				Agent:      "gemini",
				SessionID:  base.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  fmt.Sprintf("permission:%s", base.Timestamp),
			}, nil
		}
		return nil, nil

	case "AfterTool":
		var afterTool AfterToolPayload
		if err := json.Unmarshal(payload, &afterTool); err != nil {
			return nil, fmt.Errorf("failed to parse AfterTool payload: %w", err)
		}

		if afterTool.ToolResponse == nil {
			return nil, nil
		}

		// 1. Check verified tool_response.error (supports verified object and string shapes)
		if hasErr, reason := parseToolErrorObject(afterTool.ToolResponse.Error); hasErr {
			return &events.Event{
				Kind:       events.EventError,
				Agent:      "gemini",
				SessionID:  base.SessionID,
				ObservedAt: observedAt,
				DedupeKey:  fmt.Sprintf("tool_error:%s:%s", afterTool.ToolName, base.Timestamp),
				ReasonCode: reason,
			}, nil
		}

		// 2. Check shell nonzero exit metadata (only for run_shell_command)
		if afterTool.ToolName == "run_shell_command" {
			if isExitErr, _ := parseShellExitCode(afterTool.ToolResponse.LLMContent); isExitErr {
				return &events.Event{
					Kind:       events.EventError,
					Agent:      "gemini",
					SessionID:  base.SessionID,
					ObservedAt: observedAt,
					DedupeKey:  fmt.Sprintf("shell_error:%s:%s", afterTool.ToolName, base.Timestamp),
					ReasonCode: "shell_nonzero_exit",
				}, nil
			}
		}

		// 3. Check for tests_passed (only for run_shell_command)
		// Error detection (steps 1 & 2) strictly takes precedence.
		if afterTool.ToolName == "run_shell_command" {
			if isPass, reason := DetectTestsPassed(afterTool.ToolInput, afterTool.ToolResponse.LLMContent); isPass {
				return &events.Event{
					Kind:       events.EventTestsPassed,
					Agent:      "gemini",
					SessionID:  base.SessionID,
					ObservedAt: observedAt,
					DedupeKey:  fmt.Sprintf("tests_passed:%s:%s", afterTool.ToolName, base.Timestamp),
					ReasonCode: reason,
				}, nil
			}
		}

		// Successful tool execution produces no audio
		return nil, nil

	case "SessionStart":
		return nil, nil

	default:
		return nil, nil
	}
}
