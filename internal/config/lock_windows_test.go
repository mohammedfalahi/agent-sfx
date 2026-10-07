//go:build windows

package config

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Helper for separate-process lock contention testing
func init() {
	if os.Getenv("AGENT_SFX_TEST_LOCK_CHILD") == "1" {
		lockPath := os.Getenv("AGENT_SFX_TEST_LOCK_PATH")
		if lockPath == "" {
			os.Exit(2)
		}
		lock, err := acquireFileLockWithTimeout(lockPath, 2*time.Second)
		if err != nil {
			os.Exit(3)
		}
		// Signal to parent that lock is held
		os.Stdout.WriteString("LOCKED\n")
		os.Stdout.Sync()

		// Hold lock for 300ms
		time.Sleep(300 * time.Millisecond)
		lock.release()
		os.Exit(0)
	}
}

func TestWindowsConfigLock_LockFileExContentionAndTimeout(t *testing.T) {
	t.Log("Note: Windows runtime execution is pending execution on native Windows host")

	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "win_config_lock.lock")

	// 1. Initial lock acquisition
	lock1, err := acquireFileLockWithTimeout(lockPath, 1*time.Second)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// 2. Second acquisition must timeout while lock1 is held
	start := time.Now()
	timeout := 100 * time.Millisecond
	_, err = acquireFileLockWithTimeout(lockPath, timeout)
	if err == nil {
		lock1.release()
		t.Fatalf("expected second acquire to time out, but succeeded")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
	if time.Since(start) < timeout {
		t.Errorf("timed out prematurely: %v < %v", time.Since(start), timeout)
	}

	// 3. Release lock1
	lock1.release()

	// 4. Third acquisition must now succeed immediately
	lock2, err := acquireFileLockWithTimeout(lockPath, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
	lock2.release()
}

func TestWindowsConfigLock_SeparateProcessContention(t *testing.T) {
	t.Log("Note: Windows runtime execution is pending execution on native Windows host")

	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "win_process_contention.lock")

	// Launch child process to hold lock
	cmd := exec.Command(os.Args[0], "-test.run=TestWindowsConfigLock_SeparateProcessContention")
	cmd.Env = append(os.Environ(),
		"AGENT_SFX_TEST_LOCK_CHILD=1",
		"AGENT_SFX_TEST_LOCK_PATH="+lockPath,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start child lock process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()

	// Wait for child to confirm it acquired the lock
	reader := bufio.NewReader(stdoutPipe)
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "LOCKED" {
		t.Fatalf("child process failed to signal LOCKED: %v (line: %q)", err, line)
	}

	// Parent attempts to acquire lock with short timeout (should fail due to cross-process contention)
	start := time.Now()
	timeout := 80 * time.Millisecond
	_, err = acquireFileLockWithTimeout(lockPath, timeout)
	if err == nil {
		t.Fatalf("expected cross-process acquire to time out while child holds lock, but succeeded")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message in error, got: %v", err)
	}
	if time.Since(start) < timeout {
		t.Errorf("timed out prematurely: %v < %v", time.Since(start), timeout)
	}

	// Wait for child to release lock and exit cleanly
	if err := cmd.Wait(); err != nil {
		t.Fatalf("child process exited with error: %v", err)
	}

	// Now parent must acquire lock successfully
	parentLock, err := acquireFileLockWithTimeout(lockPath, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("parent failed to acquire lock after child exit: %v", err)
	}
	parentLock.release()
}
