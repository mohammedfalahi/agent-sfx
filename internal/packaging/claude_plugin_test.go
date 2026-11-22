// Package packaging_test checks the shipped Claude Code plugin and Gemini
// extension files against the adapters and against Claude Code's documented
// plugin, hook, and skill formats (code.claude.com/docs, checked 2026-11-19).
package packaging_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"agent-sfx/internal/adapters/claude"
	"agent-sfx/internal/doctor"
	"agent-sfx/internal/events"
	"agent-sfx/internal/testutil"
)

// claudeHookCommand is the only command the plugin registers.
const claudeHookCommand = `"${CLAUDE_PLUGIN_ROOT}/runtime/agent-sfx" hook claude`

// claudeEventFixtures maps each registered sound-producing hook event to a
// sanitized fixture and the sound event it must produce.
var claudeEventFixtures = map[string]struct {
	fixture string
	want    events.EventKind
}{
	"UserPromptSubmit":   {"user_prompt_submit.json", events.EventTaskStarted},
	"Stop":               {"stop.json", events.EventTaskFinished},
	"StopFailure":        {"stop_failure.json", events.EventError},
	"PermissionRequest":  {"permission_request_bash.json", events.EventPermissionRequested},
	"PreToolUse":         {"pre_tool_use_ask_user.json", events.EventWaitingForUser},
	"PostToolUseFailure": {"post_tool_use_failure.json", events.EventError},
}

// Events on which Claude Code ignores matchers (hooks reference, "matcher").
var matcherIgnoredEvents = map[string]bool{"UserPromptSubmit": true, "Stop": true}

type hookHandler map[string]any

type matcherGroup struct {
	Matcher *string       `json:"matcher"`
	Hooks   []hookHandler `json:"hooks"`
}

func pluginDir(t *testing.T) string {
	return filepath.Join(testutil.RepoRoot(t), "npm", "claude")
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func loadClaudeHooks(t *testing.T) map[string][]matcherGroup {
	t.Helper()
	var root map[string]json.RawMessage
	readJSON(t, filepath.Join(pluginDir(t), "hooks", "hooks.json"), &root)
	if len(root) != 1 || root["hooks"] == nil {
		t.Fatalf("hooks.json must contain exactly one top-level \"hooks\" key, got %v", keys(root))
	}
	var hooks map[string][]matcherGroup
	if err := json.Unmarshal(root["hooks"], &hooks); err != nil {
		t.Fatal(err)
	}
	return hooks
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestClaudeHooks_RegisteredEventsAreExactlyTheSupportedOnes(t *testing.T) {
	hooks := loadClaudeHooks(t)
	want := []string{"SessionStart"}
	for ev := range claudeEventFixtures {
		want = append(want, ev)
	}
	sort.Strings(want)
	if got := keys(hooks); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("registered hook events = %v, want %v", got, want)
	}
}

func TestClaudeHooks_HandlersUseOnlyDocumentedFields(t *testing.T) {
	allowed := map[string]bool{"type": true, "command": true, "timeout": true}
	for ev, groups := range loadClaudeHooks(t) {
		if len(groups) != 1 {
			t.Errorf("%s: expected one matcher group, got %d", ev, len(groups))
		}
		for _, g := range groups {
			if len(g.Hooks) != 1 {
				t.Errorf("%s: expected one handler, got %d", ev, len(g.Hooks))
			}
			for _, h := range g.Hooks {
				for k := range h {
					if !allowed[k] {
						t.Errorf("%s: handler field %q is not used by this plugin's schema (allowed: type, command, timeout)", ev, k)
					}
				}
				if h["type"] != "command" {
					t.Errorf("%s: type = %v, want command", ev, h["type"])
				}
				if h["command"] != claudeHookCommand {
					t.Errorf("%s: command = %v, want %s", ev, h["command"], claudeHookCommand)
				}
				timeout, ok := h["timeout"].(float64)
				if !ok || timeout < 1 || timeout > 30 {
					t.Errorf("%s: timeout = %v, want a bound between 1 and 30 seconds", ev, h["timeout"])
				}
			}
		}
	}
}

func TestClaudeHooks_Matchers(t *testing.T) {
	for ev, groups := range loadClaudeHooks(t) {
		for _, g := range groups {
			switch {
			case ev == "PreToolUse":
				// A plain identifier is an exact tool-name match in Claude Code.
				if g.Matcher == nil || *g.Matcher != "AskUserQuestion" {
					t.Errorf("PreToolUse must match exactly AskUserQuestion, got %v", g.Matcher)
				}
			case matcherIgnoredEvents[ev]:
				if g.Matcher != nil {
					t.Errorf("%s ignores matchers; remove matcher %q", ev, *g.Matcher)
				}
			default:
				if g.Matcher != nil && *g.Matcher != "" && *g.Matcher != "*" {
					t.Errorf("%s: unexpected narrowing matcher %q", ev, *g.Matcher)
				}
			}
		}
	}
}

// Every registered event must actually produce its sound in the adapter, so the
// plugin never runs hooks that can only ever be ignored.
func TestClaudeHooks_FixturesNormalizeToExpectedSounds(t *testing.T) {
	for ev, fx := range claudeEventFixtures {
		data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "testdata", "claude", fx.fixture))
		if err != nil {
			t.Fatal(err)
		}
		var probe struct {
			HookEventName string `json:"hook_event_name"`
		}
		if err := json.Unmarshal(data, &probe); err != nil || probe.HookEventName != ev {
			t.Fatalf("fixture %s is not a %s payload", fx.fixture, ev)
		}
		got, err := claude.Normalize(data)
		if err != nil || len(got) != 1 || got[0].Kind != fx.want {
			t.Errorf("%s (%s): got %+v, err %v; want one %s event", ev, fx.fixture, got, err, fx.want)
		}
	}
}

func TestClaudePlugin_Layout(t *testing.T) {
	dir := pluginDir(t)
	// Claude Code puts a plugin's bin/ directory on the Bash tool's PATH; this
	// plugin must not expose its launcher or binaries there.
	if _, err := os.Stat(filepath.Join(dir, "bin")); err == nil {
		t.Error("plugin must not contain a bin/ directory (it would be added to Claude's Bash PATH)")
	}
	for _, rel := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		"hooks/hooks.json",
		"skills/sfx/SKILL.md",
		"runtime/agent-sfx",
		"LICENSE",
		"SOUND-LICENSES.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}
	for _, k := range events.AllEvents {
		wavs, _ := filepath.Glob(filepath.Join(dir, "sounds", string(k), "*.wav"))
		if len(wavs) == 0 {
			t.Errorf("no bundled starter sound for %s", k)
		}
	}
	// Bundled plugin sounds must be the seven generated starter sounds only.
	_ = filepath.WalkDir(filepath.Join(dir, "sounds"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".wav") {
			if strings.TrimSuffix(filepath.Base(p), ".wav") != filepath.Base(filepath.Dir(p)) {
				t.Errorf("unexpected bundled sound %s (only <event>/<event>.wav starter sounds ship in the plugin)", p)
			}
		}
		return nil
	})
}

// fakePluginCopy copies the plugin's launcher into a temporary plugin layout
// with a fake binary for this Mac that prints its arguments and environment.
func fakePluginCopy(t *testing.T) (root string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("the Claude plugin launcher targets macOS")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(root, "plugin dir") // exercise paths with spaces
	launcher, err := os.ReadFile(filepath.Join(pluginDir(t), "runtime", "agent-sfx"))
	if err != nil {
		t.Fatal(err)
	}
	archDir := "darwin-arm64"
	if runtime.GOARCH == "amd64" {
		archDir = "darwin-x64"
	}
	if err := os.MkdirAll(filepath.Join(root, "runtime", archDir), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "runtime", "agent-sfx"), launcher, 0755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nfor a in \"$@\"; do printf 'ARG=%s\\n' \"$a\"; done\nprintf 'SOUNDS=%s\\n' \"$AGENT_SFX_BUNDLED_SOUNDS_DIR\"\nexit ${FAKE_EXIT:-0}\n"
	if err := os.WriteFile(filepath.Join(root, "runtime", archDir, "agent-sfx"), []byte(fake), 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

func runLauncher(t *testing.T, root string, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, "runtime", "agent-sfx"), args...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

func TestClaudeLauncher_ForwardsArgsAndBundledSounds(t *testing.T) {
	root := fakePluginCopy(t)
	out, errOut, code := runLauncher(t, root, []string{"AGENT_SFX_BUNDLED_SOUNDS_DIR="}, "hook", "claude", `a b "c"`)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if !strings.Contains(out, "ARG=hook\nARG=claude\nARG=a b \"c\"\n") {
		t.Errorf("arguments not forwarded unchanged:\n%s", out)
	}
	if !strings.Contains(out, "SOUNDS="+filepath.Join(root, "sounds")+"\n") {
		t.Errorf("bundled sounds dir not advertised as %s:\n%s", filepath.Join(root, "sounds"), out)
	}
}

func TestClaudeLauncher_KeepsExistingBundledSoundsEnv(t *testing.T) {
	root := fakePluginCopy(t)
	out, _, _ := runLauncher(t, root, []string{"AGENT_SFX_BUNDLED_SOUNDS_DIR=/custom/bundled"})
	if !strings.Contains(out, "SOUNDS=/custom/bundled\n") {
		t.Errorf("existing AGENT_SFX_BUNDLED_SOUNDS_DIR was overwritten:\n%s", out)
	}
}

func TestClaudeLauncher_PreservesExitCode(t *testing.T) {
	root := fakePluginCopy(t)
	if _, _, code := runLauncher(t, root, []string{"FAKE_EXIT=3"}); code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

// Users may symlink the launcher onto their PATH (e.g. ~/.local/bin/agent-sfx).
func TestClaudeLauncher_ViaSymlink(t *testing.T) {
	root := fakePluginCopy(t)
	link := filepath.Join(t.TempDir(), "agent-sfx")
	if err := os.Symlink(filepath.Join(root, "runtime", "agent-sfx"), link); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(link, "status")
	cmd.Env = append(os.Environ(), "AGENT_SFX_BUNDLED_SOUNDS_DIR=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("symlinked launcher failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ARG=status\n") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestClaudeLauncher_MissingBinary(t *testing.T) {
	root := fakePluginCopy(t)
	_ = os.RemoveAll(filepath.Join(root, "runtime", "darwin-arm64"))
	_ = os.RemoveAll(filepath.Join(root, "runtime", "darwin-x64"))
	_, errOut, code := runLauncher(t, root, nil, "status")
	if code != 1 || !strings.Contains(errOut, "prebuilt binary not found") {
		t.Errorf("want exit 1 with a clear message, got %d: %s", code, errOut)
	}
}

func TestClaudePlugin_ManifestsAgree(t *testing.T) {
	dir := pluginDir(t)
	var plugin struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	readJSON(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), &plugin)
	if plugin.Name != "agent-sfx" {
		t.Errorf("plugin name = %q, want agent-sfx (skill becomes /agent-sfx:sfx)", plugin.Name)
	}
	var market struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  string `json:"source"`
		} `json:"plugins"`
	}
	readJSON(t, filepath.Join(dir, ".claude-plugin", "marketplace.json"), &market)
	if market.Name == "" || len(market.Plugins) != 1 {
		t.Fatalf("marketplace.json must name the marketplace and list one plugin: %+v", market)
	}
	p := market.Plugins[0]
	if p.Name != plugin.Name || p.Version != plugin.Version || p.Source != "./" {
		t.Errorf("marketplace entry %+v does not match plugin %+v with source ./", p, plugin)
	}
	if !strings.HasPrefix(doctor.AppVersion, plugin.Version) {
		t.Errorf("binary version %q does not match plugin version %q", doctor.AppVersion, plugin.Version)
	}
}

func parseFrontmatter(t *testing.T, path string) (map[string]string, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("%s: missing frontmatter", path)
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		t.Fatalf("%s: unterminated frontmatter", path)
	}
	fm := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text[4 : 4+end]))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if ok {
			fm[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return fm, text[4+end+5:]
}

func TestClaudeSkill(t *testing.T) {
	skillPath := filepath.Join(pluginDir(t), "skills", "sfx", "SKILL.md")
	fm, body := parseFrontmatter(t, skillPath)
	if fm["name"] != "sfx" || fm["description"] == "" {
		t.Errorf("skill needs name sfx and a description, got %v", fm)
	}
	if fm["disable-model-invocation"] != "true" {
		t.Error("control skill must only run when the user invokes /agent-sfx:sfx")
	}
	const launcher = `"${CLAUDE_SKILL_DIR}/../../runtime/agent-sfx" <subcommand>`
	if !strings.Contains(body, launcher) {
		t.Errorf("skill must run the launcher via ${CLAUDE_SKILL_DIR}: %q", launcher)
	}
	if strings.Contains(body, "<skill_dir>") {
		t.Error("skill uses the undocumented <skill_dir> placeholder")
	}
	// ${CLAUDE_SKILL_DIR}/../../runtime/agent-sfx must resolve inside the plugin.
	resolved := filepath.Join(filepath.Dir(skillPath), "..", "..", "runtime", "agent-sfx")
	if _, err := os.Stat(resolved); err != nil {
		t.Errorf("skill launcher path does not resolve: %s", resolved)
	}
}

// Gemini requires hook stdout to be JSON only, so every Gemini hook must go
// through the neutral receiver (which prints {}), never a chatty CLI command.
func TestGeminiHooks_AllUseNeutralReceiver(t *testing.T) {
	var root struct {
		Hooks map[string][]struct {
			Hooks []hookHandler `json:"hooks"`
		} `json:"hooks"`
	}
	readJSON(t, filepath.Join(testutil.RepoRoot(t), "npm", "hooks", "hooks.json"), &root)
	if len(root.Hooks) == 0 {
		t.Fatal("no Gemini hooks found")
	}
	for ev, groups := range root.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h["command"] != `"${extensionPath}/bin/run.js" hook gemini` {
					t.Errorf("Gemini %s hook command = %v, want the neutral hook receiver", ev, h["command"])
				}
			}
		}
	}
}

// End to end: run the exact hooks.json command strings through a shell, Node,
// the plugin launcher, and a freshly built binary, against an isolated worker
// with sounds disabled (no audio plays).
func TestClaudePlugin_HookCommandsEndToEnd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the plugin launcher targets macOS")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found")
	}
	env := testutil.NewIsolatedEnv(t)

	// Copy the plugin (without any locally built binaries) and add a fresh build.
	plugin := filepath.Join(env.Base, "plugin")
	if err := os.CopyFS(plugin, os.DirFS(pluginDir(t))); err != nil {
		t.Fatal(err)
	}
	archDir := "darwin-arm64"
	if runtime.GOARCH == "amd64" {
		archDir = "darwin-x64"
	}
	binDst := filepath.Join(plugin, "runtime", archDir, "agent-sfx")
	_ = os.RemoveAll(filepath.Dir(binDst))
	if err := os.MkdirAll(filepath.Dir(binDst), 0755); err != nil {
		t.Fatal(err)
	}
	built, err := os.ReadFile(testutil.BuildAgentSFX(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binDst, built, 0755); err != nil {
		t.Fatal(err)
	}

	cmdEnv := env.Env("CLAUDE_PLUGIN_ROOT="+plugin, "AGENT_SFX_TRACE_LOG="+env.TraceLog)
	run := func(payload []byte) {
		t.Helper()
		cmd := exec.Command("/bin/sh", "-c", claudeHookCommand)
		cmd.Env = cmdEnv
		cmd.Stdin = bytes.NewReader(payload)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("hook command failed: %v\nstderr: %s", err, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("hook stdout must be empty, got %q", stdout.String())
		}
	}

	// SessionStart starts the worker; it must use the configured sounds dir and
	// fall back to the plugin's bundled sounds.
	run([]byte(`{"session_id":"e2e","hook_event_name":"SessionStart","source":"startup"}`))
	st := env.WaitForWorker(t)
	if !testutil.SamePath(st.SoundsDir, env.CustomDir) {
		t.Errorf("worker sounds dir = %q, want configured %q", st.SoundsDir, env.CustomDir)
	}
	if !testutil.SamePath(st.FallbackSoundsDir, filepath.Join(plugin, "sounds")) {
		t.Errorf("worker fallback = %q, want plugin sounds %q", st.FallbackSoundsDir, filepath.Join(plugin, "sounds"))
	}

	// Each sound-producing hook is classified and delivered to the worker.
	want := map[string]events.EventKind{}
	for ev, fx := range claudeEventFixtures {
		data, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "testdata", "claude", fx.fixture))
		if err != nil {
			t.Fatal(err)
		}
		run(data)
		want[ev] = fx.want
	}

	f, err := os.Open(env.TraceLog)
	if err != nil {
		t.Fatalf("trace log not written: %v", err)
	}
	defer f.Close()
	seen := map[string]claude.TraceEntry{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e claude.TraceEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad trace line %q: %v", sc.Text(), err)
		}
		seen[e.HookEventName] = e
	}
	for ev, kind := range want {
		e, ok := seen[ev]
		if !ok {
			t.Errorf("%s: no trace entry", ev)
			continue
		}
		if e.Classification != string(kind) || e.IPCDelivery != "ok" {
			t.Errorf("%s: classification %q delivery %q, want %q delivered ok", ev, e.Classification, e.IPCDelivery, kind)
		}
	}
}

// TestClaudePlugin_RootMarketplace checks the marketplace that
// `claude plugin marketplace add mohammedfalahi/agent-sfx` reads. Claude Code
// only looks for .claude-plugin/marketplace.json at the repository root.
func TestClaudePlugin_RootMarketplace(t *testing.T) {
	var plugin struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	readJSON(t, filepath.Join(pluginDir(t), ".claude-plugin", "plugin.json"), &plugin)

	var market struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Source  string `json:"source"`
		} `json:"plugins"`
	}
	readJSON(t, filepath.Join(testutil.RepoRoot(t), ".claude-plugin", "marketplace.json"), &market)
	// A distinct name keeps the GitHub marketplace from colliding with the
	// local "agent-sfx-local" marketplace that `agent-sfx setup deploy` creates.
	if market.Name != "agent-sfx" {
		t.Errorf("root marketplace name = %q, want agent-sfx (plugin ID agent-sfx@agent-sfx)", market.Name)
	}
	if len(market.Plugins) != 1 {
		t.Fatalf("root marketplace must list exactly one plugin, got %d", len(market.Plugins))
	}
	p := market.Plugins[0]
	if p.Name != plugin.Name || p.Version != plugin.Version || p.Source != "./npm/claude" {
		t.Errorf("root marketplace entry %+v does not match plugin %+v with source ./npm/claude", p, plugin)
	}
}

// TestClaudePlugin_BinariesAreShipped guards the GitHub install: Claude Code
// copies the plugin from a git clone and runs no build step, so the Mac
// binaries must be committed, executable, and not ignored by git.
func TestClaudePlugin_BinariesAreShipped(t *testing.T) {
	root := testutil.RepoRoot(t)
	for _, arch := range []string{"darwin-arm64", "darwin-x64"} {
		rel := filepath.Join("npm", "claude", "runtime", arch, "agent-sfx")
		fi, err := os.Stat(filepath.Join(root, rel))
		if err != nil {
			t.Errorf("missing %s (run scripts/build-claude-plugin.sh)", rel)
			continue
		}
		if fi.Mode()&0111 == 0 {
			t.Errorf("%s is not executable", rel)
		}
		if _, err := exec.LookPath("git"); err == nil {
			cmd := exec.Command("git", "check-ignore", "-q", "--no-index", rel)
			cmd.Dir = root
			if cmd.Run() == nil {
				t.Errorf("%s is ignored by git; a GitHub install would not include it", rel)
			}
		}
	}

	if runtime.GOOS != "darwin" {
		return
	}
	var plugin struct {
		Version string `json:"version"`
	}
	readJSON(t, filepath.Join(pluginDir(t), ".claude-plugin", "plugin.json"), &plugin)
	out, err := exec.Command(filepath.Join(pluginDir(t), "runtime", "agent-sfx"), "help").Output()
	if err != nil {
		t.Fatalf("shipped binary for this Mac did not run: %v", err)
	}
	if !strings.Contains(string(out), "(v"+plugin.Version) {
		first, _, _ := strings.Cut(string(out), "\n")
		t.Errorf("shipped binary reports %q, want version %s (rebuild with scripts/build-claude-plugin.sh)", first, plugin.Version)
	}
}
