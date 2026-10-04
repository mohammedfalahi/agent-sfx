//go:build !windows

package gemini_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-sfx/internal/adapters/gemini"
	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/scheduler"
	"agent-sfx/internal/sounds"
	"agent-sfx/internal/worker"
)

func TestIntegration_HookToWorkerWithFakePlayer(t *testing.T) {
	tmpDir := fmt.Sprintf("/tmp/asfx-integ-%d", os.Getuid())
	_ = os.RemoveAll(tmpDir)
	_ = os.MkdirAll(tmpDir, 0700)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	sockPath := filepath.Join(tmpDir, "w.sock")

	// Setup sound assets in tmpDir
	soundsDir := filepath.Join(tmpDir, "sounds")
	for _, k := range events.AllEvents {
		eventDir := filepath.Join(soundsDir, string(k))
		_ = os.MkdirAll(eventDir, 0755)
		wavData := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})
		_ = os.WriteFile(filepath.Join(eventDir, "clip.wav"), wavData, 0644)
	}

	cache, err := audio.NewCache(filepath.Join(tmpDir, "cache"))
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	fakePlayer := audio.NewFakePlayer()
	// Zero cooldown for fast integration testing
	cfg := config.DefaultConfig()
	cfg.CooldownMS = 0

	daemon, err := worker.NewDaemon(
		cfg,
		soundsDir,
		scheduler.RealClock{},
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

	// Wait for worker ready
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		if st, err := worker.StatusDaemon(sockPath); err == nil && st.OK {
			break
		}
	}

	rcv := &gemini.Receiver{
		SocketPath: sockPath,
		TestMode:   false, // Production mode with real fixtures
	}

	fixtures := []struct {
		file       string
		shouldPlay bool
	}{
		{"before_agent.json", true},
		{"after_agent.json", true},
		{"notification_tool_permission.json", true},
		{"after_tool_error.json", true},
		{"after_tool_success.json", false}, // Successful tool must NOT play sound
	}

	for _, tc := range fixtures {
		t.Run(tc.file, func(t *testing.T) {
			data := loadFixture(t, tc.file)
			var stdout bytes.Buffer

			callsBefore := len(fakePlayer.Calls())
			res, err := rcv.ProcessHook(context.Background(), bytes.NewReader(data), &stdout)
			if err != nil {
				t.Fatalf("ProcessHook error: %v", err)
			}

			// Invariant: stdout must always be exactly "{}\n"
			if stdout.String() != "{}\n" {
				t.Fatalf("expected stdout \"{}\\n\", got %q", stdout.String())
			}

			if tc.shouldPlay {
				if !res.SentEvent {
					t.Fatalf("expected SentEvent to be true, got drop reason: %s", res.DropReason)
				}
				// Allow worker scheduling (100ms coalesce window + playback)
				time.Sleep(150 * time.Millisecond)
				callsAfter := len(fakePlayer.Calls())
				if callsAfter <= callsBefore {
					t.Fatalf("expected sound played for %s", tc.file)
				}
			} else {
				time.Sleep(50 * time.Millisecond)
				callsAfter := len(fakePlayer.Calls())
				if callsAfter != callsBefore {
					t.Fatalf("expected no sound for %s, but played %d sounds", tc.file, callsAfter-callsBefore)
				}
			}
		})
	}

	// Stop worker
	_, _ = worker.StopDaemon(sockPath)
}
