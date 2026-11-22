---
name: sfx
description: Turn Agent SFX sound effects on or off, or show whether they are enabled. Use only when the user explicitly asks to mute, unmute, or check Agent SFX sounds.
argument-hint: "[on|off|status]"
disable-model-invocation: true
---

# Agent SFX controls

Agent SFX plays short sounds for Claude Code moments (prompt submitted, turn finished, permission request, question, tool or API error). Its hooks detect those moments automatically. This skill only switches sounds on or off and reports status. It is not needed for sounds to play.

The user's request is: `$ARGUMENTS`

## What to do

1. Map the request to exactly one subcommand:
   - `on`: unmute or enable sounds
   - `off`: mute or disable sounds (also stops a sound that is playing now)
   - `status`: show whether sounds are enabled and whether the background worker is running
   If the request is empty or unclear, use `status`.
2. Run exactly one Bash command, keeping the quotes:

   ```
   "${CLAUDE_SKILL_DIR}/../../runtime/agent-sfx" <subcommand>
   ```

   Never add other arguments or text from the request to the command.
3. Report the result in one or two plain sentences, based on the actual output:
   - `on` and `off` print `Persisted config : enabled|disabled (in <path>)`, followed by `Worker daemon    : acknowledged [...]`, or a line saying the worker is not running (the setting then applies the next time it starts).
   - `status` prints an `Agent SFX Status` block with `Persisted Config`, `Worker Daemon`, and, when the worker is running, `Worker State` and `Worker Sounds`.
   - If the command fails, say so and quote the error line. Do not retry with other commands.

Do not edit Agent SFX config files directly. The user can do the same thing from a terminal with the `agent-sfx on|off|status` command.
