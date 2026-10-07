//go:build !windows

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"agent-sfx/internal/ipc"
)

var (
	ErrWorkerNotRunning = errors.New("worker daemon is not running")
)

// StatusDaemon queries the worker status via IPC.
func StatusDaemon(socketPath string) (*ipc.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := ipc.Send(ctx, socketPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStatus,
	})
	if err != nil {
		return nil, ErrWorkerNotRunning
	}
	return resp, nil
}

// SetWorkerEnabled instructs an existing worker to enable or disable audio via IPC.
// It does not spawn a new worker if absent.
func SetWorkerEnabled(socketPath string, enabled bool) (*ipc.Response, error) {
	msgType := ipc.TypeEnable
	if !enabled {
		msgType = ipc.TypeDisable
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	resp, err := ipc.Send(ctx, socketPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    msgType,
	})
	if err != nil {
		return nil, ErrWorkerNotRunning
	}
	return resp, nil
}

// StopDaemon instructs the worker to shut down via its owned IPC endpoint.
func StopDaemon(socketPath string) (*ipc.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	resp, err := ipc.Send(ctx, socketPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeStop,
	})
	if err != nil {
		return nil, ErrWorkerNotRunning
	}

	// Wait up to 1.5 seconds for worker to shut down and release socket
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
		_, err := StatusDaemon(socketPath)
		if err != nil {
			// Successfully stopped
			return resp, nil
		}
	}

	return resp, nil
}

// SpawnDetachedWorker launches the background worker process detached from the current
// process group with setsid and closed stdio. It returns immediately after child creation
// without polling or waiting for IPC readiness.
func SpawnDetachedWorker(binaryPath, socketPath, configPath, soundsDir string) error {
	// If already running, do nothing
	if existing, err := StatusDaemon(socketPath); err == nil && existing.OK {
		return nil
	}

	if binaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("unable to determine executable path: %w", err)
		}
		binaryPath = exe
	}

	args := []string{"worker", "run"}
	if configPath != "" {
		args = append(args, "--config", configPath)
	}
	if soundsDir != "" {
		args = append(args, "--sounds-dir", soundsDir)
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Env = append(os.Environ(), fmt.Sprintf("AGENT_SFX_SOCKET_PATH=%s", socketPath))
	// Close and redirect stdio to prevent pipe inheritance
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start detached worker process: %w", err)
	}

	// Release process handle so parent does not wait or retain child resources
	_ = cmd.Process.Release()
	return nil
}

// StartDaemon starts a detached worker and polls until the IPC socket is responsive.
func StartDaemon(binaryPath, socketPath, configPath, soundsDir string) (*ipc.Response, bool, error) {
	// 1. Check if already running
	existing, err := StatusDaemon(socketPath)
	if err == nil && existing.OK {
		return existing, false, nil // already running
	}

	// 2. Spawn detached child
	if err := SpawnDetachedWorker(binaryPath, socketPath, configPath, soundsDir); err != nil {
		return nil, false, err
	}

	// 3. Poll IPC socket until responsive
	deadline := time.Now().Add(1000 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		resp, err := StatusDaemon(socketPath)
		if err == nil && resp.OK {
			return resp, true, nil // newly started
		}
	}

	return nil, false, errors.New("timed out waiting for worker daemon to become responsive on IPC socket")
}
