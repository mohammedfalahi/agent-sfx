//go:build windows

package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/Microsoft/go-winio"
)

// Send communicates synchronously with the worker via the Windows named pipe.
// It enforces time bounds (DefaultSendTimeout if ctx has no deadline) and message size bounds (8 KiB).
func Send(ctx context.Context, socketPath string, req Request) (*Response, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	if len(payload) > MaxMessageSizeBytes {
		return nil, ErrOversizedMessage
	}

	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultSendTimeout)
		defer cancel()
		deadline = time.Now().Add(DefaultSendTimeout)
	}

	conn, err := winio.DialPipeContext(ctx, socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to worker pipe at %s: %w", socketPath, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	// Send message with newline delimiter
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write to worker pipe: %w", err)
	}

	// Read response (bounded up to 8 KiB)
	reader := bufio.NewReader(io.LimitReader(conn, MaxMessageSizeBytes+1))
	respLine, err := reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read worker response: %w", err)
	}

	if len(respLine) > MaxMessageSizeBytes {
		return nil, ErrOversizedMessage
	}

	if len(respLine) == 0 {
		return nil, fmt.Errorf("empty response received from worker")
	}

	var resp Response
	if err := json.Unmarshal(respLine, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode worker response: %w", err)
	}

	return &resp, nil
}
