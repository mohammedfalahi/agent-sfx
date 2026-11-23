# Agent SFX

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Audio: CC0-1.0](https://img.shields.io/badge/Audio-CC0_1.0-lightgrey.svg)](SOUND-LICENSES.md)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platforms](https://img.shields.io/badge/Platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey.svg)](#platform-support--limitations)
[![Architecture: Local-Only](https://img.shields.io/badge/Architecture-Local--Only-success.svg)](#architecture)

Agent SFX is a fast, local-only command-line accessory that provides distinct audio feedback for terminal coding agents. It plays short, non-intrusive sound effects for exactly seven canonical moments—such as when an agent starts a prompt turn, requests tool permission, asks an interactive question, finishes a turn, or finishes running test suites—without altering agent behavior, prompts, permissions, or context windows.

> **Getting Started**: For a complete step-by-step setup, configuration, and uninstallation walkthrough, see **[`SETUP.md`](SETUP.md)**.

---

## Features

- **Local & Private by Design**: Binds exclusively to local IPC endpoints (Unix domain sockets on macOS/Linux, local named pipes on Windows). Operates with zero network listeners, zero telemetry, zero analytics, and zero prompt or transcript retention.
- **Fail-Open Hook Contract**: Hook receivers read bounded stdin (max 1 MiB), communicate over IPC within a 25 ms deadline, and unconditionally exit `0` with neutral `{}` output. Sound accessory issues will never stall, crash, or interrupt coding agent sessions.
- **Dedicated Worker Daemon**: Playback runs asynchronously in a detached singleton background process acquired via OS-level file locking (`flock` on Unix, `LockFileEx` on Windows), preventing terminal audio lag.
- **Strict Non-Preemptive Scheduling**: At most one sound plays at any time. Rapid subsequent events are dropped immediately rather than queued into stale backlogs, with a configurable global cooldown (1,200 ms default).
- **Safe Child Execution**: All child processes use direct operating system argument arrays—zero shell string interpolation. Windows playback uses a fixed PowerShell script with hidden console windows (`CREATE_NO_WINDOW`) and environment variable path passing.
- **Audio Guardrails**: Enforces 16-bit PCM WAV validation (MP3, AAC, AIFF, and CAF are converted to WAV first on macOS), a 25 MiB maximum file size, and a strict 15-second (`15000` ms) hard duration ceiling.
- **100% CC0 Starter Sounds**: Bundles seven original, mathematically synthesized sound assets dedicated to the public domain under Creative Commons CC0 1.0 Universal. Users can easily drop in custom WAV audio files.

---

## Supported Product Events

Agent SFX uses a closed enumeration of seven canonical event kinds. No unprompted automatic sounds exist (no startup chimes, compaction noise, or periodic reminders).

| Canonical Event | Description | Default Sound Character |
| :--- | :--- | :--- |
| `permission_requested` | Agent pauses to ask for user tool confirmation | Melodic dual-tone chime (440 Hz → 554 Hz) |
| `task_started` | User prompt submitted; turn execution begins | Ascending frequency chirp sweep (330 Hz → 660 Hz) |
| `task_finished` | Agent response turn concludes (does not guarantee overall goal success) | Crisp major triad chord (523 Hz → 659 Hz → 784 Hz) |
| `waiting_for_user` | Agent explicitly requests user input via an interactive question dialog | Gentle double pulse ping (698 Hz) |
| `tests_passed` | Direct test runner completes with verified positive test summary | Triumphant 4-note ascending fanfare (392 Hz → 784 Hz) |
| `usage_exhausted` | Structured quota or billing limit reached | Descending 2-note minor chime (659 Hz → 523 Hz) |
| `error` | Agent tool failure, non-zero shell exit code, or verified runtime failure | Low dual-tone caution interval (220 Hz + 233 Hz) |

---

## Supported Agents

### 1. Gemini CLI (v0.62.0+)
Verified native hook integration. Agent SFX installs owned hooks into `.gemini/settings.json` (project scope default) or `~/.gemini/settings.json` (user scope).

| Gemini Hook Event | Payload Match Condition | SFX Event | Verification & Semantics |
| :--- | :--- | :--- | :--- |
| `BeforeAgent` | Any valid prompt turn | `task_started` | Direct turn submission |
| `AfterAgent` | `stop_hook_active == false` | `task_finished` | Turn completion only (not proof overall goal succeeded) |
| `Notification` | `notification_type == "ToolPermission"` | `permission_requested` | Tool permission dialog; suppressed if `details.type == "ask_user"` |
| `BeforeTool` | `tool_name == "ask_user"` (valid schema) | `waiting_for_user` | Validates questions array (1–4 items, non-empty labels); anchored `^ask_user$` |
| `AfterTool` | `tool_response.error` non-empty or non-zero exit | `error` | Validates error structure or non-zero `Exit Code: X` in trailing metadata |
| `AfterTool` | Direct `go test` or `pytest` with passed summary | `tests_passed` | Synchronous runner exit with positive summary; rejects compound commands |
| `SessionStart` | Startup / resume / clear | *(none)* | Silently launches background worker if offline without blocking agent |

### 2. Claude Code (macOS; formats checked against the 2.1.306 docs)
The Claude Code plugin lives in [`npm/claude/`](npm/claude/README.md). It is implemented and covered by automated tests, including an end-to-end run of the real hook commands against recorded payloads. In a live Claude Code session (loaded with `claude --plugin-dir npm/claude`), `task_started`, `task_finished`, `permission_requested`, `waiting_for_user`, and `error` for a failed tool were heard. The `/agent-sfx:sfx` skill, custom sounds and fallback, and the silent `SessionStart` are covered by automated tests but not yet confirmed live. The plugin needs no Node.js: hooks run a small shell launcher, and hook stdout is always empty, which Claude Code documents as a no-op.

| Claude Hook Event | Payload Match Condition | SFX Event | Verification & Semantics |
| :--- | :--- | :--- | :--- |
| `UserPromptSubmit` | Any valid prompt turn | `task_started` | Direct prompt submission |
| `Stop` | Any turn end (does not fire on user interrupt or API error) | `task_finished` | Turn completion only; `stop_hook_active` is not checked, so another plugin's blocking Stop hook can cause a second finish sound (5 s dedupe usually drops it) |
| `PermissionRequest` | `tool_name != "AskUserQuestion"` | `permission_requested` | Suppressed when presenting `AskUserQuestion` to avoid duplicate audio |
| `PreToolUse` | `tool_name == "AskUserQuestion"` (valid schema) | `waiting_for_user` | Validates question schema; all other tool calls return zero events |
| `PostToolUseFailure` | Non-empty `error` and `is_interrupt == false` | `error` | Tool failure; user cancellations (`is_interrupt == true`) produce no sound |
| `StopFailure` | `error` is one of the documented values (`rate_limit`, `billing_error`, `authentication_failed`, `server_error`, …) | `error` | Billing and rate-limit errors stay a general `error` on purpose; none is treated as `usage_exhausted` |
| `SessionStart` | Any valid session start | *(none)* | Silently launches background worker if offline without blocking agent |

---

## Installation

Agent SFX is designed to be installed **once per user machine**. You do **not** need to clone Agent SFX into each project, run `npm install`, add git submodules, or commit any Agent SFX files into your project repositories.

### Prerequisites
- **Go**: Version 1.22 or higher (to compile from source).
- **Node.js**: Version 18.0.0 or higher (required by the Gemini extension and npm launcher; the Claude Code plugin does not need Node).
- **Operating Systems**: macOS (Apple Silicon `darwin/arm64` or Intel `darwin/x64`), Windows 64-bit (`windows/amd64`), or Linux 64-bit (`linux/amd64`).

---

### Step 1: Building from Source

- **macOS / Linux**:
  ```bash
  git clone https://github.com/mohammedfalahi/agent-sfx.git
  cd agent-sfx
  go build -o bin/agent-sfx ./cmd/agent-sfx
  ```
- **Windows (PowerShell)**:
  ```powershell
  git clone https://github.com/mohammedfalahi/agent-sfx.git
  Set-Location agent-sfx
  go build -o bin\agent-sfx.exe .\cmd\agent-sfx
  ```

---

### Step 2: Deploy Self-Contained Packages (User-Wide)
Deploy self-contained extension and plugin packages to your permanent user application directory (`~/Library/Application Support/agent-sfx` on macOS, `%LOCALAPPDATA%\agent-sfx` on Windows). This completely decouples Agent SFX from your git clone so you can safely move or archive the repository:

- **macOS / Linux**:
  ```bash
  ./bin/agent-sfx setup deploy
  ```
- **Windows (PowerShell)**:
  ```powershell
  .\bin\agent-sfx.exe setup deploy
  ```

---

### Step 3: Activate User-Wide Integrations

#### Gemini CLI
Link the deployed user-wide extension into Gemini CLI:
- **macOS / Linux**:
  ```bash
  gemini extensions link "$HOME/Library/Application Support/agent-sfx/gemini-extension"
  ```
- **Windows (PowerShell)**:
  ```powershell
  gemini extensions link "$env:LOCALAPPDATA\agent-sfx\gemini-extension"
  ```

#### Claude Code
Recommended (macOS): install from GitHub. No clone or build needed. Beginner guide: [`docs/install-claude-code.md`](docs/install-claude-code.md).
```bash
claude plugin marketplace add mohammedfalahi/agent-sfx
claude plugin install agent-sfx@agent-sfx
```

For development, rebuild the plugin binaries and try it for one session without installing anything. See [`npm/claude/README.md`](npm/claude/README.md):
```bash
scripts/build-claude-plugin.sh
claude --plugin-dir npm/claude
```

> **Caution:** `setup deploy` rewrites **both** the deployed Gemini extension and the Claude plugin folders. If Gemini CLI is linked to the deployed extension, redeploying also replaces Gemini's files.

To install from the deployed copy instead:
- **macOS / Linux**:
  ```bash
  claude plugin marketplace add "$HOME/Library/Application Support/agent-sfx/claude-plugin" --scope user
  claude plugin install agent-sfx@agent-sfx-local --scope user
  ```
- **Windows (PowerShell)**:
  ```powershell
  claude plugin marketplace add "$env:LOCALAPPDATA\agent-sfx\claude-plugin" --scope user
  claude plugin install agent-sfx@agent-sfx-local --scope user
  ```

---

### Step 4: Install Safe User-Level CLI Shortcut (Enables Bare `agent-sfx` Command)
By default, the deployed packages live in your user application directory. To run bare `agent-sfx` commands (e.g. `agent-sfx on`, `agent-sfx off`, `agent-sfx status`, `agent-sfx doctor`) directly from any terminal prompt or within Gemini's subshell without `sudo` or admin privileges, create a user-level CLI shortcut to the permanent launcher:

- **macOS / Linux**:
  ```bash
  # Ensure ~/.local/bin exists
  mkdir -p ~/.local/bin

  # Create safe user-level symlink pointing to permanent launcher (no sudo required)
  ln -sf "$HOME/Library/Application Support/agent-sfx/gemini-extension/bin/run.js" ~/.local/bin/agent-sfx

  # Ensure ~/.local/bin is in your PATH (add to ~/.zshrc or ~/.bashrc if not already present)
  export PATH="$HOME/.local/bin:$PATH"
  ```
- **Windows (PowerShell)**:
  ```powershell
  # Create user-level agent-sfx.cmd in WindowsApps (already in user PATH, no Admin required)
  "@node `"$env:LOCALAPPDATA\agent-sfx\gemini-extension\bin\run.js`" %*" | Out-File -FilePath "$env:LOCALAPPDATA\Microsoft\WindowsApps\agent-sfx.cmd" -Encoding ascii
  ```

---

### Step 5: Restart Your Agent
Restart or open a new terminal session for Gemini CLI and Claude Code. The background audio worker will initialize silently on session start.

---

### Project Overrides
- **Gemini CLI**: If a workspace defines `.gemini/settings.json`, Gemini CLI merges project settings with user settings. Project-level hooks take precedence if conflicts exist.
- **Claude Code**: If a workspace defines `.claude/settings.json`, you can disable the user plugin for that workspace via `{"enabledPlugins": {"agent-sfx@agent-sfx-local": false}}`.

---

## Usage

> **Note**: Bare `agent-sfx` commands require installing the user-level CLI shortcut (Step 4 above). If the shortcut has not been installed yet, invoke commands using the permanent launcher (`"$HOME/Library/Application Support/agent-sfx/gemini-extension/bin/run.js"` on macOS/Linux or `& "$env:LOCALAPPDATA\agent-sfx\gemini-extension\bin\run.js"` on Windows) or `./bin/agent-sfx` from your clone.

### Command Summary

| Command | Description |
| :--- | :--- |
| `agent-sfx doctor` | Diagnoses OS, audio backend, config, worker daemon state, and sound assets |
| `agent-sfx preview <event>` | Manually plays a sound for one of the seven canonical events |
| `agent-sfx status` | Displays persisted configuration state and live running worker daemon status |
| `agent-sfx on` | Enables sound playback in configuration and re-enables live worker playback |
| `agent-sfx off` | Mutes sound playback, cancels active audio, and clears pending queues |
| `agent-sfx worker start` | Starts the detached background sound worker daemon |
| `agent-sfx worker status` | Inspects background worker daemon PID, uptime, and IPC socket |
| `agent-sfx worker stop` | Gracefully stops the active background worker daemon |
| `agent-sfx setup deploy` | Deploys self-contained packages to the permanent user application directory |
| `agent-sfx setup gemini --dry-run` | Inspects Gemini CLI integrations, extensions, and migration plans |
| `agent-sfx setup claude --dry-run` | Inspects Claude Code plugin integrations, marketplaces, and migration plans |
| `agent-sfx install gemini` | Merges owned hooks into `.gemini/settings.json` (supports `--scope project|user`, `--dry-run`) |
| `agent-sfx uninstall gemini` | Removes owned hooks cleanly while preserving user settings |
| `agent-sfx hook <agent>` | Neutral, fail-open hook receiver reading stdin from agent lifecycle events |

### Basic Workflow Example

```bash
# 1. Verify audio capabilities and event sounds
./bin/agent-sfx doctor

# 2. Preview sound playback
./bin/agent-sfx preview task_started
./bin/agent-sfx preview task_finished

# 3. Inspect proposed hook changes before touching settings
./bin/agent-sfx install gemini --dry-run

# 4. Install hooks into the current project (.gemini/settings.json)
./bin/agent-sfx install gemini

# 5. Verify the hook receiver with a simulated turn
echo '{"hook_event_name": "BeforeAgent", "session_id": "test", "timestamp": "2026-10-04T12:00:00Z"}' | ./bin/agent-sfx hook gemini
# Output is strictly "{}" with exit code 0
```

### Uninstallation and Rollback

To cleanly deactivate user-wide integrations:
```bash
# Deactivate Gemini CLI extension
gemini extensions uninstall agent-sfx

# Deactivate Claude Code plugin
claude plugin uninstall agent-sfx@agent-sfx-local --scope user
claude plugin marketplace remove agent-sfx-local --scope user

# Stop background worker daemon
agent-sfx worker stop
```

---

## Configuration

### Permanent Storage Locations

| Asset | macOS Path | Windows Path | Linux Path |
| :--- | :--- | :--- | :--- |
| **User Directory** | `~/Library/Application Support/agent-sfx` | `%LOCALAPPDATA%\agent-sfx` | `~/.config/agent-sfx` |
| **Config File** | `.../agent-sfx/config.json` | `...\agent-sfx\config.json` | `.../agent-sfx/config.json` |
| **Sounds Directory** | `.../agent-sfx/sounds/` | `...\agent-sfx\sounds\` | `.../agent-sfx/sounds/` |
| **Gemini Extension** | `.../agent-sfx/gemini-extension/` | `...\agent-sfx\gemini-extension\` | `.../agent-sfx/gemini-extension/` |
| **Claude Plugin** | `.../agent-sfx/claude-plugin/` | `...\agent-sfx\claude-plugin\` | `.../agent-sfx/claude-plugin/` |
| **IPC Endpoint** | `.../agent-sfx/worker.sock` | `\\.\pipe\agent-sfx-<UserSID>` | `.../agent-sfx/worker.sock` |

### Default `config.json`
```json
{
  "version": 1,
  "enabled": true,
  "volume": 0.6,
  "cooldown_ms": 1200,
  "max_clip_ms": 15000,
  "sounds_dir": "/path/to/agent-sfx/sounds",
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

### Configuration Parameters
- `enabled` *(boolean)*: Master switch for sound playback. Can be toggled live via `agent-sfx on` and `agent-sfx off`.
- `volume` *(float, `0.0`–`1.0`)*: Applied directly to PCM WAV samples; never modifies operating system master volume.
- `cooldown_ms` *(integer)*: Global quiet window (default `1200` ms) following playback to prevent rapid turn spam.
- `max_clip_ms` *(integer)*: Clip duration ceiling (default and hard maximum `15000` ms / 15 seconds). Any clip exceeding this value is rejected without truncation.
- `sounds_dir` *(string)*: Absolute path to the user's active sound directory. Preserved across updates.
- `events` *(map)*: Granular per-event enablement flags.

### Safely Adding Custom Sounds
Custom WAV audio files (e.g. meme clips, brand audio, or custom chimes) placed in the permanent sounds directory become immediately audible across **all** your terminal projects:

- **macOS / Linux**:
  ```bash
  cp my-sound.wav "$HOME/Library/Application Support/agent-sfx/sounds/task_finished/"
  ```
- **Windows (PowerShell)**:
  ```powershell
  Copy-Item my-sound.wav "$env:LOCALAPPDATA\agent-sfx\sounds\task_finished\"
  ```

*Rules: 16-bit PCM `.wav` works on every platform. On macOS, `.mp3`, `.m4a`, `.aac`, `.aif`, `.aiff`, and `.caf` also work: they're converted once with the built-in `/usr/bin/afconvert` into the Agent SFX cache, and the originals are never changed. Every sound must be at most 25 MiB and 15 seconds long. If multiple sounds exist in a folder, Agent SFX selects randomly without immediate repeats.*

*Your sounds always take priority over the bundled starter sounds. An event with no sound files in your folder, or whose chosen file can't be played, falls back to the bundled sound, so you can customize just one event. `agent-sfx doctor` names any file that can't be played and why, and `agent-sfx status` shows which folders the worker is using.*

---

## Architecture

Agent SFX cleanly decouples agent hook execution from audio playback:

```text
Agent Hook Event (JSON on stdin)
               │
               ▼
   Agent Adapter (gemini / claude)
  ┌────────────────────────────────────────────────────────┐
  │ Pure normalization: schema validation, event mapping,   │
  │ deduplication scoping, and diagnostic filtering        │
  └────────────────────────────┬───────────────────────────┘
                               │ Normalized Event (< 8 KiB)
                               ▼
                 Bounded IPC Handoff (≤ 25 ms)
   [Unix Domain Socket: 0700 / Windows Named Pipe: User DACL]
                               │
                               ▼
                Background Sound Worker Daemon
  ┌────────────────────────────────────────────────────────┐
  │ • Atomic Singleton Lock (flock / LockFileEx)           │
  │ • Priority Coalescing Window (100 ms)                  │
  │ • Deduplication Gate & Global Cooldown (1200 ms)       │
  │ • No-Repeat Random WAV File Selector                   │
  │ • 16-bit PCM Sample Volume Scaling                     │
  └────────────────────────────┬───────────────────────────┘
                               │
                               ▼
                    Native OS Audio Player
       [macOS: afplay | Windows: SoundPlayer | Linux: aplay]
```

### Key Architectural Invariants
1. **Hook Neutrality**: Hook handlers parse stdin defensively (1 MiB cap), forward normalized events over IPC, and exit `0` with `{}` within 25 ms.
2. **Worker Isolation**: The audio daemon runs as a detached child process with inherited stdio closed. If the worker is offline, hooks drop sound handoffs silently without blocking the agent.
3. **Non-Preemptive Playback**: New events arriving while audio is actively playing or during the cooldown window are dropped immediately.

---

## Development

### Repository Layout
```text
├── cmd/
│   ├── agent-sfx/         # CLI entry point and command routing
│   └── gen-sounds/        # Mathematical synthesizer for CC0 sound assets
├── internal/
│   ├── adapters/          # Agent hook adapters (gemini, claude)
│   ├── audio/             # Player abstraction and PCM gain scaling
│   ├── config/            # JSON config loader and file locking
│   ├── doctor/            # Environment diagnostic engine
│   ├── events/            # Closed 7-event canonical enumeration
│   ├── installer/         # Hook installer, backup, and uninstaller
│   ├── ipc/               # Unix domain socket and Windows named pipe transports
│   ├── preview/           # Direct manual sound playback
│   ├── scheduler/         # Deduplication, prioritization, and cooldown
│   ├── setup/             # Dry-run integration inspector
│   ├── sounds/            # Asset discovery and WAV validation
│   └── worker/            # Daemon lifecycle and singleton lock
├── npm/                   # npm distribution package & Gemini extension
│   ├── bin/               # Prebuilt platform binaries and Node launcher
│   ├── claude/            # Self-contained Claude Code plugin
│   └── sounds/            # Bundled CC0 1.0 Universal starter sounds
├── sounds/                # Active sound asset directory
└── testdata/              # Sanitized minimal JSON hook fixtures
```

### Reproducible Sound Synthesis
The starter sounds are generated mathematically without external audio files. To regenerate them:
```bash
go run ./cmd/gen-sounds
```

---

## Testing

Run unit, integration, and race detection suites:

```bash
# Run all Go test packages with race detector
go test -race ./...

# Run static checks and formatting verification
go vet ./...
gofmt -l .

# Test cross-compilation
GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/agent-sfx
GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/agent-sfx

# Test Node cross-platform launchers
node npm/test_launcher.js
```

---

## Platform Support & Limitations

| Operating System | Audio Backend | IPC Mechanism | Singleton Lock | Status |
| :--- | :--- | :--- | :--- | :--- |
| **macOS** (`arm64`, `x64`) | `/usr/bin/afplay` | Unix Domain Socket (`0600`) | `flock` | **Verified** on local hardware |
| **Windows** (`amd64` / `x64`) | PowerShell `SoundPlayer` | Named Pipe (Current User SID DACL) | `LockFileEx` | **Architecture Tested & Cross-Compiled** (Physical audio output pending physical Windows host) |
| **Linux** (`amd64`) | `pw-play` / `paplay` / `aplay` | Unix Domain Socket (`0600`) | `flock` | **Cross-Compiled** (Audio output is best-effort depending on desktop audio daemon) |

### Specific Platform Limitations:
1. **Windows Gemini Manual Hook Auto-Installation**: `agent-sfx install gemini` is currently blocked on Windows (`ErrWindowsUnsupported`). Use the recommended Gemini extension link (`gemini extensions link`) instead.
2. **Windows Physical Audio Playback**: While `agent-sfx.exe`, named-pipe IPC, and PowerShell `SoundPlayer` command generation pass automated unit suites, physical sound output and lock contention remain pending runtime testing on physical Windows hardware.
3. **Claude Code Live Verification**: The plugin hooks, launcher, and skill pass automated tests, including an end-to-end run of the real `hooks.json` commands. Live, with `claude --plugin-dir npm/claude`: `task_started`, `task_finished`, `permission_requested`, `waiting_for_user`, and `error` for a failed tool were heard. The `/agent-sfx:sfx` skill, custom sounds and fallback, the silent `SessionStart`, and a live session after a GitHub install are not yet verified live. `StopFailure` is deliberately tested only with recorded payloads. The Claude plugin targets macOS only.

---

## Contributing

Contributions are welcome! Please review **[`CONTRIBUTING.md`](CONTRIBUTING.md)** for toolchain requirements, code formatting, test conventions, and audio asset licensing rules.

All pull requests must maintain the closed seven-event model, ensure fail-open hook behavior, and pass `go test -race ./...` and `go vet ./...`.

---

## Security

Please see **[`SECURITY.md`](SECURITY.md)** for our security architecture, threat model, and vulnerability reporting procedures.

Agent SFX is strictly local-only and does not handle network traffic, tokens, or credentials. Discovered vulnerabilities should be reported privately to the maintainers rather than via public GitHub issues.

---

## License

- **Source Code**: [MIT License](LICENSE) © 2026 Agent SFX Contributors.
- **Starter Sound Assets**: [Creative Commons CC0 1.0 Universal Public Domain Dedication](SOUND-LICENSES.md).
