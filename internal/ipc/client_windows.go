//go:build windows

package ipc

import (
	"context"
)

// Send returns ErrUnsupportedPlatform on Windows for Milestone M1.
func Send(ctx context.Context, socketPath string, req Request) (*Response, error) {
	return nil, ErrUnsupportedPlatform
}
