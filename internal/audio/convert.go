package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// ConversionTimeout bounds one afconvert run. Clips are at most 15 s, which
	// converts in well under a second; the margin covers a busy machine.
	ConversionTimeout = 15 * time.Second

	conversionPollInterval = 25 * time.Millisecond
)

// afconvertPath is the macOS system converter. It ships with every Mac.
var afconvertPath = "/usr/bin/afconvert"

// convertibleExts are the formats converted to WAV on macOS.
var convertibleExts = map[string]bool{
	".mp3":  true,
	".m4a":  true,
	".aac":  true,
	".aif":  true,
	".aiff": true,
	".caf":  true,
}

// ErrConversionUnavailable means a non-WAV sound was found on a system with
// no converter (anything but macOS).
var ErrConversionUnavailable = errors.New("converting non-WAV sounds requires macOS (afconvert)")

// Converter decodes the audio file at src and writes a WAV file to dst.
type Converter func(ctx context.Context, src, dst string) error

// ConversionAvailable reports whether non-WAV sounds can be converted here.
func ConversionAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := os.Stat(afconvertPath)
	return err == nil
}

// IsSupportedSoundFile reports whether a file in a sounds folder can be used:
// .wav everywhere, plus MP3, AAC/M4A, AIFF, and CAF where they can be converted.
func IsSupportedSoundFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".wav" {
		return true
	}
	return convertibleExts[ext] && ConversionAvailable()
}

// SetConverter replaces the converter (used by tests).
func (c *Cache) SetConverter(conv Converter) {
	c.converter = conv
}

// ToWAV returns a WAV path for the sound at src. WAV files are returned as
// they are. Other supported formats are converted once into the cache, keyed
// by the source's path, size, and modification time, so replacing the file
// triggers a fresh conversion. A failed conversion is remembered the same way
// and is not retried until the file changes. The source is never modified.
func (c *Cache) ToWAV(src string) (string, error) {
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".wav" {
		return src, nil
	}
	if !convertibleExts[ext] {
		return "", fmt.Errorf("unsupported sound format %q", ext)
	}

	abs, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("failed to stat sound: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("sound %s is not a regular file", src)
	}
	if fi.Size() > MaxWAVSizeBytes {
		return "", fmt.Errorf("sound file is %d bytes, larger than the %d byte limit", fi.Size(), MaxWAVSizeBytes)
	}

	h := sha256.New()
	fmt.Fprintf(h, "convert:%s:%d:%d", abs, fi.ModTime().UnixNano(), fi.Size())
	key := hex.EncodeToString(h.Sum(nil))[:16]
	cachedPath := filepath.Join(c.dir, "converted_"+key+".wav")
	failedPath := filepath.Join(c.dir, "failed_"+key+".txt")

	if _, err := os.Stat(cachedPath); err == nil {
		now := time.Now()
		_ = os.Chtimes(cachedPath, now, now)
		return cachedPath, nil
	}
	if msg, err := os.ReadFile(failedPath); err == nil {
		return "", fmt.Errorf("could not convert %s (cached result; change the file to retry): %s", filepath.Base(src), msg)
	}

	conv := c.converter
	if conv == nil {
		if !ConversionAvailable() {
			return "", ErrConversionUnavailable
		}
		conv = afconvertToWAV
	}

	tmp, err := os.CreateTemp(c.dir, "converting-*.wav.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create conversion file: %w", err)
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	ctx, cancel := context.WithTimeout(context.Background(), ConversionTimeout)
	defer cancel()
	convErr := conv(ctx, abs, tmpPath)
	if convErr == nil {
		if _, err := ValidateWAVFile(tmpPath); err != nil {
			convErr = fmt.Errorf("converted audio is not usable: %w", err)
		}
	}
	if convErr != nil {
		_ = os.WriteFile(failedPath, []byte(convErr.Error()), 0600)
		c.prune()
		return "", fmt.Errorf("could not convert %s: %w", filepath.Base(src), convErr)
	}

	if err := os.Rename(tmpPath, cachedPath); err != nil {
		return "", fmt.Errorf("failed to store converted sound: %w", err)
	}
	c.prune()
	return cachedPath, nil
}

// Prepare turns a chosen sound file into a playable WAV: it converts non-WAV
// formats, validates the result, enforces maxClipMS, and applies volume.
func (c *Cache) Prepare(src string, volume float64, maxClipMS int) (string, *WAVInfo, error) {
	wavPath, err := c.ToWAV(src)
	if err != nil {
		return "", nil, err
	}
	info, err := ValidateWAVFile(wavPath)
	if err != nil {
		return "", nil, fmt.Errorf("sound %s is invalid: %w", src, err)
	}
	if err := info.ValidateClipDuration(maxClipMS); err != nil {
		return "", nil, fmt.Errorf("sound %s: %w", src, err)
	}
	playPath, err := c.GetOrScale(wavPath, volume)
	if err != nil {
		return "", nil, err
	}
	return playPath, info, nil
}

// afconvertToWAV converts with the macOS system converter to 16-bit
// little-endian PCM WAV, keeping the source's sample rate and channels.
func afconvertToWAV(ctx context.Context, src, dst string) error {
	return runBoundedConversion(ctx, afconvertPath,
		[]string{"-f", "WAVE", "-d", "LEI16", src, dst}, dst, MaxWAVSizeBytes, ConversionTimeout)
}

// runBoundedConversion runs a converter process and stops it if it runs past
// timeout or its output at dst grows beyond maxBytes, so a long or malformed
// source can't fill the disk.
func runBoundedConversion(ctx context.Context, name string, args []string, dst string, maxBytes int64, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", filepath.Base(name), err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ticker := time.NewTicker(conversionPollInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("conversion took longer than %v", timeout)
			}
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = err.Error()
				}
				return fmt.Errorf("%s failed: %s", filepath.Base(name), msg)
			}
			if fi, err := os.Stat(dst); err == nil && fi.Size() > maxBytes {
				return fmt.Errorf("converted audio exceeds the %d byte limit", maxBytes)
			}
			return nil
		case <-ticker.C:
			if fi, err := os.Stat(dst); err == nil && fi.Size() > maxBytes {
				cancel()
				<-done
				return fmt.Errorf("converted audio exceeds the %d byte limit (is the clip longer than 15 seconds?)", maxBytes)
			}
		}
	}
}
