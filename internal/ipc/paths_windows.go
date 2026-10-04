//go:build windows

package ipc

// ResolveSocketPath returns ErrUnsupportedPlatform on Windows for Milestone M1.
func ResolveSocketPath() (string, error) {
	return "", ErrUnsupportedPlatform
}

// SocketDir returns an empty string on Windows.
func SocketDir(socketPath string) string {
	return ""
}
