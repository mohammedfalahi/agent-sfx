//go:build !windows

package gemini_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"agent-sfx/internal/worker"
)

func TestSubprocess_SessionStartLaunchesDetachedWorker(t *testing.T) {
	// 1. Locate compiled binary
	binPath, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "agent-sfx"))
	if err != nil {
		t.Fatalf("failed to resolve binary path: %v", err)
	}

	if _, err := os.Stat(binPath); err != nil {
		// Compile binary if missing
		cmd := exec.Command("go", "build", "-o", binPath, "../../../cmd/agent-sfx")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to compile binary: %v, output: %s", err, string(out))
		}
	}

	// 2. Setup isolated temporary directory for test socket
	tmpDir := filepath.Join("/tmp", fmt.Sprintf("asfx-subproc-test-%d", os.Getuid()))
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		t.Fatalf("failed to create isolated test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	isolatedSocket := filepath.Join(tmpDir, "test-worker.sock")

	// 3. Prepare SessionStart payload
	sessionStartPayload := loadFixture(t, "session_start.json")

	// 4. Run the actual hook executable as an OS child process
	hookCmd := exec.Command(binPath, "hook", "gemini")
	hookCmd.Env = append(os.Environ(), fmt.Sprintf("AGENT_SFX_SOCKET_PATH=%s", isolatedSocket))
	hookCmd.Stdin = bytes.NewReader(sessionStartPayload)

	var stdout, stderr bytes.Buffer
	hookCmd.Stdout = &stdout
	hookCmd.Stderr = &stderr

	start := time.Now()
	runErr := hookCmd.Run()
	elapsed := time.Since(start)

	if runErr != nil {
		t.Fatalf("hook execution failed: %v, stderr: %s", runErr, stderr.String())
	}

	// 5. Verify hook process exited quickly and emitted exactly "{}\n"
	if elapsed > 1500*time.Millisecond {
		t.Errorf("hook process took too long to exit: %v", elapsed)
	}
	if stdout.String() != "{}\n" {
		t.Errorf("expected stdout \"{}\\n\", got %q", stdout.String())
	}

	// 6. Verify that the detached worker process became reachable AFTER hook process exited
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

	// 7. Verify worker STAYS alive independently of the hook process
	time.Sleep(150 * time.Millisecond)
	st2, err := worker.StatusDaemon(isolatedSocket)
	if err != nil || !st2.OK || st2.PID != workerPID {
		t.Fatalf("worker died or became unreachable: %v, status: %+v", err, st2)
	}

	// 8. Clean up only the test-owned worker
	stopResp, err := worker.StopDaemon(isolatedSocket)
	if err != nil || !stopResp.OK {
		t.Errorf("failed to stop test worker: %v, resp: %+v", err, stopResp)
	}

	// Confirm worker has stopped
	time.Sleep(50 * time.Millisecond)
	_, err = worker.StatusDaemon(isolatedSocket)
	if err == nil {
		t.Errorf("worker on %s should be stopped", isolatedSocket)
	}
}
