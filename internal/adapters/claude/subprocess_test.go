//go:build !windows

package claude_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/worker"
)

func getCompiledBinary(t *testing.T) string {
	binPath, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "agent-sfx"))
	if err != nil {
		t.Fatalf("failed to resolve binary path: %v", err)
	}

	if _, err := os.Stat(binPath); err != nil {
		cmd := exec.Command("go", "build", "-o", binPath, "../../../cmd/agent-sfx")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to compile binary: %v, output: %s", err, string(out))
		}
	}
	return binPath
}

func TestSubprocess_SessionStartLaunchesDetachedWorker(t *testing.T) {
	binPath := getCompiledBinary(t)

	// Setup isolated temporary directory for test socket
	tmpDir := filepath.Join("/tmp", fmt.Sprintf("asfx-cl-subproc-%d", os.Getuid()))
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("failed to create isolated test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	isolatedSocket := filepath.Join(tmpDir, "cw.sock")

	// Prepare SessionStart payload
	sessionStartPayload := `{"session_id":"sess-subproc-claude","hook_event_name":"SessionStart"}`

	// Run the hook claude command as an OS child process
	hookCmd := exec.Command(binPath, "hook", "claude")
	hookCmd.Env = append(os.Environ(), fmt.Sprintf("AGENT_SFX_SOCKET_PATH=%s", isolatedSocket))
	hookCmd.Stdin = strings.NewReader(sessionStartPayload)

	var stdout, stderr bytes.Buffer
	hookCmd.Stdout = &stdout
	hookCmd.Stderr = &stderr

	start := time.Now()
	runErr := hookCmd.Run()
	elapsed := time.Since(start)

	if runErr != nil {
		t.Fatalf("hook execution failed: %v, stderr: %s", runErr, stderr.String())
	}

	// 1. Verify hook process exited quickly and emitted exactly "{}\n"
	if elapsed > 1500*time.Millisecond {
		t.Errorf("hook process took too long to exit: %v", elapsed)
	}
	if stdout.String() != "{}\n" {
		t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	// 2. Verify that detached worker became reachable after hook exit
	var workerPID int
	var reachable bool
	deadline := time.Now().Add(1000 * time.Millisecond)

	for time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
		st, err := worker.StatusDaemon(isolatedSocket)
		if err == nil && st.OK {
			reachable = true
			workerPID = st.PID
			break
		}
	}

	if !reachable || workerPID <= 0 {
		t.Fatalf("worker did not become reachable on isolated socket %s after hook process exit", isolatedSocket)
	}

	// 3. Verify worker stays alive independently after hook exit
	time.Sleep(150 * time.Millisecond)
	st2, err := worker.StatusDaemon(isolatedSocket)
	if err != nil || !st2.OK || st2.PID != workerPID {
		t.Fatalf("worker died or became unreachable: %v, status: %+v", err, st2)
	}

	// 4. Verify that running SessionStart a second time does not launch a second worker
	hookCmd2 := exec.Command(binPath, "hook", "claude")
	hookCmd2.Env = append(os.Environ(), fmt.Sprintf("AGENT_SFX_SOCKET_PATH=%s", isolatedSocket))
	hookCmd2.Stdin = strings.NewReader(sessionStartPayload)
	var stdout2 bytes.Buffer
	hookCmd2.Stdout = &stdout2
	if err := hookCmd2.Run(); err != nil {
		t.Fatalf("second hook run failed: %v", err)
	}
	if stdout2.String() != "{}\n" {
		t.Errorf("expected second hook stdout \"{}\\n\", got %q", stdout2.String())
	}

	st3, err := worker.StatusDaemon(isolatedSocket)
	if err != nil || st3.PID != workerPID {
		t.Errorf("expected worker PID to remain unchanged (%d), got: %+v", workerPID, st3)
	}

	// 5. Clean up isolated worker
	stopResp, err := worker.StopDaemon(isolatedSocket)
	if err != nil || !stopResp.OK {
		t.Errorf("failed to stop test worker: %v, resp: %+v", err, stopResp)
	}
}

func TestSubprocess_NeutralOutputAndExitZero(t *testing.T) {
	binPath := getCompiledBinary(t)

	cases := []struct {
		name  string
		stdin string
	}{
		{"empty_stdin", ""},
		{"malformed_json", "{malformed-json"},
		{"unsupported_event", `{"hook_event_name": "CwdChanged"}`},
		{"oversized_stdin", strings.Repeat("Y", 1024*1024+50)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binPath, "hook", "claude")
			cmd.Env = append(os.Environ(), "AGENT_SFX_SOCKET_PATH=/tmp/nonexistent-dead.sock")
			cmd.Stdin = strings.NewReader(tc.stdin)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				t.Fatalf("expected exit code 0, got error: %v, stderr: %s", err, stderr.String())
			}

			if stdout.String() != "{}\n" {
				t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
			}
		})
	}
}

func TestSubprocess_StalledStdinTimeout(t *testing.T) {
	binPath := getCompiledBinary(t)

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	defer pr.Close()
	defer pw.Close()

	cmd := exec.Command(binPath, "hook", "claude")
	cmd.Env = append(os.Environ(), "AGENT_SFX_SOCKET_PATH=/tmp/nonexistent-dead.sock")
	cmd.Stdin = pr

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected exit code 0 on stalled stdin, got error: %v, stderr: %s", err, stderr.String())
	}

	if stdout.String() != "{}\n" {
		t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	if elapsed > 1500*time.Millisecond {
		t.Errorf("stalled stdin took too long: %v", elapsed)
	}
}
