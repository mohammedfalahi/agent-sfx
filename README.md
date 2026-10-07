# Agent SFX

Agent SFX is a free, open-source, local command-line accessory that plays short, randomly selected sound effects for terminal coding agents.

## Supported Product Events

Agent SFX uses a closed enumeration of seven canonical events:
1. `permission_requested` — When an agent pauses to ask for user tool permission
2. `task_started` — When a user prompt/turn starts
3. `task_finished` — When an agent turn/response finishes
4. `waiting_for_user` — When the agent explicitly waits for user input
5. `tests_passed` — When test suites pass with unambiguous positive verification
6. `usage_exhausted` — When structured quota/billing exhaustion occurs
7. `error` — When an agent tool encounters an error

No unprompted sounds (no startup greetings, compaction noise, or periodic nagging).

---

## Status and Command Availability

### Implemented Commands (Milestones M0, M1, & M2)

- **`agent-sfx preview <event> [--config <path>] [--sounds-dir <path>]`**
  Plays a sound effect for the given canonical event manually (M0).
- **`agent-sfx doctor [--json] [--config <path>] [--sounds-dir <path>]`**
  Diagnoses the local environment: reports OS, architecture, Go version, audio backend availability, worker daemon state, resolved paths, and sound asset validity.
- **`agent-sfx worker start [--config <path>] [--sounds-dir <path>]`**
  Starts the background audio daemon with atomic singleton acquisition (`flock`), detached execution (inherited stdio closed, setsid), and owner-only Unix socket permissions (`0700`).
- **`agent-sfx worker stop`**
  Gracefully stops the active worker daemon via its owned IPC endpoint.
- **`agent-sfx worker status`**
  Queries the running worker daemon for PID, uptime, active sound state, and socket path.
- **`agent-sfx on [--config <path>]`**
  Enables sound playback. Atomically updates persisted configuration, preserves unknown fields and exact numeric precision, and re-enables live worker playback without spawning an absent worker.
- **`agent-sfx off [--config <path>]`**
  Disables sound playback. Atomically updates persisted configuration and acknowledges live worker muting (cancels active playback, clears pending queue, resets cooldown) while keeping the daemon alive.
- **`agent-sfx status [--config <path>] [--json]`**
  Displays current status distinguishing persisted configuration state from running worker daemon state (PID, uptime, live audio enabled/muted, and active playback state).
- **`agent-sfx hook gemini`**
  Verified hook receiver for Gemini CLI (v0.62.0). Reads stdin with time and size limits (max 1 MiB), maps verified events, emits exactly `{}` with a newline on stdout, and exits `0`.
- **`agent-sfx install gemini [--scope project|user] [--dry-run]`**
  Installs owned hook definitions into `.gemini/settings.json` (project scope default) or `~/.gemini/settings.json` (user scope). Features atomic writes, backup preservation (`settings.json.bak`), platform-safe shell quoting, and conflict detection.
- **`agent-sfx uninstall gemini [--scope project|user] [--dry-run]`**
  Removes only exact owned hooks whose commands match this installation. Preserves user-customized hooks and reports conflicts.
- **`agent-sfx setup gemini --dry-run`**
  Performs read-only inspection of manual hooks and extension status, detects duplicate integration risks in both directions, and plans safe migration without modifying settings.

---

## Distribution Packaging & npm Scaffold

Agent SFX provides an npm distribution package (`@agent-sfx/agent-sfx`) containing prebuilt macOS binaries and starter sound assets:
- **Prerequisites**: macOS (`darwin-arm64`, `darwin-x64`) and Node.js >= 18.0.0.
- **Network Requirement**: No additional install-script downloads; npm package acquisition still requires a download unless supplied locally.
- **Self-Contained Launcher**: `bin/run.js` automatically routes to the appropriate prebuilt Go executable using safe argument arrays (`spawnSync`), safely handling package paths with spaces and metacharacters.
- **Asset Provenance**: The package bundles only the 7 original software-generated CC0 sound assets; user meme clips and custom files are excluded.

---

## Gemini Support Matrix (CLI v0.62.0)

| Gemini Hook Event | Payload Match Condition | Canonical SFX Event | Notes |
| :--- | :--- | :--- | :--- |
| `BeforeAgent` | Any valid prompt turn | `task_started` | User prompt submitted; turn starts |
| `AfterAgent` | `stop_hook_active == false` | `task_finished` | Agent response/turn complete (does not guarantee overall task success) |
| `Notification` | `notification_type == "ToolPermission"` (excluding `details.type == "ask_user"`) | `permission_requested` | Agent pauses for user permission on a tool call (dialog collision suppressed) |
| `BeforeTool` | `tool_name == "ask_user"` with validated schema | `waiting_for_user` | Explicit question dialog requested (M4); anchored matcher `^ask_user$` |
| `AfterTool` | `tool_response.error` is non-empty, or `run_shell_command` non-zero `Exit Code` in trailing metadata | `error` | Tool execution failure across all tools, or verified non-zero shell exit code (not output text) |
| `AfterTool` | Verified direct `go test` or `pytest` invocation with positive test summary | `tests_passed` | Requires verified complete command execution and qualifying passed tests (M3) |
| `AfterTool` | Successful non-test tool result or normal `ask_user` answer/dismissal | *(none / dropped)* | Ordinary successful tool calls do not trigger completion sounds |
| `SessionStart` | `source: "startup"` | *(none)* | Silently launches background worker if offline without blocking agent |
| `AfterModel` | Any | *(unsupported)* | Ignored |
| `PreCompress` | Any | *(unsupported)* | Ignored |
| — | General API exhaustion | *(unsupported initially)* | Unobserved API errors and quota are not guessed |
| — | General idle waiting | *(unsupported)* | Generic waiting or silence is not detected; only explicit question dialogs via `ask_user` are supported |

---

## Claude Code Support Matrix (CLI v2.1.162)

*Status: Implemented & automated verification complete (adapter normalization, verified StopFailure allowlist, neutral hook receiver `agent-sfx hook claude`, self-contained plugin packaging in `npm/claude/`, local marketplace manifest, and `agent-sfx setup claude --dry-run`). Live event, audio, and management-skill verification in an authenticated session are pending future tester access.*

| Claude Hook Event | Payload Match Condition | Canonical SFX Event | Notes |
| :--- | :--- | :--- | :--- |
| `UserPromptSubmit` | Any valid prompt turn | `task_started` | User prompt submitted; turn starts |
| `Stop` | `stop_hook_active == false` | `task_finished` | Response/turn complete (does not guarantee overall task success) |
| `PermissionRequest` | `tool_name != "AskUserQuestion"` | `permission_requested` | Hook executed before presenting tool permission prompt; suppressed when `tool_name == "AskUserQuestion"` |
| `PreToolUse` | `tool_name == "AskUserQuestion"` with validated schema | `waiting_for_user` | Explicit question dialog requested; validated against native schema; all other tools produce 0 events |
| `PostToolUseFailure` | Non-empty `error` and `is_interrupt == false` | `error` | Tool execution failure across all tools; user cancellations (`is_interrupt == true`) produce no sound |
| `StopFailure` | Exact match against verified 10-value enum | `error` | Source- and fixture-tested against Claude Code 2.1.162 binary schema (`rate_limit`, `overloaded`, `authentication_failed`, `oauth_org_not_allowed`, `billing_error`, `invalid_request`, `model_not_found`, `server_error`, `max_output_tokens`, `unknown`); prose and model refusals strictly rejected; live API failure unconfirmed |
| `SessionStart` | Any valid session start | *(none)* | Silently launches background worker if offline without blocking agent |
| — | Tool execution output (`Bash`) | *(unsupported)* | Direct shell output parsing differs from Gemini metadata; `tests_passed` remains explicitly unsupported |
| — | Billing / Quota zero | *(unsupported)* | Claude Code lacks a dedicated quota exhaustion hook; `usage_exhausted` remains explicitly unsupported |

---

## Explicit Question Detection (Milestone M4)

Agent SFX recognizes explicit user-question dialogs triggered by Gemini CLI:
- **Hook Event**: Detected via `BeforeTool` with anchored matcher `^ask_user$`.
- **Schema Validation**: Strictly validates against Gemini CLI 0.62.0's native schema:
  - `questions` array with 1 to 4 question objects.
  - Required non-empty `question` and `header` strings.
  - Supported question types: `choice` (default), `text`, and `yesno`.
  - For `choice`: requires 2 to 4 options, each with non-empty `label` and valid `description`.
  - Missing, empty, malformed, or unsupported question inputs produce no sound.
- **Single Event**: Emits exactly one `waiting_for_user` event per invocation, regardless of question count.
- **Invariants & Limitations**:
  - `BeforeTool` signals a question-tool request from the model, not proof that the dialog was displayed (a later hook or policy may block it).
  - Deduplication: `base.Timestamp` is the hook event creation timestamp, not a persistent tool call ID. Deduplication covers identical payload replays without hashing question content.
  - Collision Avoidance: Gemini CLI emits `NotificationType == "ToolPermission"` with `details: {"type": "ask_user", ...}` during confirmation. Agent SFX inspects `details.type` and suppresses `permission_requested` so that no duplicate permission sound is triggered.
  - `AfterTool` for `ask_user`: user answers and dialog dismissal produce no second sound; genuine structured errors retain standard `error` classification.
  - Generic waiting detection (e.g. inference from terminal silence or prose questions) is explicitly unsupported.

---

## Test-Pass Detection (Milestone M3)

Agent SFX recognizes direct test runner invocations executed via `run_shell_command`:
- **Go Tests**: Direct `go test [flags] [packages...]`. Requires at least one qualifying passed package (`ok <pkg> <duration>`). Excludes packages marked `[no test files]` or `[no tests to run]` from qualifying counts. Fully cached test runs (`ok <pkg> (cached)`) are classified with reason code `go_test_cached`; runs with newly executed tests use `go_test_passed`.
- **Pytest**: Direct `pytest [args...]`, `python -m pytest [args...]`, or `python3 -m pytest [args...]`. Requires at least one passed test (`=== N passed ... ===`), zero failures, zero errors, and no interruption.
- **Strict Invariants**:
  - Error precedence: tool errors and non-zero exit codes strictly preempt test detection.
  - Background execution (`is_background: true`) is rejected.
  - Compound commands (`&&`, `||`, `;`, `&`, `|`), subshells (`$()`, backticks), redirections (`>`, `<`), and multi-line commands are rejected.
  - Leading environment variable assignments (e.g. `CGO_ENABLED=0 go test`) are unsupported in initial M3.
  - Unsupported output formats (e.g. `go test -json`) are explicitly rejected.
  - Command completion must be verified via synchronous trailing metadata (`Process Group PGID: <pid>`); timeouts and cancellations are rejected.

---

## Configuration

Agent SFX reads configuration from `os.UserConfigDir()/agent-sfx/config.json` (or via `--config <path>`). If absent, safe defaults are used automatically:

```json
{
  "version": 1,
  "enabled": true,
  "volume": 0.6,
  "cooldown_ms": 1200,
  "max_clip_ms": 15000,
  "events": {
    "permission_requested": true,
    "task_started": true,
    "task_finished": true,
    "waiting_for_user": true,
    "tests_passed": true,
    "usage_exhausted": true,
    "error": true
  }
}
```

### Supporting Longer Custom Meme Audio Clips
- **15-Second Hard Maximum**: `max_clip_ms` defaults to `15000` (15 seconds) and is the hard maximum. Valid configuration range is `1`–`15000` ms.
- **Precise Duration Comparison**: Audio duration is evaluated without integer rounding down; any audio longer than the configured limit is rejected without truncation.
- **Process Overhead Headroom**: Process execution timeouts grant actual audio duration + 5 seconds headroom to absorb OS audio server initialization and buffer teardown latency. The overhead does not permit longer audio.
- **File Size Bound**: Up to 25 MiB per WAV file (supports uncompressed stereo 16-bit PCM).
- **Updating Existing Configs**: If your existing `config.json` specifies `"max_clip_ms": 3000`, increase it to `15000` in `~/Library/Application Support/agent-sfx/config.json` (macOS) or `~/.config/agent-sfx/config.json` (Linux) to allow longer custom clips to play completely.
- **Non-Preemptive Playback**: At most one sound plays at a time. All events arriving while a clip is actively playing or during the subsequent cooldown window (1200 ms default) are dropped immediately without creating stale backlogs.

---

## Platform Verification & Limitations

- **macOS (`darwin/arm64`, `darwin/x64`)**: Fully verified on local hardware (audio playback via `afplay`, Unix domain socket IPC, `flock` singleton locking, process detachment, settings installation, and npm launcher execution).
- **Linux (`linux/amd64`)**: Cross-compilation verified. POSIX-compliant socket and locking primitives implemented; audio backend detection for `pw-play`, `paplay`, and `aplay`.
- **Windows (`windows/amd64` / `win32-x64`)**: Full architecture implemented and cross-compiled (named-pipe IPC with user SID DACL, atomic singleton locking via `LockFileEx`, hidden PowerShell player using `Media.SoundPlayer` and `CREATE_NO_WINDOW`, Node launcher support). Cross-compilation and automated unit tests pass; physical Windows audible playback and separate-process contention remain pending physical Windows host verification.

---

## Quick Start & Verification

> **Getting Started**: For complete step-by-step setup, configuration, and uninstallation instructions, see **[`SETUP.md`](SETUP.md)**.

### 1. Building from Source

- **macOS / Linux**:
  ```bash
  go build -o bin/agent-sfx ./cmd/agent-sfx
  ```
- **Windows PowerShell**:
  ```powershell
  go build -o bin\agent-sfx.exe .\cmd\agent-sfx
  ```

### 2. Environment Diagnostics & Audio Preview

- **Run Diagnostics**:
  ```bash
  ./bin/agent-sfx doctor
  ```
- **Preview Audio Playback**:
  ```bash
  ./bin/agent-sfx preview task_finished
  ```
  *(On Windows, run `.\bin\agent-sfx.exe doctor` and `.\bin\agent-sfx.exe preview task_finished`)*.

### 3. Gemini CLI Integration

- **Inspect Proposed Hook Installation (Dry-Run)**:
  ```bash
  ./bin/agent-sfx install gemini --dry-run
  ```
- **Install Hooks (Project Scope Default)**:
  ```bash
  ./bin/agent-sfx install gemini
  ```
- **Test Gemini Hook Receiver**:
  ```bash
  # Valid BeforeAgent turn: outputs "{}" and exits 0
  echo '{"hook_event_name": "BeforeAgent", "session_id": "test", "timestamp": "2026-10-04T12:00:00Z"}' | ./bin/agent-sfx hook gemini
  ```
- **Uninstall Hooks**:
  ```bash
  ./bin/agent-sfx uninstall gemini
  ```

### 4. Background Worker & Sound Controls

- **Start / Stop Worker Daemon**:
  ```bash
  ./bin/agent-sfx worker start
  ./bin/agent-sfx worker status
  ./bin/agent-sfx worker stop
  ```
- **Toggle Sound Playback**:
  ```bash
  ./bin/agent-sfx off    # Mutes sound playback and clears queue
  ./bin/agent-sfx on     # Re-enables sound playback
  ./bin/agent-sfx status # Checks configuration and live worker status
  ```

### 5. Claude Code Plugin & Local Node Launcher (Clone Usage)

- **Claude Code Plugin**: Located in `npm/claude/` with dedicated hooks, skills, prebuilt universal binaries, and CC0 starter sounds. See [`npm/claude/README.md`](npm/claude/README.md) and [`TESTING-CLAUDE-WINDOWS.md`](TESTING-CLAUDE-WINDOWS.md) for local installation and testing.
- **Local Node Launcher**: If running from a clone without installing the Go binary into your PATH, you can execute commands directly with Node: `node npm/bin/run.js <command>` (e.g. `node npm/bin/run.js doctor`). See [`npm/README.md`](npm/README.md).

For complete setup guides, troubleshooting, and architectural details, refer to:
- [`SETUP.md`](SETUP.md) — Comprehensive setup, installation, and uninstallation guide.
- [`SECURITY.md`](SECURITY.md) — Security policy, threat model, and IPC isolation.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — Contributor guide, coding standards, and tests.
- [`SOUND-LICENSES.md`](SOUND-LICENSES.md) — CC0 audio license and mathematical synthesis details.
