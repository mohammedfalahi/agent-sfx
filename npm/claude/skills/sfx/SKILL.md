---
name: sfx
description: Turn Agent SFX sounds on or off, check sound status, or manage audio playback via terminal commands.
---

# Agent SFX Control

Agent SFX is a local, open-source sound accessory for terminal coding agents.
It plays short, randomly selected sound effects for key agent moments.

## Instructions

When the user asks to manage Agent SFX sounds (e.g. "turn sound off", "mute sfx", "turn sound on", "unmute sfx", "sfx status", "are sounds enabled"):

1. Locate the executable runner relative to this skill file:
   - `<skill_dir>/../../bin/run.js`
2. Execute the appropriate command using the bash tool.

### Supported Commands

- **Turn sounds ON (unmute):**
  Run: `node "<skill_dir>/../../bin/run.js" on`
  Expected stdout: `Sound effects enabled`

- **Turn sounds OFF (mute):**
  Run: `node "<skill_dir>/../../bin/run.js" off`
  Expected stdout: `Sound effects disabled (muted)`

- **Check status:**
  Run: `node "<skill_dir>/../../bin/run.js" status`
  Expected output: current config status (enabled/disabled, volume, active worker PID).

Do not edit config files directly when executing on/off/status requests. Always use the CLI subcommands.
