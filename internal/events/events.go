package events

import (
	"fmt"
	"slices"
	"time"
)

// EventKind represents one of the canonical seven agent event types.
// It is a closed enumeration; no other events produce sounds.
type EventKind string

const (
	EventPermissionRequested EventKind = "permission_requested"
	EventTaskStarted         EventKind = "task_started"
	EventTaskFinished        EventKind = "task_finished"
	EventWaitingForUser      EventKind = "waiting_for_user"
	EventTestsPassed         EventKind = "tests_passed"
	EventUsageExhausted      EventKind = "usage_exhausted"
	EventError               EventKind = "error"
)

// AllEvents lists the seven canonical product events in fixed order.
var AllEvents = []EventKind{
	EventPermissionRequested,
	EventTaskStarted,
	EventTaskFinished,
	EventWaitingForUser,
	EventTestsPassed,
	EventUsageExhausted,
	EventError,
}

// Priority values for coalescing pending events (higher number = higher priority).
// Coalesce order from architecture.md:
// usage_exhausted > error > permission_requested > waiting_for_user > tests_passed > task_finished > task_started
var eventPriorities = map[EventKind]int{
	EventUsageExhausted:      7,
	EventError:               6,
	EventPermissionRequested: 5,
	EventWaitingForUser:      4,
	EventTestsPassed:         3,
	EventTaskFinished:        2,
	EventTaskStarted:         1,
}

// Priority returns the priority score for the given event kind.
func (k EventKind) Priority() int {
	return eventPriorities[k]
}

// Event represents a canonical normalized event across agent adapters.
type Event struct {
	Kind       EventKind `json:"kind"`
	Agent      string    `json:"agent"`
	SessionID  string    `json:"session_id,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	DedupeKey  string    `json:"dedupe_key,omitempty"`
	ReasonCode string    `json:"reason_code,omitempty"`
}

// Validate checks that the Event has a valid Kind and non-empty Agent.
func (e *Event) Validate() error {
	if !e.Kind.IsValid() {
		return fmt.Errorf("invalid event kind: %q", e.Kind)
	}
	if e.Agent == "" {
		return fmt.Errorf("missing agent name in event")
	}
	return nil
}

// ParseEventKind parses a raw string into an EventKind.
// Returns an error if the raw string is not one of the seven canonical events.
func ParseEventKind(raw string) (EventKind, error) {
	k := EventKind(raw)
	if !slices.Contains(AllEvents, k) {
		return "", fmt.Errorf("unknown event kind %q: must be one of %v", raw, AllEvents)
	}
	return k, nil
}

// IsValid checks whether an EventKind is valid.
func (k EventKind) IsValid() bool {
	return slices.Contains(AllEvents, k)
}

func (k EventKind) String() string {
	return string(k)
}
