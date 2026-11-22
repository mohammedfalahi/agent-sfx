package scheduler_test

import (
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

// newFallbackScheduler builds a scheduler whose primary dir has sounds only for
// task_finished, and whose fallback dir has sounds for every event.
func newFallbackScheduler(t *testing.T, withFallback bool) (*scheduler.Scheduler, *scheduler.FakeClock, *audio.FakePlayer) {
	t.Helper()
	primary, fallback := t.TempDir(), t.TempDir()
	wav := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})

	if err := os.MkdirAll(filepath.Join(primary, string(events.EventTaskFinished)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(primary, string(events.EventTaskFinished), "custom.wav"), wav, 0644); err != nil {
		t.Fatal(err)
	}
	for _, k := range events.AllEvents {
		dir := filepath.Join(fallback, string(k))
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bundled.wav"), wav, 0644); err != nil {
			t.Fatal(err)
		}
	}

	cache, err := audio.NewCache(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	player := audio.NewFakePlayer()
	clock := scheduler.NewFakeClock(time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC))
	sched := scheduler.New(config.DefaultConfig(), clock, player, cache, sounds.NewSelector(nil), primary)
	if withFallback {
		sched.SetFallbackSoundsDir(fallback)
	}
	return sched, clock, player
}

func playOne(t *testing.T, sched *scheduler.Scheduler, clock *scheduler.FakeClock, kind events.EventKind) {
	t.Helper()
	sched.Enqueue(events.Event{Kind: kind, Agent: "claude", SessionID: "s1", ObservedAt: clock.Now()})
	clock.WaitForTimers(1)
	clock.Advance(scheduler.CoalesceWindow + 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
}

func TestScheduler_FallbackPlaysForUncustomizedEvent(t *testing.T) {
	sched, clock, player := newFallbackScheduler(t, true)
	sched.Start()
	defer sched.Stop()

	playOne(t, sched, clock, events.EventError)

	if n := len(player.Calls()); n != 1 {
		t.Fatalf("expected the bundled fallback error sound to play once, got %d plays", n)
	}
}

func TestScheduler_NoFallbackMeansSilence(t *testing.T) {
	sched, clock, player := newFallbackScheduler(t, false)
	sched.Start()
	defer sched.Stop()

	playOne(t, sched, clock, events.EventError)

	if n := len(player.Calls()); n != 0 {
		t.Fatalf("expected silence without a fallback dir, got %d plays", n)
	}
}

func TestScheduler_SoundsDirsReported(t *testing.T) {
	sched, _, _ := newFallbackScheduler(t, true)
	primary, fallback := sched.SoundsDirs()
	if primary == "" || fallback == "" || primary == fallback {
		t.Fatalf("expected distinct primary and fallback dirs, got %q and %q", primary, fallback)
	}
}
