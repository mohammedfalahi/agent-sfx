package audio

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
)

const (
	WindowsPlayPathEnv = "AGENT_SFX_PLAY_PATH"
	WindowsPlayScript  = `$p = $env:AGENT_SFX_PLAY_PATH; if (-not $p -or -not (Test-Path -LiteralPath $p)) { exit 1 }; (New-Object Media.SoundPlayer $p).PlaySync()`
)

// WindowsPowerShellArgs returns the argument list for invoking PowerShell audio playback.
// The script is constant and does not interpolate user-controlled paths into script source.
func WindowsPowerShellArgs() []string {
	return []string{
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle", "Hidden",
		"-Command",
		WindowsPlayScript,
	}
}

// BuildWindowsPlayerEnv constructs the child-process environment block containing the audio path.
func BuildWindowsPlayerEnv(baseEnv []string, wavPath string) []string {
	return append(baseEnv, WindowsPlayPathEnv+"="+wavPath)
}

// Player provides an abstraction for playing uncompressed WAV audio files on the local OS.
type Player interface {
	Play(ctx context.Context, wavPath string) error
	BackendName() string
	IsAvailable() bool
}

// OSPlayer implements Player using platform-native CLI audio players.
type OSPlayer struct {
	backendName string
	binPath     string
	argsBuilder func(wavPath string) []string
	cmdPreparer func(cmd *exec.Cmd, wavPath string)
}

// DetectPlayer inspects the current OS and available utilities to return the best player backend.
func DetectPlayer() (Player, error) {
	switch runtime.GOOS {
	case "darwin":
		path, err := exec.LookPath("afplay")
		if err != nil {
			path = "/usr/bin/afplay"
		}
		return &OSPlayer{
			backendName: "afplay",
			binPath:     path,
			argsBuilder: func(wavPath string) []string {
				return []string{wavPath}
			},
		}, nil

	case "linux":
		// Probe pw-play, paplay, aplay in order of preference
		candidates := []string{"pw-play", "paplay", "aplay"}
		for _, name := range candidates {
			if path, err := exec.LookPath(name); err == nil {
				return &OSPlayer{
					backendName: name,
					binPath:     path,
					argsBuilder: func(wavPath string) []string {
						return []string{wavPath}
					},
				}, nil
			}
		}
		return &unavailablePlayer{reason: "no supported Linux player found (pw-play, paplay, or aplay)"}, nil

	case "windows":
		path, err := exec.LookPath("powershell.exe")
		if err != nil {
			return &unavailablePlayer{reason: "powershell.exe not found"}, nil
		}
		return &OSPlayer{
			backendName: "powershell-soundplayer",
			binPath:     path,
			argsBuilder: func(wavPath string) []string {
				return WindowsPowerShellArgs()
			},
			cmdPreparer: prepareWindowsPlayerCmd,
		}, nil

	default:
		return &unavailablePlayer{reason: fmt.Sprintf("unsupported operating system %q", runtime.GOOS)}, nil
	}
}

func (p *OSPlayer) BackendName() string {
	return p.backendName
}

func (p *OSPlayer) IsAvailable() bool {
	if p.binPath == "" {
		return false
	}
	_, err := exec.LookPath(p.binPath)
	return err == nil
}

func (p *OSPlayer) Play(ctx context.Context, wavPath string) error {
	if !p.IsAvailable() {
		return fmt.Errorf("player backend %q is not available on this system", p.backendName)
	}

	args := p.argsBuilder(wavPath)
	cmd := exec.CommandContext(ctx, p.binPath, args...)
	// No stdin/stdout/stderr capture to avoid blocking or logging on agent stdout
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if p.cmdPreparer != nil {
		p.cmdPreparer(cmd, wavPath)
	}

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		return fmt.Errorf("playback failed with %s: %w", p.backendName, err)
	}

	return nil
}

type unavailablePlayer struct {
	reason string
}

func (u *unavailablePlayer) Play(ctx context.Context, wavPath string) error {
	return fmt.Errorf("audio playback unavailable: %s", u.reason)
}

func (u *unavailablePlayer) BackendName() string {
	return "none"
}

func (u *unavailablePlayer) IsAvailable() bool {
	return false
}

// FakePlayer is an injectable mock player for deterministic unit testing.
type FakePlayer struct {
	mu          sync.Mutex
	PlayedPaths []string
	PlayFunc    func(ctx context.Context, wavPath string) error
	Available   bool
	Name        string
}

// NewFakePlayer creates an active FakePlayer.
func NewFakePlayer() *FakePlayer {
	return &FakePlayer{
		Available: true,
		Name:      "fake-player",
	}
}

func (f *FakePlayer) BackendName() string {
	if f.Name != "" {
		return f.Name
	}
	return "fake-player"
}

func (f *FakePlayer) IsAvailable() bool {
	return f.Available
}

func (f *FakePlayer) Play(ctx context.Context, wavPath string) error {
	f.mu.Lock()
	f.PlayedPaths = append(f.PlayedPaths, wavPath)
	fn := f.PlayFunc
	f.mu.Unlock()

	if fn != nil {
		return fn(ctx, wavPath)
	}
	return nil
}

func (f *FakePlayer) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]string, len(f.PlayedPaths))
	copy(copied, f.PlayedPaths)
	return copied
}
