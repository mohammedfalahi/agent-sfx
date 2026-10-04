package gemini_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/adapters/gemini"
	"agent-sfx/internal/events"
)

func loadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "testdata", "gemini", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %s at %s: %v", filename, path, err)
	}
	return data
}

func TestNormalize_BeforeAgent(t *testing.T) {
	data := loadFixture(t, "before_agent.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected event, got nil")
	}
	if ev.Kind != events.EventTaskStarted {
		t.Errorf("expected task_started, got %s", ev.Kind)
	}
	if ev.Agent != "gemini" {
		t.Errorf("expected agent 'gemini', got %s", ev.Agent)
	}
	if ev.SessionID != "test-session-123" {
		t.Errorf("expected session_id 'test-session-123', got %s", ev.SessionID)
	}
}

func TestNormalize_AfterAgent(t *testing.T) {
	data := loadFixture(t, "after_agent.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected event, got nil")
	}
	if ev.Kind != events.EventTaskFinished {
		t.Errorf("expected task_finished, got %s", ev.Kind)
	}
}

func TestNormalize_NotificationToolPermission(t *testing.T) {
	data := loadFixture(t, "notification_tool_permission.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected event, got nil")
	}
	if ev.Kind != events.EventPermissionRequested {
		t.Errorf("expected permission_requested, got %s", ev.Kind)
	}
}

func TestNormalize_NotificationOther(t *testing.T) {
	data := loadFixture(t, "notification_other.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected other notification to be ignored, got: %+v", ev)
	}
}

func TestNormalize_NativeErrorObject(t *testing.T) {
	// Native Gemini CLI 0.62.0 out-of-workspace error object shape
	payload := `{
		"session_id": "sess-test",
		"hook_event_name": "AfterTool",
		"timestamp": "2026-10-04T12:00:00Z",
		"tool_name": "read_file",
		"tool_response": {
			"llmContent": "Path not in workspace: Attempted path resolves outside allowed directories",
			"returnDisplay": "Path not in workspace.",
			"error": {
				"message": "Path not in workspace: Attempted path resolves outside allowed directories",
				"type": "path_not_in_workspace"
			}
		}
	}`

	ev, err := gemini.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("Normalize error on native error object: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected error event from native error object, got nil")
	}
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError, got %s", ev.Kind)
	}
	if ev.ReasonCode != "tool_failure" {
		t.Errorf("expected tool_failure reason, got %s", ev.ReasonCode)
	}
}

func TestNormalize_MissingNullEmptyError(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{
			name: "error field null",
			payload: `{
				"session_id": "sess-test",
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"llmContent": "file contents",
					"error": null
				}
			}`,
		},
		{
			name: "error field empty object",
			payload: `{
				"session_id": "sess-test",
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"llmContent": "file contents",
					"error": {}
				}
			}`,
		},
		{
			name: "error field empty string",
			payload: `{
				"session_id": "sess-test",
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"llmContent": "file contents",
					"error": ""
				}
			}`,
		},
		{
			name: "error field missing completely",
			payload: `{
				"session_id": "sess-test",
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"llmContent": "file contents"
				}
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := gemini.Normalize([]byte(tc.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev != nil {
				t.Fatalf("expected non-error to return nil, got: %+v", ev)
			}
		})
	}
}

func TestNormalize_ShellExit7NoOutput(t *testing.T) {
	payload := `{
		"session_id": "sess-test",
		"hook_event_name": "AfterTool",
		"timestamp": "2026-10-04T12:00:00Z",
		"tool_name": "run_shell_command",
		"tool_response": {
			"llmContent": "<untrusted_context>\nOutput: (empty)\nExit Code: 7\nProcess Group PGID: 71803\n</untrusted_context>",
			"returnDisplay": "Command exited with code: 7"
		}
	}`

	ev, err := gemini.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected error event for shell exit 7, got nil")
	}
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError, got %s", ev.Kind)
	}
	if ev.ReasonCode != "shell_nonzero_exit" {
		t.Errorf("expected shell_nonzero_exit reason, got %s", ev.ReasonCode)
	}
}

func TestNormalize_ShellExit1WithStderr(t *testing.T) {
	payload := `{
		"session_id": "sess-test",
		"hook_event_name": "AfterTool",
		"timestamp": "2026-10-04T12:00:00Z",
		"tool_name": "run_shell_command",
		"tool_response": {
			"llmContent": "<untrusted_context>\nOutput: cat: /nonexistent_file_test: No such file or directory\nExit Code: 1\nProcess Group PGID: 55533\n</untrusted_context>",
			"returnDisplay": "cat: /nonexistent_file_test: No such file or directory"
		}
	}`

	ev, err := gemini.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected error event for shell exit 1, got nil")
	}
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError, got %s", ev.Kind)
	}
	if ev.ReasonCode != "shell_nonzero_exit" {
		t.Errorf("expected shell_nonzero_exit reason, got %s", ev.ReasonCode)
	}
}

func TestNormalize_SuccessfulShellCommandPrintingExitCode7(t *testing.T) {
	// Command: echo "Exit Code: 7" (successful exit code 0)
	// Formatted by Gemini CLI: "Output: Exit Code: 7\nProcess Group PGID: 12345"
	payload := `{
		"session_id": "sess-test",
		"hook_event_name": "AfterTool",
		"tool_name": "run_shell_command",
		"tool_response": {
			"llmContent": "<untrusted_context>\nOutput: Exit Code: 7\nProcess Group PGID: 12345\n</untrusted_context>",
			"returnDisplay": "Exit Code: 7"
		}
	}`

	ev, err := gemini.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Fatalf("stdout containing 'Exit Code: 7' on success must NOT trigger error event, got: %+v", ev)
	}
}

func TestNormalize_SuccessfulCommandWithStderrWarnings(t *testing.T) {
	// Command: echo "warning: deprecated feature" >&2 (exits 0)
	payload := `{
		"session_id": "sess-test",
		"hook_event_name": "AfterTool",
		"tool_name": "run_shell_command",
		"tool_response": {
			"llmContent": "<untrusted_context>\nOutput: warning: deprecated feature\nProcess Group PGID: 12345\n</untrusted_context>",
			"returnDisplay": "warning: deprecated feature"
		}
	}`

	ev, err := gemini.Normalize([]byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Fatalf("successful command with stderr warnings must NOT trigger error event, got: %+v", ev)
	}
}

func TestNormalize_MalformedOrUnsupportedPayloadShapes(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{
			name: "error is an array",
			payload: `{
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"error": [1, 2, 3]
				}
			}`,
		},
		{
			name: "error is an integer",
			payload: `{
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"error": 42
				}
			}`,
		},
		{
			name: "error is a boolean",
			payload: `{
				"hook_event_name": "AfterTool",
				"tool_name": "read_file",
				"tool_response": {
					"error": true
				}
			}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := gemini.Normalize([]byte(tc.payload))
			if err != nil {
				t.Fatalf("unexpected error on unsupported shape: %v", err)
			}
			if ev != nil {
				t.Fatalf("unsupported shape must return nil, got: %+v", ev)
			}
		})
	}
}

func TestNormalize_SessionStartProducesNoAudio(t *testing.T) {
	data := loadFixture(t, "session_start.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Fatalf("SessionStart must produce nil event, got %+v", ev)
	}
}

func TestNormalize_UnsupportedEvent(t *testing.T) {
	data := loadFixture(t, "unsupported_event.json")
	ev, err := gemini.Normalize(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Fatalf("unsupported event must produce nil event, got %+v", ev)
	}
}

func TestNormalize_MalformedJSON(t *testing.T) {
	ev, err := gemini.Normalize([]byte("not valid json"))
	if err == nil {
		t.Fatalf("expected error on malformed json, got nil")
	}
	if ev != nil {
		t.Fatalf("expected nil event on malformed json")
	}
}
