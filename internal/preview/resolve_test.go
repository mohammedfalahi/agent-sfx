package preview_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/preview"
)

// isolateUserConfigDir points os.UserConfigDir at a fresh temporary home and
// returns the agent-sfx sounds path inside it (not created).
func isolateUserConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	userDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	return filepath.Join(userDir, "agent-sfx", "sounds")
}

func mustMkdir(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(dir)
	return abs
}

// Regression: the launcher's bundled sounds must never override the user's sounds.
func TestResolveSoundsDir_UserDirBeatsBundled(t *testing.T) {
	userSounds := mustMkdir(t, isolateUserConfigDir(t))
	bundled := mustMkdir(t, filepath.Join(t.TempDir(), "bundled"))
	t.Setenv(preview.BundledSoundsEnv, bundled)

	if got := preview.ResolveSoundsDir("", ""); got != userSounds {
		t.Fatalf("expected user sounds dir %s, got %s", userSounds, got)
	}
}

func TestResolveSoundsDir_ConfigBeatsBundled(t *testing.T) {
	isolateUserConfigDir(t)
	custom := mustMkdir(t, filepath.Join(t.TempDir(), "custom"))
	bundled := mustMkdir(t, filepath.Join(t.TempDir(), "bundled"))
	t.Setenv(preview.BundledSoundsEnv, bundled)

	if got := preview.ResolveSoundsDir("", custom); got != custom {
		t.Fatalf("expected configured sounds dir %s, got %s", custom, got)
	}
}

func TestResolveSoundsDir_CLIBeatsConfig(t *testing.T) {
	isolateUserConfigDir(t)
	cli := mustMkdir(t, filepath.Join(t.TempDir(), "cli"))
	custom := mustMkdir(t, filepath.Join(t.TempDir(), "custom"))

	if got := preview.ResolveSoundsDir(cli, custom); got != cli {
		t.Fatalf("expected CLI sounds dir %s, got %s", cli, got)
	}
}

func TestResolveSoundsDir_BundledUsedWhenNoUserSounds(t *testing.T) {
	isolateUserConfigDir(t) // user sounds dir intentionally not created
	bundled := mustMkdir(t, filepath.Join(t.TempDir(), "bundled"))
	t.Setenv(preview.BundledSoundsEnv, bundled)

	if got := preview.ResolveSoundsDir("", ""); got != bundled {
		t.Fatalf("expected bundled sounds dir %s, got %s", bundled, got)
	}
}

func TestBundledSoundsDir(t *testing.T) {
	t.Setenv(preview.BundledSoundsEnv, "")
	if got := preview.BundledSoundsDir(); got != "" {
		t.Fatalf("expected empty when unset, got %q", got)
	}

	t.Setenv(preview.BundledSoundsEnv, filepath.Join(t.TempDir(), "missing"))
	if got := preview.BundledSoundsDir(); got != "" {
		t.Fatalf("expected empty for a missing directory, got %q", got)
	}

	bundled := mustMkdir(t, filepath.Join(t.TempDir(), "bundled"))
	t.Setenv(preview.BundledSoundsEnv, bundled)
	if got := preview.BundledSoundsDir(); got != bundled {
		t.Fatalf("expected %s, got %s", bundled, got)
	}
}
