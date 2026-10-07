# Agent SFX — Claude Code Plugin

A self-contained Claude Code plugin providing local audio sound effects for terminal coding moments.

---

## Supported Events

The plugin maps Claude Code hook lifecycle events to canonical SFX moments:
- `UserPromptSubmit` → `task_started`
- `Stop` (normal completion) → `task_finished`
- `PermissionRequest` (excluding `AskUserQuestion`) → `permission_requested`
- `PreToolUse` (`AskUserQuestion`) → `waiting_for_user`
- `PostToolUseFailure` (excluding user cancellation interrupts) → `error`
- `StopFailure` (exact match against verified 10-value enum) → `error`
- `SessionStart` → silent background worker startup

---

## Directory Structure

```text
claude/
├── .claude-plugin/
│   ├── marketplace.json   # Local plugin marketplace manifest
│   └── plugin.json        # Plugin metadata
├── bin/
│   ├── darwin-arm64/      # Prebuilt macOS Apple Silicon binary
│   ├── darwin-x64/        # Prebuilt macOS Intel binary
│   ├── windows-amd64/     # Prebuilt Windows x64 binary
│   └── run.js             # Cross-platform Node launcher
├── hooks/
│   └── hooks.json         # Claude Code hook registrations
├── skills/
│   └── sfx/SKILL.md       # Agent-operated control skill
├── sounds/                # Bundled CC0 1.0 Universal starter sounds
├── LICENSE                # MIT License
└── SOUND-LICENSES.md      # Audio provenance and CC0 dedication
```

---

## Prerequisites

- **Claude Code**: Version 2.1.162 or higher.
- **Node.js**: Version 18.0.0 or higher (available in `PATH`).
- **Operating System**: macOS (`darwin-arm64`, `darwin-x64`) or Windows (`win32-x64`).

---

## Testing & Installation Workflow

### 1. Pre-Installation Diagnostics
Before installing the plugin into Claude Code, test the packaged binary and sound playback directly:

- macOS / Linux:
  ```bash
  node bin/run.js doctor
  node bin/run.js preview task_finished
  ```

- Windows PowerShell:
  ```powershell
  node bin\run.js doctor
  node bin\run.js preview task_finished
  ```

### 2. Registering as a Local Marketplace Plugin
Inside Claude Code:

```text
/plugin marketplace add <path-to-this-directory>
/plugin install agent-sfx@agent-sfx-local
```

### 3. Verification & In-Turn Clearance
- Agent SFX enforces a cooldown window and non-preemptive audio playback.
- When conducting live verification, incorporate explicit in-turn clearance timing (e.g. `sleep 22` on macOS or `Start-Sleep -Seconds 22` on Windows) to allow previous turn sounds to finish completely.

### 4. Uninstallation / Rollback
To cleanly remove the plugin from Claude Code:

```text
/plugin uninstall agent-sfx@agent-sfx-local
/plugin marketplace remove agent-sfx-local
```

---

## License

- Code: MIT License (see `LICENSE`)
- Starter Sound Assets: Creative Commons CC0 1.0 Universal (see `SOUND-LICENSES.md`)
