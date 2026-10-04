package scheduler_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/scheduler"
	"agent-sfx/internal/sounds"
)

func setupSchedulerTest(t *testing.T) (*scheduler.Scheduler, *scheduler.FakeClock, *audio.FakePlayer, string) {
	t.Helper()
	tmpDir := t.TempDir()

	for _, k := range events.AllEvents {
		eventDir := filepath.Join(tmpDir, string(k))
		_ = os.MkdirAll(eventDir, 0755)
		wavData := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})
		_ = os.WriteFile(filepath.Join(eventDir, "test.wav"), wavData, 0644)
	}

	cache, err := audio.NewCache(filepath.Join(tmpDir, "cache"))
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	player := audio.NewFakePlayer()
	fakeClock := scheduler.NewFakeClock(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC))
	cfg := config.DefaultConfig()
	cfg.CooldownMS = 1000 // 1000 ms cooldown

	sched := scheduler.New(
		cfg,
		fakeClock,
		player,
		cache,
		sounds.NewSelector(nil),
		tmpDir,
	)

	return sched, fakeClock, player, tmpDir
}

func TestScheduler_PriorityCoalescing(t *testing.T) {
	sched, clock, player, _ := setupSchedulerTest(t)
	sched.Start()
	defer sched.Stop()

	// Enqueue lower priority event: task_started (priority 1)
	sched.Enqueue(events.Event{
		Kind:       events.EventTaskStarted,
		Agent:      "gemini",
		SessionID:  "sess-1",
		ObservedAt: clock.Now(),
	})
	clock.WaitForTimers(1)

	// Advance clock slightly (e.g. 20ms, inside 100ms window)
	clock.Advance(20 * time.Millisecond)

	// Enqueue higher priority event: error (priority 6)
	sched.Enqueue(events.Event{
		Kind:       events.EventError,
		Agent:      "gemini",
		SessionID:  "sess-1",
		ObservedAt: clock.Now(),
	})
	time.Sleep(10 * time.Millisecond)

	// Advance clock slightly more
	clock.Advance(20 * time.Millisecond)

	// Enqueue lower priority event: task_finished (priority 2)
	sched.Enqueue(events.Event{
		Kind:       events.EventTaskFinished,
		Agent:      "gemini",
		SessionID:  "sess-1",
		ObservedAt: clock.Now(),
	})
	time.Sleep(10 * time.Millisecond)

	// Advance clock beyond 100ms coalescing window to trigger playback
	clock.Advance(100 * time.Millisecond)
	time.Sleep(50 * time.Millisecond) // Allow goroutine execution

	calls := player.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 coalesced sound played, got %d", len(calls))
	}

	// Verify error was played, not task_started or task_finished
	if !filepath.IsAbs(calls[0]) && filepath.Base(filepath.Dir(calls[0])) != "error" {
		t.Errorf("expected error sound to win priority, played: %s", calls[0])
	}
}

func TestScheduler_Deduplication(t *testing.T) {
	sched, clock, player, _ := setupSchedulerTest(t)
	sched.Start()
	defer sched.Stop()

	ev := events.Event{
		Kind:       events.EventTaskFinished,
		Agent:      "gemini",
		SessionID:  "sess-1",
		DedupeKey:  "turn-42",
		ObservedAt: clock.Now(),
	}

	// First submission
	sched.Enqueue(ev)
	clock.WaitForTimers(1)

	// Immediate duplicate submission
	sched.Enqueue(ev)
	time.Sleep(10 * time.Millisecond)

	// Trigger coalescing window
	clock.Advance(150 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	calls := player.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected duplicate to be dropped, got %d calls", len(calls))
	}
}

func TestScheduler_CooldownDropping(t *testing.T) {
	sched, clock, player, _ := setupSchedulerTest(t)
	sched.Start()
	defer sched.Stop()

	// Play first event
	sched.Enqueue(events.Event{
		Kind:       events.EventTaskStarted,
		Agent:      "gemini",
		SessionID:  "sess-1",
		ObservedAt: clock.Now(),
	})

	clock.WaitForTimers(1)
	clock.Advance(150 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	if len(player.Calls()) != 1 {
		t.Fatalf("expected first sound played, got %d", len(player.Calls()))
	}

	// In cooldown for 1000ms. Advance clock 200ms (still within cooldown)
	clock.Advance(200 * time.Millisecond)

	// Enqueue another event during cooldown
	sched.Enqueue(events.Event{
		Kind:       events.EventTaskFinished,
		Agent:      "gemini",
		SessionID:  "sess-2",
		ObservedAt: clock.Now(),
	})
	time.Sleep(10 * time.Millisecond)

	clock.Advance(200 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	// Should still be only 1 call because second event arrived during cooldown
	if len(player.Calls()) != 1 {
		t.Fatalf("event arriving during cooldown should have been dropped, total calls: %d", len(player.Calls()))
	}

	// Advance past cooldown (1000ms total cooldown)
	clock.Advance(1000 * time.Millisecond)

	// Now third event should play
	sched.Enqueue(events.Event{
		Kind:       events.EventTestsPassed,
		Agent:      "gemini",
		SessionID:  "sess-3",
		ObservedAt: clock.Now(),
	})

	clock.WaitForTimers(1)
	clock.Advance(150 * time.Millisecond)
	time.Sleep(50 * time.Millisecond)

	if len(player.Calls()) != 2 {
		t.Fatalf("expected event after cooldown to play, got %d calls", len(player.Calls()))
	}
}

func TestScheduler_PlayerTimeout(t *testing.T) {
	sched, clock, player, _ := setupSchedulerTest(t)

	startedPlay := make(chan struct{})
	var timedOut bool
	player.PlayFunc = func(ctx context.Context, wavPath string) error {
		close(startedPlay)
		select {
		case <-ctx.Done():
			timedOut = true
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}

	sched.Start()
	defer sched.Stop()

	sched.Enqueue(events.Event{
		Kind:       events.EventTaskStarted,
		Agent:      "gemini",
		SessionID:  "sess-1",
		ObservedAt: clock.Now(),
	})

	clock.WaitForTimers(1)
	clock.Advance(150 * time.Millisecond)

	// Wait until play has started
	select {
	case <-startedPlay:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("playback did not start")
	}

	// Stop triggers cancelPlay
	sched.Stop()

	if !timedOut {
		t.Errorf("expected player context to receive cancellation")
	}
}
