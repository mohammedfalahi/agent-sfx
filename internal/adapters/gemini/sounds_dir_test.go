//go:build !windows

package gemini_test

import (
	"os/exec"
	"strings"
	"testing"

	"agent-sfx/internal/preview"
	"agent-sfx/internal/testutil"
)

// Regression test: a worker started by the Gemini SessionStart hook must use the
// sounds_dir from the user's config, not the extension's bundled starter sounds.
func TestSubprocess_SessionStartWorkerHonorsConfiguredSoundsDir(t *testing.T) {
	binPath := testutil.BuildAgentSFX(t)
	env := testutil.NewIsolatedEnv(t)

	cmd := exec.Command(binPath, "hook", "gemini")
	cmd.Env = env.Env(preview.BundledSoundsEnv + "=" + env.BundledDir)
	cmd.Stdin = strings.NewReader(`{"session_id":"sounds-dir","hook_event_name":"SessionStart","source":"startup","timestamp":"2026-11-20T00:00:00Z"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hook failed: %v", err)
	}
	// Gemini requires stdout to be JSON only.
	if strings.TrimSpace(string(out)) != "{}" {
		t.Fatalf("expected {} on stdout, got %q", out)
	}

	st := env.WaitForWorker(t)
	if !testutil.SamePath(st.SoundsDir, env.CustomDir) {
		t.Fatalf("worker sounds dir = %q, want configured %q (bundled is %q)", st.SoundsDir, env.CustomDir, env.BundledDir)
	}
	if !testutil.SamePath(st.FallbackSoundsDir, env.BundledDir) {
		t.Fatalf("worker fallback sounds dir = %q, want bundled %q", st.FallbackSoundsDir, env.BundledDir)
	}
}
