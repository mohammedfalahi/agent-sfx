//go:build windows

package worker

import (
	"agent-sfx/internal/ipc"
)

type Lock struct{}

func AcquireSingletonLock(socketDir string) (*Lock, error) {
	return nil, ipc.ErrUnsupportedPlatform
}

func (l *Lock) Release() error {
	return nil
}
