package ipc

import (
	"errors"
	"fmt"
	"time"

	"agent-sfx/internal/events"
)

const (
	ProtocolVersion     = 1
	MaxMessageSizeBytes = 8192 // 8 KiB
	DefaultSendTimeout  = 25 * time.Millisecond
)

var (
	ErrOversizedMessage    = errors.New("ipc message exceeds maximum size of 8 KiB")
	ErrUnsupportedPlatform = errors.New("ipc worker is currently unsupported on this operating system")
	ErrInvalidProtocol     = errors.New("unsupported ipc protocol version")
)

type MessageType string

const (
	TypeEvent  MessageType = "event"
	TypeStop   MessageType = "stop"
	TypeStatus MessageType = "status"
)

// Request is sent from clients (hook or CLI) to the background worker.
type Request struct {
	Version int           `json:"version"`
	Type    MessageType   `json:"type"`
	Event   *events.Event `json:"event,omitempty"`
}

// Response is sent back by the worker.
type Response struct {
	Version int    `json:"version"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Uptime  int64  `json:"uptime_sec,omitempty"`
	Playing bool   `json:"playing,omitempty"`
}

// Validate checks request invariants.
func (r *Request) Validate() error {
	if r.Version != ProtocolVersion {
		return fmt.Errorf("%w: got %d, expected %d", ErrInvalidProtocol, r.Version, ProtocolVersion)
	}
	switch r.Type {
	case TypeEvent:
		if r.Event == nil {
			return errors.New("event payload missing for event request")
		}
		return r.Event.Validate()
	case TypeStop, TypeStatus:
		return nil
	default:
		return fmt.Errorf("unknown ipc request type: %q", r.Type)
	}
}
