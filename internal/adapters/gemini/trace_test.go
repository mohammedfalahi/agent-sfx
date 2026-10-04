package gemini_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-sfx/internal/adapters/gemini"
)

func TestTrace_DefaultDisabledCreatesNothing(t *testing.T) {
	t.Setenv("AGENT_SFX_TRACE_LOG", "")

	// Verify TraceLogPath is empty
	if p := gemini.TraceLogPath(); p != "" {
		t.Fatalf("expected empty TraceLogPath when AGENT_SFX_TRACE_LOG is unset, got %q", p)
	}

	tempDir := t.TempDir()
	canaryPath := filepath.Join(tempDir, "canary.log")

	// Call LogTrace directly
	gemini.LogTrace(gemini.TraceEntry{
		HookEventName:  "AfterTool",
		ToolName:       "read_file",
		Classification: "error",
	})

	// Call ProcessHook with error payload
	rcv := &gemini.Receiver{
		SocketPath: "/nonexistent.sock",
		TestMode:   false,
	}

	payload := `{
		"session_id": "test-sess",
		"hook_event_name": "AfterTool",
		"tool_name": "read_file",
		"tool_response": {
			"error": {
				"message": "some error",
				"type": "some_type"
			}
		}
	}`

	var stdout bytes.Buffer
	res, err := rcv.ProcessHook(context.Background(), strings.NewReader(payload), &stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "{}\n" {
		t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
	}
	if res == nil {
		t.Fatalf("expected non-nil HookResult")
	}

	// Assert no trace file created at canary or default paths
	if _, err := os.Stat(canaryPath); !os.IsNotExist(err) {
		t.Errorf("canary file unexpectedly exists: %v", err)
	}
}

func TestTrace_ExplicitOptInProducesExpectedMetadata(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "trace-sub", "trace.log")
	t.Setenv("AGENT_SFX_TRACE_LOG", logPath)

	rcv := &gemini.Receiver{
		SocketPath: "/nonexistent.sock",
		TestMode:   false,
	}

	payload := `{
		"session_id": "test-sess",
		"hook_event_name": "AfterTool",
		"tool_name": "read_file",
		"tool_response": {
			"error": {
				"message": "file not found",
				"type": "not_found"
			}
		}
	}`

	var stdout bytes.Buffer
	_, err := rcv.ProcessHook(context.Background(), strings.NewReader(payload), &stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "{}\n" {
		t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	// Verify trace file exists
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read trace log file: %v", err)
	}

	// Check file permissions on Unix systems (owner-only 0600)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(logPath)
		if err != nil {
			t.Fatalf("failed to stat trace log: %v", err)
		}
		perm := info.Mode().Perm()
		if perm != 0600 {
			t.Errorf("expected 0600 permissions, got %04o", perm)
		}
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected at least one trace log line")
	}

	// Parse JSON and verify allowlisted schema
	var rawMap map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &rawMap); err != nil {
		t.Fatalf("failed to unmarshal trace entry: %v", err)
	}

	allowedKeys := map[string]bool{
		"timestamp":       true,
		"hook_event_name": true,
		"tool_name":       true,
		"has_error_field": true,
		"error_json_type": true,
		"classification":  true,
		"ipc_delivery":    true,
	}

	for k := range rawMap {
		if !allowedKeys[k] {
			t.Errorf("unexpected key %q in trace entry", k)
		}
	}

	if rawMap["hook_event_name"] != "AfterTool" {
		t.Errorf("expected hook_event_name AfterTool, got %v", rawMap["hook_event_name"])
	}
	if rawMap["tool_name"] != "read_file" {
		t.Errorf("expected tool_name read_file, got %v", rawMap["tool_name"])
	}
	if rawMap["has_error_field"] != true {
		t.Errorf("expected has_error_field true, got %v", rawMap["has_error_field"])
	}
	if rawMap["error_json_type"] != "object" {
		t.Errorf("expected error_json_type 'object', got %v", rawMap["error_json_type"])
	}
	if rawMap["classification"] != "error" {
		t.Errorf("expected classification 'error', got %v", rawMap["classification"])
	}
	if rawMap["ipc_delivery"] != "failed" {
		t.Errorf("expected ipc_delivery 'failed', got %v", rawMap["ipc_delivery"])
	}
}

func TestTrace_SensitiveSentinelStringsNeverAppear(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "trace.log")
	t.Setenv("AGENT_SFX_TRACE_LOG", logPath)

	sentinels := []string{
		"SECRET_USER_PROMPT_VERY_CONFIDENTIAL_12345",
		"API_KEY_SUPER_SECRET_TOKEN_99999",
		"PASSWORD_IN_ERROR_MESSAGE_ABCDEF",
		"/Users/private/passwords.txt",
		"rm -rf /sensitive/path",
		"Sensitive tool output with private customer data",
	}

	payload := fmt.Sprintf(`{
		"session_id": "test-sess",
		"hook_event_name": "AfterTool",
		"prompt": "%s",
		"tool_name": "run_shell_command",
		"tool_input": {
			"command": "%s",
			"key": "%s"
		},
		"tool_response": {
			"llmContent": "%s",
			"returnDisplay": "%s",
			"error": {
				"message": "%s",
				"type": "auth_error"
			}
		}
	}`, sentinels[0], sentinels[4], sentinels[1], sentinels[5], sentinels[3], sentinels[2])

	rcv := &gemini.Receiver{
		SocketPath: "/nonexistent.sock",
		TestMode:   false,
	}

	var stdout bytes.Buffer
	_, err := rcv.ProcessHook(context.Background(), strings.NewReader(payload), &stdout)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read trace log: %v", err)
	}

	logContent := string(data)
	for _, sentinel := range sentinels {
		if strings.Contains(logContent, sentinel) {
			t.Errorf("SECURITY LEAK: sensitive sentinel string %q was found in trace log content: %s", sentinel, logContent)
		}
	}
}

func TestTrace_UnwritableLogDestinationPreservesNeutralOutput(t *testing.T) {
	// Point to an impossible/unwritable file location
	unwritablePath := "/dev/null/impossible/dir/trace.log"
	if runtime.GOOS == "windows" {
		unwritablePath = `Z:\impossible_nonexistent_drive\trace.log`
	}
	t.Setenv("AGENT_SFX_TRACE_LOG", unwritablePath)

	rcv := &gemini.Receiver{
		SocketPath: "/nonexistent.sock",
		TestMode:   false,
	}

	payload := `{
		"session_id": "test-sess",
		"hook_event_name": "AfterTool",
		"tool_name": "read_file",
		"tool_response": {
			"error": {
				"message": "some error",
				"type": "error"
			}
		}
	}`

	var stdout bytes.Buffer
	res, err := rcv.ProcessHook(context.Background(), strings.NewReader(payload), &stdout)
	if err != nil {
		t.Fatalf("unexpected error returned on unwritable trace log: %v", err)
	}
	if stdout.String() != "{}\n" {
		t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
	}
	if res == nil {
		t.Fatalf("expected non-nil HookResult")
	}
}
