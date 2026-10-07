//go:build windows

package worker_test

import (
	"fmt"
	"os"
	"os/exec"
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

func findAgentSfxBinary(t *testing.T) string {
	t.Helper()
	// 1. Check next to test executable
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), "agent-sfx.exe")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	// 2. Check in ../../bin/agent-sfx.exe
	cand := filepath.Join("..", "..", "bin", "agent-sfx.exe")
	if _, err := os.Stat(cand); err == nil {
		if abs, err := filepath.Abs(cand); err == nil {
			return abs
		}
		return cand
	}
	// 3. Fallback to agent-sfx.exe in PATH
	if path, err := exec.LookPath("agent-sfx.exe"); err == nil {
		return path
	}
	return ""
}

func TestWorker_SingletonLock_Windows(t *testing.T) {
	tmpDir := t.TempDir()

	lock1, err := worker.AcquireSingletonLock(tmpDir)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// Second acquire must fail with ErrWorkerAlreadyRunning
	_, err = worker.AcquireSingletonLock(tmpDir)
	if err != worker.ErrWorkerAlreadyRunning {
		t.Fatalf("expected ErrWorkerAlreadyRunning on second acquire, got: %v", err)
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

func TestWorker_DaemonLifecycle_Windows(t *testing.T) {
	tmpDir := t.TempDir()
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-test-lifecycle-%d`, time.Now().UnixNano())

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

	cfg := config.DefaultConfig()
	daemon, err := worker.NewDaemon(
		cfg,
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
		daemonErrCh <- daemon.Run(pipePath)
	}()

	// Wait for pipe to become ready
	var status *ipc.Response
	deadline := time.Now().Add(1000 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
		s, err := worker.StatusDaemon(pipePath)
		if err == nil && s.OK {
			status = s
			break
		}
	}

	if status == nil {
		t.Fatalf("worker failed to start within deadline")
	}

	if !status.Enabled {
		t.Errorf("expected initial status enabled == true")
	}

	// 1. Test live disable (agent-sfx off)
	respOff, err := worker.SetWorkerEnabled(pipePath, false)
	if err != nil || !respOff.OK || respOff.Enabled {
		t.Fatalf("failed to disable worker: resp=%+v err=%v", respOff, err)
	}

	statusOff, err := worker.StatusDaemon(pipePath)
	if err != nil || statusOff.Enabled {
		t.Fatalf("expected status to show disabled: %+v", statusOff)
	}

	// 2. Test live enable (agent-sfx on)
	respOn, err := worker.SetWorkerEnabled(pipePath, true)
	if err != nil || !respOn.OK || !respOn.Enabled {
		t.Fatalf("failed to enable worker: resp=%+v err=%v", respOn, err)
	}

	statusOn, err := worker.StatusDaemon(pipePath)
	if err != nil || !statusOn.Enabled {
		t.Fatalf("expected status to show enabled: %+v", statusOn)
	}

	// 3. Test stop (agent-sfx stop)
	respStop, err := worker.StopDaemon(pipePath)
	if err != nil || !respStop.OK {
		t.Fatalf("StopDaemon failed: resp=%+v err=%v", respStop, err)
	}

	// 4. Verify worker exited
	select {
	case err := <-daemonErrCh:
		if err != nil {
			t.Fatalf("daemon exited with error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for daemon to exit after stop")
	}
}

func TestWorker_OffOnAbsentWorker_FailsCleanly_Windows(t *testing.T) {
	deadPipe := fmt.Sprintf(`\\.\pipe\agent-sfx-dead-absent-%d`, time.Now().UnixNano())

	// Setting disabled on absent worker must return ErrWorkerNotRunning without spawning
	_, err := worker.SetWorkerEnabled(deadPipe, false)
	if err != worker.ErrWorkerNotRunning {
		t.Errorf("expected ErrWorkerNotRunning for off on absent worker, got: %v", err)
	}

	// Setting enabled on absent worker must return ErrWorkerNotRunning without spawning
	_, err = worker.SetWorkerEnabled(deadPipe, true)
	if err != worker.ErrWorkerNotRunning {
		t.Errorf("expected ErrWorkerNotRunning for on on absent worker, got: %v", err)
	}
}

func TestWorker_DetachedProcessSurvival_Windows(t *testing.T) {
	binPath := findAgentSfxBinary(t)
	if binPath == "" {
		t.Skip("agent-sfx.exe not found next to test binary or in bin/; skipping detached survival test")
	}

	tmpDir := t.TempDir()
	pipePath := fmt.Sprintf(`\\.\pipe\agent-sfx-test-survival-%d`, time.Now().UnixNano())
	soundsDir := filepath.Join(tmpDir, "sounds")
	_ = os.MkdirAll(soundsDir, 0755)

	configPath := filepath.Join(tmpDir, "config.json")
	_ = os.WriteFile(configPath, []byte(`{"version":1,"enabled":true,"volume":0.6}`), 0644)

	// 1. Spawn detached worker
	err := worker.SpawnDetachedWorker(binPath, pipePath, configPath, soundsDir)
	if err != nil {
		t.Fatalf("SpawnDetachedWorker failed: %v", err)
	}

	// 2. Poll until responsive
	var initialStatus *ipc.Response
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
		s, err := worker.StatusDaemon(pipePath)
		if err == nil && s.OK {
			initialStatus = s
			break
		}
	}

	if initialStatus == nil {
		t.Fatalf("detached worker failed to become responsive on pipe %s", pipePath)
	}

	workerPID := initialStatus.PID

	// 3. Simulate launcher completion / wait brief moment
	time.Sleep(100 * time.Millisecond)

	// 4. Verify worker survives and responds with same PID
	survStatus, err := worker.StatusDaemon(pipePath)
	if err != nil || !survStatus.OK {
		t.Fatalf("worker failed status check after parent disengaged: %v", err)
	}
	if survStatus.PID != workerPID {
		t.Errorf("worker PID changed: expected %d, got %d", workerPID, survStatus.PID)
	}

	// 5. Duplicate spawn attempt must be a no-op
	err = worker.SpawnDetachedWorker(binPath, pipePath, configPath, soundsDir)
	if err != nil {
		t.Errorf("duplicate SpawnDetachedWorker on running worker failed: %v", err)
	}

	// 6. Clean up: stop the detached worker
	stopResp, err := worker.StopDaemon(pipePath)
	if err != nil || !stopResp.OK {
		t.Fatalf("StopDaemon failed on detached worker: %v", err)
	}

	// 7. Verify worker has terminated
	postStopStatus, err := worker.StatusDaemon(pipePath)
	if err == nil && postStopStatus.OK {
		t.Errorf("expected worker to be stopped, but status succeeded: %+v", postStopStatus)
	}
}
