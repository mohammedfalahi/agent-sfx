//go:build !windows

package claude_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/adapters/claude"
)

// fakeWorkerBinary writes a script that records its arguments instead of
// starting a real worker, and returns the script and marker paths.
func fakeWorkerBinary(t *testing.T) (script, marker string) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "spawned")
	script = filepath.Join(dir, "fake-agent-sfx")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + marker + "'\n"
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return script, marker
}

func waitForFile(path string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

const sessionStart = `{"session_id":"guard","hook_event_name":"SessionStart","source":"startup"}`

// Without a resolvable socket path the hook must not start a worker: it would
// bind a different default endpoint that hooks can never reach.
func TestSessionStart_NoSocketPathDoesNotSpawn(t *testing.T) {
	script, marker := fakeWorkerBinary(t)
	rcv := &claude.Receiver{SocketPath: "", BinaryPath: script, ConfigPath: filepath.Join(t.TempDir(), "c.json")}
	var out bytes.Buffer
	if _, err := rcv.ProcessHook(context.Background(), strings.NewReader(sessionStart), &out); err != nil {
		t.Fatal(err)
	}
	if waitForFile(marker, 300*time.Millisecond) {
		t.Fatal("worker was spawned without a socket path")
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", out.String())
	}
}

// With a socket path and no running worker, the hook spawns one, and does not
// force a sounds directory onto its command line unless one was set explicitly.
func TestSessionStart_SpawnsWithoutForcedSoundsDir(t *testing.T) {
	script, marker := fakeWorkerBinary(t)
	sock := filepath.Join(t.TempDir(), "none.sock")
	rcv := &claude.Receiver{SocketPath: sock, BinaryPath: script, ConfigPath: filepath.Join(t.TempDir(), "c.json")}
	var out bytes.Buffer
	if _, err := rcv.ProcessHook(context.Background(), strings.NewReader(sessionStart), &out); err != nil {
		t.Fatal(err)
	}
	if !waitForFile(marker, 2*time.Second) {
		t.Fatal("expected a worker spawn")
	}
	time.Sleep(50 * time.Millisecond) // let the script finish writing
	args, _ := os.ReadFile(marker)
	if !strings.HasPrefix(string(args), "worker\nrun\n") {
		t.Fatalf("unexpected worker args: %q", args)
	}
	if strings.Contains(string(args), "--sounds-dir") {
		t.Fatalf("hook must not force a sounds dir; args: %q", args)
	}
	if !strings.Contains(string(args), "--config") {
		t.Fatalf("expected the config path to be forwarded; args: %q", args)
	}
}
