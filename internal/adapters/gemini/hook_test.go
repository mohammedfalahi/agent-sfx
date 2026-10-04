package gemini_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/adapters/gemini"
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
			name:       "unsupported gemini payload",
			input:      `{"event": "BeforeAgent", "session_id": "123"}`,
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
			rcv := &gemini.Receiver{
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
				t.Errorf("expected payload to be dropped in M1")
			}
		})
	}
}

func TestHook_DeadWorkerNeutral(t *testing.T) {
	rcv := &gemini.Receiver{
		SocketPath: "/tmp/nonexistent-dead-worker.sock",
		TestMode:   true,
	}

	synthInput := `{"kind": "task_started", "agent": "gemini"}`
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

func TestHook_SyntheticEventWithActiveWorker(t *testing.T) {
	// Use short path to respect macOS 104-byte sockaddr_un limit
	shortDir := fmt.Sprintf("/tmp/asfx-test-%d", os.Getuid())
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

	rcv := &gemini.Receiver{
		SocketPath: sockPath,
		TestMode:   true, // Synthetic injection enabled
	}

	synth := `{"kind": "tests_passed", "agent": "gemini", "session_id": "test-sess"}`
	var stdout bytes.Buffer

	res, err := rcv.ProcessHook(context.Background(), strings.NewReader(synth), &stdout)
	if err != nil {
		t.Fatalf("ProcessHook error: %v", err)
	}

	if stdout.String() != "{}\n" {
		t.Errorf("expected \"{}\\n\", got %q", stdout.String())
	}

	if !res.SentEvent {
		t.Fatalf("expected SentEvent to be true, got reason: %s", res.DropReason)
	}

	if receivedReq == nil || receivedReq.Event == nil || receivedReq.Event.Kind != "tests_passed" {
		t.Errorf("worker did not receive expected event: %+v", receivedReq)
	}
}
