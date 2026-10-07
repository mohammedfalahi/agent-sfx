//go:build windows

package audio

import (
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsPlayerCmdPreparation(t *testing.T) {
	cmd := exec.Command("powershell.exe", WindowsPowerShellArgs()...)
	testPath := `C:\Users\Tester\Special $Path\sound.wav`

	prepareWindowsPlayerCmd(cmd, testPath)

	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be configured on Windows")
	}

	if cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Errorf("expected CREATE_NO_WINDOW flag to be set in SysProcAttr.CreationFlags, got: %x", cmd.SysProcAttr.CreationFlags)
	}

	found := false
	expectedPrefix := WindowsPlayPathEnv + "="
	for _, env := range cmd.Env {
		if strings.HasPrefix(env, expectedPrefix) {
			found = true
			val := strings.TrimPrefix(env, expectedPrefix)
			if val != testPath {
				t.Errorf("expected env value %q, got %q", testPath, val)
			}
			break
		}
	}

	if !found {
		t.Errorf("expected %s to be present in cmd.Env", WindowsPlayPathEnv)
	}
}
