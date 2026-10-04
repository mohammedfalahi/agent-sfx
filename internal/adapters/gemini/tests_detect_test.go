package gemini_test

import (
	"encoding/json"
	"testing"

	"agent-sfx/internal/adapters/gemini"
	"agent-sfx/internal/events"
)

func buildShellPayload(command string, isBackground bool, llmContent string, errorField interface{}) []byte {
	toolInput := map[string]interface{}{
		"command":       command,
		"is_background": isBackground,
	}
	toolResponse := map[string]interface{}{
		"llmContent":    llmContent,
		"returnDisplay": "display",
	}
	if errorField != nil {
		toolResponse["error"] = errorField
	}

	payload := map[string]interface{}{
		"session_id":      "test-session-123",
		"hook_event_name": "AfterTool",
		"timestamp":       "2026-10-04T12:00:00Z",
		"tool_name":       "run_shell_command",
		"tool_input":      toolInput,
		"tool_response":   toolResponse,
	}

	bytes, _ := json.Marshal(payload)
	return bytes
}

func TestTestsDetect_GoTest_PositiveCases(t *testing.T) {
	cases := []struct {
		name       string
		command    string
		output     string
		wantReason string
	}{
		{
			name:       "multi-package newly executed pass",
			command:    "go test -count=1 -race ./...",
			output:     "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t1.860s\nok  \tagent-sfx/internal/config\t2.523s\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_passed",
		},
		{
			name:       "single-package pass with PASS line",
			command:    "go test .",
			output:     "<untrusted_context>\nOutput: PASS\nok  \tagent-sfx\t0.045s\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_passed",
		},
		{
			name:       "all packages cached pass",
			command:    "go test ./...",
			output:     "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t(cached)\nok  \tagent-sfx/internal/config\t(cached)\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_cached",
		},
		{
			name:       "mixed executed and cached pass",
			command:    "go test ./...",
			output:     "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t1.234s\nok  \tagent-sfx/internal/config\t(cached)\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_passed",
		},
		{
			name:       "mixed qualifying pass with [no test files] package",
			command:    "go test ./...",
			output:     "<untrusted_context>\nOutput: ?   \tagent-sfx/cmd/agent-sfx\t[no test files]\nok  \tagent-sfx/internal/audio\t1.500s\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_passed",
		},
		{
			name:       "command with quotes in -run filter",
			command:    `go test -run "TestFoo" ./...`,
			output:     "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t0.500s\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "go_test_passed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildShellPayload(tc.command, false, tc.output, nil)
			ev, err := gemini.Normalize(payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev == nil {
				t.Fatalf("expected tests_passed event, got nil")
			}
			if ev.Kind != events.EventTestsPassed {
				t.Errorf("expected EventTestsPassed, got %s", ev.Kind)
			}
			if ev.ReasonCode != tc.wantReason {
				t.Errorf("expected reason %s, got %s", tc.wantReason, ev.ReasonCode)
			}
		})
	}
}

func TestTestsDetect_GoTest_NegativeCases(t *testing.T) {
	cases := []struct {
		name    string
		command string
		output  string
	}{
		{
			name:    "only [no test files]",
			command: "go test ./cmd/...",
			output:  "<untrusted_context>\nOutput: ?   \tagent-sfx/cmd/agent-sfx\t[no test files]\n?   \tagent-sfx/cmd/gen-sounds\t[no test files]\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "only [no tests to run]",
			command: "go test -run NoMatch ./...",
			output:  "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t0.001s [no tests to run]\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "package test failure (exit code 1 is handled as error)",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: --- FAIL: TestBar (0.01s)\nFAIL\nFAIL\tagent-sfx/internal/audio\t0.050s\nExit Code: 1\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "package test failure even if exit code metadata was omitted",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: --- FAIL: TestBar (0.01s)\nFAIL\nFAIL\tagent-sfx/internal/audio\t0.050s\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "compilation failure [build failed]",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: # agent-sfx/internal/audio\ninternal/audio/wav.go:10: syntax error\nFAIL\tagent-sfx/internal/audio [build failed]\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "panic during test",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: panic: runtime error\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "unsupported -json output mode",
			command: "go test -json ./...",
			output:  `<untrusted_context>\nOutput: {"Action":"pass","Package":"pkg"}\nProcess Group PGID: 12345\n</untrusted_context>`,
		},
		{
			name:    "go build is not a test runner",
			command: "go build ./...",
			output:  "<untrusted_context>\nOutput: (empty)\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "go vet is not a test runner",
			command: "go vet ./...",
			output:  "<untrusted_context>\nOutput: (empty)\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "echo fake test pass output",
			command: `echo "ok  agent-sfx/pkg 1.00s"`,
			output:  "<untrusted_context>\nOutput: ok  agent-sfx/pkg 1.00s\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "misleading summary inside test body followed by real failure",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: ok  fake/passed/pkg 0.01s\n--- FAIL: TestReal (0.01s)\nFAIL\tagent-sfx/real\t0.05s\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "truncated output missing process group pgid",
			command: "go test ./...",
			output:  "<untrusted_context>\nOutput: ok  agent-sfx/internal/audio\t1.00s\n</untrusted_context>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildShellPayload(tc.command, false, tc.output, nil)
			ev, err := gemini.Normalize(payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev != nil && ev.Kind == events.EventTestsPassed {
				t.Fatalf("test case %q must NOT produce EventTestsPassed, got: %+v", tc.name, ev)
			}
		})
	}
}

func TestTestsDetect_Pytest_PositiveCases(t *testing.T) {
	cases := []struct {
		name       string
		command    string
		output     string
		wantReason string
	}{
		{
			name:       "direct pytest pass",
			command:    "pytest tests/",
			output:     "<untrusted_context>\nOutput: ========================= 5 passed in 0.12s =========================\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "pytest_passed",
		},
		{
			name:       "python -m pytest pass",
			command:    "python -m pytest tests/test_core.py",
			output:     "<untrusted_context>\nOutput: ========================= 12 passed in 1.45s =========================\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "pytest_passed",
		},
		{
			name:       "python3 -m pytest pass",
			command:    "python3 -m pytest",
			output:     "<untrusted_context>\nOutput: ========================= 1 passed in 0.01s =========================\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "pytest_passed",
		},
		{
			name:       "mixed passed and skipped",
			command:    "pytest",
			output:     "<untrusted_context>\nOutput: ==================== 4 passed, 2 skipped in 0.35s ====================\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "pytest_passed",
		},
		{
			name:       "passed with warnings",
			command:    "pytest -v",
			output:     "<untrusted_context>\nOutput: ==================== 8 passed, 1 warning in 0.88s ====================\nProcess Group PGID: 12345\n</untrusted_context>",
			wantReason: "pytest_passed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildShellPayload(tc.command, false, tc.output, nil)
			ev, err := gemini.Normalize(payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev == nil {
				t.Fatalf("expected tests_passed event, got nil")
			}
			if ev.Kind != events.EventTestsPassed {
				t.Errorf("expected EventTestsPassed, got %s", ev.Kind)
			}
			if ev.ReasonCode != tc.wantReason {
				t.Errorf("expected reason %s, got %s", tc.wantReason, ev.ReasonCode)
			}
		})
	}
}

func TestTestsDetect_Pytest_NegativeCases(t *testing.T) {
	cases := []struct {
		name    string
		command string
		output  string
	}{
		{
			name:    "skipped only",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ========================= 3 skipped in 0.02s =========================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "no tests ran",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ======================= no tests ran in 0.01s ========================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "mixed passed and failed (exit code 1)",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ==================== 1 failed, 2 passed in 0.15s ====================\nExit Code: 1\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "mixed passed and failed without exit code metadata",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ==================== 1 failed, 2 passed in 0.15s ====================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "pytest error section present",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: === ERRORS ===\n========================= 1 error in 0.05s =========================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "interrupted execution",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ==================== 2 passed, KeyboardInterrupt ====================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "misleading fake pass printed inside test output followed by failure",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: ========================= 10 passed in 0.05s =========================\n=== FAILURES ===\n=================== 1 failed, 1 passed in 0.20s ===================\nProcess Group PGID: 12345\n</untrusted_context>",
		},
		{
			name:    "incomplete output missing summary line",
			command: "pytest",
			output:  "<untrusted_context>\nOutput: tests/test_one.py ..\nProcess Group PGID: 12345\n</untrusted_context>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := buildShellPayload(tc.command, false, tc.output, nil)
			ev, err := gemini.Normalize(payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev != nil && ev.Kind == events.EventTestsPassed {
				t.Fatalf("test case %q must NOT produce EventTestsPassed, got: %+v", tc.name, ev)
			}
		})
	}
}

func TestTestsDetect_CompoundAndPipelineRejection(t *testing.T) {
	compoundCommands := []string{
		"go test ./...; true",
		"pytest || true",
		"go test ./... && echo done",
		"pytest | cat",
		"go test $(echo ./...)",
		"pytest `cat test_list.txt`",
		"go test ./... > /dev/null",
		"pytest < inputs.txt",
		"CGO_ENABLED=0 go test ./...",
		"go test ./...\nrm -rf /tmp/test",
	}

	validOutput := "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t1.00s\nProcess Group PGID: 12345\n</untrusted_context>"

	for _, cmd := range compoundCommands {
		t.Run(cmd, func(t *testing.T) {
			payload := buildShellPayload(cmd, false, validOutput, nil)
			ev, err := gemini.Normalize(payload)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev != nil && ev.Kind == events.EventTestsPassed {
				t.Fatalf("compound command %q must be rejected, got %+v", cmd, ev)
			}
		})
	}
}

func TestTestsDetect_BackgroundAndAbortedRejection(t *testing.T) {
	validOutput := "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t1.00s\nProcess Group PGID: 12345\n</untrusted_context>"

	// 1. Tool input has is_background = true
	t.Run("tool_input is_background", func(t *testing.T) {
		payload := buildShellPayload("go test ./...", true, validOutput, nil)
		ev, err := gemini.Normalize(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev != nil {
			t.Fatalf("background command must produce nil event, got %+v", ev)
		}
	})

	// 2. LLM content contains background notice
	t.Run("llmContent background message", func(t *testing.T) {
		bgOutput := "Command moved to background (PID: 9999). Output hidden. Press Ctrl+B to view."
		payload := buildShellPayload("go test ./...", false, bgOutput, nil)
		ev, err := gemini.Normalize(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev != nil {
			t.Fatalf("backgrounded output must produce nil event, got %+v", ev)
		}
	})

	// 3. LLM content contains cancellation notice
	t.Run("llmContent cancellation", func(t *testing.T) {
		abortedOutput := "Command cancelled by user.\nOutput before cancellation:\nok  agent-sfx/internal/audio 0.5s"
		payload := buildShellPayload("go test ./...", false, abortedOutput, nil)
		ev, err := gemini.Normalize(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev != nil {
			t.Fatalf("cancelled output must produce nil event, got %+v", ev)
		}
	})

	// 4. LLM content contains timeout notice
	t.Run("llmContent timeout", func(t *testing.T) {
		timeoutOutput := "Command timed out after 30000ms"
		payload := buildShellPayload("go test ./...", false, timeoutOutput, nil)
		ev, err := gemini.Normalize(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ev != nil {
			t.Fatalf("timed out output must produce nil event, got %+v", ev)
		}
	})
}

func TestTestsDetect_ErrorPrecedenceOverTestsPassed(t *testing.T) {
	// If a command prints passing test output but has an Exit Code: 1 or tool_response.error,
	// it must be classified as EventError, NEVER EventTestsPassed.
	output := "<untrusted_context>\nOutput: ok  \tagent-sfx/internal/audio\t1.00s\nExit Code: 1\nProcess Group PGID: 12345\n</untrusted_context>"
	payload := buildShellPayload("go test ./...", false, output, nil)

	ev, err := gemini.Normalize(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev == nil {
		t.Fatalf("expected EventError, got nil")
	}
	if ev.Kind != events.EventError {
		t.Errorf("expected EventError precedence over tests_passed, got: %s", ev.Kind)
	}
	if ev.ReasonCode != "shell_nonzero_exit" {
		t.Errorf("expected shell_nonzero_exit reason, got: %s", ev.ReasonCode)
	}

	// Also verify with tool_response.error object
	errorObj := map[string]string{
		"message": "execution failed",
		"type":    "runtime_error",
	}
	payloadWithErrorObj := buildShellPayload("go test ./...", false, output, errorObj)
	ev2, err := gemini.Normalize(payloadWithErrorObj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev2 == nil || ev2.Kind != events.EventError {
		t.Fatalf("expected EventError from tool_response.error precedence, got %+v", ev2)
	}
}
