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
- **`agent-sfx hook gemini`**
  Verified hook receiver for Gemini CLI (v0.62.0). Reads stdin with time and size limits (max 1 MiB), maps verified events, emits exactly `{}` with a newline on stdout, and exits `0`.
- **`agent-sfx install gemini [--scope project|user] [--dry-run]`**
  Installs owned hook definitions into `.gemini/settings.json` (project scope default) or `~/.gemini/settings.json` (user scope). Features atomic writes, backup preservation (`settings.json.bak`), platform-safe shell quoting, and conflict detection.
- **`agent-sfx uninstall gemini [--scope project|user] [--dry-run]`**
  Removes only exact owned hooks whose commands match this installation. Preserves user-customized hooks and reports conflicts.

---

## Gemini Support Matrix (CLI v0.62.0)

| Gemini Hook Event | Payload Match Condition | Canonical SFX Event | Notes |
| :--- | :--- | :--- | :--- |
| `BeforeAgent` | Any valid prompt turn | `task_started` | User prompt submitted; turn starts |
| `AfterAgent` | `stop_hook_active == false` | `task_finished` | Agent response/turn complete (does not guarantee overall task success) |
| `Notification` | `notification_type == "ToolPermission"` | `permission_requested` | Agent pauses for user permission on a tool call |
| `AfterTool` | `tool_response.error` is non-empty, or `run_shell_command` non-zero `Exit Code` in trailing metadata | `error` | Tool execution failure across all tools, or verified non-zero shell exit code (not output text) |
| `AfterTool` | Verified direct `go test` or `pytest` invocation with positive test summary | `tests_passed` | Requires verified complete command execution and qualifying passed tests (M3) |
| `AfterTool` | Successful non-test tool result | *(none / dropped)* | Ordinary successful tool calls do not trigger completion sounds |
| `SessionStart` | `source: "startup"` | *(none)* | Silently launches background worker if offline without blocking agent |
| `BeforeTool` | Any | *(unsupported)* | Ignored |
| `AfterModel` | Any | *(unsupported)* | Ignored |
| `PreCompress` | Any | *(unsupported)* | Ignored |
| — | General API exhaustion | *(unsupported in M3)* | Unobserved API errors and quota are not guessed |
| — | Waiting for input | *(unsupported in M3)* | Generic wait signal not present in hook schema |

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

- **macOS (`darwin/arm64`)**: Fully verified on local hardware (audio playback via `afplay`, Unix domain socket IPC, `flock` singleton locking, process detachment, and settings installation).
- **Linux (`linux/amd64`)**: Cross-compilation verified. POSIX-compliant socket and locking primitives implemented.
- **Windows (`windows/amd64`)**: Cross-compilation verified. Background worker and hook installation remain disabled/stubbed pending Windows named-pipe IPC implementation. *Cross-compilation verifies only build compatibility, not runtime verification.*

---

## Quick Start & Verification

### Building
```bash
go build -o bin/agent-sfx ./cmd/agent-sfx
```

### Inspecting Proposed Hook Installation (Dry-Run)
```bash
./bin/agent-sfx install gemini --dry-run
```

### Installing Hooks
```bash
./bin/agent-sfx install gemini
```

### Testing Gemini Hook Receiver
```bash
# Valid BeforeAgent turn
echo '{"hook_event_name": "BeforeAgent", "session_id": "test", "timestamp": "2026-10-04T12:00:00Z"}' | ./bin/agent-sfx hook gemini
# Output is always "{}" with exit code 0
```

### Uninstalling Hooks
```bash
./bin/agent-sfx uninstall gemini
```
