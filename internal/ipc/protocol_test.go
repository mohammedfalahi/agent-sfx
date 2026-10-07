package ipc_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
)

func TestIPCProtocol_Validation(t *testing.T) {
	// Valid event
	validReq := ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event: &events.Event{
			Kind:       events.EventTaskStarted,
			Agent:      "gemini",
			ObservedAt: time.Now(),
		},
	}
	if err := validReq.Validate(); err != nil {
		t.Fatalf("unexpected error on valid request: %v", err)
	}

	// Invalid version
	badVer := validReq
	badVer.Version = 99
	if err := badVer.Validate(); err == nil {
		t.Fatalf("expected error on version mismatch, got nil")
	}

	// Event missing
	missingEv := validReq
	missingEv.Event = nil
	if err := missingEv.Validate(); err == nil {
		t.Fatalf("expected error on missing event, got nil")
	}

	// Unknown message type
	badType := validReq
	badType.Type = "invalid_type"
	if err := badType.Validate(); err == nil {
		t.Fatalf("expected error on unknown message type, got nil")
	}

	// Stop & status
	stopReq := ipc.Request{Version: ipc.ProtocolVersion, Type: ipc.TypeStop}
	if err := stopReq.Validate(); err != nil {
		t.Fatalf("unexpected error on stop request: %v", err)
	}

	statusReq := ipc.Request{Version: ipc.ProtocolVersion, Type: ipc.TypeStatus}
	if err := statusReq.Validate(); err != nil {
		t.Fatalf("unexpected error on status request: %v", err)
	}

	enableReq := ipc.Request{Version: ipc.ProtocolVersion, Type: ipc.TypeEnable}
	if err := enableReq.Validate(); err != nil {
		t.Fatalf("unexpected error on enable request: %v", err)
	}

	disableReq := ipc.Request{Version: ipc.ProtocolVersion, Type: ipc.TypeDisable}
	if err := disableReq.Validate(); err != nil {
		t.Fatalf("unexpected error on disable request: %v", err)
	}
}

func TestIPCProtocol_FramingLimits(t *testing.T) {
	// Verify MaxMessageSizeBytes is 8 KiB
	if ipc.MaxMessageSizeBytes != 8192 {
		t.Errorf("expected MaxMessageSizeBytes to be 8192, got %d", ipc.MaxMessageSizeBytes)
	}

	// Serialization exceeding 8 KiB
	hugeEvent := &events.Event{
		Kind:       events.EventTaskStarted,
		Agent:      strings.Repeat("X", 9000),
		ObservedAt: time.Now(),
	}
	req := ipc.Request{
		Version: ipc.ProtocolVersion,
		Type:    ipc.TypeEvent,
		Event:   hugeEvent,
	}

	payload, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	if len(payload) <= ipc.MaxMessageSizeBytes {
		t.Fatalf("expected payload to exceed %d bytes, got %d", ipc.MaxMessageSizeBytes, len(payload))
	}
}
