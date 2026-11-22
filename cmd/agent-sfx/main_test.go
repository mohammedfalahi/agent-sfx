//go:build !windows

package main_test

import (
	"os/exec"
	"strings"
	"testing"

	"agent-sfx/internal/preview"
	"agent-sfx/internal/testutil"
)

// Regression test: `agent-sfx worker start` (run through a launcher that sets the
// bundled sounds variable) must honor the configured sounds_dir.
func TestWorkerStartHonorsConfiguredSoundsDir(t *testing.T) {
	binPath := testutil.BuildAgentSFX(t)
	env := testutil.NewIsolatedEnv(t)
	cmdEnv := env.Env(preview.BundledSoundsEnv + "=" + env.BundledDir)

	start := exec.Command(binPath, "worker", "start")
	start.Env = cmdEnv
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("worker start failed: %v\n%s", err, out)
	}

	st := env.WaitForWorker(t)
	if !testutil.SamePath(st.SoundsDir, env.CustomDir) {
		t.Fatalf("worker sounds dir = %q, want configured %q", st.SoundsDir, env.CustomDir)
	}

	// `status` shows the active sounds directory to the user.
	status := exec.Command(binPath, "status")
	status.Env = cmdEnv
	out, err := status.CombinedOutput()
	if err != nil {
		t.Fatalf("status failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Worker Sounds    : ") || !strings.Contains(string(out), "custom") {
		t.Fatalf("status output does not report the custom sounds dir:\n%s", out)
	}
	if !strings.Contains(string(out), "Persisted Config : disabled") {
		t.Fatalf("expected the isolated config (sounds disabled) to be used:\n%s", out)
	}
}

// An explicit --sounds-dir still wins over config.
func TestWorkerStartExplicitSoundsDirWins(t *testing.T) {
	binPath := testutil.BuildAgentSFX(t)
	env := testutil.NewIsolatedEnv(t)

	start := exec.Command(binPath, "worker", "start", "--sounds-dir", env.BundledDir)
	start.Env = env.Env()
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("worker start failed: %v\n%s", err, out)
	}
	st := env.WaitForWorker(t)
	if !testutil.SamePath(st.SoundsDir, env.BundledDir) {
		t.Fatalf("worker sounds dir = %q, want explicit %q", st.SoundsDir, env.BundledDir)
	}
}
