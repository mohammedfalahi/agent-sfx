package sounds_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

func writeWAVFile(t *testing.T, dir string, kind events.EventKind, name string) string {
	t.Helper()
	eventDir := filepath.Join(dir, string(kind))
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(eventDir, name)
	if err := os.WriteFile(p, []byte("RIFF"), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSelectSoundFromDirs_PrimaryWinsWhenItHasSounds(t *testing.T) {
	primary, fallback := t.TempDir(), t.TempDir()
	want := writeWAVFile(t, primary, events.EventTaskFinished, "custom.wav")
	writeWAVFile(t, fallback, events.EventTaskFinished, "bundled.wav")

	got, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventTaskFinished, primary, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected custom sound %s, got %s", want, got)
	}
}

func TestSelectSoundFromDirs_FallsBackPerEvent(t *testing.T) {
	primary, fallback := t.TempDir(), t.TempDir()
	// The user customized only task_finished; error must still play the bundled sound.
	writeWAVFile(t, primary, events.EventTaskFinished, "custom.wav")
	want := writeWAVFile(t, fallback, events.EventError, "bundled.wav")

	got, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventError, primary, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected bundled fallback %s, got %s", want, got)
	}
}

func TestSelectSoundFromDirs_EmptyEventFolderFallsBack(t *testing.T) {
	primary, fallback := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(primary, string(events.EventError)), 0755); err != nil {
		t.Fatal(err)
	}
	// Non-WAV files do not count as sounds.
	if err := os.WriteFile(filepath.Join(primary, string(events.EventError), "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	want := writeWAVFile(t, fallback, events.EventError, "bundled.wav")

	got, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventError, primary, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected bundled fallback %s, got %s", want, got)
	}
}

func TestSelectSoundFromDirs_NoSoundsAnywhere(t *testing.T) {
	_, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventError, t.TempDir(), "", t.TempDir())
	if !errors.Is(err, sounds.ErrNoSounds) {
		t.Fatalf("expected ErrNoSounds, got %v", err)
	}
}

func TestSelectSoundFromDirs_NoDirs(t *testing.T) {
	_, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventError)
	if !errors.Is(err, sounds.ErrNoSounds) {
		t.Fatalf("expected ErrNoSounds, got %v", err)
	}
}
