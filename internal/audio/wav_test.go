package audio_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agent-sfx/internal/audio"
)

// Helper to create a valid test PCM16 WAV byte slice
func createTestWAV(numChannels uint16, sampleRate uint32, samples []int16) []byte {
	return audio.EncodePCM16WAV(numChannels, sampleRate, samples)
}

func TestValidateAndParseWAV_Valid(t *testing.T) {
	samples := []int16{0, 1000, -1000, 2000, -2000}
	wavData := createTestWAV(1, 44100, samples)

	r := bytes.NewReader(wavData)
	info, err := audio.ValidateAndParseWAV(r, int64(len(wavData)))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if info.NumChannels != 1 {
		t.Errorf("expected 1 channel, got %d", info.NumChannels)
	}
	if info.SampleRate != 44100 {
		t.Errorf("expected sample rate 44100, got %d", info.SampleRate)
	}
	if info.BitsPerSample != 16 {
		t.Errorf("expected 16 bits per sample, got %d", info.BitsPerSample)
	}
	if info.DataSize != uint32(len(samples)*2) {
		t.Errorf("expected data size %d, got %d", len(samples)*2, info.DataSize)
	}
}

func TestValidateClipDuration_FifteenSeconds(t *testing.T) {
	// 1. Exactly 15 seconds at 44100 Hz = 661,500 samples
	sampleCount15s := 44100 * 15
	samples15s := make([]int16, sampleCount15s)
	wavData15s := createTestWAV(1, 44100, samples15s)
	info15s, err := audio.ValidateAndParseWAV(bytes.NewReader(wavData15s), int64(len(wavData15s)))
	if err != nil {
		t.Fatalf("unexpected error on 15s WAV: %v", err)
	}

	if err := info15s.ValidateClipDuration(15000); err != nil {
		t.Errorf("exactly 15s clip must be accepted, got: %v", err)
	}

	// 2. Slightly over 15 seconds: 1 extra sample (661,501 samples = 15000.02ms)
	samplesSlightlyOver := make([]int16, sampleCount15s+1)
	wavDataSlightlyOver := createTestWAV(1, 44100, samplesSlightlyOver)
	infoSlightlyOver, err := audio.ValidateAndParseWAV(bytes.NewReader(wavDataSlightlyOver), int64(len(wavDataSlightlyOver)))
	if err != nil {
		t.Fatalf("unexpected error on slightly over WAV: %v", err)
	}

	if err := infoSlightlyOver.ValidateClipDuration(15000); err == nil {
		t.Errorf("clip slightly over 15s must be rejected, but was accepted (duration: %dms)", infoSlightlyOver.DurationMS)
	}

	// 3. 16 seconds: 44100 * 16 = 705,600 samples
	sampleCount16s := 44100 * 16
	samples16s := make([]int16, sampleCount16s)
	wavData16s := createTestWAV(1, 44100, samples16s)
	info16s, err := audio.ValidateAndParseWAV(bytes.NewReader(wavData16s), int64(len(wavData16s)))
	if err != nil {
		t.Fatalf("unexpected error on 16s WAV: %v", err)
	}

	if err := info16s.ValidateClipDuration(15000); err == nil {
		t.Errorf("16s clip must be rejected under 15000ms limit")
	}
}

func TestComputePlayerTimeout(t *testing.T) {
	tests := []struct {
		clipMS   int64
		expected time.Duration
	}{
		// 15-second clip -> 15s + 5s = 20s
		{15000, 20 * time.Second},
		// 6-second clip -> 6s + 5s = 11s
		{6000, 11 * time.Second},
		// 200ms short clip -> 0.2s + 5s = 5.2s
		{200, 5200 * time.Millisecond},
		// 0ms clip -> 0s + 5s = 5s
		{0, 5 * time.Second},
	}

	for _, tt := range tests {
		got := audio.ComputePlayerTimeout(tt.clipMS)
		if got != tt.expected {
			t.Errorf("ComputePlayerTimeout(%d) = %v, want %v", tt.clipMS, got, tt.expected)
		}
	}
}

func TestValidateAndParseWAV_Stereo(t *testing.T) {
	samples := []int16{100, -100, 200, -200}
	wavData := createTestWAV(2, 48000, samples)

	r := bytes.NewReader(wavData)
	info, err := audio.ValidateAndParseWAV(r, int64(len(wavData)))
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if info.NumChannels != 2 {
		t.Errorf("expected 2 channels, got %d", info.NumChannels)
	}
	if info.SampleRate != 48000 {
		t.Errorf("expected sample rate 48000, got %d", info.SampleRate)
	}
}

func TestValidateAndParseWAV_InvalidHeaders(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{
			name:    "too small",
			data:    []byte("RIFF1234"),
			wantErr: "too small",
		},
		{
			name:    "not RIFF",
			data:    append([]byte("FORM0000WAVE"), make([]byte, 36)...),
			wantErr: "invalid magic",
		},
		{
			name:    "not WAVE",
			data:    append([]byte("RIFF0000AIFF"), make([]byte, 36)...),
			wantErr: "invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bytes.NewReader(tt.data)
			_, err := audio.ValidateAndParseWAV(r, int64(len(tt.data)))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
		})
	}
}

func TestValidateAndParseWAV_UnsupportedFormat(t *testing.T) {
	wavData := createTestWAV(1, 44100, []int16{0, 100})
	binary.LittleEndian.PutUint16(wavData[20:22], 3) // IEEE float

	r := bytes.NewReader(wavData)
	_, err := audio.ValidateAndParseWAV(r, int64(len(wavData)))
	if err == nil {
		t.Fatalf("expected error for non-PCM format, got nil")
	}
}

func TestValidateAndParseWAV_UnsupportedBits(t *testing.T) {
	wavData := createTestWAV(1, 44100, []int16{0, 100})
	binary.LittleEndian.PutUint16(wavData[34:36], 24)

	r := bytes.NewReader(wavData)
	_, err := audio.ValidateAndParseWAV(r, int64(len(wavData)))
	if err == nil {
		t.Fatalf("expected error for 24-bit format, got nil")
	}
}

func TestScaleWAVVolume(t *testing.T) {
	initialSamples := []int16{0, 10000, -10000, math.MaxInt16, math.MinInt16}
	wavData := createTestWAV(1, 44100, initialSamples)

	// Test scaling to 0.0 (mute)
	scaledMute, err := audio.ScaleWAVVolume(bytes.NewReader(wavData), int64(len(wavData)), 0.0)
	if err != nil {
		t.Fatalf("ScaleWAVVolume(0.0) error: %v", err)
	}

	infoMute, err := audio.ValidateAndParseWAV(bytes.NewReader(scaledMute), int64(len(scaledMute)))
	if err != nil {
		t.Fatalf("scaled mute WAV invalid: %v", err)
	}

	for i := 0; i < len(initialSamples); i++ {
		sample := int16(binary.LittleEndian.Uint16(scaledMute[infoMute.DataOffset+int64(i*2) : infoMute.DataOffset+int64(i*2)+2]))
		if sample != 0 {
			t.Errorf("sample %d at volume 0.0 should be 0, got %d", i, sample)
		}
	}

	// Test scaling to 0.5
	scaledHalf, err := audio.ScaleWAVVolume(bytes.NewReader(wavData), int64(len(wavData)), 0.5)
	if err != nil {
		t.Fatalf("ScaleWAVVolume(0.5) error: %v", err)
	}

	infoHalf, err := audio.ValidateAndParseWAV(bytes.NewReader(scaledHalf), int64(len(scaledHalf)))
	if err != nil {
		t.Fatalf("scaled half WAV invalid: %v", err)
	}

	sample1 := int16(binary.LittleEndian.Uint16(scaledHalf[infoHalf.DataOffset+2 : infoHalf.DataOffset+4]))
	if sample1 != 5000 {
		t.Errorf("sample at index 1 should be 5000, got %d", sample1)
	}
}

func TestAudioCache(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := audio.NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache error: %v", err)
	}

	// Create a real WAV file on disk
	wavPath := filepath.Join(tmpDir, "source.wav")
	sampleData := createTestWAV(1, 44100, []int16{1000, 2000, -2000})
	if err := os.WriteFile(wavPath, sampleData, 0644); err != nil {
		t.Fatalf("failed to write source WAV: %v", err)
	}

	// Volume = 1.0 returns original path
	path1, err := cache.GetOrScale(wavPath, 1.0)
	if err != nil {
		t.Fatalf("GetOrScale(1.0) error: %v", err)
	}
	if path1 != wavPath {
		t.Errorf("expected original path for volume 1.0, got %s", path1)
	}

	// Volume = 0.5 creates cached scaled file
	pathScaled, err := cache.GetOrScale(wavPath, 0.5)
	if err != nil {
		t.Fatalf("GetOrScale(0.5) error: %v", err)
	}
	if pathScaled == wavPath {
		t.Errorf("expected scaled cached path, got original")
	}

	// Second call returns existing cached file
	pathScaled2, err := cache.GetOrScale(wavPath, 0.5)
	if err != nil {
		t.Fatalf("GetOrScale(0.5) repeated error: %v", err)
	}
	if pathScaled2 != pathScaled {
		t.Errorf("expected cache hit %s, got %s", pathScaled, pathScaled2)
	}
}

func TestFakePlayer(t *testing.T) {
	player := audio.NewFakePlayer()
	if !player.IsAvailable() {
		t.Errorf("FakePlayer should be available")
	}

	ctx := context.Background()
	if err := player.Play(ctx, "/path/to/sound.wav"); err != nil {
		t.Fatalf("FakePlayer.Play failed: %v", err)
	}

	calls := player.Calls()
	if len(calls) != 1 || calls[0] != "/path/to/sound.wav" {
		t.Errorf("unexpected player calls: %v", calls)
	}

	// Test error injection
	player.PlayFunc = func(ctx context.Context, wavPath string) error {
		return context.DeadlineExceeded
	}
	if err := player.Play(ctx, "/path/to/sound2.wav"); err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestPlaybackTimeoutCancellation(t *testing.T) {
	player := audio.NewFakePlayer()
	player.PlayFunc = func(ctx context.Context, wavPath string) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := player.Play(ctx, "test.wav")
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}
