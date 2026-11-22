package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/events"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/worker"
)

// IsolatedEnv is a throwaway user environment for subprocess tests: its own
// HOME, config, sounds directories and worker socket. Sounds are disabled in
// its config, so a worker started inside it never plays audio.
type IsolatedEnv struct {
	Base       string // short root directory (kept short for socket path limits)
	Home       string
	ConfigPath string
	CustomDir  string // the config's sounds_dir, with a task_finished WAV
	BundledDir string // simulated launcher-bundled sounds, with every event
	Socket     string
	TraceLog   string
}

// NewIsolatedEnv creates the environment and registers cleanup that stops any
// worker bound to its socket and removes all files.
func NewIsolatedEnv(t testing.TB) *IsolatedEnv {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("isolated subprocess environment is Unix-only")
	}
	base, err := os.MkdirTemp("/tmp", "asfx-")
	if err != nil {
		t.Fatal(err)
	}
	e := &IsolatedEnv{
		Base:       base,
		Home:       filepath.Join(base, "h"),
		CustomDir:  filepath.Join(base, "custom"),
		BundledDir: filepath.Join(base, "bundled"),
		Socket:     filepath.Join(base, "w.sock"),
		TraceLog:   filepath.Join(base, "trace.jsonl"),
	}
	t.Cleanup(func() {
		_, _ = worker.StopDaemon(e.Socket)
		_ = os.RemoveAll(base)
	})

	var cfgDir string
	if runtime.GOOS == "darwin" {
		cfgDir = filepath.Join(e.Home, "Library", "Application Support", "agent-sfx")
	} else {
		cfgDir = filepath.Join(e.Home, ".config", "agent-sfx")
	}
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	e.ConfigPath = filepath.Join(cfgDir, "config.json")

	cfg := config.DefaultConfig()
	cfg.Enabled = false
	cfg.SoundsDir = e.CustomDir
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.ConfigPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	wav := audio.EncodePCM16WAV(1, 44100, []int16{0, 0, 0})
	writeWAV := func(dir string, kind events.EventKind) {
		d := filepath.Join(dir, string(kind))
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "sound.wav"), wav, 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeWAV(e.CustomDir, events.EventTaskFinished)
	for _, k := range events.AllEvents {
		writeWAV(e.BundledDir, k)
	}
	return e
}

// Env returns os.Environ() with HOME, XDG config, and every AGENT_SFX_* variable
// replaced by this environment's values, plus any extra KEY=VALUE entries.
func (e *IsolatedEnv) Env(extra ...string) []string {
	var out []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") ||
			strings.HasPrefix(kv, "AGENT_SFX_") || strings.HasPrefix(kv, "CLAUDE_PLUGIN_ROOT=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out,
		"HOME="+e.Home,
		"XDG_CONFIG_HOME="+filepath.Join(e.Home, ".config"),
		"AGENT_SFX_SOCKET_PATH="+e.Socket,
	)
	return append(out, extra...)
}

// WaitForWorker polls the isolated socket until the worker answers status.
func (e *IsolatedEnv) WaitForWorker(t testing.TB) *ipc.Response {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := worker.StatusDaemon(e.Socket); err == nil && st.OK {
			return st
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("worker did not become reachable on %s", e.Socket)
	return nil
}

// SamePath reports whether two paths name the same location after resolving
// symlinks (macOS /tmp is a symlink to /private/tmp).
func SamePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}
