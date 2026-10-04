package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/events"
)

const sampleRate = 44100

// synthTone generates a tone with given frequency, duration, and exponential decay
func synthTone(freq float64, durationSec float64, attackSec float64, decaySec float64, harmonics bool) []int16 {
	numSamples := int(durationSec * float64(sampleRate))
	samples := make([]int16, numSamples)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// Envelope
		var env float64
		if t < attackSec && attackSec > 0 {
			env = t / attackSec
		} else {
			remaining := t - attackSec
			env = math.Exp(-remaining / decaySec)
		}

		// Fundamental + optional subtle harmonics
		val := math.Sin(2 * math.Pi * freq * t)
		if harmonics {
			val = 0.8*val + 0.2*math.Sin(4*math.Pi*freq*t)
		}

		sampleVal := val * env * 24000.0 // Peak around 73% of int16 max to prevent clipping
		if sampleVal > math.MaxInt16 {
			sampleVal = math.MaxInt16
		} else if sampleVal < math.MinInt16 {
			sampleVal = math.MinInt16
		}
		samples[i] = int16(sampleVal)
	}
	return samples
}

// synthSweep generates a linear pitch sweep from startFreq to endFreq
func synthSweep(startFreq, endFreq float64, durationSec float64) []int16 {
	numSamples := int(durationSec * float64(sampleRate))
	samples := make([]int16, numSamples)

	phase := 0.0
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		progress := t / durationSec

		// Current instantaneous frequency
		freq := startFreq + (endFreq-startFreq)*progress

		// Bell-like amplitude window (Hann-like window)
		env := math.Sin(math.Pi * progress)

		phase += 2 * math.Pi * freq / float64(sampleRate)
		val := math.Sin(phase) * env * 22000.0

		if val > math.MaxInt16 {
			val = math.MaxInt16
		} else if val < math.MinInt16 {
			val = math.MinInt16
		}
		samples[i] = int16(val)
	}
	return samples
}

func main() {
	outBase := "sounds"

	// 1. permission_requested: 2-tone melodic chime (A4 440Hz -> C#5 554.37Hz)
	s1 := append(
		synthTone(440.0, 0.10, 0.01, 0.06, true),
		synthTone(554.37, 0.18, 0.01, 0.09, true)...,
	)

	// 2. task_started: ascending chirp sweep (330Hz to 660Hz)
	s2 := synthSweep(330.0, 660.0, 0.18)

	// 3. task_finished: crisp major triad (C5 523Hz -> E5 659Hz -> G5 784Hz)
	s3 := append(
		synthTone(523.25, 0.07, 0.005, 0.04, false),
		append(
			synthTone(659.25, 0.07, 0.005, 0.04, false),
			synthTone(783.99, 0.18, 0.005, 0.08, true)...,
		)...,
	)

	// 4. waiting_for_user: gentle double pulse (698.46Hz ping, 40ms silence, 698.46Hz ping)
	silence40ms := make([]int16, int(0.04*float64(sampleRate)))
	s4 := append(
		synthTone(698.46, 0.05, 0.005, 0.03, false),
		append(silence40ms, synthTone(698.46, 0.12, 0.005, 0.06, true)...)...,
	)

	// 5. tests_passed: triumphant ascending fanfare (G4 -> C5 -> E5 -> G5)
	s5 := append(
		synthTone(392.00, 0.05, 0.005, 0.03, false),
		append(
			synthTone(523.25, 0.05, 0.005, 0.03, false),
			append(
				synthTone(659.25, 0.05, 0.005, 0.03, false),
				synthTone(783.99, 0.18, 0.005, 0.08, true)...,
			)...,
		)...,
	)

	// 6. usage_exhausted: gentle descending chime (E5 659Hz -> C5 523Hz)
	s6 := append(
		synthTone(659.25, 0.09, 0.01, 0.05, true),
		synthTone(523.25, 0.18, 0.01, 0.09, true)...,
	)

	// 7. error: low dual-tone caution chord (220Hz + 233.08Hz)
	numSamplesErr := int(0.22 * float64(sampleRate))
	s7 := make([]int16, numSamplesErr)
	for i := 0; i < numSamplesErr; i++ {
		t := float64(i) / float64(sampleRate)
		env := math.Exp(-t / 0.08)
		val := (0.5*math.Sin(2*math.Pi*220.0*t) + 0.5*math.Sin(2*math.Pi*233.08*t)) * env * 22000.0
		s7[i] = int16(val)
	}

	soundMap := map[events.EventKind][]int16{
		events.EventPermissionRequested: s1,
		events.EventTaskStarted:         s2,
		events.EventTaskFinished:        s3,
		events.EventWaitingForUser:      s4,
		events.EventTestsPassed:         s5,
		events.EventUsageExhausted:      s6,
		events.EventError:               s7,
	}

	for kind, samples := range soundMap {
		dir := filepath.Join(outBase, string(kind))
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create directory %s: %v\n", dir, err)
			os.Exit(1)
		}
		wavBytes := audio.EncodePCM16WAV(1, sampleRate, samples)
		filePath := filepath.Join(dir, fmt.Sprintf("%s.wav", kind))
		if err := os.WriteFile(filePath, wavBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Printf("Generated %s (%d bytes, %d samples)\n", filePath, len(wavBytes), len(samples))
	}
}
