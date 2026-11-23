package scheduler_test

import (
	"context"
	"errors"
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

// newUserSoundScheduler puts one user file for task_finished in the primary
// dir and a valid bundled WAV for every event in the fallback dir.
func newUserSoundScheduler(t *testing.T, userFile string, userData []byte, conv audio.Converter, withFallback bool) (*scheduler.Scheduler, *scheduler.FakeClock, *audio.FakePlayer) {
	t.Helper()
	primary, fallback := t.TempDir(), t.TempDir()
	d := filepath.Join(primary, string(events.EventTaskFinished))
	if err := os.MkdirAll(d, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, userFile), userData, 0644); err != nil {
		t.Fatal(err)
	}
	wav := audio.EncodePCM16WAV(1, 44100, []int16{100, 200, -200})
	for _, k := range events.AllEvents {
		dir := filepath.Join(fallback, string(k))
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filepath.Join(dir, "bundled.wav"), wav, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cache, err := audio.NewCache(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	if conv != nil {
		cache.SetConverter(conv)
	}
	player := audio.NewFakePlayer()
	clock := scheduler.NewFakeClock(time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC))
	sched := scheduler.New(config.DefaultConfig(), clock, player, cache, sounds.NewSelector(nil), primary)
	if withFallback {
		sched.SetFallbackSoundsDir(fallback)
	}
	return sched, clock, player
}

func TestScheduler_PlaysConvertedMP3(t *testing.T) {
	if !audio.ConversionAvailable() {
		t.Skip("MP3 sounds are only used where they can be converted")
	}
	calls := 0
	conv := func(ctx context.Context, src, dst string) error {
		calls++
		return os.WriteFile(dst, audio.EncodePCM16WAV(1, 44100, []int16{1, 2, 3}), 0600)
	}
	sched, clock, player := newUserSoundScheduler(t, "mine.mp3", []byte("mp3"), conv, true)
	sched.Start()
	defer sched.Stop()

	playOne(t, sched, clock, events.EventTaskFinished)
	if n := len(player.Calls()); n != 1 || calls != 1 {
		t.Fatalf("plays = %d, conversions = %d; want the converted MP3 played once", n, calls)
	}
}

func TestScheduler_UnplayableUserSoundFallsBackToBundled(t *testing.T) {
	cases := map[string]struct {
		file string
		data []byte
		conv audio.Converter
	}{
		"corrupt wav": {"broken.wav", []byte("RIFF not a wav"), nil},
		"mp3 that fails to convert": {"broken.mp3", []byte("x"), func(ctx context.Context, src, dst string) error {
			return errors.New("bad mp3")
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if filepath.Ext(tc.file) != ".wav" && !audio.ConversionAvailable() {
				t.Skip("MP3 sounds are only used where they can be converted")
			}
			sched, clock, player := newUserSoundScheduler(t, tc.file, tc.data, tc.conv, true)
			sched.Start()
			defer sched.Stop()

			playOne(t, sched, clock, events.EventTaskFinished)
			if n := len(player.Calls()); n != 1 {
				t.Fatalf("plays = %d; want the bundled sound to play instead of silence", n)
			}
		})
	}
}

func TestScheduler_UnplayableUserSoundWithoutFallbackIsSilent(t *testing.T) {
	sched, clock, player := newUserSoundScheduler(t, "broken.wav", []byte("RIFF not a wav"), nil, false)
	sched.Start()
	defer sched.Stop()

	playOne(t, sched, clock, events.EventTaskFinished)
	if n := len(player.Calls()); n != 0 {
		t.Fatalf("plays = %d; want silence with no fallback dir", n)
	}
}
