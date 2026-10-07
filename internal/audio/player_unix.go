//go:build !windows

package audio

import (
	"os/exec"
)

func prepareWindowsPlayerCmd(cmd *exec.Cmd, wavPath string) {
	// Fallback/stub for cross-platform unit testing
	cmd.Env = BuildWindowsPlayerEnv(cmd.Env, wavPath)
}
