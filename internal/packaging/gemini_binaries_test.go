package packaging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agent-sfx/internal/testutil"
)

// TestGeminiBinaries_ShippedAndCurrent guards the Gemini extension and npm
// package: their committed binaries must exist, be executable, and report the
// same version as gemini-extension.json and package.json.
func TestGeminiBinaries_ShippedAndCurrent(t *testing.T) {
	npm := filepath.Join(testutil.RepoRoot(t), "npm")
	var ext, pkg struct {
		Version string `json:"version"`
	}
	readJSON(t, filepath.Join(npm, "gemini-extension.json"), &ext)
	readJSON(t, filepath.Join(npm, "package.json"), &pkg)
	if ext.Version == "" || ext.Version != pkg.Version {
		t.Errorf("gemini-extension.json version %q and package.json version %q must match", ext.Version, pkg.Version)
	}

	for _, arch := range []string{"darwin-arm64", "darwin-x64"} {
		bin := filepath.Join(npm, "bin", arch, "agent-sfx")
		fi, err := os.Stat(bin)
		if err != nil {
			t.Errorf("missing npm/bin/%s/agent-sfx (run scripts/build-gemini-binaries.sh)", arch)
			continue
		}
		if fi.Mode()&0111 == 0 {
			t.Errorf("npm/bin/%s/agent-sfx is not executable", arch)
		}
	}

	if runtime.GOOS != "darwin" {
		return
	}
	arch := "darwin-arm64"
	if runtime.GOARCH == "amd64" {
		arch = "darwin-x64"
	}
	out, err := exec.Command(filepath.Join(npm, "bin", arch, "agent-sfx"), "help").Output()
	if err != nil {
		t.Fatalf("Gemini binary for this Mac did not run: %v", err)
	}
	if !strings.Contains(string(out), "(v"+ext.Version) {
		first, _, _ := strings.Cut(string(out), "\n")
		t.Errorf("Gemini binary reports %q, want version %s (rebuild with scripts/build-gemini-binaries.sh)", first, ext.Version)
	}
}
