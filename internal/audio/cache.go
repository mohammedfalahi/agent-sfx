package audio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	MaxCacheEntries = 100
)

// Cache manages cached volume-scaled WAV audio files.
type Cache struct {
	dir string
}

// NewCache returns a Cache using the given directory or the default OS cache directory.
func NewCache(customDir string) (*Cache, error) {
	var dir string
	if customDir != "" {
		dir = customDir
	} else {
		userCache, err := os.UserCacheDir()
		if err != nil {
			dir = filepath.Join(os.TempDir(), "agent-sfx-cache")
		} else {
			dir = filepath.Join(userCache, "agent-sfx", "cache")
		}
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", dir, err)
	}

	return &Cache{dir: dir}, nil
}

// Dir returns the cache directory path.
func (c *Cache) Dir() string {
	return c.dir
}

// GetOrScale returns the path to a playable WAV file. If volume is 1.0, the original file
// path is returned after validation. Otherwise, a cached scaled WAV is returned or created.
func (c *Cache) GetOrScale(sourcePath string, volume float64) (string, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("failed to open source WAV: %w", err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat source WAV: %w", err)
	}

	// Validate source WAV
	if _, err := ValidateAndParseWAV(file, fi.Size()); err != nil {
		return "", fmt.Errorf("source WAV %s validation failed: %w", sourcePath, err)
	}

	if volume == 1.0 {
		return sourcePath, nil
	}

	// Compute key from source path, modtime, size, and volume
	h := sha256.New()
	h.Write([]byte(sourcePath))
	h.Write([]byte(fmt.Sprintf(":%d:%d:%.4f", fi.ModTime().UnixNano(), fi.Size(), volume)))
	hashStr := hex.EncodeToString(h.Sum(nil))[:16]

	cachedFileName := fmt.Sprintf("scaled_%s.wav", hashStr)
	cachedPath := filepath.Join(c.dir, cachedFileName)

	// Check if already in cache
	if _, err := os.Stat(cachedPath); err == nil {
		// Update access time for LRU tracking
		_ = os.Chtimes(cachedPath, time.Now(), time.Now())
		return cachedPath, nil
	}

	// Scale and write to cache
	scaledBytes, err := ScaleWAVVolume(file, fi.Size(), volume)
	if err != nil {
		return "", fmt.Errorf("failed to scale WAV volume: %w", err)
	}

	tmpPath := cachedPath + ".tmp"
	if err := os.WriteFile(tmpPath, scaledBytes, 0600); err != nil {
		return "", fmt.Errorf("failed to write temp scaled WAV: %w", err)
	}

	if err := os.Rename(tmpPath, cachedPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to atomically commit scaled WAV to cache: %w", err)
	}

	// Prune cache if it exceeds bounds
	c.prune()

	return cachedPath, nil
}

// prune limits the number of files in the cache.
func (c *Cache) prune() {
	entries, err := os.ReadDir(c.dir)
	if err != nil || len(entries) <= MaxCacheEntries {
		return
	}

	type fileWithTime struct {
		path    string
		modTime time.Time
	}

	files := make([]fileWithTime, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileWithTime{
			path:    filepath.Join(c.dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	if len(files) <= MaxCacheEntries {
		return
	}

	// Sort oldest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	toDelete := len(files) - MaxCacheEntries
	for i := 0; i < toDelete; i++ {
		_ = os.Remove(files[i].path)
	}
}
