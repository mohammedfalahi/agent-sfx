---
name: sfx
description: Turn Agent SFX sounds on or off, check sound status, or manage audio playback via terminal commands.
---

# Agent SFX Management Skill

Use this skill when the user asks to turn sounds on or off, mute audio, check sound playback status, or inspect Agent SFX.

## Operational Nature
- **Agent-Operated Controls**: These controls are operated by the assistant invoking the CLI binary through the shell execution tool (`run_shell_command`).
- **Not a Native Local Dialog**: This is an agent-operated tool invocation, not a native local UI settings selector (like `/model` or `/theme`).
- **Automatic Sound Independence**: This skill is strictly for user-requested sound toggling and status inspection. It is NOT required for automatic event detection or automatic audio playback.

## Instructions
When the user asks to turn sound effects on or off, or check sound status:
1. Resolve the executable:
   - Try `agent-sfx` first if available in PATH.
   - Otherwise, resolve the launcher relative to this skill's `<location>` tag:
     `<skill_dir>/../../bin/run.js` (e.g. `node "<skill_dir>/../../bin/run.js"`).
2. Execute the command via your shell tool (`run_shell_command`):
   - To turn sounds ON: `agent-sfx on` (or `node "<skill_dir>/../../bin/run.js" on`)
   - To turn sounds OFF / mute: `agent-sfx off` (or `node "<skill_dir>/../../bin/run.js" off`)
   - To inspect status: `agent-sfx status` (or `node "<skill_dir>/../../bin/run.js" status`)
3. **Execution Rules**:
   - Do NOT use interactive human-facing `!` syntax (e.g. `!agent-sfx on`) inside shell tool calls.
   - Report both the persisted configuration state and the live worker acknowledgment back to the user.
