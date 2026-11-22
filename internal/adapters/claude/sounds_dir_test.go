//go:build !windows

package claude_test

import (
	"os/exec"
	"strings"
	"testing"

	"agent-sfx/internal/preview"
	"agent-sfx/internal/testutil"
)

// Regression test: a worker started by the Claude SessionStart hook must use the
// sounds_dir from the user's config, not the plugin's bundled starter sounds.
// The bundled sounds are only a per-event fallback.
func TestSubprocess_SessionStartWorkerHonorsConfiguredSoundsDir(t *testing.T) {
	binPath := testutil.BuildAgentSFX(t)
	env := testutil.NewIsolatedEnv(t)

	cmd := exec.Command(binPath, "hook", "claude")
	// The plugin launcher (runtime/run.js) always sets the bundled sounds variable.
	cmd.Env = env.Env(preview.BundledSoundsEnv + "=" + env.BundledDir)
	cmd.Stdin = strings.NewReader(`{"session_id":"sounds-dir","hook_event_name":"SessionStart","source":"startup"}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hook failed: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty stdout, got %q", out)
	}

	st := env.WaitForWorker(t)
	if !testutil.SamePath(st.SoundsDir, env.CustomDir) {
		t.Fatalf("worker sounds dir = %q, want configured %q (bundled is %q)", st.SoundsDir, env.CustomDir, env.BundledDir)
	}
	if !testutil.SamePath(st.FallbackSoundsDir, env.BundledDir) {
		t.Fatalf("worker fallback sounds dir = %q, want bundled %q", st.FallbackSoundsDir, env.BundledDir)
	}
}
