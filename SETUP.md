# Agent SFX Setup and Installation Guide

Agent SFX is a local, open-source sound-only accessory for terminal coding agents. It plays short, randomly selected sound effects for seven canonical agent moments without modifying agent prompts, permissions, or outputs.

Agent SFX is designed to be installed **once per user machine**. You do **not** need to clone Agent SFX into each project, run `npm install`, add git submodules, or commit any Agent SFX files into your project repositories.

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

Follow this 5-step pipeline to activate Agent SFX across all terminal workspaces globally:

```text
Clone / Build ──► Setup Deploy ──► Link Gemini Ext ──► Register Claude Plugin ──► Restart Agent
```

### Step 1: Deploy Self-Contained Packages
Deploy self-contained extension and plugin packages to your permanent user application directory (`~/Library/Application Support/agent-sfx` on macOS, `%LOCALAPPDATA%\agent-sfx` on Windows). This decouples Agent SFX completely from your git checkout so you can safely move or archive your clone:

- **macOS / Linux**:
  ```bash
  # Preview planned deployment
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

*This deploys self-contained packages with their own universal binaries, hooks, and starter sounds into your user application directory. It never touches your `config.json` or custom sound folders.*

### Step 2: Activate Gemini CLI Extension (User-Wide)
Link the deployed user-wide extension into Gemini CLI:

- **macOS / Linux**:
  ```bash
  gemini extensions link "$HOME/Library/Application Support/agent-sfx/gemini-extension"
  ```
- **Windows (PowerShell)**:
  ```powershell
  gemini extensions link "$env:LOCALAPPDATA\agent-sfx\gemini-extension"
  ```

### Step 3: Activate Claude Code Plugin (User-Wide)
Register the deployed user-wide plugin as a local marketplace in Claude Code:

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

### Step 4: Restart Your Agent
Restart or open a new terminal session for Gemini CLI and Claude Code. On startup, the agent's `SessionStart` hook silently initializes the background audio daemon, enabling sound effects across all projects.

### Step 5: Verify Active Integration
Verify that both agents recognize the integration:
- Gemini CLI: Run `gemini extensions list` (should display `agent-sfx (0.1.0)` as linked and enabled).
- Claude Code: Run `claude plugin list` (should display `agent-sfx@agent-sfx-local` under `Scope: user` as `✔ enabled`).

---

## 4. Permanent Settings and Sound Locations

Agent SFX stores all configuration and user audio in standardized OS locations:

| Asset | macOS Path | Windows Path | Linux Path |
| :--- | :--- | :--- | :--- |
| **User Directory** | `~/Library/Application Support/agent-sfx` | `%LOCALAPPDATA%\agent-sfx` | `~/.config/agent-sfx` |
| **Config File** | `.../agent-sfx/config.json` | `...\agent-sfx\config.json` | `.../agent-sfx/config.json` |
| **Sounds Directory** | `.../agent-sfx/sounds/` | `...\agent-sfx\sounds\` | `.../agent-sfx/sounds/` |
| **Gemini Extension** | `.../agent-sfx/gemini-extension/` | `...\agent-sfx\gemini-extension\` | `.../agent-sfx/gemini-extension/` |
| **Claude Plugin** | `.../agent-sfx/claude-plugin/` | `...\agent-sfx\claude-plugin\` | `.../agent-sfx/claude-plugin/` |
| **IPC Endpoint** | `.../agent-sfx/worker.sock` | `\\.\pipe\agent-sfx-<UserSID>` | `.../agent-sfx/worker.sock` |

### Configuration (`config.json`)
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

### Safely Adding Custom Sounds
Your configured `sounds_dir` in `config.json` is always preserved. To add custom WAV audio files (e.g. meme clips or custom chimes) to the permanent user sound directory:

- **macOS / Linux**:
  ```bash
  # Copy custom audio to an event subfolder
  cp my-sound.wav "$HOME/Library/Application Support/agent-sfx/sounds/task_finished/"
  ```
- **Windows (PowerShell)**:
  ```powershell
  # Copy custom audio to an event subfolder
  Copy-Item my-sound.wav "$env:LOCALAPPDATA\agent-sfx\sounds\task_finished\"
  ```

*Sound rules: Must be uncompressed 16-bit PCM `.wav` format, maximum 25 MiB in file size, and maximum 15 seconds (15,000 ms) in duration. If multiple sounds exist in a folder, Agent SFX selects randomly without immediate repeats.*

---

## 5. Project Overrides and Workspace Settings

Because Agent SFX is installed user-wide, project repositories stay clean. However, workspaces can customize behavior when needed:

- **Gemini CLI Project Overrides**:
  If a workspace defines `.gemini/settings.json`, Gemini CLI merges project settings with user settings. If conflicting hook definitions exist, the project workspace settings take precedence.
- **Claude Code Project Overrides**:
  A specific workspace can disable the user-wide plugin without affecting other projects by adding an override to `.claude/settings.json`:
  ```json
  {
    "enabledPlugins": {
      "agent-sfx@agent-sfx-local": false
    }
  }
  ```

---

## 6. Optional: Project-Scoped Manual Hook Installation

If you specifically require manual command hooks written directly into a project's `.gemini/settings.json` (instead of using the user-wide extension):

```bash
# Inspect proposed changes
./bin/agent-sfx install gemini --scope project --dry-run

# Install hooks into project settings
./bin/agent-sfx install gemini --scope project
```
*Note: The default scope for `install` and `uninstall` remains explicitly `--scope project`.*

---

## 7. Controls and Background Daemon Management

The audio daemon runs in an isolated, singleton background process. You can control playback at any time without terminating the daemon:

### Audio Toggle Commands
```bash
# Mute playback (clears queue and stops active audio)
agent-sfx off

# Re-enable playback
agent-sfx on

# Check persisted config and live worker state
agent-sfx status
```
*(You can also use the `/sfx` skill directly within Gemini CLI or Claude Code chat sessions)*.

### Daemon Lifecycle Commands
```bash
# Start background worker daemon manually
agent-sfx worker start

# Check worker daemon PID, uptime, and socket
agent-sfx worker status

# Stop background worker daemon gracefully
agent-sfx worker stop
```

---

## 8. Clean Uninstallation and Rollback

### Remove User-Wide Integrations
- **Gemini CLI Extension**:
  ```bash
  gemini extensions uninstall agent-sfx
  ```
- **Claude Code Plugin**:
  ```bash
  claude plugin uninstall agent-sfx@agent-sfx-local --scope user
  claude plugin marketplace remove agent-sfx-local --scope user
  ```

### Remove Project-Scoped Hooks (if manually installed)
```bash
./bin/agent-sfx uninstall gemini --scope project
```
*The uninstaller removes only exact verified owned hooks whose binary signature and command match, preserving user-customized hooks and unrelated settings.*

### Stop Daemon and Remove Deployed Packages
```bash
# Stop background daemon
agent-sfx worker stop

# Remove deployed extension and plugin packages (optional)
# macOS:
rm -rf ~/Library/Application\ Support/agent-sfx/gemini-extension
rm -rf ~/Library/Application\ Support/agent-sfx/claude-plugin

# Windows:
# Remove-Item -Recurse -Force "$env:LOCALAPPDATA\agent-sfx\gemini-extension"
# Remove-Item -Recurse -Force "$env:LOCALAPPDATA\agent-sfx\claude-plugin"
```

---

## 9. Platform Support & Honest Limitations Matrix

| Platform | Gemini CLI Integration | Claude Code Integration | Audio Backend | Verification Status |
| :--- | :--- | :--- | :--- | :--- |
| **macOS** (`arm64`, `x64`) | `gemini extensions link` | `claude plugin ... --scope user` | Native `afplay` | **Fully Verified** on local hardware |
| **Linux** (`amd64`) | `gemini extensions link` | `claude plugin ... --scope user` | `pw-play` / `paplay` / `aplay` | **Cross-Compiled** (Audio output is best-effort depending on desktop audio daemon) |
| **Windows** (`amd64` / `x64`) | `gemini extensions link` (Extension only) | `claude plugin ... --scope user` | PowerShell `SoundPlayer` | **Architecture Tested & Cross-Compiled** (Physical audio output and manual Gemini hook auto-installation unverified on Windows) |

### Specific Platform Limitations:
1. **Windows Gemini Manual Hook Auto-Installation**: `agent-sfx install gemini` is currently blocked on Windows (`ErrWindowsUnsupported`). Use the recommended Gemini extension link (`gemini extensions link`) instead.
2. **Windows Physical Audio Playback**: While `agent-sfx.exe`, named-pipe IPC, and PowerShell `SoundPlayer` command generation pass automated unit suites, physical sound output and lock contention remain pending runtime testing on physical Windows hardware.
3. **Claude Code Live Turn Verification**: Adapter normalization, plugin packaging, and universal launchers pass automated test suites; live hook triggering during an active authenticated turn is pending physical tester access.
