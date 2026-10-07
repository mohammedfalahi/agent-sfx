# Agent SFX (@agent-sfx/agent-sfx)

Local sound accessory for terminal coding agents. Plays short, randomly selected sound effects for canonical agent moments.

## Prerequisites
- **macOS**: Apple Silicon (`darwin-arm64`) or Intel (`darwin-x64`)
- **Windows**: 64-bit (`win32-x64`)
- **Node.js**: >= 18.0.0
- **Network**: No additional post-install script downloads; npm package acquisition still requires standard package download unless installed from a local tarball.

## Quick Start

### Basic Commands
```bash
# Check environment, audio backend, and paths
npx @agent-sfx/agent-sfx doctor

# View status of config and background worker
npx @agent-sfx/agent-sfx status

# Toggle sound playback on or off
npx @agent-sfx/agent-sfx on
npx @agent-sfx/agent-sfx off

# Preview an event sound manually
npx @agent-sfx/agent-sfx preview task_finished
```

### Windows PowerShell
```powershell
npx @agent-sfx/agent-sfx doctor
npx @agent-sfx/agent-sfx preview task_finished
npx @agent-sfx/agent-sfx status
```

## Gemini CLI Integration
To preview proposed integration changes without modifying settings:
```bash
npx @agent-sfx/agent-sfx setup gemini --dry-run
```

To install or uninstall hooks directly:
```bash
npx @agent-sfx/agent-sfx install gemini
npx @agent-sfx/agent-sfx uninstall gemini
```

## Claude Code Integration
This package bundles a self-contained Claude Code plugin in the `claude/` directory with hook definitions, skills, prebuilt binaries, and starter sounds. See `claude/README.md` for full plugin marketplace installation and testing instructions.

## Configuration
Configuration is stored in `config.json` inside your user config directory:
- macOS: `~/Library/Application Support/agent-sfx/config.json`
- Windows: `%LOCALAPPDATA%\agent-sfx\config.json`
- Linux: `~/.config/agent-sfx/config.json`

## License
- Code: MIT License (see `LICENSE`)
- Default Sound Assets: Creative Commons CC0 1.0 Universal (see `SOUND-LICENSES.md`)
