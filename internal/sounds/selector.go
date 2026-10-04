package sounds

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agent-sfx/internal/events"
)

var (
	ErrNoSounds = errors.New("no audio files found for event")
)

// RandSource defines the random number generator interface for testability.
type RandSource interface {
	Intn(n int) int
}

type defaultRand struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newDefaultRand() *defaultRand {
	return &defaultRand{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (d *defaultRand) Intn(n int) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.rng.Intn(n)
}

// Selector manages random sound selection with immediate-repeat avoidance.
type Selector struct {
	mu         sync.Mutex
	rng        RandSource
	lastPlayed map[events.EventKind]string
}

// NewSelector constructs a Selector with the given RNG. If rng is nil, a default thread-safe RNG is used.
func NewSelector(rng RandSource) *Selector {
	if rng == nil {
		rng = newDefaultRand()
	}
	return &Selector{
		rng:        rng,
		lastPlayed: make(map[events.EventKind]string),
	}
}

// ListEventSounds returns all .wav files located directly in <baseDir>/<eventKind>/.
func ListEventSounds(baseDir string, kind events.EventKind) ([]string, error) {
	eventDir := filepath.Join(baseDir, string(kind))
	entries, err := os.ReadDir(eventDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sound directory %s: %w", eventDir, err)
	}

	var wavs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".wav") {
			wavs = append(wavs, filepath.Join(eventDir, name))
		}
	}

	sort.Strings(wavs)
	return wavs, nil
}

// SelectSound picks a random WAV for the given event, ensuring no immediate repeat
// if 2 or more files are available.
func (s *Selector) SelectSound(baseDir string, kind events.EventKind) (string, error) {
	wavs, err := ListEventSounds(baseDir, kind)
	if err != nil {
		return "", err
	}
	if len(wavs) == 0 {
		return "", ErrNoSounds
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(wavs) == 1 {
		s.lastPlayed[kind] = wavs[0]
		return wavs[0], nil
	}

	last := s.lastPlayed[kind]
	candidates := make([]string, 0, len(wavs))
	for _, w := range wavs {
		if w != last {
			candidates = append(candidates, w)
		}
	}

	if len(candidates) == 0 {
		candidates = wavs
	}

	idx := s.rng.Intn(len(candidates))
	chosen := candidates[idx]
	s.lastPlayed[kind] = chosen
	return chosen, nil
}

// LastPlayed returns the last selected file for an event kind.
func (s *Selector) LastPlayed(kind events.EventKind) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPlayed[kind]
}
