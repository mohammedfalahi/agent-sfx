package preview_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/preview"
	"agent-sfx/internal/sounds"
)

func TestPreview_UnplayableUserSoundWarnsAndPlaysBundled(t *testing.T) {
	user, bundled := t.TempDir(), t.TempDir()
	for dir, file := range map[string]struct {
		name string
		data []byte
	}{
		user:    {"broken.wav", []byte("RIFF not a wav")},
		bundled: {"task_finished.wav", audio.EncodePCM16WAV(1, 44100, []int16{1, 2, 3})},
	} {
		d := filepath.Join(dir, string(events.EventTaskFinished))
		_ = os.MkdirAll(d, 0755)
		if err := os.WriteFile(filepath.Join(d, file.name), file.data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cache, err := audio.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := &preview.Runner{
		Cfg:               config.DefaultConfig(),
		Player:            audio.NewFakePlayer(),
		Cache:             cache,
		Selector:          sounds.NewSelector(nil),
		SoundsDir:         user,
		FallbackSoundsDir: bundled,
	}
	res, err := runner.Preview(context.Background(), events.EventTaskFinished)
	if err != nil {
		t.Fatalf("preview should fall back, got %v", err)
	}
	if filepath.Base(res.SoundPath) != "task_finished.wav" || !strings.Contains(res.Warning, "broken.wav") {
		t.Errorf("SoundPath = %s, Warning = %q; want the bundled sound and a warning naming broken.wav", res.SoundPath, res.Warning)
	}
}
