package sounds_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

type fixedRNG struct {
	values []int
	idx    int
}

func (f *fixedRNG) Intn(n int) int {
	if len(f.values) == 0 {
		return 0
	}
	val := f.values[f.idx%len(f.values)]
	f.idx++
	if n <= 0 {
		return 0
	}
	return val % n
}

func TestSelectSound_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	selector := sounds.NewSelector(nil)

	_, err := selector.SelectSound(tmpDir, events.EventTaskFinished)
	if err != sounds.ErrNoSounds {
		t.Fatalf("expected ErrNoSounds, got %v", err)
	}
}

func TestSelectSound_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	eventDir := filepath.Join(tmpDir, string(events.EventTaskStarted))
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	soundFile := filepath.Join(eventDir, "sound1.wav")
	if err := os.WriteFile(soundFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	selector := sounds.NewSelector(nil)
	for i := 0; i < 5; i++ {
		chosen, err := selector.SelectSound(tmpDir, events.EventTaskStarted)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chosen != soundFile {
			t.Errorf("expected %s, got %s", soundFile, chosen)
		}
	}
}

func TestSelectSound_NoImmediateRepeat(t *testing.T) {
	tmpDir := t.TempDir()
	eventDir := filepath.Join(tmpDir, string(events.EventTaskFinished))
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	fileA := filepath.Join(eventDir, "a.wav")
	fileB := filepath.Join(eventDir, "b.wav")
	_ = os.WriteFile(fileA, []byte("fake wav a"), 0644)
	_ = os.WriteFile(fileB, []byte("fake wav b"), 0644)

	// Also write a non-wav file to ensure it's ignored
	_ = os.WriteFile(filepath.Join(eventDir, "notes.txt"), []byte("notes"), 0644)

	selector := sounds.NewSelector(nil)
	var last string

	for i := 0; i < 20; i++ {
		chosen, err := selector.SelectSound(tmpDir, events.EventTaskFinished)
		if err != nil {
			t.Fatalf("unexpected error on turn %d: %v", i, err)
		}
		if chosen == last {
			t.Fatalf("immediate repeat detected on turn %d: picked %s twice in a row", i, chosen)
		}
		last = chosen
	}
}

func TestSelectSound_DeterministicRNG(t *testing.T) {
	tmpDir := t.TempDir()
	eventDir := filepath.Join(tmpDir, string(events.EventError))
	_ = os.MkdirAll(eventDir, 0755)

	f1 := filepath.Join(eventDir, "1.wav")
	f2 := filepath.Join(eventDir, "2.wav")
	f3 := filepath.Join(eventDir, "3.wav")
	_ = os.WriteFile(f1, []byte("1"), 0644)
	_ = os.WriteFile(f2, []byte("2"), 0644)
	_ = os.WriteFile(f3, []byte("3"), 0644)

	// Inject controlled RNG
	rng := &fixedRNG{values: []int{0, 0, 1}}
	selector := sounds.NewSelector(rng)

	// Pick 1: candidates [1.wav, 2.wav, 3.wav], RNG returns 0 -> 1.wav
	c1, err := selector.SelectSound(tmpDir, events.EventError)
	if err != nil || c1 != f1 {
		t.Fatalf("pick 1: expected %s, got %s (err: %v)", f1, c1, err)
	}

	// Pick 2: candidates excluding 1.wav -> [2.wav, 3.wav], RNG returns 0 -> 2.wav
	c2, err := selector.SelectSound(tmpDir, events.EventError)
	if err != nil || c2 != f2 {
		t.Fatalf("pick 2: expected %s, got %s (err: %v)", f2, c2, err)
	}

	// Pick 3: candidates excluding 2.wav -> [1.wav, 3.wav], RNG returns 1 -> 3.wav
	c3, err := selector.SelectSound(tmpDir, events.EventError)
	if err != nil || c3 != f3 {
		t.Fatalf("pick 3: expected %s, got %s (err: %v)", f3, c3, err)
	}
}
