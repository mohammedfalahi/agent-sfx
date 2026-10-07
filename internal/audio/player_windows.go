//go:build windows

package audio

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func prepareWindowsPlayerCmd(cmd *exec.Cmd, wavPath string) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	base := cmd.Env
	if len(base) == 0 {
		base = os.Environ()
	}
	cmd.Env = BuildWindowsPlayerEnv(base, wavPath)
}
