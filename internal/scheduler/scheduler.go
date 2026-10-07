package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/sounds"
)

const (
	CoalesceWindow = 100 * time.Millisecond
	DedupeTTL      = 5 * time.Second
)

type controlCmd struct {
	enabled bool
	resp    chan struct{}
}

// Scheduler coordinates playback state, deduplication, cooldown, and priority coalescing.
type Scheduler struct {
	mu        sync.Mutex
	cfg       config.Config
	clock     Clock
	player    audio.Player
	cache     *audio.Cache
	selector  *sounds.Selector
	soundsDir string

	playing       bool
	cooldownUntil time.Time
	dedupeCache   map[string]time.Time
	pendingEvent  *events.Event
	pendingTimer  <-chan time.Time
	cancelPlay    context.CancelFunc

	eventCh   chan events.Event
	controlCh chan controlCmd
	quit      chan struct{}
	done      chan struct{}
}

// New creates a Scheduler ready to run.
func New(
	cfg config.Config,
	clock Clock,
	player audio.Player,
	cache *audio.Cache,
	selector *sounds.Selector,
	soundsDir string,
) *Scheduler {
	if clock == nil {
		clock = RealClock{}
	}
	return &Scheduler{
		cfg:         cfg,
		clock:       clock,
		player:      player,
		cache:       cache,
		selector:    selector,
		soundsDir:   soundsDir,
		dedupeCache: make(map[string]time.Time),
		eventCh:     make(chan events.Event, 32),
		controlCh:   make(chan controlCmd),
		quit:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Start launches the background scheduler event loop.
func (s *Scheduler) Start() {
	go s.loop()
}

// Stop terminates the scheduler and cleans up active playback.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	select {
	case <-s.quit:
		s.mu.Unlock()
		return
	default:
		close(s.quit)
		if s.cancelPlay != nil {
			s.cancelPlay()
		}
	}
	s.mu.Unlock()
	<-s.done
}

// Enqueue submits an event for scheduling. It is non-blocking: if the queue is full,
// the event is dropped.
func (s *Scheduler) Enqueue(ev events.Event) bool {
	select {
	case s.eventCh <- ev:
		return true
	default:
		return false
	}
}

// IsPlaying reports if audio is actively playing or in cooldown.
func (s *Scheduler) IsPlaying() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	return s.playing || now.Before(s.cooldownUntil)
}

// IsEnabled reports whether the scheduler currently allows sound playback.
func (s *Scheduler) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg.Enabled
}

// SetEnabled updates the scheduler's enabled state through its owning execution loop.
// When disabling, any active audio playback is immediately canceled and pending
// events are cleared. It waits for the loop to apply the changes before returning.
func (s *Scheduler) SetEnabled(enabled bool) {
	if !enabled {
		s.mu.Lock()
		if s.cancelPlay != nil {
			s.cancelPlay()
		}
		s.mu.Unlock()
	}

	resp := make(chan struct{})
	cmd := controlCmd{
		enabled: enabled,
		resp:    resp,
	}

	select {
	case <-s.done:
		return
	case s.controlCh <- cmd:
	}

	select {
	case <-s.done:
		return
	case <-resp:
	}
}

func (s *Scheduler) loop() {
	defer close(s.done)

	for {
		select {
		case <-s.quit:
			return

		case cmd := <-s.controlCh:
			s.handleControl(cmd)

		case ev := <-s.eventCh:
			s.handleArrivedEvent(ev)

		case <-s.pendingTimer:
			s.triggerPending()
		}
	}
}

func (s *Scheduler) handleControl(cmd controlCmd) {
	s.mu.Lock()
	s.cfg.Enabled = cmd.enabled
	if !cmd.enabled {
		s.pendingEvent = nil
		s.pendingTimer = nil
		s.cooldownUntil = time.Time{}
		// Drain any events that arrived in eventCh
		for {
			select {
			case <-s.eventCh:
			default:
				goto drained
			}
		}
	drained:
	}
	s.mu.Unlock()

	close(cmd.resp)
}

func (s *Scheduler) handleArrivedEvent(ev events.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock.Now()

	// 1. Config check
	if !s.cfg.IsEventEnabled(ev.Kind) {
		return
	}

	// 2. Scoped Deduplication
	dedupeKey := s.buildDedupeKey(ev)
	if expiry, exists := s.dedupeCache[dedupeKey]; exists && now.Before(expiry) {
		// Duplicate event; drop
		return
	}
	s.dedupeCache[dedupeKey] = now.Add(DedupeTTL)
	s.cleanOldDedupeEntries(now)

	// 3. Active playback / Cooldown gate
	// In v1, there is NO preemption. Drop arriving events if playing or cooling down.
	if s.playing || now.Before(s.cooldownUntil) {
		return
	}

	// 4. Coalescing window
	if s.pendingEvent == nil {
		// First event in window: start coalescing timer
		s.pendingEvent = &ev
		s.pendingTimer = s.clock.After(CoalesceWindow)
	} else {
		// Event already pending: apply priority coalescing
		// Higher priority replaces lower priority; lower or equal priority is dropped.
		if ev.Kind.Priority() > s.pendingEvent.Kind.Priority() {
			s.pendingEvent = &ev
		}
	}
}

func (s *Scheduler) triggerPending() {
	s.mu.Lock()
	ev := s.pendingEvent
	s.pendingEvent = nil
	s.pendingTimer = nil

	if ev == nil {
		s.mu.Unlock()
		return
	}

	now := s.clock.Now()
	if s.playing || now.Before(s.cooldownUntil) {
		s.mu.Unlock()
		return
	}

	s.playing = true
	s.mu.Unlock()

	// Perform playback outside lock
	s.playEvent(*ev)

	s.mu.Lock()
	s.playing = false
	if s.cfg.Enabled {
		cooldownDuration := time.Duration(s.cfg.CooldownMS) * time.Millisecond
		s.cooldownUntil = s.clock.Now().Add(cooldownDuration)
	} else {
		s.cooldownUntil = time.Time{}
	}
	s.mu.Unlock()
}

func (s *Scheduler) playEvent(ev events.Event) {
	chosenSound, err := s.selector.SelectSound(s.soundsDir, ev.Kind)
	if err != nil {
		return
	}

	info, err := audio.ValidateWAVFile(chosenSound)
	if err != nil {
		return
	}

	// Validate actual PCM duration against configured max_clip_ms
	if err := info.ValidateClipDuration(s.cfg.MaxClipMS); err != nil {
		return
	}

	scaledPath, err := s.cache.GetOrScale(chosenSound, s.cfg.Volume)
	if err != nil {
		return
	}

	timeout := audio.ComputePlayerTimeout(info.DurationMS)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	s.mu.Lock()
	s.cancelPlay = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.cancelPlay = nil
		s.mu.Unlock()
		cancel()
	}()

	_ = s.player.Play(ctx, scaledPath)
}

func (s *Scheduler) buildDedupeKey(ev events.Event) string {
	if ev.DedupeKey != "" {
		return fmt.Sprintf("%s:%s:%s:%s", ev.Agent, ev.SessionID, ev.Kind, ev.DedupeKey)
	}
	if ev.SessionID != "" {
		return fmt.Sprintf("%s:%s:%s", ev.Agent, ev.SessionID, ev.Kind)
	}
	return fmt.Sprintf("%s:%s", ev.Agent, ev.Kind)
}

func (s *Scheduler) cleanOldDedupeEntries(now time.Time) {
	for k, exp := range s.dedupeCache {
		if now.After(exp) {
			delete(s.dedupeCache, k)
		}
	}
}
