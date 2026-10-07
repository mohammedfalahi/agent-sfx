package audio_test

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/audio"
)

func TestWindowsPowerShellArgs_SafetyAndFixedScript(t *testing.T) {
	args := audio.WindowsPowerShellArgs()

	expectedFlags := []string{"-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command"}
	for i, flag := range expectedFlags {
		if i >= len(args) {
			t.Fatalf("expected flag %s at index %d, but args too short", flag, i)
		}
		if args[i] != flag {
			t.Errorf("expected arg[%d] to be %q, got %q", i, flag, args[i])
		}
	}

	if len(args) != 6 {
		t.Errorf("expected exactly 6 arguments (5 flags + script), got %d: %v", len(args), args)
	}

	script := args[len(args)-1]

	// 1. Script must use environment variable, NOT concatenated path
	if !strings.Contains(script, "$env:"+audio.WindowsPlayPathEnv) {
		t.Errorf("script does not read from $env:%s: %s", audio.WindowsPlayPathEnv, script)
	}

	// 2. Script must use -LiteralPath for safe handling of wildcards, brackets, and quotes
	if !strings.Contains(script, "-LiteralPath") {
		t.Errorf("script does not use -LiteralPath: %s", script)
	}

	// 3. Script must use Media.SoundPlayer and PlaySync()
	if !strings.Contains(script, "Media.SoundPlayer") || !strings.Contains(script, "PlaySync()") {
		t.Errorf("script missing Media.SoundPlayer or PlaySync: %s", script)
	}

	// 4. Script must NOT contain format placeholders
	if strings.Contains(script, "%s") || strings.Contains(script, "%v") {
		t.Errorf("script contains unexpected format placeholder: %s", script)
	}
}

func TestBuildWindowsPlayerEnv_ComplexPaths(t *testing.T) {
	testCases := []struct {
		name string
		path string
	}{
		{
			name: "spaces in directory and filename",
			path: `C:\Program Files\Agent SFX\custom sounds\task finished.wav`,
		},
		{
			name: "apostrophe in path",
			path: `C:\Users\O'Connor\sounds\error.wav`,
		},
		{
			name: "dollar signs in path",
			path: `C:\Users\$User\Desktop\$sounds\alert.wav`,
		},
		{
			name: "unicode characters",
			path: `C:\Users\测试用户\음악\효과음\완료.wav`,
		},
		{
			name: "mixed symbols and ampersands",
			path: `C:\Data & Media\Audio (V2) [100%]\beep#1.wav`,
		},
		{
			name: "escaped and quote symbols",
			path: `C:\Path\With"Quotes"\and\backslashes\sound.wav`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv := []string{"SYSTEMROOT=C:\\Windows", "PATH=C:\\Windows\\System32"}
			env := audio.BuildWindowsPlayerEnv(baseEnv, tc.path)

			// Find AGENT_SFX_PLAY_PATH in env
			var found string
			prefix := audio.WindowsPlayPathEnv + "="
			for _, e := range env {
				if strings.HasPrefix(e, prefix) {
					found = strings.TrimPrefix(e, prefix)
					break
				}
			}

			if found == "" {
				t.Fatalf("environment variable %s not found in generated env", audio.WindowsPlayPathEnv)
			}

			// Value must match exact byte-for-byte input path without corruption
			if found != tc.path {
				t.Errorf("path was mutated in env: expected %q, got %q", tc.path, found)
			}
		})
	}
}

func TestOSPlayer_ContextCancellation(t *testing.T) {
	var bin string
	if runtime.GOOS == "windows" {
		bin = "powershell.exe"
	} else {
		bin = "sleep"
	}

	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("skipping test: %s not available in PATH", bin)
	}

	ctx, cancel := context.WithCancel(context.Background())

	p, err := audio.DetectPlayer()
	if err != nil {
		t.Fatalf("DetectPlayer failed: %v", err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err = p.Play(ctx, "nonexistent.wav")
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		if elapsed > 2*time.Second {
			t.Errorf("Play did not cancel promptly: elapsed %v, err: %v", elapsed, err)
		}
	}
}

func TestOSPlayer_TimeoutExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	p, err := audio.DetectPlayer()
	if err != nil {
		t.Fatalf("DetectPlayer failed: %v", err)
	}

	start := time.Now()
	err = p.Play(ctx, "nonexistent.wav")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("Play did not timeout promptly: elapsed %v, err: %v", elapsed, err)
	}
	if err == nil {
		t.Errorf("expected error for nonexistent file with expired context, got nil")
	}
}

func TestFakePlayer_RecordingAndMocking(t *testing.T) {
	fp := audio.NewFakePlayer()

	if !fp.IsAvailable() {
		t.Errorf("expected fake player to be available")
	}
	if fp.BackendName() != "fake-player" {
		t.Errorf("expected backend name fake-player, got %s", fp.BackendName())
	}

	ctx := context.Background()
	_ = fp.Play(ctx, "test1.wav")
	_ = fp.Play(ctx, "test2.wav")

	calls := fp.Calls()
	if len(calls) != 2 || calls[0] != "test1.wav" || calls[1] != "test2.wav" {
		t.Errorf("unexpected calls: %v", calls)
	}
}
