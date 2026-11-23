# Agent SFX for Claude Code: install and first-use guide

Agent SFX plays short sounds while you work with Claude Code: when you send a message, when Claude finishes, when Claude needs your permission, when Claude asks you a question, and when something fails. It runs entirely on your Mac, with no internet connection, no tracking, and no change to how Claude behaves.

This guide takes you from nothing installed to hearing your own custom sounds.

---

## Install status (checked 2026-11-23)

Each step below is marked:

- ✅ **Verified**: tested on a Mac with Claude Code 2.1.330, or checked against the project's code and the official Claude Code docs.
- 🧪 **Needs testing**: expected to work, but not yet confirmed end to end.

What has been tested so far:

- **Installing from GitHub, listing, and uninstalling** with the commands in this guide worked in a clean, throwaway Claude Code setup, using the real `mohammedfalahi/agent-sfx` repository. The installed copy kept its program files and permissions, and Claude Code listed its 7 hooks and the `/agent-sfx:sfx` command.
- **The hooks installed from GitHub** were run against recorded Claude Code events. Each event produced the right sound event, with the plugin's built-in sounds.
- **Sounds in a live Claude Code session:** the maintainer heard `task_started`, `task_finished`, `permission_requested`, `waiting_for_user`, and `error` for a failed tool, with the plugin loaded from a local copy (`claude --plugin-dir npm/claude`), not installed from GitHub.
- **Not yet tested live:** the `/agent-sfx:sfx` command, custom sounds and fallback, the silent session start, and a live session after installing from GitHub.

---

## 1. Before you start

### What you need

| Requirement | How to check | Status |
| --- | --- | --- |
| A Mac (Apple Silicon or Intel). Windows and Linux aren't supported by this plugin. | Apple menu → About This Mac | ✅ |
| Claude Code installed and signed in | Run `claude --version` in Terminal | ✅ |
| Sound on, with the right output device selected | Play any sound on your Mac | n/a |
| Node.js | **Not needed** | ✅ |

### Open Terminal

Press `⌘ Space`, type **Terminal**, and press Return. You'll type (or paste) the commands from this guide into that window.

### Check Claude Code

```bash
claude --version
```

You should see something like `2.1.330 (Claude Code)`. If you see `command not found`, install Claude Code first.

### Make a safe test folder

You'll use an empty folder for testing, so nothing Claude does during the tests touches your real work.

```bash
mkdir -p ~/agent-sfx-test
cd ~/agent-sfx-test
```

- `mkdir -p` creates the folder (it does nothing if the folder already exists).
- `cd` moves Terminal into it.

---

## 2. Install the plugin

A **marketplace** is a list of plugins that Claude Code knows where to download from. You add the Agent SFX marketplace once, then install the plugin from it.

- Marketplace name: `agent-sfx`
- Plugin name: `agent-sfx`
- Full plugin ID, used in commands: `agent-sfx@agent-sfx`

### Option A: from Terminal (recommended)

**Step 1. Add the marketplace.** ✅

```bash
claude plugin marketplace add mohammedfalahi/agent-sfx
```

This tells Claude Code to read the plugin list from the GitHub repository `github.com/mohammedfalahi/agent-sfx`. Nothing is installed yet. You should see `✔ Successfully added marketplace: agent-sfx`.

**Step 2. Install the plugin.** ✅

```bash
claude plugin install agent-sfx@agent-sfx
```

This downloads the plugin and turns it on for every project on your Mac (Claude Code calls this "user scope"). You should see `✔ Successfully installed plugin: agent-sfx@agent-sfx (scope: user)`.

**Step 3. Confirm it's installed and enabled.** ✅

```bash
claude plugin list
```

Look for `agent-sfx@agent-sfx` in the list, with `Version: 0.2.0` and `Status: ✔ enabled`. Plugins are enabled as soon as they're installed. If it shows as disabled, run:

```bash
claude plugin enable agent-sfx@agent-sfx
```

### Option B: from inside Claude Code

Start Claude Code, then type these one at a time:

```text
/plugin marketplace add mohammedfalahi/agent-sfx
/plugin install agent-sfx@agent-sfx
```

The second command opens a details panel where you choose where to install it. Pick the option for **you / all projects** (user scope). 🧪 *(the in-session version hasn't been tried)*

### Start a new session

Plugins load when Claude Code starts. Exit any open Claude Code session (type `/exit` or press `Ctrl+C` twice), then start a fresh one in your test folder:

```bash
cd ~/agent-sfx-test
claude
```

If you installed from inside an open session, you can type `/reload-plugins` instead of restarting. ✅ *(documented)*

> **Why no sound at startup?** That's intentional. When a session starts, Agent SFX quietly launches a small background helper (the "worker") that plays the sounds. There's no startup chime.

---

## 3. Check it's working, and turn sounds on or off

### Inside Claude Code (easiest)

Type any of these into Claude Code:

| What you type | What it does |
| --- | --- |
| `/agent-sfx:sfx status` | Shows whether sounds are on and whether the background helper is running |
| `/agent-sfx:sfx off` | Mutes all sounds (and stops a sound that's playing) |
| `/agent-sfx:sfx on` | Turns sounds back on |

Status: ✅ *(the command it runs was tested on an installed copy)* · 🧪 *(the `/agent-sfx:sfx` command itself hasn't been tried in a live session)*

Claude will ask permission to run one command. That's expected, so approve it. You'll probably hear the **permission** chime at that moment, which is a good first sign the plugin works.

Your on/off choice is saved, so it stays in effect in future sessions and for the Agent SFX Gemini CLI extension, if you use that too.

### From Terminal (optional)

The plugin's program lives inside Claude Code's plugin folder. To avoid typing the long path every time, save it in a shortcut variable for this Terminal window:

```bash
SFX="$HOME/.claude/plugins/cache/agent-sfx/agent-sfx/0.2.0/runtime/agent-sfx"
```

Status: ✅ *(this is where the test install put it; `status`, `on`, `off`, and `worker stop` were run from there)*

If you get `no such file or directory`, the version number may have changed after an update. Run `claude plugin list`, and put the version it shows in place of `0.2.0`.

Then:

```bash
"$SFX" status      # Is it on? Is the helper running? Which sounds folder is it using?
"$SFX" off         # Mute
"$SFX" on          # Unmute
"$SFX" doctor      # Health check: audio player, settings, sound files
"$SFX" preview task_finished   # Play one sound right now, without Claude
```

`preview` is the quickest way to answer "can my Mac play these sounds at all?". If `preview` is silent, check your Mac's volume and output device before anything else.

---

## 4. Use your own sounds

### Where the files go

Your sounds go in this folder, with **one subfolder per event**:

```text
~/Library/Application Support/agent-sfx/sounds/
├── task_started/          ← you send a message
├── task_finished/         ← Claude finishes its reply
├── permission_requested/  ← Claude asks permission to use a tool
├── waiting_for_user/      ← Claude asks you a multiple-choice question
└── error/                 ← a command or tool fails, or an API error ends the turn
```

The subfolder names must match exactly. `tests_passed` and `usage_exhausted` folders are allowed, but Claude Code never triggers them (see [Limitations](#7-current-limitations)).

**Create the folders and open them in Finder:**

```bash
cd "$HOME/Library/Application Support"
mkdir -p agent-sfx/sounds/task_started agent-sfx/sounds/task_finished agent-sfx/sounds/permission_requested agent-sfx/sounds/waiting_for_user agent-sfx/sounds/error
open agent-sfx/sounds
```

Now drag your sound files into the right subfolders in Finder. MP3s work as they are; there's no need to convert them.

### Supported formats ✅

You can use any of these. There's nothing to convert:

| Format | File endings |
| --- | --- |
| MP3 | `.mp3` |
| AAC | `.m4a`, `.aac` |
| AIFF | `.aif`, `.aiff` |
| Core Audio | `.caf` |
| WAV (16-bit) | `.wav` |

Every sound must also be **15 seconds or shorter**, and the file **25 MB or smaller**. Mono or stereo both work.

**How it works:** the first time Agent SFX plays a non-WAV sound, it converts a copy to WAV with macOS's built-in converter (`afconvert`). It keeps that copy in `~/Library/Caches/agent-sfx/cache`, so later plays are instant. Your original files are never changed, and no extra files appear in your sounds folders.

✅ *Checked with real MP3 and AAC files, through `preview` and through the background helper.*

**Not supported:** OGG, FLAC, and WAV files that aren't 16-bit (for example 24-bit or 32-bit float). Files with other endings are ignored. To use one of those, convert it to MP3 or WAV first, for example with the free `ffmpeg` tool: `ffmpeg -i my-sound.flac my-sound.mp3`.

**Keep sounds short.** Only one sound plays at a time, and any event that happens while a sound is playing is skipped. Clips under a second work best.

### How several sounds and fallback sounds work ✅

- **More than one file in a folder:** Agent SFX picks one at random each time, and never plays the same one twice in a row.
- **Empty or missing folder:** that event uses the plugin's built-in sound. So you can customize just one event (for example, only `task_finished`) and keep the built-in sounds for the rest.
- **A file that can't be played** (a damaged file, the wrong kind of WAV, or longer than 15 seconds): that event plays the **built-in sound** instead. If you keep hearing the built-in sound after adding your own, run `"$SFX" doctor`. It names the file and says why it can't be played.
- **Hidden files** (names starting with `.`) are ignored.

### Make Agent SFX notice a brand-new sounds folder ✅

The background helper picks its sounds folder when it starts. If `~/Library/Application Support/agent-sfx/sounds` didn't exist when the helper started, it keeps using the built-in sounds until it restarts. After creating the folder the first time:

1. Stop the helper: `"$SFX" worker stop`
2. Quit Claude Code and start a new session. The helper restarts by itself.

After that, adding, removing, or replacing files takes effect right away. A replaced file is converted again automatically.

`"$SFX" status` shows which sounds folder the helper is using.

### Use a different folder (optional)

To keep your sounds somewhere else, edit `~/Library/Application Support/agent-sfx/config.json` and set `"sounds_dir"` to the full path of your folder, then restart the helper as described above. Other settings in that file:

| Setting | Default | What it does |
| --- | --- | --- |
| `volume` | `0.6` | Agent SFX's volume, from `0.0` to `1.0`. It doesn't change your Mac's volume. |
| `cooldown_ms` | `1200` | Quiet gap after each sound, in milliseconds |
| `events` | all `true` | Set one to `false` to mute just that event |

Only add sounds you have the rights to use.

---

## 5. Live tests: one per event

Do these in your test folder (`~/agent-sfx-test`) in a fresh Claude Code session. Status: every event was checked with recorded Claude Code events on an installed copy. Tests 1–5 were also heard live by the maintainer with a local copy (✅ live below). 🧪 None has been run live after a GitHub install yet.

**Wait about 6 seconds between tests.** Two timing rules can make a test look broken when it isn't:

- **Quiet gap:** after any sound, there's a 1.2-second pause. Anything that happens during the pause is skipped, not saved for later.
- **No repeats within 5 seconds:** the same sound won't play twice for the same session within 5 seconds.

**Check the permission mode:** the permission test needs Claude Code to *ask* you before running commands, which it does by default. If you've turned on auto-accept, auto mode, or "bypass permissions", or you've always-allowed shell commands, you won't get the prompt and so won't hear that sound. Press `Shift+Tab` to cycle modes; the current mode is shown at the bottom of the screen.

### Test 1: message sent → `task_started` ✅ live

Type:

```text
Say hello in one short sentence.
```

**You should hear:** a quick rising chirp as soon as you press Return, then (test 2) a chord when Claude finishes.

### Test 2: reply finished → `task_finished` ✅ live

This plays at the end of test 1. **You should hear:** a bright three-note chord when Claude's reply ends.

If Claude answers in under about 1.5 seconds, the finish sound can land in the quiet gap after the start sound and be skipped. Try a slightly longer request, such as "Write a four-line poem about sound."

The finish sound means Claude's turn ended. It doesn't mean the task succeeded. It also doesn't play if you interrupt Claude with `Esc`.

### Test 3: permission needed → `permission_requested` ✅ live

Type:

```text
Use the Bash tool to run: mkdir agent-sfx-permission-test
```

**You should hear:** a two-note chime when the "Allow this command?" prompt appears. Choose **Yes**. The command only creates an empty folder inside your test folder. You'll also hear the finish chord when Claude wraps up.

### Test 4: Claude asks you a question → `waiting_for_user` ✅ live

Type:

```text
Use the AskUserQuestion tool to ask me to pick a color: red, green, or blue.
```

**You should hear:** a gentle double "ping" when the multiple-choice question appears. You should *not* also hear the permission chime for this. Pick any answer.

### Test 5: something fails → `error` ✅ live

Type:

```text
Use the Bash tool to run exactly: ls /this-folder-does-not-exist
```

If Claude asks permission first, you'll hear the permission chime; choose **Yes**. **You should hear:** a low, buzzy two-tone sound when the command fails, then the finish chord.

If you cancel a command yourself, no error sound plays.

### Test 6: session start (silent) 🧪

Start a new session, wait a few seconds, then type `/agent-sfx:sfx status`. **You should see** that the worker is running. You don't hear anything at session start.

### Test 7: mute and unmute 🧪

Type `/agent-sfx:sfx off`, wait 6 seconds, then repeat test 1. **You should hear nothing.** Type `/agent-sfx:sfx on`, wait 6 seconds, and repeat test 1. The sounds should be back.

### Not testable on purpose: API errors

The `error` sound also plays when a turn ends because of an API problem, such as a rate limit, a billing problem, sign-in trouble, or a server error. **Don't try to cause one.** This case is tested only with recorded examples.

### Clean up after testing

```bash
rm -r ~/agent-sfx-test
```

This deletes the test folder and the empty folder created in test 3. Only run it on the test folder.

---

## 6. Troubleshooting, updating, and uninstalling

### Troubleshooting

| What you see or hear | What to try |
| --- | --- |
| **No sounds at all** | 1) Run `"$SFX" preview task_finished`. If that's silent, check your Mac's volume and output device. 2) Run `/agent-sfx:sfx status` and make sure sounds are on. 3) Run `claude plugin list` and confirm the plugin is enabled. 4) Quit Claude Code and start a new session. |
| An error mentioning **"prebuilt binary not found"** | The installed copy is missing its program. Uninstall and reinstall the plugin (see below). If it still happens, report it to the project: the maintainer may need to publish the program again. |
| An error mentioning **"supports macOS only"** | You're not on a Mac. The Claude Code plugin works only on macOS. |
| `marketplace add` says **"Marketplace file not found"** | GitHub has an older copy of the project without the plugin list. Wait for the maintainer to publish it, then try again. |
| `marketplace add` says the marketplace **already exists** | You added it before. Skip to step 2, or run `claude plugin marketplace update agent-sfx`. |
| **You hear the built-in sound instead of yours** | Your file can't be played. Run `"$SFX" doctor`: it lists the file and the reason, such as a damaged file, an unsupported format, or a clip longer than 15 seconds. `"$SFX" preview <event>` shows the same warning. See [Supported formats](#supported-formats-). |
| **Custom sounds ignored; built-in ones still play** | The helper started before your sounds folder existed. Run `"$SFX" worker stop`, then start a new Claude Code session. Check the folder names are spelled exactly right. |
| **Some sounds are skipped** | That's the 1.2-second quiet gap and 5-second no-repeat rule. Use shorter clips, or lower `cooldown_ms` in `config.json`. |
| **No permission sound** | Claude didn't actually ask, because of auto-accept, bypass mode, or a saved "always allow" rule. |
| **Two finish sounds in a row** | Another plugin might be keeping Claude going after it stops. This is a known edge case. |
| `/agent-sfx:sfx` isn't recognized | The plugin isn't loaded. Run `/reload-plugins` or restart Claude Code, and check `claude plugin list`. |

`"$SFX" doctor` prints a health report you can share when asking for help. It contains file paths from your Mac, so check it before posting publicly.

### Updating 🧪

```bash
claude plugin marketplace update agent-sfx
claude plugin update agent-sfx@agent-sfx
"$SFX" worker stop
```

- The first line refreshes the plugin list from GitHub.
- The second installs the newer version.
- The third stops the old helper so the new one starts next time.

Then quit Claude Code and start a new session.

Updates aren't automatic for this marketplace. Claude Code only auto-updates official marketplaces unless you switch it on under `/plugin` → **Marketplaces**. After an update, the version number in the `SFX` path changes. Use the new version from `claude plugin list`.

### Uninstalling ✅

Run these in order:

```bash
"$SFX" worker stop
claude plugin uninstall agent-sfx@agent-sfx
claude plugin marketplace remove agent-sfx
```

1. Stops the background helper. Do this first, because the next step deletes the program.
2. Removes the plugin.
3. Removes the marketplace from Claude Code's list.

Claude Code may keep a copy of the plugin files in its cache for a while after you uninstall. That's normal, and Claude Code cleans it up itself.

**Optional, permanent:** delete your settings and custom sounds. This folder is also used by the Agent SFX Gemini CLI extension, so skip this step if you use that.

```bash
rm -r "$HOME/Library/Application Support/agent-sfx"
```

To turn the plugin off for **just one project** without uninstalling, add this to that project's `.claude/settings.json`:

```json
{ "enabledPlugins": { "agent-sfx@agent-sfx": false } }
```

---

## 7. Current limitations

**Platforms**

- **macOS only** (Apple Silicon and Intel). On Windows or Linux the plugin stops with "supports macOS only". The core Agent SFX program has Windows and Linux code, but the Claude Code plugin doesn't ship it.

**Events Claude Code doesn't trigger**

- **`tests_passed`**: no sound when tests pass in Claude Code. (This works only in the Gemini CLI version.)
- **`usage_exhausted`**: no special sound for running out of usage. Rate-limit and billing errors play the ordinary `error` sound on purpose, because they don't prove your allowance is used up.
- **Session start**: always silent, by design.
- No sounds for other Claude Code moments, such as background notifications, context compaction, subagents finishing, or the session ending.

**Behavior to expect**

- One sound at a time. Events during a sound or the 1.2-second gap afterwards are skipped, not queued.
- The same sound won't repeat within 5 seconds in the same session.
- No finish sound when you interrupt Claude with `Esc`, and no error sound when you cancel a command.
- Custom sounds can be MP3, AAC (`.m4a`, `.aac`), AIFF, CAF, or 16-bit WAV. OGG, FLAC, and 24-bit or 32-bit WAV aren't supported; a file that can't be played falls back to the built-in sound.
- The `/agent-sfx:sfx` controls need you to approve one shell command each time.

**Not yet verified**

- A live session after installing from GitHub. (The install itself is verified.)
- In a live session: the `/agent-sfx:sfx` controls, custom sounds and fallback to the built-in sounds, and the silent session start. (These were checked with automated tests and recorded events only.)
- The `error` sound for API errors (`StopFailure`). This is deliberately tested only with recorded examples.
- Installing from inside Claude Code with `/plugin install`, instead of from Terminal.
- Updating to a newer version (no newer version exists yet).

---

## For the maintainer: release checklist

GitHub install works when all of these are true on `master`:

- `.claude-plugin/marketplace.json` at the repository root names the marketplace `agent-sfx` and lists the plugin with `"source": "./npm/claude"`. `claude plugin marketplace add owner/repo` only reads the root file. `npm/claude/.claude-plugin/marketplace.json` (`agent-sfx-local`) stays for local installs and `setup deploy`, and the different name keeps the two from colliding.
- The Mac binaries in `npm/claude/runtime/darwin-arm64/` and `npm/claude/runtime/darwin-x64/` are committed. Claude Code copies the plugin from a git clone and runs no build step.

For every release:

1. Run `scripts/build-claude-plugin.sh` and commit the two binaries. The build is reproducible: unchanged source and the same Go version give identical files, so binaries only show up in the diff when the code changed.
2. Bump `version` in `npm/claude/.claude-plugin/plugin.json`, in both `marketplace.json` files, and in `AppVersion` (`internal/doctor/doctor.go`). Claude Code treats a pinned version as unchanged across commits, so `claude plugin update` won't deliver new code until the number changes. Update the version number in this guide's `SFX` path and in `npm/claude/README.md` too.
3. Run `go test ./...` (`internal/packaging` checks that the manifests agree, that the binaries are present, executable, not ignored by git, and report the plugin version), then `claude plugin validate .` and `claude plugin validate ./npm/claude`.
