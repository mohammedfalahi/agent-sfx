// Package testutil holds helpers shared by subprocess tests.
package testutil

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// RepoRoot returns the absolute path of the repository root.
func RepoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to locate testutil source file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// BuildAgentSFX compiles the current agent-sfx sources into a per-test temporary
// directory and returns the binary path. It always builds from source, so tests
// never exercise a stale binary left in bin/.
func BuildAgentSFX(t testing.TB) string {
	t.Helper()
	name := "agent-sfx"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-o", out, "./cmd/agent-sfx")
	cmd.Dir = RepoRoot(t)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build agent-sfx: %v\n%s", err, b)
	}
	return out
}
