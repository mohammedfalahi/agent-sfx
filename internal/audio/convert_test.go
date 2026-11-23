package audio_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/audio"
)

func newTestCache(t *testing.T) *audio.Cache {
	t.Helper()
	c, err := audio.NewCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// fakeConverter writes a valid WAV to dst and counts its calls.
func fakeConverter(calls *int, samples int) audio.Converter {
	return func(ctx context.Context, src, dst string) error {
		*calls++
		return os.WriteFile(dst, audio.EncodePCM16WAV(1, 8000, make([]int16, samples)), 0600)
	}
}

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIsSupportedSoundFile(t *testing.T) {
	for _, name := range []string{"a.wav", "B.WAV"} {
		if !audio.IsSupportedSoundFile(name) {
			t.Errorf("%s should always be supported", name)
		}
	}
	for _, name := range []string{"a.ogg", "a.flac", "notes.txt", "mp3", ".DS_Store"} {
		if audio.IsSupportedSoundFile(name) {
			t.Errorf("%s should not be supported", name)
		}
	}
	for _, name := range []string{"a.mp3", "A.MP3", "a.m4a", "a.aac", "a.aif", "a.aiff", "a.caf"} {
		if got, want := audio.IsSupportedSoundFile(name), audio.ConversionAvailable(); got != want {
			t.Errorf("IsSupportedSoundFile(%s) = %v, want %v (conversion available: %v)", name, got, want, want)
		}
	}
}

func TestToWAV_PassesThroughWAV(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(fakeConverter(&calls, 10))
	src := writeFile(t, filepath.Join(t.TempDir(), "clip.wav"), audio.EncodePCM16WAV(1, 8000, []int16{0}))
	got, err := c.ToWAV(src)
	if err != nil || got != src || calls != 0 {
		t.Fatalf("ToWAV(wav) = %q, %v (calls %d); want the source path unchanged", got, err, calls)
	}
}

func TestToWAV_ConvertsOnceAndReconvertsWhenSourceChanges(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(fakeConverter(&calls, 10))
	src := writeFile(t, filepath.Join(t.TempDir(), "clip.mp3"), []byte("not really mp3"))

	first, err := c.ToWAV(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(first) != c.Dir() || !strings.HasSuffix(first, ".wav") {
		t.Errorf("converted file %s should be a .wav inside the cache %s", first, c.Dir())
	}
	if _, err := audio.ValidateWAVFile(first); err != nil {
		t.Fatalf("converted file invalid: %v", err)
	}
	second, err := c.ToWAV(src)
	if err != nil || second != first || calls != 1 {
		t.Fatalf("second ToWAV = %q, %v with %d conversions; want cached %q after 1", second, err, calls, first)
	}

	// Replacing the source (new size and mtime) must trigger a fresh conversion.
	writeFile(t, src, []byte("a different, longer not-mp3"))
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(src, later, later)
	third, err := c.ToWAV(src)
	if err != nil || third == first || calls != 2 {
		t.Fatalf("after replacing source: %q, %v with %d conversions; want a new file after 2", third, err, calls)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source file must be left in place: %v", err)
	}
}

func TestToWAV_RemembersFailures(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(func(ctx context.Context, src, dst string) error {
		calls++
		return errors.New("decoder exploded")
	})
	src := writeFile(t, filepath.Join(t.TempDir(), "broken.mp3"), []byte("junk"))
	for i := 0; i < 3; i++ {
		_, err := c.ToWAV(src)
		if err == nil || !strings.Contains(err.Error(), "decoder exploded") {
			t.Fatalf("attempt %d: err = %v, want the converter's error", i, err)
		}
	}
	if calls != 1 {
		t.Errorf("converter ran %d times; a failed file should be converted once until it changes", calls)
	}
	entries, _ := os.ReadDir(c.Dir())
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("temporary file %s left in cache", e.Name())
		}
	}
}

func TestToWAV_RejectsInvalidConverterOutput(t *testing.T) {
	c := newTestCache(t)
	c.SetConverter(func(ctx context.Context, src, dst string) error {
		return os.WriteFile(dst, []byte("RIFF garbage"), 0600)
	})
	src := writeFile(t, filepath.Join(t.TempDir(), "clip.m4a"), []byte("x"))
	if _, err := c.ToWAV(src); err == nil {
		t.Fatal("expected an error for invalid converter output")
	}
}

func TestToWAV_RejectsOversizedSource(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(fakeConverter(&calls, 10))
	src := filepath.Join(t.TempDir(), "huge.mp3")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(audio.MaxWAVSizeBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := c.ToWAV(src); err == nil || calls != 0 {
		t.Fatalf("oversized source: err = %v, conversions = %d; want rejection before converting", err, calls)
	}
}

func TestToWAV_UnsupportedExtension(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(fakeConverter(&calls, 10))
	src := writeFile(t, filepath.Join(t.TempDir(), "clip.ogg"), []byte("x"))
	if _, err := c.ToWAV(src); err == nil || calls != 0 {
		t.Fatalf("unsupported extension: err = %v, conversions = %d", err, calls)
	}
}

func TestPrepare_ConvertsValidatesAndScales(t *testing.T) {
	c := newTestCache(t)
	calls := 0
	c.SetConverter(fakeConverter(&calls, 800)) // 100 ms at 8 kHz
	src := writeFile(t, filepath.Join(t.TempDir(), "clip.mp3"), []byte("x"))

	path, info, err := c.Prepare(src, 0.5, 15000)
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMS != 100 || filepath.Dir(path) != c.Dir() {
		t.Errorf("Prepare = %s, %+v", path, info)
	}
	if _, _, err := c.Prepare(src, 0.5, 50); err == nil {
		t.Error("a converted clip longer than max_clip_ms must be rejected")
	}
}

func TestRunBoundedConversion_StopsOversizedOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	dst := filepath.Join(t.TempDir(), "out.wav")
	// Writes 4 MB slowly; the cap is 1 MB.
	script := `i=0; while [ $i -lt 64 ]; do head -c 65536 /dev/zero >> "$1"; sleep 0.02; i=$((i+1)); done`
	err := audio.RunBoundedConversion(context.Background(), "/bin/sh", []string{"-c", script, "sh", dst}, dst, 1<<20, 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want size-limit error", err)
	}
}

func TestRunBoundedConversion_Timeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sleep")
	}
	dst := filepath.Join(t.TempDir(), "out.wav")
	start := time.Now()
	err := audio.RunBoundedConversion(context.Background(), "/bin/sleep", []string{"5"}, dst, 1<<20, 200*time.Millisecond)
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("err = %v after %v, want a timeout well under 5 s", err, time.Since(start))
	}
}

// End to end with the real macOS converter: AAC (.m4a) in, playable WAV out.
func TestToWAV_RealAfconvert(t *testing.T) {
	if !audio.ConversionAvailable() {
		t.Skip("afconvert not available")
	}
	dir := t.TempDir()
	samples := make([]int16, 44100/4)
	for i := range samples {
		samples[i] = int16((i % 100) * 300)
	}
	wav := writeFile(t, filepath.Join(dir, "in.wav"), audio.EncodePCM16WAV(1, 44100, samples))
	m4a := filepath.Join(dir, "my clip (1).m4a")
	if out, err := exec.Command("/usr/bin/afconvert", "-f", "m4af", "-d", "aac", wav, m4a).CombinedOutput(); err != nil {
		t.Fatalf("could not create AAC fixture: %v %s", err, out)
	}
	c := newTestCache(t)
	path, info, err := c.Prepare(m4a, 0.6, 15000)
	if err != nil {
		t.Fatalf("Prepare(.m4a): %v", err)
	}
	if info.DurationMS < 200 || info.DurationMS > 400 {
		t.Errorf("converted duration %d ms, want about 250", info.DurationMS)
	}
	if _, err := audio.ValidateWAVFile(path); err != nil {
		t.Errorf("prepared file invalid: %v", err)
	}
}
