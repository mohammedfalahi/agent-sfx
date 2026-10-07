# Agent SFX Setup and Installation Guide

Agent SFX is a local, open-source sound-only accessory for terminal coding agents. It plays short, randomly selected sound effects for seven canonical agent moments without modifying agent prompts, permissions, or outputs.

---

## Prerequisites

- **Go**: Version 1.22 or higher (required for building from source).
- **Node.js**: Version 18.0.0 or higher (required for npm launchers and Claude Code plugin execution).
- **Git**: Modern git client.
- **Operating Systems & Audio Backends**:
  - **macOS** (`darwin-arm64`, `darwin-x64`): Native playback via `/usr/bin/afplay`.
  - **Windows** (`win32-x64`): Native playback via PowerShell and `.NET SoundPlayer` with hidden console windows (`CREATE_NO_WINDOW`).
  - **Linux** (`linux-amd64`): Best-effort playback via `pw-play`, `paplay`, or `aplay`.

---

## 1. Building from Source

### macOS / Linux
```bash
# Clone the repository
git clone https://github.com/your-username/agent-sfx.git
cd agent-sfx

# Build the executable into bin/
go build -o bin/agent-sfx ./cmd/agent-sfx

# Verify binary execution
./bin/agent-sfx doctor
```

### Windows (PowerShell)
```powershell
# Clone the repository
git clone https://github.com/your-username/agent-sfx.git
Set-Location agent-sfx

# Build the executable into bin\
go build -o bin\agent-sfx.exe .\cmd\agent-sfx

# Verify binary execution
.\bin\agent-sfx.exe doctor
```

---

## 2. Pre-Installation Diagnostics and Verification

Before integrating Agent SFX with your agent, verify your local audio capabilities and sound directory:

### Run Diagnostics (`doctor`)
- macOS / Linux:
  ```bash
  ./bin/agent-sfx doctor
  ```
- Windows:
  ```powershell
  .\bin\agent-sfx.exe doctor
  ```

Doctor checks:
- Operating system and architecture.
- Available audio player backends (`afplay`, `powershell-soundplayer`, or Linux utilities).
- User configuration file path and active settings.
- Sound directory resolution and presence of valid WAV files for each canonical event.
- Running status of the background audio daemon.

### Preview Audio Playback
Test manual audio playback for each of the seven canonical events:
```bash
./bin/agent-sfx preview task_started
./bin/agent-sfx preview task_finished
./bin/agent-sfx preview permission_requested
./bin/agent-sfx preview waiting_for_user
./bin/agent-sfx preview tests_passed
./bin/agent-sfx preview usage_exhausted
./bin/agent-sfx preview error
```
*(On Windows, substitute `./bin/agent-sfx` with `.\bin\agent-sfx.exe`)*.

---

## 3. Gemini CLI Integration

Agent SFX integrates natively with Gemini CLI (v0.62.0+) using its hook system.

### Step 1: Inspect Proposed Changes (Dry-Run)
Inspect what hooks would be added to `.gemini/settings.json` without modifying any files:
```bash
./bin/agent-sfx install gemini --dry-run
```
Or run the two-way migration and conflict planner:
```bash
./bin/agent-sfx setup gemini --dry-run
```

### Step 2: Install Owned Hooks
Install the verified hook definitions into your project settings (default) or user settings (`--scope user`):
```bash
# Project scope (recommended: writes to .gemini/settings.json with .bak backup)
./bin/agent-sfx install gemini

# Or user scope (writes to ~/.gemini/settings.json)
./bin/agent-sfx install gemini --scope user
```

### Step 3: Verify Hook Receiver
Test the fail-open hook receiver with a mock prompt turn:
```bash
echo '{"hook_event_name": "BeforeAgent", "session_id": "test", "timestamp": "2026-10-04T12:00:00Z"}' | ./bin/agent-sfx hook gemini
```
Expected output: exactly `{}` on stdout and exit code `0`.

---

## 4. Claude Code Plugin Integration

Agent SFX provides a self-contained Claude Code plugin inside `npm/claude/` featuring native hook definitions, prebuilt universal binaries, and CC0 starter sounds.

### Step 1: Verify Plugin Diagnostics
```bash
node npm/claude/bin/run.js doctor
node npm/claude/bin/run.js preview task_finished
```

### Step 2: Register Local Marketplace & Install
Within your Claude Code terminal interface:
```text
/plugin marketplace add ./npm/claude
/plugin install agent-sfx@agent-sfx-local
```

### Step 3: Testing & In-Turn Clearance
- Agent SFX uses non-preemptive audio playback with a global cooldown (1,200 ms default).
- When running live tests in Claude Code, incorporate explicit timing (e.g. `sleep 22` or `Start-Sleep -Seconds 22`) between actions so the turn-start sound clears before completion sounds trigger.

---

## 5. Background Daemon and Runtime Controls

Audio playback runs in an isolated, singleton background worker daemon so hook execution remains non-blocking (<25ms).

### Worker Lifecycle Commands
```bash
# Start the background worker daemon manually
./bin/agent-sfx worker start

# Check worker daemon PID, uptime, and socket path
./bin/agent-sfx worker status

# Stop the worker daemon gracefully
./bin/agent-sfx worker stop
```
*(Note: With hooks installed, the worker starts automatically on `SessionStart` without delaying the agent).*

### Sound Playback Controls
Toggle playback at any time without terminating the daemon:
```bash
# Mute sound playback (cancels active audio and clears queue)
./bin/agent-sfx off

# Re-enable sound playback
./bin/agent-sfx on

# Check persisted config and live worker audio status
./bin/agent-sfx status
```

---

## 6. Configuration and Custom Sounds

### Configuration File
Settings are read from:
- **macOS**: `~/Library/Application Support/agent-sfx/config.json`
- **Windows**: `%LOCALAPPDATA%\agent-sfx\config.json`
- **Linux**: `~/.config/agent-sfx/config.json`

Example `config.json`:
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

### Adding Custom Sounds
1. Find your active sounds folder (reported by `agent-sfx doctor`).
2. Add uncompressed 16-bit PCM `.wav` files into the appropriate event subfolder:
   - `sounds/permission_requested/`
   - `sounds/task_started/`
   - `sounds/task_finished/`
   - `sounds/waiting_for_user/`
   - `sounds/tests_passed/`
   - `sounds/usage_exhausted/`
   - `sounds/error/`
3. If multiple WAV files exist in a folder, Agent SFX selects one randomly without immediate repeats.
4. **Limits**: Max duration 15,000 ms (15 seconds); max file size 25 MiB.

---

## 7. Clean Uninstallation and Rollback

### Uninstall Gemini CLI Hooks
```bash
# Remove project-scoped hooks
./bin/agent-sfx uninstall gemini

# Or remove user-scoped hooks
./bin/agent-sfx uninstall gemini --scope user
```
*The uninstaller removes only exact owned hooks whose binary paths match this installation, preserving user hooks and other settings.*

### Uninstall Claude Code Plugin
Inside Claude Code:
```text
/plugin uninstall agent-sfx@agent-sfx-local
/plugin marketplace remove agent-sfx-local
```

### Stop Background Daemon & Clean State
```bash
# Stop daemon
./bin/agent-sfx worker stop

# Remove local config and cache (optional)
# macOS:
rm -rf ~/Library/Application\ Support/agent-sfx
rm -rf ~/Library/Caches/agent-sfx

# Windows:
# Remove-Item -Recurse -Force "$env:LOCALAPPDATA\agent-sfx"
```
