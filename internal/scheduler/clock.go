package scheduler

import (
	"sync"
	"time"
)

// Clock provides an injectable monotonic time source for scheduling and tests.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
}

// RealClock uses the standard library time package.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

func (RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// FakeClock provides deterministic time control for unit tests.
type FakeClock struct {
	mu      sync.Mutex
	current time.Time
	timers  []fakeTimer
}

type fakeTimer struct {
	triggerAt time.Time
	ch        chan time.Time
}

// NewFakeClock initializes a fake clock at a fixed reference time.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{current: start}
}

func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func (f *FakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- f.current
		return ch
	}

	f.timers = append(f.timers, fakeTimer{
		triggerAt: f.current.Add(d),
		ch:        ch,
	})
	return ch
}

func (f *FakeClock) Sleep(d time.Duration) {
	f.Advance(d)
}

// WaitForTimers blocks until at least n timers are waiting.
func (f *FakeClock) WaitForTimers(n int) {
	for i := 0; i < 50; i++ {
		f.mu.Lock()
		count := len(f.timers)
		f.mu.Unlock()
		if count >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Advance moves fake time forward and fires any matured timers.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.current = f.current.Add(d)
	remaining := make([]fakeTimer, 0, len(f.timers))

	for _, t := range f.timers {
		if !t.triggerAt.After(f.current) {
			select {
			case t.ch <- f.current:
			default:
			}
		} else {
			remaining = append(remaining, t)
		}
	}
	f.timers = remaining
}
