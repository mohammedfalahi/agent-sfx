# Agent SFX Setup and Installation Guide

Agent SFX is a local, open-source sound-only accessory for terminal coding agents. It plays short, randomly selected sound effects for seven canonical agent moments without modifying agent prompts, permissions, or outputs.

---

## Prerequisites

- **Go**: Version 1.22 or higher (required for building from source).
- **Node.js**: Version 18.0.0 or higher (required for Node launchers and Claude Code plugin execution).
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
git clone https://github.com/mohammedfalahi/agent-sfx.git
cd agent-sfx

# Build the executable into bin/
go build -o bin/agent-sfx ./cmd/agent-sfx

# Verify binary execution
./bin/agent-sfx doctor
```

### Windows (PowerShell)
```powershell
# Clone the repository
git clone https://github.com/mohammedfalahi/agent-sfx.git
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

## 3. Recommended: One-Time User-Wide Installation

Agent SFX is designed to be installed **once per user machine**. You do not need to clone the repository into each project or add any Agent SFX files to your project repositories.

### Step 1: Deploy Self-Contained Packages to User Location
Deploy the self-contained Gemini extension and Claude plugin to your permanent user application directory (`~/Library/Application Support/agent-sfx` on macOS, `%LOCALAPPDATA%\agent-sfx` on Windows):

- **macOS / Linux**:
  ```bash
  # Preview planned deployment without modifying files
  ./bin/agent-sfx setup deploy --dry-run

  # Deploy packages
  ./bin/agent-sfx setup deploy
  ```
- **Windows (PowerShell)**:
  ```powershell
  # Preview planned deployment
  .\bin\agent-sfx.exe setup deploy --dry-run

  # Deploy packages
  .\bin\agent-sfx.exe setup deploy
  ```

*This copies self-contained extension and plugin directories with their own binaries, hooks, and starter sounds into your user application directory. It never modifies your `config.json` or custom sound folders.*

### Step 2: Activate Gemini CLI Extension (User-Wide)
Link the deployed extension into Gemini CLI:

- **macOS / Linux**:
  ```bash
  gemini extensions link "$HOME/Library/Application Support/agent-sfx/gemini-extension"
  ```
- **Windows (PowerShell)**:
  ```powershell
  gemini extensions link "$env:LOCALAPPDATA\agent-sfx\gemini-extension"
  ```

Gemini CLI will now run Agent SFX hooks and provide the `/sfx` management skill across all terminal workspaces automatically.

### Step 3: Activate Claude Code Plugin (User-Wide)
Register the deployed plugin as a user-level marketplace in Claude Code:

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

### Understanding Project Overrides
- **Gemini CLI**: If a specific project defines `.gemini/settings.json`, Gemini CLI merges project settings with user settings. If project hooks conflict, project-specific settings take precedence.
- **Claude Code**: If a project defines `.claude/settings.json`, it can disable the user plugin for that specific workspace via:
  ```json
  {
    "enabledPlugins": {
      "agent-sfx@agent-sfx-local": false
    }
  }
  ```

---

## 4. Optional: Project-Scoped Manual Hook Installation

If you prefer not to use extensions/plugins and want to install manual command hooks directly into a specific project workspace:

### Inspect Proposed Project Changes (Dry-Run)
```bash
./bin/agent-sfx install gemini --scope project --dry-run
```

### Install Project Hooks
```bash
./bin/agent-sfx install gemini --scope project
```
*Note: This writes hook commands directly into `.gemini/settings.json` in the current working directory with an automatic `.bak` backup.*

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
*(Note: With the extension or plugin installed, the worker starts automatically on `SessionStart` without delaying the agent).*

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

### Preserving Existing Sound Directories & Adding Custom Sounds
- Your configured `sounds_dir` in `config.json` is always preserved and respected.
- To add custom sounds, place uncompressed 16-bit PCM `.wav` files into your configured event subfolders:
  - `sounds/permission_requested/`
  - `sounds/task_started/`
  - `sounds/task_finished/`
  - `sounds/waiting_for_user/`
  - `sounds/tests_passed/`
  - `sounds/usage_exhausted/`
  - `sounds/error/`
- If multiple WAV files exist in an event folder, Agent SFX selects one randomly without immediate repeats.
- **Constraints**: Maximum duration 15,000 ms (15 seconds); maximum file size 25 MiB.

---

## 7. Clean Uninstallation and Rollback

### Remove User-Wide Integrations
- **Gemini Extension**:
  ```bash
  gemini extensions unlink agent-sfx
  ```
- **Claude Code Plugin**:
  ```bash
  claude plugin uninstall agent-sfx@agent-sfx-local --scope user
  claude plugin marketplace remove agent-sfx-local --scope user
  ```

### Remove Project-Scoped Hooks (if installed)
```bash
./bin/agent-sfx uninstall gemini --scope project
```
*The uninstaller removes only exact verified owned hooks whose binary and command signatures match, preserving user-customized hooks and unrelated settings.*

### Stop Background Daemon & Clean State
```bash
# Stop daemon
./bin/agent-sfx worker stop

# Remove deployed packages (optional)
# macOS:
rm -rf ~/Library/Application\ Support/agent-sfx/gemini-extension
rm -rf ~/Library/Application\ Support/agent-sfx/claude-plugin

# Windows:
# Remove-Item -Recurse -Force "$env:LOCALAPPDATA\agent-sfx\gemini-extension"
# Remove-Item -Recurse -Force "$env:LOCALAPPDATA\agent-sfx\claude-plugin"
```

---

## 8. Platform Support & Verification Status

- **macOS (`arm64`, `x64`)**: Fully verified on local hardware (audio playback via `afplay`, Unix domain socket IPC, `flock` singleton locking, Gemini extension linking, and Claude plugin execution).
- **Linux (`amd64`)**: Cross-compilation verified. POSIX-compliant socket and locking primitives implemented; audio backend detection for `pw-play`, `paplay`, and `aplay`. Audio output is best-effort.
- **Windows (`amd64` / `x64`)**: Architecture implemented and cross-compiled (named-pipe IPC with user SID DACL, `LockFileEx` singleton locking, hidden PowerShell `SoundPlayer`). Node launcher tests pass. Physical audible playback and manual Gemini CLI hook auto-installation on Windows remain pending verification on physical Windows hardware.
