package sounds_test

import (
	"path/filepath"
	"testing"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

func TestBundledSounds(t *testing.T) {
	// Root sounds folder relative to repository
	baseDir := filepath.Join("..", "..", "sounds")

	for _, kind := range events.AllEvents {
		t.Run(string(kind), func(t *testing.T) {
			wavs, err := sounds.ListEventSounds(baseDir, kind)
			if err != nil {
				t.Fatalf("ListEventSounds(%s) error: %v", kind, err)
			}
			if len(wavs) == 0 {
				t.Fatalf("no WAV sounds found in %s/%s", baseDir, kind)
			}

			for _, wavPath := range wavs {
				info, err := audio.ValidateWAVFile(wavPath)
				if err != nil {
					t.Fatalf("WAV %s failed validation: %v", wavPath, err)
				}
				if info.BitsPerSample != 16 {
					t.Errorf("WAV %s has bitsPerSample=%d, want 16", wavPath, info.BitsPerSample)
				}
				if info.SampleRate < 8000 || info.SampleRate > 192000 {
					t.Errorf("WAV %s has sampleRate=%d out of range", wavPath, info.SampleRate)
				}
				// Verify duration is positive and within configured default maximum
				if info.DurationMS <= 0 || info.DurationMS > int64(config.DefaultMaxClipMS) {
					t.Errorf("WAV %s has duration %d ms, exceeding default maximum %d ms",
						wavPath, info.DurationMS, config.DefaultMaxClipMS)
				}
			}
		})
	}
}
