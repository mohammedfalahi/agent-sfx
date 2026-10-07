package claude_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/adapters/claude"
	"agent-sfx/internal/ipc"
)

func TestHook_NeutralOutputAlways(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		socketPath string
	}{
		{
			name:       "empty input",
			input:      "",
			socketPath: "/nonexistent.sock",
		},
		{
			name:       "garbage input",
			input:      "not valid json at all",
			socketPath: "/nonexistent.sock",
		},
		{
			name:       "unsupported claude payload",
			input:      `{"hook_event_name": "PostToolUse", "session_id": "123"}`,
			socketPath: "/nonexistent.sock",
		},
		{
			name:       "oversized input",
			input:      strings.Repeat("X", 1024*1024+10),
			socketPath: "/nonexistent.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rcv := &claude.Receiver{
				SocketPath: tt.socketPath,
				TestMode:   false,
			}

			stdin := strings.NewReader(tt.input)
			var stdout bytes.Buffer

			res, err := rcv.ProcessHook(context.Background(), stdin, &stdout)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Output MUST be exactly "{}\n"
			if stdout.String() != "{}\n" {
				t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
			}

			if !res.Dropped {
				t.Errorf("expected payload to be dropped")
			}
		})
	}
}

func TestHook_StalledStdinNeutral(t *testing.T) {
	rcv := &claude.Receiver{
		SocketPath: "/nonexistent.sock",
		TestMode:   false,
	}

	// Create a pipe where reader blocks without data
	pr, pw := io.Pipe()
	defer pw.Close()

	var stdout bytes.Buffer
	start := time.Now()

	// ProcessHook should hit HookReadTimeout and recover neutrally
	res, err := rcv.ProcessHook(context.Background(), pr, &stdout)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != "{}\n" {
		t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	if !res.Dropped {
		t.Errorf("expected stalled stdin to be dropped")
	}

	if elapsed > 1000*time.Millisecond {
		t.Errorf("stalled stdin took too long to recover: %v", elapsed)
	}
}

func TestHook_DeadWorkerNeutral(t *testing.T) {
	rcv := &claude.Receiver{
		SocketPath: "/tmp/nonexistent-dead-claude-worker.sock",
		TestMode:   true,
	}

	synthInput := `{"kind": "task_started", "agent": "claude"}`
	var stdout bytes.Buffer

	start := time.Now()
	res, err := rcv.ProcessHook(context.Background(), strings.NewReader(synthInput), &stdout)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != "{}\n" {
		t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	// Must fail quickly due to 25ms IPC deadline
	if elapsed > 150*time.Millisecond {
		t.Errorf("hook took too long with dead worker: %v", elapsed)
	}

	if res.IPCDuration > 100*time.Millisecond {
		t.Errorf("IPC duration %v exceeded reasonable bound", res.IPCDuration)
	}
}

func TestHook_RealPayloadDispatchesToWorker(t *testing.T) {
	// Use short path to respect macOS 104-byte sockaddr_un limit
	shortDir := fmt.Sprintf("/tmp/asfx-cl-test-%d", os.Getuid())
	_ = os.MkdirAll(shortDir, 0700)
	defer func() { _ = os.RemoveAll(shortDir) }()

	sockPath := shortDir + "/w.sock"

	var receivedReq *ipc.Request
	server, err := ipc.NewServer(sockPath, func(req ipc.Request) ipc.Response {
		receivedReq = &req
		return ipc.Response{OK: true}
	})
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()
	time.Sleep(20 * time.Millisecond)

	rcv := &claude.Receiver{
		SocketPath: sockPath,
		TestMode:   false,
	}

	payload := `{
		"session_id": "test-claude-sess",
		"hook_event_name": "PreToolUse",
		"tool_name": "AskUserQuestion",
		"tool_use_id": "toolu_99XYZ",
		"tool_input": {
			"questions": [
				{
					"question": "Deploy now?",
					"options": [
						{"label": "Yes", "description": "Deploy to prod"},
						{"label": "No", "description": "Wait"}
					]
				}
			]
		}
	}`

	var stdout bytes.Buffer
	res, err := rcv.ProcessHook(context.Background(), strings.NewReader(payload), &stdout)
	if err != nil {
		t.Fatalf("ProcessHook error: %v", err)
	}

	if stdout.String() != "{}\n" {
		t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	if !res.SentEvent {
		t.Fatalf("expected SentEvent to be true, got drop reason: %s", res.DropReason)
	}

	if receivedReq == nil || receivedReq.Event == nil {
		t.Fatalf("worker did not receive event")
	}

	if receivedReq.Event.Kind != "waiting_for_user" {
		t.Errorf("expected waiting_for_user kind, got %s", receivedReq.Event.Kind)
	}
	if receivedReq.Event.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %s", receivedReq.Event.Agent)
	}
	if receivedReq.Event.DedupeKey != "claude:test-claude-sess:waiting_for_user:toolu_99XYZ" {
		t.Errorf("expected invocation dedupe key, got %s", receivedReq.Event.DedupeKey)
	}
}
