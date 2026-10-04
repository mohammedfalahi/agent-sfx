//go:build !windows

package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var (
	ErrWorkerAlreadyRunning = errors.New("worker daemon is already running")
)

type Lock struct {
	file *os.File
	path string
}

// AcquireSingletonLock attempts to atomically acquire an exclusive, non-blocking
// advisory lock on worker.lock in the designated socket directory.
func AcquireSingletonLock(socketDir string) (*Lock, error) {
	lockPath := filepath.Join(socketDir, "worker.lock")
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}

	// Try atomic exclusive non-blocking lock
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, ErrWorkerAlreadyRunning
		}
		return nil, fmt.Errorf("failed to acquire flock on %s: %w", lockPath, err)
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

// Release releases the flock and closes the lock file.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	err := l.file.Close()
	_ = os.Remove(l.path)
	l.file = nil
	return err
}
