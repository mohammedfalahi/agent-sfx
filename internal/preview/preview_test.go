package preview_test

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
	"agent-sfx/internal/preview"
	"agent-sfx/internal/sounds"
)

func setupTestEnvironment(t *testing.T) (string, *audio.Cache, *audio.FakePlayer) {
	t.Helper()
	tmpDir := t.TempDir()

	cacheDir := filepath.Join(tmpDir, "cache")
	cache, err := audio.NewCache(cacheDir)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	player := audio.NewFakePlayer()
	return tmpDir, cache, player
}

func writeValidWAV(t *testing.T, path string, samples []int16) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	wavBytes := audio.EncodePCM16WAV(1, 44100, samples)
	if err := os.WriteFile(path, wavBytes, 0644); err != nil {
		t.Fatalf("failed to write WAV: %v", err)
	}
}

func TestPreview_ConfigDisabledGlobal(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()
	cfg.Enabled = false

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	res, err := runner.Preview(context.Background(), events.EventTaskFinished)
	if !errors.Is(err, preview.ErrEventDisabled) {
		t.Fatalf("expected ErrEventDisabled, got %v", err)
	}
	if res == nil || !res.Skipped {
		t.Fatalf("expected result.Skipped to be true")
	}
	if len(player.Calls()) != 0 {
		t.Fatalf("player should not be called when disabled")
	}
}

func TestPreview_ConfigDisabledEvent(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()
	cfg.Events[events.EventTaskStarted] = false

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	res, err := runner.Preview(context.Background(), events.EventTaskStarted)
	if !errors.Is(err, preview.ErrEventDisabled) {
		t.Fatalf("expected ErrEventDisabled, got %v", err)
	}
	if res == nil || !res.Skipped {
		t.Fatalf("expected result.Skipped to be true")
	}
	if len(player.Calls()) != 0 {
		t.Fatalf("player should not be called when disabled")
	}
}

func TestPreview_MissingWAV(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir, // empty directory
	}

	_, err := runner.Preview(context.Background(), events.EventTaskFinished)
	if err == nil {
		t.Fatalf("expected error for missing sound files, got nil")
	}
	if len(player.Calls()) != 0 {
		t.Fatalf("player should not be called on missing sound")
	}
}

func TestPreview_InvalidWAV(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()

	// Write corrupt/invalid file with .wav extension
	corruptPath := filepath.Join(tmpDir, string(events.EventError), "corrupt.wav")
	if err := os.MkdirAll(filepath.Dir(corruptPath), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(corruptPath, []byte("NOT A REAL WAV FILE CONTENT"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	_, err := runner.Preview(context.Background(), events.EventError)
	if err == nil {
		t.Fatalf("expected error on corrupt WAV file, got nil")
	}
	if len(player.Calls()) != 0 {
		t.Fatalf("player should not be called on corrupt WAV")
	}
}

func TestPreview_VolumeBoundsAndScaling(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()
	cfg.Volume = 0.5 // 50% volume

	soundPath := filepath.Join(tmpDir, string(events.EventTestsPassed), "pass.wav")
	writeValidWAV(t, soundPath, []int16{1000, 2000, 3000})

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	res, err := runner.Preview(context.Background(), events.EventTestsPassed)
	if err != nil {
		t.Fatalf("unexpected preview error: %v", err)
	}

	calls := player.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 player call, got %d", len(calls))
	}

	// Should have played the scaled cached file, not the raw source
	if calls[0] == soundPath {
		t.Errorf("expected scaled cached path to be played at volume 0.5, got source path")
	}
	if res.ScaledPath != calls[0] {
		t.Errorf("result ScaledPath mismatch: %s vs %s", res.ScaledPath, calls[0])
	}
	if res.Volume != 0.5 {
		t.Errorf("expected result volume 0.5, got %v", res.Volume)
	}

	// Test volume = 1.0 (unscaled)
	cfg.Volume = 1.0
	runner.Cfg = cfg
	res1, err := runner.Preview(context.Background(), events.EventTestsPassed)
	if err != nil {
		t.Fatalf("unexpected preview error at volume 1.0: %v", err)
	}
	calls2 := player.Calls()
	if len(calls2) != 2 {
		t.Fatalf("expected 2 player calls, got %d", len(calls2))
	}
	if calls2[1] != soundPath {
		t.Errorf("volume 1.0 should play original path directly: %s vs %s", calls2[1], soundPath)
	}
	if res1.ScaledPath != soundPath {
		t.Errorf("expected ScaledPath == soundPath at volume 1.0")
	}
}

func TestPreview_NoImmediateRepeat(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()

	file1 := filepath.Join(tmpDir, string(events.EventTaskFinished), "f1.wav")
	file2 := filepath.Join(tmpDir, string(events.EventTaskFinished), "f2.wav")
	writeValidWAV(t, file1, []int16{500, 600})
	writeValidWAV(t, file2, []int16{700, 800})

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	var lastPlayed string
	for i := 0; i < 15; i++ {
		res, err := runner.Preview(context.Background(), events.EventTaskFinished)
		if err != nil {
			t.Fatalf("preview failed on iteration %d: %v", i, err)
		}
		if res.SoundPath == lastPlayed {
			t.Fatalf("iteration %d: immediate repeat of %s", i, res.SoundPath)
		}
		lastPlayed = res.SoundPath
	}
}

func TestPreview_PlayerOffline(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	player.Available = false
	cfg := config.DefaultConfig()

	soundPath := filepath.Join(tmpDir, string(events.EventPermissionRequested), "perm.wav")
	writeValidWAV(t, soundPath, []int16{100, 200})

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	_, err := runner.Preview(context.Background(), events.EventPermissionRequested)
	if !errors.Is(err, preview.ErrPlayerOffline) {
		t.Fatalf("expected ErrPlayerOffline, got %v", err)
	}
	if len(player.Calls()) != 0 {
		t.Fatalf("offline player should not be called")
	}
}

func TestPreview_FifteenSecondClipPlaysToCompletion(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig() // default max_clip_ms = 15000

	// Synthesize exactly 15 seconds of 44100Hz audio (661,500 samples)
	sampleCount := 44100 * 15
	samples := make([]int16, sampleCount)
	for i := range samples {
		samples[i] = int16(i % 500)
	}

	soundPath := filepath.Join(tmpDir, string(events.EventTaskFinished), "fifteen_sec.wav")
	writeValidWAV(t, soundPath, samples)

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	res, err := runner.Preview(context.Background(), events.EventTaskFinished)
	if err != nil {
		t.Fatalf("unexpected error playing 15s clip: %v", err)
	}

	if res.DurationMS != 15000 {
		t.Errorf("expected exactly 15000ms duration, got %dms", res.DurationMS)
	}

	if len(player.Calls()) != 1 {
		t.Fatalf("expected 1 player call, got %d", len(player.Calls()))
	}
}

func TestPreview_SixteenSecondClipRejected(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig() // default max_clip_ms = 15000

	// Synthesize 16 seconds of 44100Hz audio (705,600 samples)
	sampleCount := 44100 * 16
	samples := make([]int16, sampleCount)
	soundPath := filepath.Join(tmpDir, string(events.EventError), "sixteen_sec.wav")
	writeValidWAV(t, soundPath, samples)

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	_, err := runner.Preview(context.Background(), events.EventError)
	if err == nil {
		t.Fatalf("expected 16s clip to be rejected under 15000ms limit, got nil")
	}

	if len(player.Calls()) != 0 {
		t.Fatalf("player must not be called when clip duration is exceeded")
	}
}

func TestPreview_ClipExceedingMaxDurationRejected(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()
	cfg.MaxClipMS = 3000 // configured limit is 3 seconds

	// Synthesize 6 seconds audio
	sampleCount := 44100 * 6
	samples := make([]int16, sampleCount)
	soundPath := filepath.Join(tmpDir, string(events.EventError), "too_long.wav")
	writeValidWAV(t, soundPath, samples)

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	_, err := runner.Preview(context.Background(), events.EventError)
	if err == nil {
		t.Fatalf("expected clip exceeding max_clip_ms to be rejected, got nil")
	}

	if len(player.Calls()) != 0 {
		t.Fatalf("player must not be called when clip duration is exceeded")
	}
}

func TestPreview_HungPlayerCanceled(t *testing.T) {
	tmpDir, cache, player := setupTestEnvironment(t)
	cfg := config.DefaultConfig()
	cfg.MaxClipMS = 100 // 100ms clip limit

	soundPath := filepath.Join(tmpDir, string(events.EventTaskStarted), "short.wav")
	writeValidWAV(t, soundPath, []int16{100, 200, 300}) // ~0.06ms

	var timedOut bool
	player.PlayFunc = func(ctx context.Context, wavPath string) error {
		select {
		case <-ctx.Done():
			timedOut = true
			return ctx.Err()
		case <-time.After(10 * time.Second): // simulates hung player process
			return nil
		}
	}

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: tmpDir,
	}

	// The player process timeout for 0ms clip with max_clip_ms=100 is:
	// clipDur (0) + 5s = 5s. But if we cancel the parent context or let timeout hit:
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := runner.Preview(ctx, events.EventTaskStarted)
	if err == nil {
		t.Fatalf("expected error on hung player, got nil")
	}

	if !timedOut {
		t.Errorf("expected player to receive context cancellation")
	}
}
