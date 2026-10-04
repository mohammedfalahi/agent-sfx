//go:build windows

package worker

import (
	"agent-sfx/internal/ipc"
)

func StatusDaemon(socketPath string) (*ipc.Response, error) {
	return nil, ipc.ErrUnsupportedPlatform
}

func StopDaemon(socketPath string) (*ipc.Response, error) {
	return nil, ipc.ErrUnsupportedPlatform
}

func SpawnDetachedWorker(binaryPath, socketPath, configPath, soundsDir string) error {
	return ipc.ErrUnsupportedPlatform
}

func StartDaemon(binaryPath, socketPath, configPath, soundsDir string) (*ipc.Response, bool, error) {
	return nil, false, ipc.ErrUnsupportedPlatform
}
