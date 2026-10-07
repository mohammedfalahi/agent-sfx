//go:build windows

package ipc_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
	"github.com/Microsoft/go-winio"
)

func TestResolveSocketPath_Windows(t *testing.T) {
	path, err := ipc.ResolveSocketPath()
	if err != nil {
		t.Fatalf("unexpected error resolving Windows pipe path: %v", err)
	}
	if !strings.HasPrefix(path, `\\.\pipe\agent-sfx-`) {
		t.Errorf("expected pipe path to start with \\\\.\\pipe\\agent-sfx-, got: %s", path)
	}
}

func TestResolveSocketPath_Custom_Windows(t *testing.T) {
	// Valid custom pipe path
	validPipe := `\\.\pipe\agent-sfx-test-custom-123`
	t.Setenv("AGENT_SFX_SOCKET_PATH", validPipe)
	resolved, err := ipc.ResolveSocketPath()
	if err != nil {
		t.Fatalf("unexpected error with valid custom pipe: %v", err)
	}
	if resolved != validPipe {
		t.Errorf("expected %s, got %s", validPipe, resolved)
	}

	// Invalid custom pipe path (not in \\.\pipe\)
	t.Setenv("AGENT_SFX_SOCKET_PATH", `C:\tmp\test.sock`)
	_, err = ipc.ResolveSocketPath()
	if err == nil {
		t.Fatalf("expected error on invalid custom pipe path, got nil")
	}
}

func TestSocketDir_Windows(t *testing.T) {
	dir := ipc.SocketDir("")
	if dir == "" {
		t.Errorf("expected non-empty SocketDir on Windows")
	}
	if !strings.Contains(dir, "agent-sfx") {
		t.Errorf("expected SocketDir to contain 'agent-sfx', got: %s", dir)
	}
}

func TestIPC_RoundTrip_Windows(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-test-rt-%d`, time.Now().UnixNano())

	handler := func(req ipc.Request) ipc.Response {
		switch req.Type {
		case ipc.TypeEvent:
			return ipc.Response{OK: true}
		case ipc.TypeStatus:
			return ipc.Response{OK: true, PID: 4321, Uptime: 99, Enabled: true}
		case ipc.TypeStop:
			return ipc.Response{OK: true}
		default:
			return ipc.Response{OK: false, Error: "unknown"}
		}
	}

	server, err := ipc.NewServer(pipePath, handler)
	if err != nil {
		t.Fatalf("NewServer failed on pipe %s: %v", pipePath, err)
	}
	defer server.Close()

	go func() {
		_ = server.Serve()
	}()

	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 1. Send status
	resp, err := ipc.Send(ctx, pipePath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStatus,
	})
	if err != nil {
		t.Fatalf("Send(Status) error: %v", err)
	}
	if !resp.OK || resp.PID != 4321 || resp.Uptime != 99 || !resp.Enabled {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	// 2. Send event
	resp2, err := ipc.Send(ctx, pipePath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      "claude",
			ObservedAt: time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Send(Event) error: %v", err)
	}
	if !resp2.OK {
		t.Fatalf("expected OK response on event: %+v", resp2)
	}
}

func TestIPC_OccupiedEndpoint_Windows(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-test-occ-%d`, time.Now().UnixNano())

	server1, err := ipc.NewServer(pipePath, func(req ipc.Request) ipc.Response {
		return ipc.Response{OK: true}
	})
	if err != nil {
		t.Fatalf("first NewServer failed: %v", err)
	}
	defer server1.Close()

	// Second attempt on occupied endpoint must fail immediately
	_, err = ipc.NewServer(pipePath, func(req ipc.Request) ipc.Response {
		return ipc.Response{OK: true}
	})
	if err == nil {
		t.Fatalf("expected error creating second server on occupied pipe %s, got nil", pipePath)
	}
}

func TestIPC_DeadEndpoint_Windows(t *testing.T) {
	deadPipe := fmt.Sprintf(`\\.\pipe\agent-sfx-dead-nonexistent-%d`, time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ipc.Send(ctx, deadPipe, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStatus,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error on dead pipe, got nil")
	}

	if elapsed > 150*time.Millisecond {
		t.Errorf("dead endpoint check took too long: %v", elapsed)
	}
}

func TestIPC_OversizedClientSend_Windows(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-oversized-%d`, time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	hugeAgent := strings.Repeat("W", 9000)
	hugeReq := ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      hugeAgent,
			ObservedAt: time.Now(),
		},
	}

	_, err := ipc.Send(ctx, pipePath, hugeReq)
	if err != ipc.ErrOversizedMessage {
		t.Fatalf("expected ErrOversizedMessage, got %v", err)
	}
}

func TestIPC_OversizedOnWire_Windows(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-wire-oversized-%d`, time.Now().UnixNano())

	server, err := ipc.NewServer(pipePath, func(req ipc.Request) ipc.Response {
		return ipc.Response{OK: true}
	})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		t.Fatalf("DialPipeContext error: %v", err)
	}
	defer conn.Close()

	// Send raw payload exceeding 8 KiB without newline
	hugePayload := []byte(strings.Repeat("A", 9000) + "\n")
	_, err = conn.Write(hugePayload)
	if err != nil {
		t.Fatalf("failed to write raw oversized payload: %v", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp ipc.Response
	if err := json.Unmarshal(respLine, &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.OK {
		t.Errorf("expected rejection on oversized payload, got OK=true")
	}
	if resp.Error != ipc.ErrOversizedMessage.Error() {
		t.Errorf("expected error %q, got %q", ipc.ErrOversizedMessage.Error(), resp.Error)
	}
}

func TestIPC_MalformedAndIncompleteJSON_Windows(t *testing.T) {
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-wire-malformed-%d`, time.Now().UnixNano())

	server, err := ipc.NewServer(pipePath, func(req ipc.Request) ipc.Response {
		return ipc.Response{OK: true}
	})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// 1. Malformed JSON with newline
	conn1, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		t.Fatalf("DialPipeContext error: %v", err)
	}
	defer conn1.Close()

	_, _ = conn1.Write([]byte("{ this is not json }\n"))
	reader1 := bufio.NewReader(conn1)
	respLine1, err := reader1.ReadBytes('\n')
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp1 ipc.Response
	if err := json.Unmarshal(respLine1, &resp1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp1.OK || !strings.Contains(resp1.Error, "malformed json") {
		t.Errorf("expected malformed json error, got: %+v", resp1)
	}

	// 2. Invalid protocol version
	conn2, err := winio.DialPipeContext(ctx, pipePath)
	if err != nil {
		t.Fatalf("DialPipeContext error: %v", err)
	}
	defer conn2.Close()

	_, _ = conn2.Write([]byte(`{"version":99,"type":"status"}` + "\n"))
	reader2 := bufio.NewReader(conn2)
	respLine2, err := reader2.ReadBytes('\n')
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp2 ipc.Response
	if err := json.Unmarshal(respLine2, &resp2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp2.OK || !strings.Contains(resp2.Error, "protocol") {
		t.Errorf("expected protocol version error, got: %+v", resp2)
	}
}
