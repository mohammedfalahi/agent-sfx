package events_test

import (
	"testing"
	"time"

	"agent-sfx/internal/events"
)

func TestAllEventsCompleteness(t *testing.T) {
	if len(events.AllEvents) != 7 {
		t.Fatalf("expected exactly 7 events, got %d", len(events.AllEvents))
	}

	expected := map[events.EventKind]bool{
		events.EventPermissionRequested: true,
		events.EventTaskStarted:         true,
		events.EventTaskFinished:        true,
		events.EventWaitingForUser:      true,
		events.EventTestsPassed:         true,
		events.EventUsageExhausted:      true,
		events.EventError:               true,
	}

	for _, k := range events.AllEvents {
		if !expected[k] {
			t.Errorf("unexpected event in AllEvents: %s", k)
		}
		if !k.IsValid() {
			t.Errorf("event %s should be valid", k)
		}
	}
}

func TestEventPriorityOrder(t *testing.T) {
	// usage_exhausted > error > permission_requested > waiting_for_user > tests_passed > task_finished > task_started
	ordered := []events.EventKind{
		events.EventUsageExhausted,
		events.EventError,
		events.EventPermissionRequested,
		events.EventWaitingForUser,
		events.EventTestsPassed,
		events.EventTaskFinished,
		events.EventTaskStarted,
	}

	for i := 0; i < len(ordered)-1; i++ {
		higher := ordered[i]
		lower := ordered[i+1]
		if higher.Priority() <= lower.Priority() {
			t.Errorf("expected %s (priority %d) > %s (priority %d)",
				higher, higher.Priority(), lower, lower.Priority())
		}
	}
}

func TestEventValidation(t *testing.T) {
	valid := events.Event{
		Kind:       events.EventTaskStarted,
		Agent:      "gemini",
		ObservedAt: time.Now(),
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid event, got: %v", err)
	}

	invalidKind := events.Event{
		Kind:  events.EventKind("unknown"),
		Agent: "gemini",
	}
	if err := invalidKind.Validate(); err == nil {
		t.Fatalf("expected error on unknown kind, got nil")
	}

	missingAgent := events.Event{
		Kind: events.EventTaskFinished,
	}
	if err := missingAgent.Validate(); err == nil {
		t.Fatalf("expected error on missing agent, got nil")
	}
}

func TestParseEventKind(t *testing.T) {
	tests := []struct {
		input   string
		want    events.EventKind
		wantErr bool
	}{
		{"permission_requested", events.EventPermissionRequested, false},
		{"task_started", events.EventTaskStarted, false},
		{"task_finished", events.EventTaskFinished, false},
		{"waiting_for_user", events.EventWaitingForUser, false},
		{"tests_passed", events.EventTestsPassed, false},
		{"usage_exhausted", events.EventUsageExhausted, false},
		{"error", events.EventError, false},
		{"invalid_event", "", true},
		{"", "", true},
		{"TASK_STARTED", "", true},
		{"ignore", "", true},
	}

	for _, tt := range tests {
		got, err := events.ParseEventKind(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseEventKind(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseEventKind(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
