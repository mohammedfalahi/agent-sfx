package sounds_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

func TestListEventSounds_IncludesConvertibleFormats(t *testing.T) {
	dir := t.TempDir()
	evDir := filepath.Join(dir, string(events.EventTaskFinished))
	if err := os.MkdirAll(evDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.wav", "b.MP3", "c.m4a", "d.ogg", "notes.txt", ".DS_Store"} {
		if err := os.WriteFile(filepath.Join(evDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := sounds.ListEventSounds(dir, events.EventTaskFinished)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.wav"}
	if audio.ConversionAvailable() {
		want = []string{"a.wav", "b.MP3", "c.m4a"}
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if filepath.Base(got[i]) != want[i] {
			t.Errorf("got[%d] = %s, want %s", i, filepath.Base(got[i]), want[i])
		}
	}
}

// A folder holding only MP3s counts as customized, so it overrides the
// bundled sounds instead of falling back to them.
func TestSelectSoundFromDirs_MP3OnlyFolderWins(t *testing.T) {
	if !audio.ConversionAvailable() {
		t.Skip("MP3 is only listed where it can be converted")
	}
	user, bundled := t.TempDir(), t.TempDir()
	for dir, name := range map[string]string{user: "mine.mp3", bundled: "task_finished.wav"} {
		d := filepath.Join(dir, string(events.EventTaskFinished))
		_ = os.MkdirAll(d, 0755)
		_ = os.WriteFile(filepath.Join(d, name), []byte("x"), 0644)
	}
	got, err := sounds.NewSelector(nil).SelectSoundFromDirs(events.EventTaskFinished, user, bundled)
	if err != nil || filepath.Base(got) != "mine.mp3" {
		t.Fatalf("got %q, %v; want mine.mp3", got, err)
	}
}
