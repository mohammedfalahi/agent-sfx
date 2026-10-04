//go:build !windows

package ipc

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	// MaxSafeUnixSocketPathLength stays defensively under macOS 104-byte limit
	MaxSafeUnixSocketPathLength = 95
)

// EnsureSecureDir verifies that dir exists, is not a symlink, is owned by current user,
// and has restricted permissions (0700).
func EnsureSecureDir(dir string) error {
	fi, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
			fi, err = os.Lstat(dir)
			if err != nil {
				return fmt.Errorf("failed to stat created directory %s: %w", dir, err)
			}
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("insecure directory %s: symlinks are not permitted", dir)
	}

	if !fi.IsDir() {
		return fmt.Errorf("path %s is not a directory", dir)
	}

	stat, ok := fi.Sys().(*syscall.Stat_t)
	if ok && stat.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("insecure directory %s: owned by uid %d, expected uid %d", dir, stat.Uid, os.Getuid())
	}

	// Ensure permissions are 0700 (owner only)
	if fi.Mode().Perm() != 0700 {
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("failed to enforce 0700 permissions on %s: %w", dir, err)
		}
	}

	return nil
}

// ResolveSocketPath determines the active Unix domain socket path, falling back
// to a short /tmp path if the standard OS user-config path exceeds Unix limits.
func ResolveSocketPath() (string, error) {
	// Support explicit override via environment variable for isolated testing
	if custom := os.Getenv("AGENT_SFX_SOCKET_PATH"); custom != "" {
		if err := EnsureSecureDir(filepath.Dir(custom)); err != nil {
			return "", fmt.Errorf("insecure custom socket directory: %w", err)
		}
		if len(custom) > MaxSafeUnixSocketPathLength {
			return "", fmt.Errorf("custom socket path length %d exceeds safe maximum of %d: %s",
				len(custom), MaxSafeUnixSocketPathLength, custom)
		}
		return custom, nil
	}

	var preferredDir string
	if configDir, err := os.UserConfigDir(); err == nil {
		preferredDir = filepath.Join(configDir, "agent-sfx")
	}

	uid := os.Getuid()
	shortDir := filepath.Join("/tmp", fmt.Sprintf("asfx-%d", uid))

	// Check preferred path
	if preferredDir != "" {
		candidate := filepath.Join(preferredDir, "worker.sock")
		if len(candidate) <= MaxSafeUnixSocketPathLength {
			if err := EnsureSecureDir(preferredDir); err == nil {
				return candidate, nil
			}
		}
	}

	// Fallback to short /tmp path
	if err := EnsureSecureDir(shortDir); err != nil {
		return "", fmt.Errorf("failed to secure fallback socket directory %s: %w", shortDir, err)
	}

	shortPath := filepath.Join(shortDir, "s.sock")
	if len(shortPath) > MaxSafeUnixSocketPathLength {
		return "", fmt.Errorf("socket path length %d exceeds safe maximum of %d: %s",
			len(shortPath), MaxSafeUnixSocketPathLength, shortPath)
	}

	return shortPath, nil
}

// SocketDir returns the directory holding the socket.
func SocketDir(socketPath string) string {
	return filepath.Dir(socketPath)
}
