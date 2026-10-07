//go:build !windows

package ipc_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
)

func TestIPC_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	handler := func(req ipc.Request) ipc.Response {
		switch req.Type {
		case ipc.TypeEvent:
			return ipc.Response{OK: true}
		case ipc.TypeStatus:
			return ipc.Response{OK: true, PID: 1234, Uptime: 42}
		case ipc.TypeStop:
			return ipc.Response{OK: true}
		default:
			return ipc.Response{OK: false, Error: "unknown"}
		}
	}

	server, err := ipc.NewServer(sockPath, handler)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer server.Close()

	go func() {
		_ = server.Serve()
	}()

	// Wait briefly for server ready
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// 1. Send status
	resp, err := ipc.Send(ctx, sockPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStatus,
	})
	if err != nil {
		t.Fatalf("Send(Status) error: %v", err)
	}
	if !resp.OK || resp.PID != 1234 || resp.Uptime != 42 {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	// 2. Send event
	resp2, err := ipc.Send(ctx, sockPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskFinished,
			Agent:      "gemini",
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

func TestIPC_DeadEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	deadSock := filepath.Join(tmpDir, "dead.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := ipc.Send(ctx, deadSock, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStatus,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected error on dead socket, got nil")
	}

	// Must fail quickly (under 100ms)
	if elapsed > 150*time.Millisecond {
		t.Errorf("dead endpoint check took too long: %v", elapsed)
	}
}

func TestIPC_OversizedMessage(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "oversized.sock")

	server, err := ipc.NewServer(sockPath, func(req ipc.Request) ipc.Response {
		return ipc.Response{OK: true}
	})
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
	defer server.Close()
	go func() { _ = server.Serve() }()
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Make request exceeding 8 KiB
	hugeAgent := strings.Repeat("A", 9000)
	hugeReq := ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      hugeAgent,
			ObservedAt: time.Now(),
		},
	}

	_, err = ipc.Send(ctx, sockPath, hugeReq)
	if err != ipc.ErrOversizedMessage {
		t.Fatalf("expected ErrOversizedMessage, got %v", err)
	}
}

func TestEnsureSecureDir_SymlinkRejection(t *testing.T) {
	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	if err := os.MkdirAll(realDir, 0700); err != nil {
		t.Fatalf("failed to create real dir: %v", err)
	}

	symlinkDir := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	err := ipc.EnsureSecureDir(symlinkDir)
	if err == nil {
		t.Fatalf("expected error on symlink directory, got nil")
	}
	if !strings.Contains(err.Error(), "symlinks are not permitted") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolveSocketPath(t *testing.T) {
	path, err := ipc.ResolveSocketPath()
	if err != nil {
		t.Fatalf("ResolveSocketPath error: %v", err)
	}

	if len(path) > ipc.MaxSafeUnixSocketPathLength {
		t.Errorf("socket path %s exceeds maximum length %d", path, ipc.MaxSafeUnixSocketPathLength)
	}

	dir := filepath.Dir(path)
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat on socket dir failed: %v", err)
	}
	if fi.Mode().Perm() != 0700 {
		t.Errorf("expected socket dir mode 0700, got %o", fi.Mode().Perm())
	}
}
