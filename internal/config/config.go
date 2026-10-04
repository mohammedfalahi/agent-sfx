package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agent-sfx/internal/events"
)

// Default settings as defined in architecture.md
const (
	CurrentVersion   = 1
	DefaultVolume    = 0.6
	DefaultCooldown  = 1200
	DefaultMaxClipMS = 15000 // 15 seconds default and hard maximum
	MaxAllowedClipMS = 15000 // 15 seconds is both the default and the hard maximum
)

// Config represents the agent-sfx configuration schema v1.
type Config struct {
	Version    int                       `json:"version"`
	Enabled    bool                      `json:"enabled"`
	Volume     float64                   `json:"volume"`
	CooldownMS int                       `json:"cooldown_ms"`
	MaxClipMS  int                       `json:"max_clip_ms"`
	Events     map[events.EventKind]bool `json:"events"`
	SoundsDir  string                    `json:"sounds_dir,omitempty"`
}

// DefaultConfig returns the baseline configuration with all 7 events enabled.
func DefaultConfig() Config {
	evs := make(map[events.EventKind]bool, len(events.AllEvents))
	for _, k := range events.AllEvents {
		evs[k] = true
	}
	return Config{
		Version:    CurrentVersion,
		Enabled:    true,
		Volume:     DefaultVolume,
		CooldownMS: DefaultCooldown,
		MaxClipMS:  DefaultMaxClipMS,
		Events:     evs,
		SoundsDir:  "",
	}
}

// Validate checks whether the configuration values are within acceptable bounds.
func (c *Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version: %d (expected %d)", c.Version, CurrentVersion)
	}
	if c.Volume < 0.0 || c.Volume > 1.0 {
		return fmt.Errorf("volume %v out of range: must be between 0.0 and 1.0", c.Volume)
	}
	if c.CooldownMS < 0 {
		return fmt.Errorf("cooldown_ms %d must be non-negative", c.CooldownMS)
	}
	if c.MaxClipMS <= 0 || c.MaxClipMS > MaxAllowedClipMS {
		return fmt.Errorf("max_clip_ms %d out of range: must be between 1 and %d ms", c.MaxClipMS, MaxAllowedClipMS)
	}
	for k := range c.Events {
		if !k.IsValid() {
			return fmt.Errorf("unknown event kind in config: %q", k)
		}
	}
	return nil
}

// IsEventEnabled checks if global audio is enabled and the specific event is enabled.
func (c *Config) IsEventEnabled(kind events.EventKind) bool {
	if !c.Enabled {
		return false
	}
	enabled, exists := c.Events[kind]
	if !exists {
		return false
	}
	return enabled
}

// DefaultConfigPath returns the canonical path in os.UserConfigDir.
func DefaultConfigPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine user config directory: %w", err)
	}
	return filepath.Join(base, "agent-sfx", "config.json"), nil
}

// Load reads and parses configuration from the given path or the default location.
// If customPath is empty and the default file does not exist, it returns DefaultConfig().
func Load(customPath string) (Config, string, error) {
	var targetPath string
	if customPath != "" {
		targetPath = customPath
	} else {
		defPath, err := DefaultConfigPath()
		if err != nil {
			return DefaultConfig(), "", err
		}
		targetPath = defPath
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) && customPath == "" {
			// No config file at default location; use defaults
			cfg := DefaultConfig()
			return cfg, targetPath, nil
		}
		return Config{}, targetPath, fmt.Errorf("failed to read config file at %s: %w", targetPath, err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, targetPath, fmt.Errorf("invalid config JSON at %s: %w", targetPath, err)
	}

	// Ensure all standard events have at least a default entry if missing in JSON
	if cfg.Events == nil {
		cfg.Events = make(map[events.EventKind]bool)
	}
	for _, k := range events.AllEvents {
		if _, exists := cfg.Events[k]; !exists {
			cfg.Events[k] = true
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, targetPath, fmt.Errorf("config validation error: %w", err)
	}

	return cfg, targetPath, nil
}
