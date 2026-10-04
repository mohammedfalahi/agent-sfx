package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultConfig() should be valid, got: %v", err)
	}

	if cfg.Version != config.CurrentVersion {
		t.Errorf("expected version %d, got %d", config.CurrentVersion, cfg.Version)
	}
	if !cfg.Enabled {
		t.Errorf("expected enabled to be true")
	}
	if cfg.Volume != 0.6 {
		t.Errorf("expected volume 0.6, got %v", cfg.Volume)
	}
	if cfg.CooldownMS != 1200 {
		t.Errorf("expected cooldown_ms 1200, got %d", cfg.CooldownMS)
	}
	if cfg.MaxClipMS != 15000 {
		t.Errorf("expected max_clip_ms 15000, got %d", cfg.MaxClipMS)
	}

	for _, k := range events.AllEvents {
		if !cfg.IsEventEnabled(k) {
			t.Errorf("expected event %s to be enabled by default", k)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr bool
	}{
		{
			name:    "valid defaults",
			mutate:  func(c *config.Config) {},
			wantErr: false,
		},
		{
			name:    "volume too low",
			mutate:  func(c *config.Config) { c.Volume = -0.1 },
			wantErr: true,
		},
		{
			name:    "volume too high",
			mutate:  func(c *config.Config) { c.Volume = 1.05 },
			wantErr: true,
		},
		{
			name:    "volume boundary 0.0",
			mutate:  func(c *config.Config) { c.Volume = 0.0 },
			wantErr: false,
		},
		{
			name:    "volume boundary 1.0",
			mutate:  func(c *config.Config) { c.Volume = 1.0 },
			wantErr: false,
		},
		{
			name:    "negative cooldown",
			mutate:  func(c *config.Config) { c.CooldownMS = -1 },
			wantErr: true,
		},
		{
			name:    "zero max_clip_ms",
			mutate:  func(c *config.Config) { c.MaxClipMS = 0 },
			wantErr: true,
		},
		{
			name:    "negative max_clip_ms",
			mutate:  func(c *config.Config) { c.MaxClipMS = -100 },
			wantErr: true,
		},
		{
			name:    "max_clip_ms boundary 15000 accepted",
			mutate:  func(c *config.Config) { c.MaxClipMS = 15000 },
			wantErr: false,
		},
		{
			name:    "max_clip_ms 15001 rejected",
			mutate:  func(c *config.Config) { c.MaxClipMS = 15001 },
			wantErr: true,
		},
		{
			name:    "wrong version",
			mutate:  func(c *config.Config) { c.Version = 2 },
			wantErr: true,
		},
		{
			name: "unknown event kind in map",
			mutate: func(c *config.Config) {
				c.Events[events.EventKind("unknown_event")] = true
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsEventEnabled(t *testing.T) {
	cfg := config.DefaultConfig()

	// All enabled initially
	if !cfg.IsEventEnabled(events.EventTaskStarted) {
		t.Errorf("task_started should be enabled")
	}

	// Disable specific event
	cfg.Events[events.EventTaskStarted] = false
	if cfg.IsEventEnabled(events.EventTaskStarted) {
		t.Errorf("task_started should now be disabled")
	}
	if !cfg.IsEventEnabled(events.EventTaskFinished) {
		t.Errorf("task_finished should still be enabled")
	}

	// Disable globally
	cfg.Enabled = false
	if cfg.IsEventEnabled(events.EventTaskFinished) {
		t.Errorf("all events should be disabled when global Enabled=false")
	}
}

func TestLoadConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	validConfigPath := filepath.Join(tmpDir, "config.json")

	content := `{
		"version": 1,
		"enabled": true,
		"volume": 0.8,
		"cooldown_ms": 1500,
		"max_clip_ms": 12000,
		"events": {
			"task_finished": true,
			"error": false
		}
	}`

	if err := os.WriteFile(validConfigPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, path, err := config.Load(validConfigPath)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if path != validConfigPath {
		t.Errorf("expected path %s, got %s", validConfigPath, path)
	}
	if cfg.Volume != 0.8 {
		t.Errorf("expected volume 0.8, got %v", cfg.Volume)
	}
	if cfg.CooldownMS != 1500 {
		t.Errorf("expected cooldown_ms 1500, got %d", cfg.CooldownMS)
	}
	if cfg.MaxClipMS != 12000 {
		t.Errorf("expected max_clip_ms 12000, got %d", cfg.MaxClipMS)
	}
	if !cfg.IsEventEnabled(events.EventTaskFinished) {
		t.Errorf("task_finished should be enabled")
	}
	if cfg.IsEventEnabled(events.EventError) {
		t.Errorf("error should be disabled")
	}
	// Missing in file should default to true
	if !cfg.IsEventEnabled(events.EventTaskStarted) {
		t.Errorf("unspecified task_started should default to enabled")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("failed to write bad config: %v", err)
	}

	_, _, err := config.Load(badPath)
	if err == nil {
		t.Fatalf("expected error for invalid JSON, got nil")
	}
}

func TestLoadMissingExplicitPath(t *testing.T) {
	_, _, err := config.Load("/path/to/nonexistent/config.json")
	if err == nil {
		t.Fatalf("expected error for nonexistent custom path, got nil")
	}
}
