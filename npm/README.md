# Agent SFX (@agent-sfx/agent-sfx)

Local sound accessory for terminal coding agents. Plays short, randomly selected sound effects for canonical agent moments.

## Prerequisites
- **macOS**: Apple Silicon (`darwin-arm64`) or Intel (`darwin-x64`)
- **Windows**: 64-bit (`win32-x64`)
- **Node.js**: >= 18.0.0
- **Network**: No additional post-install script downloads; npm package acquisition still requires standard package download unless installed from a local tarball.

## Quick Start

### Running from a Repository Clone
```bash
# Check environment, audio backend, and paths
node bin/run.js doctor

# View status of config and background worker
node bin/run.js status

# Toggle sound playback on or off
node bin/run.js on
node bin/run.js off

# Preview an event sound manually
node bin/run.js preview task_finished
```

### Windows PowerShell (from Clone)
```powershell
node bin\run.js doctor
node bin\run.js preview task_finished
node bin\run.js status
```

### When Installed via npm (or via npx)
Once published or installed via `npm install -g @agent-sfx/agent-sfx`:
```bash
agent-sfx doctor
agent-sfx preview task_finished
# Or via npx:
npx @agent-sfx/agent-sfx doctor
```

## Gemini CLI Integration
To preview proposed integration changes without modifying settings:
```bash
node bin/run.js setup gemini --dry-run
```

To install or uninstall hooks directly:
```bash
node bin/run.js install gemini
node bin/run.js uninstall gemini
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
