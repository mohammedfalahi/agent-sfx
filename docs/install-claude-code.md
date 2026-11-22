# Agent SFX for Claude Code: install and first-use guide

Agent SFX plays short sounds while you work with Claude Code: when you send a message, when Claude finishes, when Claude needs your permission, when Claude asks you a question, and when something fails. It runs entirely on your Mac, with no internet connection, no tracking, and no change to how Claude behaves.

This guide takes you from nothing installed to hearing your own custom sounds.

---

## ⚠️ Read this first: install status (checked 2026-11-22)

**You can't install this from GitHub yet.** The steps in this guide describe how installation is *meant* to work. They won't work until the project maintainer publishes four fixes, listed in [For the maintainer](#for-the-maintainer-what-must-happen-before-github-install-works) at the end.

Here's what goes wrong today, in plain terms:

1. **Claude Code can't find the plugin in the repository.** Claude Code looks for a file called `.claude-plugin/marketplace.json` at the top level of the GitHub repository. This project keeps that file in a subfolder (`npm/claude/`), so the "add marketplace" step fails.
2. **The latest version isn't on GitHub.** The working version exists only on the maintainer's computer. GitHub still has an older version that needs Node.js and uses different files.
3. **The program the sounds depend on would be missing.** The sound program for Mac (`runtime/darwin-arm64/agent-sfx` and `runtime/darwin-x64/agent-sfx`) is deliberately excluded from GitHub. Even after the fixes above, an installed copy would have no program to run, and you'd hear nothing.
4. **Nobody has tested it in a real Claude Code session yet.** Automated tests pass, but no one has yet confirmed that the sounds play in a live session.

Each step below is marked:

- ✅ **Verified**: checked against the project's code, the official Claude Code docs, or Claude Code 2.1.330 on a Mac.
- 🧪 **Needs testing**: should work, but hasn't been tried end to end. Depends on the fixes above.

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

- Marketplace name: `agent-sfx-local`
- Plugin name: `agent-sfx`
- Full plugin ID, used in commands: `agent-sfx@agent-sfx-local`

### Option A: from Terminal (recommended)

**Step 1. Add the marketplace.** 🧪 *(Fails today. See the status note at the top.)*

```bash
claude plugin marketplace add mohammedfalahi/agent-sfx
```

This tells Claude Code to read the plugin list from the GitHub repository `github.com/mohammedfalahi/agent-sfx`. Nothing is installed yet.

**Step 2. Install the plugin.** 🧪

```bash
claude plugin install agent-sfx@agent-sfx-local
```

This downloads the plugin and turns it on for every project on your Mac (Claude Code calls this "user scope").

**Step 3. Confirm it's installed and enabled.** ✅ *(the command exists)*

```bash
claude plugin list
```

Look for `agent-sfx@agent-sfx-local` in the list, shown as enabled. Plugins are enabled as soon as they're installed. If it shows as disabled, run:

```bash
claude plugin enable agent-sfx@agent-sfx-local
```

### Option B: from inside Claude Code

Start Claude Code, then type these one at a time:

```text
/plugin marketplace add mohammedfalahi/agent-sfx
/plugin install agent-sfx@agent-sfx-local
```

The second command opens a details panel where you choose where to install it. Pick the option for **you / all projects** (user scope). 🧪

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

Status: 🧪 *(not tried in a live session yet)*

Claude will ask permission to run one command. That's expected, so approve it. You'll probably hear the **permission** chime at that moment, which is a good first sign the plugin works.

Your on/off choice is saved, so it stays in effect in future sessions and for the Agent SFX Gemini CLI extension, if you use that too.

### From Terminal (optional)

The plugin's program lives inside Claude Code's plugin folder. To avoid typing the long path every time, save it in a shortcut variable for this Terminal window:

```bash
SFX="$HOME/.claude/plugins/cache/agent-sfx-local/agent-sfx/0.1.0/runtime/agent-sfx"
```

Status: 🧪 *(path follows Claude Code's documented layout: `cache/<marketplace>/<plugin>/<version>/`)*

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

Now drag your sound files into the right subfolders in Finder.

### Supported format ✅

Agent SFX checks every file strictly. A file plays only if it is all of these:

- A **`.wav`** file
- **Uncompressed 16-bit PCM** (the plain, classic kind of WAV)
- **Mono or stereo**
- **15 seconds or shorter**, and **25 MB or smaller**

MP3, M4A, AAC, AIFF, 24-bit or 32-bit WAV, and "extensible" WAV files are **not** played.

**Converting a file:** if you have the free `ffmpeg` tool, this converts almost anything into the right format:

```bash
ffmpeg -i my-sound.mp3 -c:a pcm_s16le my-sound.wav
```

✅ *Checked: the output is plain 16-bit PCM WAV.*

> **Don't use macOS's built-in `afconvert` for this.** It produces an "extensible" WAV, which Agent SFX rejects, so the event goes silent. ✅ *Checked.*

If you prefer a free app, Audacity can export **WAV, signed 16-bit PCM** (🧪 not checked).

**Keep sounds short.** Only one sound plays at a time, and any event that happens while a sound is playing is skipped. Clips under a second work best.

### How several sounds and fallback sounds work ✅

- **More than one file in a folder:** Agent SFX picks one at random each time, and never plays the same one twice in a row.
- **Empty or missing folder:** that event uses the plugin's built-in sound. So you can customize just one event (for example, only `task_finished`) and keep the built-in sounds for the rest.
- **A file in the wrong format:** that event goes **silent**. It does *not* fall back to the built-in sound. If a custom sound never plays, check the format first.

### Make Agent SFX notice a brand-new sounds folder ✅

The background helper picks its sounds folder when it starts. If `~/Library/Application Support/agent-sfx/sounds` didn't exist when the helper started, it keeps using the built-in sounds until it restarts. After creating the folder the first time:

1. Stop the helper: `"$SFX" worker stop`
2. Quit Claude Code and start a new session. The helper restarts by itself.

After that, adding or removing files takes effect right away. If you **replace** a file, give the new one a new name so an old copy isn't reused.

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

Do these in your test folder (`~/agent-sfx-test`) in a fresh Claude Code session. Status for all of these: 🧪 *(none has been confirmed live yet)*.

**Wait about 6 seconds between tests.** Two timing rules can make a test look broken when it isn't:

- **Quiet gap:** after any sound, there's a 1.2-second pause. Anything that happens during the pause is skipped, not saved for later.
- **No repeats within 5 seconds:** the same sound won't play twice for the same session within 5 seconds.

**Check the permission mode:** the permission test needs Claude Code to *ask* you before running commands, which it does by default. If you've turned on auto-accept, auto mode, or "bypass permissions", or you've always-allowed shell commands, you won't get the prompt and so won't hear that sound. Press `Shift+Tab` to cycle modes; the current mode is shown at the bottom of the screen.

### Test 1: message sent → `task_started`

Type:

```text
Say hello in one short sentence.
```

**You should hear:** a quick rising chirp as soon as you press Return, then (test 2) a chord when Claude finishes.

### Test 2: reply finished → `task_finished`

This plays at the end of test 1. **You should hear:** a bright three-note chord when Claude's reply ends.

If Claude answers in under about 1.5 seconds, the finish sound can land in the quiet gap after the start sound and be skipped. Try a slightly longer request, such as "Write a four-line poem about sound."

The finish sound means Claude's turn ended. It doesn't mean the task succeeded. It also doesn't play if you interrupt Claude with `Esc`.

### Test 3: permission needed → `permission_requested`

Type:

```text
Use the Bash tool to run: mkdir agent-sfx-permission-test
```

**You should hear:** a two-note chime when the "Allow this command?" prompt appears. Choose **Yes**. The command only creates an empty folder inside your test folder. You'll also hear the finish chord when Claude wraps up.

### Test 4: Claude asks you a question → `waiting_for_user`

Type:

```text
Use the AskUserQuestion tool to ask me to pick a color: red, green, or blue.
```

**You should hear:** a gentle double "ping" when the multiple-choice question appears. You should *not* also hear the permission chime for this. Pick any answer.

### Test 5: something fails → `error`

Type:

```text
Use the Bash tool to run exactly: ls /this-folder-does-not-exist
```

If Claude asks permission first, you'll hear the permission chime; choose **Yes**. **You should hear:** a low, buzzy two-tone sound when the command fails, then the finish chord.

If you cancel a command yourself, no error sound plays.

### Test 6: session start (silent)

Start a new session, wait a few seconds, then type `/agent-sfx:sfx status`. **You should see** that the worker is running. You don't hear anything at session start.

### Test 7: mute and unmute

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
| An error mentioning **"prebuilt binary not found"** | The sound program is missing from the installed copy. This is blocker 3 in the status note at the top. It's not something you can fix; the maintainer has to publish the program. |
| An error mentioning **"supports macOS only"** | You're not on a Mac. The Claude Code plugin works only on macOS. |
| `marketplace add` says it **can't find a marketplace** | This is blocker 1 in the status note at the top. It's expected until it's fixed. |
| **One specific custom sound never plays** | The file is probably the wrong format (see [Supported format](#supported-format-)). Convert it with `ffmpeg`. Wrong-format files make that event silent. |
| **Custom sounds ignored; built-in ones still play** | The helper started before your sounds folder existed. Run `"$SFX" worker stop`, then start a new Claude Code session. Check the folder names are spelled exactly right. |
| **Some sounds are skipped** | That's the 1.2-second quiet gap and 5-second no-repeat rule. Use shorter clips, or lower `cooldown_ms` in `config.json`. |
| **No permission sound** | Claude didn't actually ask, because of auto-accept, bypass mode, or a saved "always allow" rule. |
| **Two finish sounds in a row** | Another plugin might be keeping Claude going after it stops. This is a known edge case. |
| `/agent-sfx:sfx` isn't recognized | The plugin isn't loaded. Run `/reload-plugins` or restart Claude Code, and check `claude plugin list`. |

`"$SFX" doctor` prints a health report you can share when asking for help. It contains file paths from your Mac, so check it before posting publicly.

### Updating 🧪

```bash
claude plugin marketplace update agent-sfx-local
claude plugin update agent-sfx@agent-sfx-local
"$SFX" worker stop
```

- The first line refreshes the plugin list from GitHub.
- The second installs the newer version.
- The third stops the old helper so the new one starts next time.

Then quit Claude Code and start a new session.

Updates aren't automatic for this marketplace. Claude Code only auto-updates official marketplaces unless you switch it on under `/plugin` → **Marketplaces**. After an update, the version number in the `SFX` path changes. Use the new version from `claude plugin list`.

### Uninstalling 🧪

Run these in order:

```bash
"$SFX" worker stop
claude plugin uninstall agent-sfx@agent-sfx-local
claude plugin marketplace remove agent-sfx-local
```

1. Stops the background helper. Do this first, because the next step deletes the program.
2. Removes the plugin.
3. Removes the marketplace from Claude Code's list.

**Optional, permanent:** delete your settings and custom sounds. This folder is also used by the Agent SFX Gemini CLI extension, so skip this step if you use that.

```bash
rm -r "$HOME/Library/Application Support/agent-sfx"
```

To turn the plugin off for **just one project** without uninstalling, add this to that project's `.claude/settings.json`:

```json
{ "enabledPlugins": { "agent-sfx@agent-sfx-local": false } }
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
- Custom sounds must be 16-bit PCM WAV files. Other formats make that event silent instead of falling back.
- The `/agent-sfx:sfx` controls need you to approve one shell command each time.

**Not yet verified**

- The GitHub install itself (see the status note at the top).
- Sounds firing at the right moments in a live Claude Code session.
- The `/agent-sfx:sfx` controls in a live session.
- The terminal shortcut path and the update and uninstall steps.

---

## For the maintainer: what must happen before GitHub install works

Checked on 2026-11-22. The local branch `master` and GitHub `origin/master` both point at `e18bab0`, and all Claude plugin changes since then are uncommitted.

1. **Add a marketplace file at the repository root.** Create `.claude-plugin/marketplace.json` at the top level of the repo, listing the plugin with `"source": "./npm/claude"`. The official docs say `claude plugin marketplace add` reads only `<repo root>/.claude-plugin/marketplace.json`, and has no option for a file in a subfolder. The alternative, an `extraKnownMarketplaces` settings entry with a custom `path`, means users have to edit settings by hand, so it isn't beginner-friendly. If you rename the marketplace (the current name `agent-sfx-local` and its description "Local Agent SFX Marketplace" read oddly for a GitHub install), update every command in this guide.
2. **Ship the Mac binaries.** `.gitignore` excludes `/npm/claude/runtime/darwin-*/`, so a GitHub install gets the launcher but not the binaries, and every hook fails with "prebuilt binary not found". Either commit the two binaries (about 6 MB each), or point the marketplace entry at a release archive (an `archive` source with a `sha256`).
3. **Commit and push the current work.** That includes the new `npm/claude/runtime/` launcher, the updated `hooks.json` and `SKILL.md`, `scripts/`, the deletion of the old `npm/claude/bin/`, and the related Go changes. GitHub currently serves the old plugin, which runs hooks through `node bin/run.js`.
4. **Bump `version` in `plugin.json` on each release.** The version is pinned at `0.1.0`, and Claude Code treats a pinned version as unchanged across commits, so `claude plugin update` won't deliver new code until the number changes.
5. **Live-test, then update this guide.** Run section 5 in a real session and switch the 🧪 marks to ✅ as each step is confirmed. `claude plugin validate ./npm/claude` passes today, with one warning about a missing plugin icon.
