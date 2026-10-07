//go:build !windows

package worker_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/scheduler"
	"agent-sfx/internal/sounds"
	"agent-sfx/internal/worker"
)

func TestWorker_SingletonLock(t *testing.T) {
	tmpDir := t.TempDir()

	lock1, err := worker.AcquireSingletonLock(tmpDir)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire must fail with ErrWorkerAlreadyRunning
	_, err = worker.AcquireSingletonLock(tmpDir)
	if err != worker.ErrWorkerAlreadyRunning {
		t.Fatalf("expected ErrWorkerAlreadyRunning, got %v", err)
	}

	// Release first lock
	if err := lock1.Release(); err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// Third acquire should now succeed
	lock2, err := worker.AcquireSingletonLock(tmpDir)
	if err != nil {
		t.Fatalf("acquire after release failed: %v", err)
	}
	_ = lock2.Release()
}

func TestWorker_DaemonLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "worker.sock")

	// Create test sounds directory
	for _, k := range events.AllEvents {
		eventDir := filepath.Join(tmpDir, "sounds", string(k))
		_ = os.MkdirAll(eventDir, 0755)
		wavData := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})
		_ = os.WriteFile(filepath.Join(eventDir, "test.wav"), wavData, 0644)
	}

	cache, err := audio.NewCache(filepath.Join(tmpDir, "cache"))
	if err != nil {
		t.Fatalf("NewCache error: %v", err)
	}

	fakePlayer := audio.NewFakePlayer()
	fakeClock := scheduler.NewFakeClock(time.Now())

	daemon, err := worker.NewDaemon(
		config.DefaultConfig(),
		filepath.Join(tmpDir, "sounds"),
		fakeClock,
		fakePlayer,
		cache,
		sounds.NewSelector(nil),
	)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	daemonErrCh := make(chan error, 1)
	go func() {
		daemonErrCh <- daemon.Run(sockPath)
	}()

	// Wait for socket to become ready
	var status *ipc.Response
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		s, err := worker.StatusDaemon(sockPath)
		if err == nil && s.OK {
			status = s
			break
		}
	}

	if status == nil {
		t.Fatalf("worker failed to start within deadline")
	}

	if status.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), status.PID)
	}

	// Verify second daemon cannot run on same socket dir
	daemon2, err := worker.NewDaemon(
		config.DefaultConfig(),
		filepath.Join(tmpDir, "sounds"),
		fakeClock,
		fakePlayer,
		cache,
		sounds.NewSelector(nil),
	)
	if err == nil {
		err2 := daemon2.Run(sockPath)
		if err2 == nil {
			t.Fatalf("expected second daemon run to fail due to lock, got nil")
		}
	}

	// Send an event over IPC
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := ipc.Send(ctx, sockPath, ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      "gemini",
			ObservedAt: time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("Send(Event) error: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected event OK response")
	}

	// Send stop request via StopDaemon
	stopResp, err := worker.StopDaemon(sockPath)
	if err != nil {
		t.Fatalf("StopDaemon error: %v", err)
	}
	if !stopResp.OK {
		t.Fatalf("expected stop OK response")
	}

	// Check daemon returned cleanly
	select {
	case err := <-daemonErrCh:
		if err != nil {
			t.Fatalf("daemon exited with error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("daemon did not exit within timeout")
	}

	// Verify socket and lock files were cleaned up
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Errorf("socket file %s was not cleaned up", sockPath)
	}
}

func TestWorker_EnableDisableControls(t *testing.T) {
	tmpDir := t.TempDir()
	// Darwin has a 104-char limit on Unix domain socket paths
	sockDir, err := os.MkdirTemp("", "sfx-*")
	if err != nil {
		t.Fatalf("failed to create temp sock dir: %v", err)
	}
	defer os.RemoveAll(sockDir)
	sockPath := filepath.Join(sockDir, "w.sock")

	for _, k := range events.AllEvents {
		eventDir := filepath.Join(tmpDir, "sounds", string(k))
		_ = os.MkdirAll(eventDir, 0755)
		wavData := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})
		_ = os.WriteFile(filepath.Join(eventDir, "test.wav"), wavData, 0644)
	}

	cache, err := audio.NewCache(filepath.Join(tmpDir, "cache"))
	if err != nil {
		t.Fatalf("NewCache error: %v", err)
	}

	fakePlayer := audio.NewFakePlayer()
	fakeClock := scheduler.NewFakeClock(time.Now())

	daemon, err := worker.NewDaemon(
		config.DefaultConfig(),
		filepath.Join(tmpDir, "sounds"),
		fakeClock,
		fakePlayer,
		cache,
		sounds.NewSelector(nil),
	)
	if err != nil {
		t.Fatalf("NewDaemon error: %v", err)
	}

	daemonErrCh := make(chan error, 1)
	go func() {
		daemonErrCh <- daemon.Run(sockPath)
	}()

	// Wait for worker readiness
	var status *ipc.Response
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case err := <-daemonErrCh:
			t.Fatalf("daemon exited prematurely: %v", err)
		default:
		}
		time.Sleep(20 * time.Millisecond)
		s, err := worker.StatusDaemon(sockPath)
		if err == nil && s.OK {
			status = s
			break
		}
	}
	if status == nil {
		t.Fatalf("worker failed to start within deadline")
	}

	// 1. Initial status has Enabled = true
	if !status.Enabled {
		t.Errorf("expected initial worker status to report Enabled: true")
	}

	// 2. Disable worker
	disResp, err := worker.SetWorkerEnabled(sockPath, false)
	if err != nil {
		t.Fatalf("SetWorkerEnabled(false) failed: %v", err)
	}
	if !disResp.OK {
		t.Errorf("expected OK response for disable")
	}
	if disResp.Enabled {
		t.Errorf("expected disable response to report Enabled: false")
	}

	// Verify status query reflects disabled state
	statusDis, err := worker.StatusDaemon(sockPath)
	if err != nil {
		t.Fatalf("StatusDaemon error: %v", err)
	}
	if statusDis.Enabled {
		t.Errorf("expected status to report Enabled: false after disable")
	}

	// 3. Re-enable worker
	enResp, err := worker.SetWorkerEnabled(sockPath, true)
	if err != nil {
		t.Fatalf("SetWorkerEnabled(true) failed: %v", err)
	}
	if !enResp.OK {
		t.Errorf("expected OK response for enable")
	}
	if !enResp.Enabled {
		t.Errorf("expected enable response to report Enabled: true")
	}

	// Verify status query reflects enabled state
	statusEn, err := worker.StatusDaemon(sockPath)
	if err != nil {
		t.Fatalf("StatusDaemon error: %v", err)
	}
	if !statusEn.Enabled {
		t.Errorf("expected status to report Enabled: true after enable")
	}

	// Stop daemon
	_, _ = worker.StopDaemon(sockPath)
	<-daemonErrCh
}

func TestWorker_StoppedWorkerControls(t *testing.T) {
	nonExistentSocket := filepath.Join(t.TempDir(), "nonexistent.sock")

	// SetWorkerEnabled on absent worker returns ErrWorkerNotRunning without spawning worker
	_, err := worker.SetWorkerEnabled(nonExistentSocket, false)
	if err != worker.ErrWorkerNotRunning {
		t.Errorf("expected ErrWorkerNotRunning, got %v", err)
	}

	_, err = worker.SetWorkerEnabled(nonExistentSocket, true)
	if err != worker.ErrWorkerNotRunning {
		t.Errorf("expected ErrWorkerNotRunning, got %v", err)
	}
}
