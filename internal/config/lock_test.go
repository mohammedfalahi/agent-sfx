package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireFileLock_ReleaseAndReacquire(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	lock1, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	lock1.release()

	// Should be able to re-acquire immediately
	lock2, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("reacquire after release failed: %v", err)
	}
	lock2.release()
}

func TestAcquireFileLock_TimeoutOnContention(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test_contention.lock")

	// Acquire first lock
	lock1, err := acquireFileLockWithTimeout(lockPath, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer lock1.release()

	// Second acquire with short timeout should time out
	start := time.Now()
	timeout := 80 * time.Millisecond
	_, err = acquireFileLockWithTimeout(lockPath, timeout)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected timeout error on second acquire, got nil")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout message in error, got: %v", err)
	}

	if elapsed < timeout {
		t.Errorf("returned before timeout expired: %v < %v", elapsed, timeout)
	}
}

func TestAcquireFileLock_ContentionResolvesOnRelease(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test_resolve.lock")

	lock1, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Release lock1 after 50ms in background
	go func() {
		time.Sleep(50 * time.Millisecond)
		lock1.release()
	}()

	// lock2 has 500ms timeout, should succeed after lock1 releases
	start := time.Now()
	lock2, err := acquireFileLockWithTimeout(lockPath, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("acquire after delayed release failed: %v", err)
	}
	defer lock2.release()

	if time.Since(start) < 40*time.Millisecond {
		t.Errorf("acquired too fast, expected to wait for lock1 release")
	}
}
