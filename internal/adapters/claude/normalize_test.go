package claude_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sfx/internal/adapters/claude"
	"agent-sfx/internal/events"
)

func TestNormalize_UserPromptSubmit(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "user_prompt_submit.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventTaskStarted {
		t.Errorf("expected EventTaskStarted, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.SessionID != "sess-claude-01" {
		t.Errorf("expected session_id 'sess-claude-01', got %s", ev.SessionID)
	}
	if ev.DedupeKey != "claude:sess-claude-01:task_started" {
		t.Errorf("expected dedupe key 'claude:sess-claude-01:task_started', got %s", ev.DedupeKey)
	}
}

func TestNormalize_Stop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "stop.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventTaskFinished {
		t.Errorf("expected EventTaskFinished, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.ReasonCode != "turn_completed" {
		t.Errorf("expected reason code 'turn_completed', got %s", ev.ReasonCode)
	}
}

func TestNormalize_PermissionRequest_NonAskUser(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "permission_request_bash.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventPermissionRequested {
		t.Errorf("expected EventPermissionRequested, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.ReasonCode != "permission_prompt" {
		t.Errorf("expected reason code 'permission_prompt', got %s", ev.ReasonCode)
	}
}

func TestNormalize_PermissionRequest_CollisionAvoidance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "permission_request_ask_user.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// Collision avoidance: AskUserQuestion in PermissionRequest must return 0 events
	if len(evs) != 0 {
		t.Errorf("expected 0 events for AskUserQuestion in PermissionRequest, got %d", len(evs))
	}
}

func TestNormalize_PreToolUse_AskUserQuestion_Valid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "pre_tool_use_ask_user.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventWaitingForUser {
		t.Errorf("expected EventWaitingForUser, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.ReasonCode != "ask_user_question" {
		t.Errorf("expected reason code 'ask_user_question', got %s", ev.ReasonCode)
	}
	if ev.DedupeKey != "claude:sess-claude-01:waiting_for_user:toolu_01ABC" {
		t.Errorf("expected invocation-level dedupe key with tool_use_id, got %s", ev.DedupeKey)
	}
}

func TestNormalize_PreToolUse_AskUserQuestion_InvalidSchema(t *testing.T) {
	// Empty questions array
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "pre_tool_use_ask_user_invalid.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected 0 events for empty questions schema, got %d", len(evs))
	}

	// Empty question text
	emptyQ := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"   "}]}}`
	evs, err = claude.Normalize([]byte(emptyQ))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for whitespace question text")
	}

	// Option missing label
	emptyOpt := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"A?","options":[{"label":""}]}]}}`
	evs, err = claude.Normalize([]byte(emptyOpt))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for option with empty label")
	}

	// Greater than 4 questions
	tooMany := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"1"},{"question":"2"},{"question":"3"},{"question":"4"},{"question":"5"}]}}`
	evs, err = claude.Normalize([]byte(tooMany))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for >4 questions")
	}
}

func TestNormalize_PreToolUse_OtherTool(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "pre_tool_use_bash.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// Non-question tools in PreToolUse must return 0 events
	if len(evs) != 0 {
		t.Errorf("expected 0 events for Bash in PreToolUse, got %d", len(evs))
	}
}

func TestNormalize_PostToolUseFailure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "post_tool_use_failure.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.ReasonCode != "tool_failure" {
		t.Errorf("expected reason code 'tool_failure', got %s", ev.ReasonCode)
	}
	if ev.DedupeKey != "claude:sess-claude-01:error:toolu_04JKL" {
		t.Errorf("expected invocation-level dedupe key with tool_use_id, got %s", ev.DedupeKey)
	}
}

func TestNormalize_PostToolUseFailure_Interrupted(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "post_tool_use_failure_interrupted.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// Interrupted tool calls must NOT emit error sound
	if len(evs) != 0 {
		t.Errorf("expected 0 events for interrupted tool failure, got %d", len(evs))
	}
}

func TestNormalize_StopFailure_APIError(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "stop_failure.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0]
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError, got %s", ev.Kind)
	}
	if ev.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", ev.Agent)
	}
	if ev.ReasonCode != "claude_stop_failure" {
		t.Errorf("expected reason code 'claude_stop_failure', got %s", ev.ReasonCode)
	}
}

func TestNormalize_StopFailure_Refusal(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "stop_failure_refusal.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	// Model refusals are not API failures and must NOT emit error sound
	if len(evs) != 0 {
		t.Errorf("expected 0 events for model refusal in StopFailure, got %d", len(evs))
	}
}

func TestNormalize_UnsupportedEvents(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "claude", "unsupported_event.json"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	evs, err := claude.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected 0 events for PostToolUse, got %d", len(evs))
	}

	// Verify additional unsupported events
	unsupported := []string{
		"SessionStart", "SessionEnd", "PreCompact", "PostCompact",
		"CwdChanged", "FileChanged", "Notification", "SubagentStart", "SubagentStop",
	}
	for _, evt := range unsupported {
		payload := `{"session_id":"s1","hook_event_name":"` + evt + `"}`
		res, err := claude.Normalize([]byte(payload))
		if err != nil {
			t.Errorf("unexpected error on %s: %v", evt, err)
		}
		if len(res) != 0 {
			t.Errorf("expected 0 events for %s, got %d", evt, len(res))
		}
	}
}

func TestNormalize_TestsPassed_Unsupported(t *testing.T) {
	// PostToolUse for Bash with test pass output must remain unsupported for Claude initially
	payload := `{
		"session_id": "sess-claude-01",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "go test ./..."},
		"tool_response": {"stdout": "ok  pkg  0.05s\nPASS\n", "code": 0}
	}`
	evs, err := claude.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected tests_passed to remain explicitly unsupported for Claude in Phase 2A, got %d events", len(evs))
	}
}

func TestNormalize_UsageExhausted_Unsupported(t *testing.T) {
	// No payload triggers usage_exhausted
	payloads := []string{
		`{"session_id":"s1","hook_event_name":"StopFailure","error":"rate_limit_error"}`,
		`{"session_id":"s1","hook_event_name":"StopFailure","error":"billing_error"}`,
	}
	for _, p := range payloads {
		evs, err := claude.Normalize([]byte(p))
		if err != nil {
			t.Fatalf("Normalize failed: %v", err)
		}
		for _, ev := range evs {
			if ev.Kind == events.EventUsageExhausted {
				t.Errorf("usage_exhausted must remain unsupported, got EventUsageExhausted")
			}
		}
	}
}

func TestNormalize_Deduplication_InvocationLevel(t *testing.T) {
	// Two distinct tool invocations with different tool_use_ids have different dedupe keys
	p1 := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"AskUserQuestion","tool_use_id":"toolu_01","tool_input":{"questions":[{"question":"Q1?"}]}}`
	p2 := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"AskUserQuestion","tool_use_id":"toolu_02","tool_input":{"questions":[{"question":"Q2?"}]}}`

	evs1, _ := claude.Normalize([]byte(p1))
	evs2, _ := claude.Normalize([]byte(p2))

	if evs1[0].DedupeKey == evs2[0].DedupeKey {
		t.Errorf("distinct tool_use_ids should produce distinct DedupeKeys: %s vs %s", evs1[0].DedupeKey, evs2[0].DedupeKey)
	}

	// Replayed identical payload produces identical DedupeKey
	evs1Replay, _ := claude.Normalize([]byte(p1))
	if evs1[0].DedupeKey != evs1Replay[0].DedupeKey {
		t.Errorf("identical payloads should produce identical DedupeKeys")
	}
}

func TestNormalize_MalformedJSON(t *testing.T) {
	// Empty payload
	evs, err := claude.Normalize([]byte{})
	if err != nil || len(evs) != 0 {
		t.Errorf("empty payload should return nil error and nil events")
	}

	// Bad JSON syntax
	_, err = claude.Normalize([]byte(`{bad-json`))
	if err == nil {
		t.Errorf("expected error for malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "malformed claude hook payload") {
		t.Errorf("expected 'malformed claude hook payload' error, got %v", err)
	}
}

func TestNormalize_StopFailure_VerifiedClassificationStrings(t *testing.T) {
	verified := []string{
		"authentication_failed",
		"oauth_org_not_allowed",
		"billing_error",
		"rate_limit",
		"overloaded",
		"invalid_request",
		"model_not_found",
		"server_error",
		"unknown",
		"max_output_tokens",
	}

	for _, errStr := range verified {
		payload := `{"session_id":"s1","hook_event_name":"StopFailure","error":"` + errStr + `"}`
		evs, err := claude.Normalize([]byte(payload))
		if err != nil {
			t.Fatalf("Normalize failed for %s: %v", errStr, err)
		}
		if len(evs) != 1 {
			t.Fatalf("expected 1 event for verified error %s, got %d", errStr, len(evs))
		}
		if evs[0].Kind != events.EventError {
			t.Errorf("expected EventError for %s, got %s", errStr, evs[0].Kind)
		}
		if evs[0].ReasonCode != "claude_stop_failure" {
			t.Errorf("expected ReasonCode 'claude_stop_failure' for %s, got %s", errStr, evs[0].ReasonCode)
		}
		// Confirm billing_error never maps to usage_exhausted
		if evs[0].Kind == events.EventUsageExhausted {
			t.Errorf("billing_error must map to EventError, NEVER EventUsageExhausted")
		}
	}
}

func TestNormalize_StopFailure_ObjectAndProseRejection(t *testing.T) {
	// 1. Object-shaped error payloads must be rejected as unsupported
	objPayload := `{"session_id":"s1","hook_event_name":"StopFailure","error":{"type":"api_error","message":"failed"}}`
	evs, err := claude.Normalize([]byte(objPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected 0 events for object-shaped error payload, got %d", len(evs))
	}

	// 2. Substring matching in assistant prose must be rejected
	prosePayload := `{"session_id":"s1","hook_event_name":"StopFailure","error":"There was an api_error when calling the server"}`
	evs, err = claude.Normalize([]byte(prosePayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected 0 events for assistant prose error, got %d", len(evs))
	}

	// 3. Unrecognized error string rejected
	unrecPayload := `{"session_id":"s1","hook_event_name":"StopFailure","error":"custom_random_failure"}`
	evs, err = claude.Normalize([]byte(unrecPayload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("expected 0 events for unrecognized error string, got %d", len(evs))
	}
}

func TestNormalize_PostToolUseFailure_MalformedRejection(t *testing.T) {
	// Missing tool_name
	p1 := `{"session_id":"s1","hook_event_name":"PostToolUseFailure","error":"command failed"}`
	evs, err := claude.Normalize([]byte(p1))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for missing tool_name in PostToolUseFailure")
	}

	// Whitespace tool_name
	p2 := `{"session_id":"s1","hook_event_name":"PostToolUseFailure","tool_name":"   ","error":"command failed"}`
	evs, err = claude.Normalize([]byte(p2))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for whitespace tool_name in PostToolUseFailure")
	}

	// Missing error field
	p3 := `{"session_id":"s1","hook_event_name":"PostToolUseFailure","tool_name":"Bash"}`
	evs, err = claude.Normalize([]byte(p3))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for missing error field in PostToolUseFailure")
	}

	// Empty string error field
	p4 := `{"session_id":"s1","hook_event_name":"PostToolUseFailure","tool_name":"Bash","error":""}`
	evs, err = claude.Normalize([]byte(p4))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for empty error string in PostToolUseFailure")
	}

	// Empty object error field
	p5 := `{"session_id":"s1","hook_event_name":"PostToolUseFailure","tool_name":"Bash","error":{}}`
	evs, err = claude.Normalize([]byte(p5))
	if err != nil || len(evs) != 0 {
		t.Errorf("expected 0 events for empty error object in PostToolUseFailure")
	}
}
