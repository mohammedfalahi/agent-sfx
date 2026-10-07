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

func TestSetEnabled_PreservesUnknownFieldsAndPrecision(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	// JSON with custom fields and large integer that would lose precision with float64
	initialJSON := `{
  "version": 1,
  "enabled": true,
  "volume": 0.8,
  "cooldown_ms": 1500,
  "max_clip_ms": 15000,
  "sounds_dir": "/custom/sounds",
  "custom_feature_flag": "active",
  "large_id": 9007199254740993,
  "nested_metadata": {
    "author": "developer",
    "tags": ["audio", "sfx"]
  },
  "events": {
    "task_finished": true,
    "error": false
  }
}`
	if err := os.WriteFile(cfgPath, []byte(initialJSON), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Toggle to false
	p, err := config.SetEnabled(cfgPath, false)
	if err != nil {
		t.Fatalf("SetEnabled(false) failed: %v", err)
	}
	if p != cfgPath {
		t.Errorf("expected path %s, got %s", cfgPath, p)
	}

	// Verify loaded typed config reflects enabled: false
	loaded, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed after SetEnabled: %v", err)
	}
	if loaded.Enabled {
		t.Errorf("expected loaded config.Enabled to be false")
	}
	if loaded.Volume != 0.8 {
		t.Errorf("expected volume 0.8, got %v", loaded.Volume)
	}
	if loaded.SoundsDir != "/custom/sounds" {
		t.Errorf("expected sounds_dir to be preserved, got %q", loaded.SoundsDir)
	}

	// Verify raw content preserves custom keys and exact large number digits
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("failed to read updated config file: %v", err)
	}
	rawStr := string(content)

	if !containsString(rawStr, `"custom_feature_flag": "active"`) {
		t.Errorf("custom_feature_flag was not preserved in raw JSON:\n%s", rawStr)
	}
	if !containsString(rawStr, `"large_id": 9007199254740993`) {
		t.Errorf("large_id was modified or lost precision in raw JSON:\n%s", rawStr)
	}
	if !containsString(rawStr, `"author": "developer"`) {
		t.Errorf("nested_metadata was not preserved in raw JSON:\n%s", rawStr)
	}
	if !containsString(rawStr, `"enabled": false`) {
		t.Errorf("enabled was not set to false in raw JSON:\n%s", rawStr)
	}

	// Toggle back to true
	if _, err := config.SetEnabled(cfgPath, true); err != nil {
		t.Fatalf("SetEnabled(true) failed: %v", err)
	}
	loadedTrue, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !loadedTrue.Enabled {
		t.Errorf("expected loaded config.Enabled to be true")
	}
}

func TestSetEnabled_RejectsInvalidExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Invalid JSON syntax
	badSyntaxPath := filepath.Join(tmpDir, "badsyntax.json")
	if err := os.WriteFile(badSyntaxPath, []byte(`{invalid-json`), 0644); err != nil {
		t.Fatalf("failed to write bad syntax file: %v", err)
	}
	if _, err := config.SetEnabled(badSyntaxPath, false); err == nil {
		t.Errorf("expected error when updating invalid JSON syntax, got nil")
	}
	// Verify bad file was NOT overwritten
	data, _ := os.ReadFile(badSyntaxPath)
	if string(data) != `{invalid-json` {
		t.Errorf("bad syntax file was overwritten: %s", string(data))
	}

	// 2. Semantic validation failure (volume > 1.0)
	invalidValPath := filepath.Join(tmpDir, "invalidval.json")
	invalidJSON := `{"version": 1, "enabled": true, "volume": 5.0, "cooldown_ms": 1000, "max_clip_ms": 15000}`
	if err := os.WriteFile(invalidValPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write invalid validation file: %v", err)
	}
	if _, err := config.SetEnabled(invalidValPath, false); err == nil {
		t.Errorf("expected error when updating semantically invalid config, got nil")
	}
	dataVal, _ := os.ReadFile(invalidValPath)
	if string(dataVal) != invalidJSON {
		t.Errorf("invalid validation file was overwritten: %s", string(dataVal))
	}
}

func TestSetEnabled_CreatesDefaultIfMissing(t *testing.T) {
	tmpDir := t.TempDir()
	missingPath := filepath.Join(tmpDir, "nested", "config.json")

	p, err := config.SetEnabled(missingPath, false)
	if err != nil {
		t.Fatalf("SetEnabled on missing file failed: %v", err)
	}
	if p != missingPath {
		t.Errorf("expected path %s, got %s", missingPath, p)
	}

	cfg, _, err := config.Load(missingPath)
	if err != nil {
		t.Fatalf("Load on created file failed: %v", err)
	}
	if cfg.Enabled {
		t.Errorf("expected created config to have Enabled: false")
	}
	if cfg.Volume != config.DefaultVolume {
		t.Errorf("expected default volume %v, got %v", config.DefaultVolume, cfg.Volume)
	}
}

func TestSetEnabled_ConcurrentUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "concurrent_config.json")

	// Create initial config
	if _, err := config.SetEnabled(cfgPath, true); err != nil {
		t.Fatalf("initial SetEnabled failed: %v", err)
	}

	const goroutines = 20
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			targetState := (idx%2 == 0)
			_, err := config.SetEnabled(cfgPath, targetState)
			errCh <- err
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent SetEnabled failed: %v", err)
		}
	}

	// Final file must be valid JSON and readable
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config corrupted after concurrent writes: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config invalid after concurrent writes: %v", err)
	}
}

func containsString(s, substr string) bool {
	return filepath.Clean(s) != "" && len(s) >= len(substr) && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
