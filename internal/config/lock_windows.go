//go:build windows

package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

const (
	DefaultLockTimeout       = 2 * time.Second
	DefaultLockRetryInterval = 20 * time.Millisecond
)

type fileLock struct {
	file *os.File
}

// acquireFileLock attempts to acquire an exclusive lock on lockPath within DefaultLockTimeout.
func acquireFileLock(lockPath string) (*fileLock, error) {
	return acquireFileLockWithTimeout(lockPath, DefaultLockTimeout)
}

// acquireFileLockWithTimeout attempts to acquire an exclusive lock on lockPath using
// windows.LockFileEx with finite deadline. Handles are closed on every error path
// to ensure no resource leaks.
func acquireFileLockWithTimeout(lockPath string, timeout time.Duration) (*fileLock, error) {
	if timeout <= 0 {
		timeout = DefaultLockTimeout
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}

	deadline := time.Now().Add(timeout)
	for {
		var overlapped windows.Overlapped
		flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
		err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &overlapped)
		if err == nil {
			return &fileLock{file: f}, nil
		}

		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			if time.Now().After(deadline) {
				_ = f.Close()
				return nil, fmt.Errorf("timed out after %v waiting to acquire lock on %s: %w", timeout, lockPath, err)
			}
			time.Sleep(DefaultLockRetryInterval)
			continue
		}

		_ = f.Close()
		return nil, fmt.Errorf("failed to acquire LockFileEx on %s: %w", lockPath, err)
	}
}

func (l *fileLock) release() {
	if l != nil && l.file != nil {
		var overlapped windows.Overlapped
		_ = windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &overlapped)
		_ = l.file.Close()
		l.file = nil
	}
}
