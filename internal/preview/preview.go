package preview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

var (
	ErrEventDisabled = errors.New("event is disabled in configuration")
	ErrPlayerOffline = errors.New("audio player backend is unavailable")
)

// Result contains diagnostics about a preview playback execution.
type Result struct {
	Event      events.EventKind `json:"event"`
	SoundPath  string           `json:"sound_path"`
	ScaledPath string           `json:"scaled_path"`
	Volume     float64          `json:"volume"`
	Player     string           `json:"player"`
	DurationMS int64            `json:"duration_ms"`
	Skipped    bool             `json:"skipped"`
	SkipReason string           `json:"skip_reason,omitempty"`
}

// Runner coordinates configuration, sound selection, volume scaling, and player execution.
type Runner struct {
	Cfg       config.Config
	Player    audio.Player
	Cache     *audio.Cache
	Selector  *sounds.Selector
	SoundsDir string
}

// ResolveSoundsDir finds the active sounds directory checking:
// 1. Explicit CLI override
// 2. Config file sounds_dir
// 3. User config directory (~/.config/agent-sfx/sounds or OS equivalent)
// 4. Executable adjacent sounds directory
// 5. Working directory ./sounds fallback
// Returns a fully resolved absolute path.
func ResolveSoundsDir(cliDir, cfgDir string) string {
	candidates := []string{}
	if cliDir != "" {
		candidates = append(candidates, cliDir)
	}
	if cfgDir != "" {
		candidates = append(candidates, cfgDir)
	}

	// 1. User config directory
	if userDir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(userDir, "agent-sfx", "sounds"))
	}

	// 2. Executable adjacent sounds directory
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, "sounds"),
			filepath.Join(exeDir, "..", "sounds"),
		)
	}

	// 3. Fallback to current working directory ./sounds
	candidates = append(candidates, "sounds")

	for _, dir := range candidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(dir); err == nil {
				return abs
			}
			return dir
		}
	}

	if abs, err := filepath.Abs("sounds"); err == nil {
		return abs
	}
	return "sounds"
}

// Preview executes a manual sound playback for the requested event kind.
func (r *Runner) Preview(ctx context.Context, kind events.EventKind) (*Result, error) {
	if !kind.IsValid() {
		return nil, fmt.Errorf("invalid event kind: %q", kind)
	}

	if !r.Cfg.IsEventEnabled(kind) {
		return &Result{
			Event:      kind,
			Skipped:    true,
			SkipReason: fmt.Sprintf("event %s is disabled in config", kind),
		}, ErrEventDisabled
	}

	if !r.Player.IsAvailable() {
		return &Result{
			Event:      kind,
			Player:     r.Player.BackendName(),
			Skipped:    true,
			SkipReason: "audio player is unavailable",
		}, ErrPlayerOffline
	}

	chosenSound, err := r.Selector.SelectSound(r.SoundsDir, kind)
	if err != nil {
		return nil, fmt.Errorf("sound selection failed for %s: %w", kind, err)
	}

	info, err := audio.ValidateWAVFile(chosenSound)
	if err != nil {
		return nil, fmt.Errorf("selected sound file %s is invalid: %w", chosenSound, err)
	}

	// 1. Validate actual PCM duration against configured max_clip_ms
	if err := info.ValidateClipDuration(r.Cfg.MaxClipMS); err != nil {
		return nil, fmt.Errorf("selected sound %s exceeds maximum duration: %w", chosenSound, err)
	}

	// 2. Scale volume
	scaledPath, err := r.Cache.GetOrScale(chosenSound, r.Cfg.Volume)
	if err != nil {
		return nil, fmt.Errorf("volume scaling failed for %s: %w", chosenSound, err)
	}

	// 3. Apply shared process execution timeout: actual clip duration + 5s overhead
	timeout := audio.ComputePlayerTimeout(info.DurationMS)
	playCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := r.Player.Play(playCtx, scaledPath); err != nil {
		return nil, fmt.Errorf("audio playback failed: %w", err)
	}

	return &Result{
		Event:      kind,
		SoundPath:  chosenSound,
		ScaledPath: scaledPath,
		Volume:     r.Cfg.Volume,
		Player:     r.Player.BackendName(),
		DurationMS: info.DurationMS,
		Skipped:    false,
	}, nil
}
