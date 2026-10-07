//go:build windows

package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

var (
	ErrWorkerAlreadyRunning = errors.New("worker daemon is already running")
)

type Lock struct {
	file *os.File
	path string
}

// AcquireSingletonLock attempts to atomically acquire an exclusive, non-blocking
// lock on worker.lock in the designated directory via windows.LockFileEx.
func AcquireSingletonLock(socketDir string) (*Lock, error) {
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create directory for lock file %s: %w", socketDir, err)
	}

	lockPath := filepath.Join(socketDir, "worker.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}

	// Try atomic exclusive non-blocking lock on first byte using LockFileEx
	var overlapped windows.Overlapped
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	err = windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, 1, 0, &overlapped)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			return nil, ErrWorkerAlreadyRunning
		}
		return nil, fmt.Errorf("failed to acquire LockFileEx on %s: %w", lockPath, err)
	}

	// Truncate and record current PID in lock file
	_ = file.Truncate(0)
	_, _ = file.Seek(0, 0)
	_, _ = fmt.Fprintf(file, "%d\n", os.Getpid())

	return &Lock{
		file: file,
		path: lockPath,
	}, nil
}

// Release unlocks the file lock and closes the handle.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	var overlapped windows.Overlapped
	_ = windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, &overlapped)
	err := l.file.Close()
	_ = os.Remove(l.path)
	l.file = nil
	return err
}
