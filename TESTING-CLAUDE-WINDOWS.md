# Agent SFX — Windows Host Testing Guide (Claude Code Integration)

This guide provides instructions for an external tester on native 64-bit Windows (`x86-64 / amd64`) to test real Claude Code sound events using their own account.

---

## 1. Prerequisites & Environment

1. **Operating System**: Windows 10, Windows 11, or Windows Server 2019+ (64-bit `x86-64 / amd64`).
   - *Note*: Windows on ARM64 is not supported in this release.
2. **Account Permissions**: **Standard User account only**. Do **NOT** run PowerShell as Administrator.
3. **Audio Hardware**: Working speakers or headphones with Windows audio enabled and unmuted.
4. **Node.js**: Node.js `>= 18.0.0` installed and available in system `PATH` (`node -v`).
5. **Claude Code CLI**: Installed and authenticated with active model access (`claude --version`).
6. **Distribution Mode**: This guide uses the self-contained offline testing bundle. Do **NOT** use `npm install` or `npx` (the package is not publicly published).

---

## 2. Unpack the Testing Bundle

Extract the testing ZIP bundle into a directory of your choice, for example:
`C:\Users\<YourUsername>\AgentSFX-Tester\`

Open a standard **PowerShell** prompt (not as Admin) and navigate to the extracted directory:
```powershell
Set-Location -Path "$HOME\AgentSFX-Tester"
```

Verify that the bundle contents are present:
```powershell
Get-ChildItem -Name
# Expected files:
#   agent-sfx.exe
#   claude-plugin/
#   sounds/
#   testdata/
#   TESTING-CLAUDE-WINDOWS.md
#   SHA256SUMS.txt
#   LICENSE
#   SOUND-LICENSES.md
```

---

## 3. Pre-Installation Diagnostics & Audible Preview

Before installing the Claude plugin, verify that the native Windows audio backend and background worker can run on your system.

### 3.1 Run Doctor Diagnostics
```powershell
.\agent-sfx.exe doctor
```
**Expected Output:**
- `Environment`: `windows/amd64`
- `Audio Player`: `powershell-soundplayer [OK] (native powershell-soundplayer backend available)`
- `Worker Daemon`: `stopped`
- `Event Sounds Status`: 7 events listed with `1 sound(s) [OK]`
- `Warnings / Diagnosed Issues`: None

### 3.2 Verify Audible Sound Playback
Test audio playback through Windows PowerShell SoundPlayer:
```powershell
.\agent-sfx.exe preview task_finished
```
**Observation Checklist:**
- [ ] You hear a short pleasant chime/major chord.
- [ ] **No console window flashes** on screen during playback (process runs with `CREATE_NO_WINDOW`).
- [ ] Output displays: `[OK] Played task_finished using powershell-soundplayer ...`

Test a second event:
```powershell
.\agent-sfx.exe preview permission_requested
```
- [ ] You hear a distinct rising two-tone prompt.

---

## 4. Claude Code Plugin Installation (Project Scope)

We recommend testing within an isolated test project directory rather than modifying global configuration.

### 4.1 Create an Isolated Test Project
In PowerShell:
```powershell
$TestDir = Join-Path $HOME "AgentSFX-TestProject"
New-Item -ItemType Directory -Path $TestDir -Force | Out-Null
Set-Location -Path $TestDir

# Initialize git repository (Claude Code expects a workspace)
git init
"Test project for Agent SFX" | Out-File -FilePath "README.md" -Encoding utf8
git add README.md
git commit -m "Initial commit"
```

### 4.2 Register the Local Marketplace
Resolve the absolute path to the extracted `claude-plugin` directory:
```powershell
$PluginDir = Join-Path $HOME "AgentSFX-Tester\claude-plugin"
claude plugin marketplace add "$PluginDir" --scope project
```
*Expected*: Claude Code confirms adding the local marketplace.

### 4.3 Install the Plugin
```powershell
claude plugin install agent-sfx@agent-sfx-local --scope project
```
*Expected*: Claude Code reports that `agent-sfx` is installed for this project.

> **Trust Warning Notice**: When Claude Code installs a local plugin with hooks and commands, it may display a security prompt asking to trust tools and hooks from the plugin. Select **Allow / Trust** for this test workspace.

---

## 5. Live Claude Code Verification Protocol

### Invariant: Turn Timing & In-Turn Clearance
- When you submit a prompt to Claude Code, `UserPromptSubmit` triggers `task_started` immediately.
- The default cooldown gate is **1200ms**, and audio clips last 1 to 2 seconds.
- Furthermore, models stream text before executing tools. To ensure that tool events (`error`, `waiting_for_user`) do not collide with earlier sounds, test prompts instruct Claude Code to execute a delay (`Start-Sleep -Seconds 22`) before the target action.

Start Claude Code in the test directory:
```powershell
claude
```

---

### Test 1: Turn Start & Turn Finished
**Action in Claude Code prompt:**
```text
Please run: powershell -Command "Start-Sleep -Seconds 22; Write-Output 'Done'"
```

**Observation Checklist:**
1. At the moment you press Enter, **`task_started` plays** (short affirmative sound).
2. The background worker is automatically started by `SessionStart` (if not already running).
3. Claude Code runs the 22-second sleep.
4. When Claude Code completes the turn and yields the prompt back to you, **`task_finished` plays** (pleasant resolution sound).

---

### Test 2: Tool Failure Detection
**Action in Claude Code prompt:**
```text
First run powershell -Command "Start-Sleep -Seconds 22". After that finishes, execute a non-existent command to trigger an error: nonexistent_command_xyz_12345
```

**Observation Checklist:**
1. `task_started` plays at prompt submission.
2. The turn sleeps for 22 seconds.
3. The shell command fails with a non-zero exit code.
4. **`error` sound plays** (descending alert sound).
5. No extraneous sound plays.

---

### Test 3: Interactive Question Dialog (`AskUserQuestion`)
**Action in Claude Code prompt:**
```text
First run powershell -Command "Start-Sleep -Seconds 22". Then use AskUserQuestion to ask me to choose between: Option A (Red) and Option B (Blue).
```

**Observation Checklist:**
1. `task_started` plays at prompt submission.
2. The turn sleeps for 22 seconds.
3. When Claude Code presents the interactive choice dialog, **`waiting_for_user` plays** (distinct question alert).
4. **Collision Avoidance**: Verify that **only** `waiting_for_user` plays — no duplicate permission sound plays for the question prompt.
5. Select an option. The turn finishes with `task_finished`.

---

### Test 4: Direct CLI Sound Controls
Open a **separate PowerShell window** while Claude Code is running or idle:
```powershell
Set-Location -Path "$HOME\AgentSFX-Tester"

# 1. Check status
.\agent-sfx.exe status
# Expected: Persisted config enabled, Worker Daemon running with active socket \\.\pipe\agent-sfx-<SID>

# 2. Turn sounds OFF (mute)
.\agent-sfx.exe off
# Expected: Persisted config: disabled, Worker daemon: acknowledged [playback canceled, live audio muted]

# 3. Verify muting
.\agent-sfx.exe preview task_finished
# Expected: [SKIPPED] task_finished: event task_finished is disabled in config

# 4. Turn sounds ON (unmute)
.\agent-sfx.exe on
# Expected: Persisted config: enabled, Worker daemon: acknowledged [live audio enabled]
```

---

### Test 5: Agent-Operated Management Skill
Back in the Claude Code session, test the agent-operated skill (`npm/claude/skills/sfx/SKILL.md`):

1. **Ask Claude Code to mute audio:**
   ```text
   Please turn sound effects off using your sfx skill.
   ```
   - Claude Code executes `node <plugin_dir>/bin/run.js off`.
   - Assistant reports that sounds are disabled.
2. **Verify live muted behavior:**
   Submit another prompt:
   ```text
   Please write a 2-line poem about rain.
   ```
   - **Observation**: Zero sounds play (muted).
3. **Ask Claude Code to unmute:**
   ```text
   Please turn sound effects back on using your sfx skill.
   ```
   - Claude Code executes `node <plugin_dir>/bin/run.js on`.
   - Assistant reports that sounds are enabled.

---

## 6. Surgical Rollback & Teardown

After completing tests, cleanly remove the test plugin and stop the background worker.

### 6.1 Uninstall the Claude Plugin (Inside the Test Project)
```powershell
Set-Location -Path "$HOME\AgentSFX-TestProject"

# Remove plugin
claude plugin uninstall agent-sfx@agent-sfx-local --scope project

# Remove local marketplace
claude plugin marketplace remove agent-sfx-local --scope project
```

### 6.2 Stop the Background Worker
```powershell
Set-Location -Path "$HOME\AgentSFX-Tester"

.\agent-sfx.exe worker stop
# Expected: [OK] Worker daemon stopped
```

Verify that the worker process has terminated:
```powershell
.\agent-sfx.exe status
# Expected: Worker Daemon : not running
```

### 6.3 Clean Up the Test Project Directory
```powershell
Set-Location -Path $HOME
Remove-Item -Path "$HOME\AgentSFX-TestProject" -Recurse -Force
```

---

## 7. Reporting Observed Results

Please report:
1. Output of `.\agent-sfx.exe doctor`.
2. Did `preview task_finished` play audibly without any console window flashing?
3. Did `task_started` and `task_finished` play during real Claude Code turns?
4. Did `error` play on command failure?
5. Did `waiting_for_user` play when the choice dialog appeared?
6. Did `agent-sfx off` and the management skill properly mute and unmute audio?
