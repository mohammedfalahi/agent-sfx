# Agent SFX: Claude Code plugin

Plays short local sound effects when key things happen in a Claude Code session. Everything runs on your machine: no network, no telemetry, and Claude's behavior is unchanged.

**Status:** macOS only. Automated tests pass. In a live Claude Code session (loaded with `claude --plugin-dir npm/claude`), `task_started`, `task_finished`, `permission_requested`, `waiting_for_user`, and `error` for a failed tool were heard. Other items are listed under "What still needs live testing" below.

## What plays when

Sounds come from the plugin's hooks, which run automatically. You don't need to ask Claude for anything.

| You hear | When | Claude Code hook |
| --- | --- | --- |
| `task_started` | You submit a prompt | `UserPromptSubmit` |
| `task_finished` | Claude finishes responding (the turn ended, not proof the task succeeded) | `Stop` (doesn't fire if you interrupt) |
| `permission_requested` | Claude asks permission to use a tool | `PermissionRequest` (skipped for `AskUserQuestion`) |
| `waiting_for_user` | Claude asks you a multiple-choice question | `PreToolUse` on `AskUserQuestion` |
| `error` | A tool fails, such as a Bash command exiting non-zero | `PostToolUseFailure` (not when you cancel the tool) |
| `error` | The turn ends because of an API error (rate limit, billing, auth, server, and others) | `StopFailure` |
| *(nothing)* | A session starts; the background sound worker starts silently | `SessionStart` |

Not supported for Claude Code yet: `tests_passed` and `usage_exhausted`. Billing and rate-limit errors play the general `error` sound on purpose, because they don't prove that your allowance is used up.

Only one sound plays at a time, followed by a short quiet period (1.2 s by default). Events that arrive during a sound or the quiet period are skipped, not queued.

## Requirements

- macOS (Apple Silicon or Intel).
- Claude Code. The hook formats were checked against the documentation for 2.1.306.
- No Node.js needed: the plugin uses a small shell launcher (`runtime/agent-sfx`).

## Try it for one session (no installation)

From the repository root:

```bash
scripts/build-claude-plugin.sh          # rebuilds runtime/darwin-*/agent-sfx (only needed after Go changes)
npm/claude/runtime/agent-sfx doctor     # checks the audio player, config, and sounds
npm/claude/runtime/agent-sfx preview task_finished
claude --plugin-dir npm/claude          # loads the plugin for this session only
```

`--plugin-dir` doesn't install anything or change your settings; the plugin is gone when the session ends.

## Install from GitHub

No clone or build needed; the Mac binaries are committed.

```bash
claude plugin marketplace add mohammedfalahi/agent-sfx
claude plugin install agent-sfx@agent-sfx
```

Then start a new Claude Code session. Step-by-step guide with live tests: [`docs/install-claude-code.md`](../../docs/install-claude-code.md).

Uninstall:

```bash
"$HOME/.claude/plugins/cache/agent-sfx/agent-sfx/0.2.0/runtime/agent-sfx" worker stop
claude plugin uninstall agent-sfx@agent-sfx
claude plugin marketplace remove agent-sfx
```

## Install from a local folder

This folder also contains its own marketplace, `agent-sfx-local`. It's for development and for `agent-sfx setup deploy`:

```text
claude plugin marketplace add <path-to-npm/claude>
claude plugin install agent-sfx@agent-sfx-local
```

Uninstall with `claude plugin uninstall agent-sfx@agent-sfx-local`, then `claude plugin marketplace remove agent-sfx-local`, then `<path-to-npm/claude>/runtime/agent-sfx worker stop`.

## Controls

- In a terminal: `runtime/agent-sfx off`, `on`, and `status`. `off` also stops a sound that is playing.
- In Claude Code: `/agent-sfx:sfx off`, `/agent-sfx:sfx on`, `/agent-sfx:sfx status`. This skill runs only when you type it, and it uses one Bash command (Claude may ask permission). The skill is optional; sounds work without it.
- `status` shows which sounds folder the worker is using.

## Your own sounds

Put sound files in your sounds folder, one subfolder per event. MP3, AAC (`.m4a`, `.aac`), AIFF, CAF, and 16-bit WAV all work:

```text
~/Library/Application Support/agent-sfx/sounds/
  task_finished/   my-clip.mp3  another.wav
  error/           oops.m4a
```

- When an event has several files, one is picked at random, never the same one twice in a row.
- An event whose folder is empty or missing falls back to the plugin's bundled sound, so you can customize just one event.
- A file that can't be played (damaged, unsupported, or too long) also falls back to the bundled sound. `doctor` names the file and the reason.
- Non-WAV files are converted once with macOS's `/usr/bin/afconvert` into `~/Library/Caches/agent-sfx/cache`. Originals are never changed, and a replaced file is converted again.
- To use a different folder, set `sounds_dir` in `~/Library/Application Support/agent-sfx/config.json`.
- Every sound must be at most 15 seconds and 25 MiB. OGG, FLAC, and non-16-bit WAV aren't supported.
- Long clips mean more skipped sounds, because nothing else plays until a clip finishes.
- You're responsible for having the rights to sounds you add.

The same worker, config, and sounds folder are shared with the Agent SFX Gemini CLI extension, so `off` mutes both.

## What still needs live testing

Verified live with `claude --plugin-dir npm/claude`: the `UserPromptSubmit`, `Stop`, `PermissionRequest`, `PreToolUse` (`AskUserQuestion`), and `PostToolUseFailure` hooks fired at the moments described above, and their sounds were audible.

These have passed automated tests that use recorded hook payloads, but haven't been confirmed in a real Claude Code session:

- The `/agent-sfx:sfx` skill.
- Custom sounds in your sounds folder, and per-event fallback to the bundled sounds.
- `SessionStart` starting the worker silently.
- A live session after installing from GitHub (`agent-sfx@agent-sfx`).
- Cancelling a tool (`is_interrupt`) producing no sound.
- The hook cost per event. It measured about 70 ms median on a heavily loaded Mac, outside Claude Code.

`StopFailure` should be tested only with recorded payloads, never by provoking real API or billing errors.

## Plugin layout

```text
npm/claude/
├── .claude-plugin/plugin.json      plugin metadata
├── .claude-plugin/marketplace.json local marketplace (source "./")
├── hooks/hooks.json                hook registrations
├── skills/sfx/SKILL.md             optional /agent-sfx:sfx control skill
├── runtime/agent-sfx               POSIX sh launcher (picks the binary for this Mac)
├── runtime/darwin-arm64/agent-sfx  Go CLI binary for Apple Silicon Macs (committed)
├── runtime/darwin-x64/agent-sfx    Go CLI binary for Intel Macs (committed)
└── sounds/<event>/<event>.wav      the seven bundled starter sounds (CC0)
```

The launcher selects the matching Go CLI binary. The binaries handle the plugin's hooks and the `/agent-sfx:sfx` controls. They are built from this repository's Go source by `scripts/build-claude-plugin.sh` and committed so GitHub installs need no build step.

The launcher lives in `runtime/`, not `bin/`, because Claude Code adds a plugin's `bin/` folder to the Bash tool's PATH.

## License

Code: MIT (`LICENSE`). Bundled starter sounds: CC0 1.0 (`SOUND-LICENSES.md`).
