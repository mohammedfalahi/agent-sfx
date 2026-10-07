//go:build !windows

package config

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

const (
	DefaultLockTimeout       = 2 * time.Second
	DefaultLockRetryInterval = 20 * time.Millisecond
)

type fileLock struct {
	file *os.File
}

// acquireFileLock attempts to acquire an exclusive flock on lockPath within DefaultLockTimeout.
func acquireFileLock(lockPath string) (*fileLock, error) {
	return acquireFileLockWithTimeout(lockPath, DefaultLockTimeout)
}

// acquireFileLockWithTimeout attempts to acquire an exclusive non-blocking flock on lockPath.
// It retries until timeout expires, ensuring bounded execution without hangs.
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
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return &fileLock{file: f}, nil
		}

		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			if time.Now().After(deadline) {
				_ = f.Close()
				return nil, fmt.Errorf("timed out after %v waiting to acquire flock on %s: %w", timeout, lockPath, err)
			}
			time.Sleep(DefaultLockRetryInterval)
			continue
		}

		_ = f.Close()
		return nil, fmt.Errorf("failed to acquire flock on %s: %w", lockPath, err)
	}
}

func (l *fileLock) release() {
	if l != nil && l.file != nil {
		_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		_ = l.file.Close()
		l.file = nil
	}
}
