//go:build windows

package ipc

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ResolveSocketPath determines the active Windows named pipe path.
// The default path is \\.\pipe\agent-sfx-<UserSID>, which is user-scoped.
// It supports AGENT_SFX_SOCKET_PATH for custom/isolated testing.
func ResolveSocketPath() (string, error) {
	if custom := os.Getenv("AGENT_SFX_SOCKET_PATH"); custom != "" {
		if !strings.HasPrefix(custom, `\\.\pipe\`) {
			return "", fmt.Errorf("custom Windows pipe path must begin with \\\\.\\pipe\\, got: %s", custom)
		}
		return custom, nil
	}

	u, err := user.Current()
	if err == nil && u.Uid != "" {
		// Clean any characters not safe for pipe names
		cleanUID := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '_'
		}, u.Uid)
		return fmt.Sprintf(`\\.\pipe\agent-sfx-%s`, cleanUID), nil
	}

	// Fallback to username if SID is unavailable
	username := os.Getenv("USERNAME")
	if username == "" {
		username = "default"
	}
	return fmt.Sprintf(`\\.\pipe\agent-sfx-%s`, username), nil
}

// SocketDir returns the directory containing the user's config and lock file.
// On Windows, named pipes do not live in the filesystem, so SocketDir returns
// the user's agent-sfx configuration directory (%LOCALAPPDATA%\agent-sfx or %USERPROFILE%\AppData\Local\agent-sfx).
func SocketDir(socketPath string) string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "agent-sfx")
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "agent-sfx")
	}
	return filepath.Join(os.Getenv("USERPROFILE"), "agent-sfx")
}
