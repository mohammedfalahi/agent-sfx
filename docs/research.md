# Source research and guardrails

Research snapshot: 2026-10-03. Documentation and agent versions can change; verify current behavior before integration.

## peon-ping audit
Snapshot: https://github.com/PeonPing/peon-ping/tree/8ef376602fa6ee19100e4632396463b5875f2672
This was a static source audit, not runtime compatibility certification.

Useful ideas: thin adapters, explicit event mapping, shared playback, persistent cooldown/dedupe, manifest random selection and original-license notices.

Do not copy these behaviors:
- Core PreCompact -> resource.limit is context compaction, not measured subscription/API quota.
- Gemini successful AfterTool -> Stop incorrectly conflates tool success with turn completion.
- Gemini expects top-level exit_code and discards notification_type. Official reference documents AfterTool.tool_response including optional error and ToolPermission notifications.
- Unix core PostToolUseFailure generally requires error plus Bash tool (Copilot exception). OpenCode/omp thin plugins emit that event without the required fields.
- OpenClaw resource.limit -> Notification/resource_limit is ignored by the Unix core. The caller supplies a label; the adapter doesn't measure quota.
- DeepAgents task.error -> Stop creates a completion category. ECA preToolCall -> PermissionRequest conflates a tool call with permission waiting.
- Idle filesystem heuristics in Antigravity/Trae cannot prove task completion. Kimi's wire.jsonl parsing is stronger because it reads explicit lifecycle events.
- Windows generated handler and Unix core differ. Never assume cross-platform mapping parity.

Relevant source files:
- peon.sh: main Unix runtime and embedded Python router.
- install.ps1: contains generated Windows peon.ps1 runtime.
- adapters/gemini.sh and gemini.ps1: Gemini hook translation.
- adapters/codex.sh: canonical stdin hooks plus legacy argv callback.
- adapters/opencode/peon-ping.ts: native plugin callbacks.
- adapters/antigravity-watcher.py: file-activity state machine, idle inference.
- adapters/kimi.sh: structured wire log watcher.
- mcp/peon-mcp.js: explicit sound catalog/playback API, not autonomous status collection.

## Authoritative Gemini references
- Context files and imports: https://geminicli.com/docs/cli/gemini-md
- Hook reference: https://geminicli.com/docs/hooks/reference/
- Hook overview: https://geminicli.com/docs/hooks
- Telemetry: https://geminicli.com/docs/cli/telemetry/

Verified reference concepts:
- GEMINI.md supports @./relative-file.md imports. /memory show displays loaded context; current docs use /memory reload to rescan. Some older versions use a different subcommand; check /help or restart.
- BeforeAgent follows prompt submission. AfterAgent follows final response.
- Notification includes notification_type ToolPermission.
- AfterTool includes tool_name, tool_input and tool_response (llmContent, returnDisplay, optional error).
- Hook stdout must be JSON only. Exit 2 can block/retry; this add-on must not use it.
- Hooks normally run synchronously. Keep handoff bounded and playback detached.
- Telemetry exposes gemini_cli.api_error/api_response/tool_call and token metrics. It can write a local outfile or use OTLP. The documentation does not establish a universal quota-remaining counter.
- telemetry.logPrompts defaults to true in the documented configuration; opt-in telemetry needs deliberate privacy review and does not become safe merely by disabling prompt logging.

## Gemini CLI 0.62.0 shell tool execution structure and test-pass detection (M3)
Empirical source verification of installed Gemini CLI 0.62.0 (`/opt/homebrew/lib/node_modules/@google/gemini-cli/bundle/chunk-72GZSNV7.js`):
- `run_shell_command` outputs structured sections in `llmContent`:
  - `Output: <stdout/stderr or (empty)>`
  - `Error: <message>` (included only if process execution error occurred)
  - `Exit Code: <codeStr>` (included ONLY if exitCode !== null && exitCode !== 0)
  - `Signal: <signal>` (included only if terminated by signal)
  - `Background PIDs: <pids>` (included only if background processes were started)
  - `Process Group PGID: <pid>` (included only if available for synchronous child process)
- Background execution sets `tool_input.is_background = true` or `result.backgrounded = true`, emitting `Command moved to background (PID: ...). Output hidden. Press Ctrl+B to view.`
- Aborted/cancelled execution emits `Command cancelled by user` or `Command timed out after ...` in `llmContent`.
- Normal completed synchronous success is unambiguously proven by:
  1. `tool_input.is_background == false`.
  2. `llmContent` starts with `Output: ` and ends with trailing metadata containing `Process Group PGID: <pid>`.
  3. Total absence of `Exit Code: `, `Error: `, `Signal: `, cancellation notices, or background notices.
- Test runner output rules:
  - Go test: parses standard `ok <pkg> <duration>` lines. Excludes `[no test files]` and `[no tests to run]`. Cached packages emit `(cached)` and are classified with `go_test_cached`. Rejects `FAIL`, `--- FAIL:`, `panic:`, `[build failed]`, and syntax errors.
  - Pytest: parses the terminal summary line (`=== N passed ... in ...s ===`). Excludes skipped-only or no-test runs. Rejects any `failed`, `error`, `KeyboardInterrupt`, or `INTERRUPTED`.
  - Invariants: rejects compound commands, subshells, pipelines, redirections, leading env assignments, and `-json` output format. Error classification takes precedence.

## Gemini CLI 0.62.0 question tool (`ask_user`) research (M4)
Empirical source verification of installed Gemini CLI 0.62.0 (`/opt/homebrew/lib/node_modules/@google/gemini-cli/bundle/chunk-72GZSNV7.js`):
- `ask_user` tool definition (`AskUserTool` / `AskUserInvocation`):
  - Parameters: `questions` (array of 1 to 4 objects).
  - Each question requires non-empty `question` and `header` strings.
  - Optional `type` enum: `"choice"` (default), `"text"`, `"yesno"`.
  - For `"choice"`: requires `options` array with 2 to 4 items, each with non-empty `label` and valid `description`.
  - For `"text"` and `"yesno"`: `options` is not required; if present, options must have valid labels/descriptions.
- Execution lifecycle in `_processToolCall`:
  1. `evaluateBeforeToolHook`: synchronous `BeforeTool` hook fires with `tool_name: "ask_user"` and `tool_input`.
  2. `checkPolicy`: evaluated next. Priority 999 rule always forces decision `"ask_user"`.
  3. `resolveConfirmation`:
     - Invokes `invocation.shouldConfirmExecute()`, returning `{ type: "ask_user", title: "Ask User", questions: [...] }`.
     - Invokes `notifyHooks(deps, details)`: fires `Notification` with `notification_type: "ToolPermission"` and `details: { type: "ask_user", title: "Ask User" }`.
     - Waits interactively for user confirmation or cancellation.
  4. User answer/dismissal: `AskUserInvocation.execute` returns `llmContent`. `AfterTool` fires.
- Crucial findings & design invariants:
  - `BeforeTool` fires before interactive question dialog is displayed to the user.
  - Invariant/Limitation: `BeforeTool` signals a question-tool request from the model, not proof that the dialog was displayed (a later hook or policy may block it).
  - Deduplication: `base.Timestamp` is the hook event creation timestamp, not a persistent tool call ID. Deduplication covers identical payload replays only; questions are not deduped using content.
  - Collision avoidance: Gemini CLI's `notifyHooks` fires `NotificationType.ToolPermission` for `ask_user` confirmations. To prevent playing a permission sound before or in place of `waiting_for_user`, Agent SFX inspects `details.type` and suppresses `permission_requested` when `details.type == "ask_user"`. All other `ToolPermission` notifications are preserved.
  - `AfterTool` completion: normal answers and user dismissal emit no sound. Genuine structured errors retain `error` classification.
  - Claude Code equivalent (documentation only): `PreToolUse` hook matching tool name `AskUserQuestion`. (Claude adapter reserved for Milestone M5).

## Other optional sources (later, not MVP dependencies)
- CodexBar: https://github.com/steipete/CodexBar/blob/main/docs/configuration.md — external quota_low/quota_reached/quota_reset hooks; provider-specific authentication and polling. macOS/Linux CLI documented; don't promise Windows support.
- Claude Monitor: https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor — machine-readable snapshots/state files with official vs estimated provenance.
- ccusage: https://ccusage.com/ — local token/cost accounting, not authoritative quota or lifecycle event detection.
- Claude hook reference: https://code.claude.com/docs/en/hooks
- Claude statusline: https://code.claude.com/docs/en/statusline — eligible accounts/version-dependent rate_limits, distinct from context usage.

## Gemini access caveat
Google announced consumer Gemini CLI access changes while enterprise/paid API-key access remained supported. Confirm the developer's actual installation still works; don't silently switch the target agent or assume the project makes agent usage free.
https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli

## Gemini CLI 0.62.0 extension, custom-command, and skill interfaces
Empirical source verification of installed Gemini CLI 0.62.0 (`/opt/homebrew/lib/node_modules/@google/gemini-cli/bundle/chunk-72GZSNV7.js` and `chunk-I47WLEEQ.js`):
- **Custom Commands (`.toml`)**: Loaded via `FileCommandLoader`. Always resolves to `{ type: "submit_prompt", content: processedContent }` which triggers an LLM turn. Pure local execution (without sending a prompt to the model) and custom UI dialogs are strictly unsupported for user/extension slash commands.
- **Extension Installation & Linking**:
  - `gemini extensions link <path>`: Links a local extension into `~/.gemini/extensions/<name>`.
  - `gemini extensions list`: The JSON output flag is `--output-format json` (or `-o json`). `--json` is unrecognized and causes an error.
  - Extension installation is user-global (`~/.gemini/extensions/`), while enablement can be scoped via `gemini extensions enable/disable [--scope user|workspace] <name>`.
- **Extension Hook Path Substitution**:
  - `loadExtensionHooks` loads `hooks/hooks.json` from the extension directory.
  - Must define an enclosing object `"hooks": { ... }`.
  - `recursivelyHydrateStrings` automatically substitutes `${extensionPath}`, `${workspacePath}`, `${/}`, and `${pathSeparator}` in hook commands.
- **Bundled Skill Discovery**:
  - `SkillManager.discoverSkills()` automatically scans `path.join(extensionPath, "skills")` with pattern `["SKILL.md", "*/SKILL.md"]`.
  - Skills require YAML frontmatter (`--- \n name: <slug> \n description: <text> \n ---`).
  - Bundled skills are exposed as available agent skills (`activate_skill`), enabling agent-operated tool invocation (invoking CLI commands via `run_shell_command`).

## Claude Code plugin, hook, and event architecture (M5 Empirical Findings)
Empirical investigation of installed Claude Code 2.1.162 (`~/.local/share/claude/versions/2.1.162`):
- **Hook Events & Execution Flow**:
  - `UserPromptSubmit`: Fires when user submits a prompt turn. Carries `prompt` and `session_id`. Maps directly to `task_started`.
  - `Stop`: Fires on turn completion (including clear, resume, compact). Carries `stop_hook_active` and `last_assistant_message`. Maps directly to `task_finished` (turn completion only, not overall task success).
  - `PermissionRequest`: Direct hook executed before displaying permission prompts. Carries `tool_name` and `tool_input`. Maps to `permission_requested`.
    - **Collision Avoidance**: When `tool_name == "AskUserQuestion"`, `PermissionRequest` is explicitly suppressed to avoid playing permission sound when an interactive question is presented.
  - `PreToolUse`: Fires before tool invocation with anchored matcher on tool name.
    - **Explicit Question Detection**: When `tool_name == "AskUserQuestion"`, tool input is validated against Claude Code's native schema (`questions` array of 1 to 4 items with non-empty `question` and valid `options` labels). Emits `waiting_for_user`. All other tools in `PreToolUse` return 0 events.
  - `PostToolUseFailure`: Fires when a tool fails. Carries `error` and `is_interrupt`.
    - **Interruption Filtering**: User cancellations/interruptions (`is_interrupt: true`) are explicitly excluded. Uninterrupted failures map to `error`.
  - `StopFailure`: Dedicated API and runtime failure hook. Carries `error` and `error_details`. Maps to `error` for verified API/provider failures (rate limits, overloads, auth, network). Excludes model refusals and ordinary assistant text.
- **Invocation-Level Deduplication**:
  - Claude payloads supply `tool_use_id` for tool lifecycle hooks (`PreToolUse`, `PostToolUse`, `PostToolUseFailure`).
  - Dedupe keys are scoped by agent, session, event kind, and tool use ID (`claude:<session_id>:<kind>:<tool_use_id>`).
- **Unresolved Limitations & Status**:
  - `tests_passed`: Claude's internal `Bash` tool response format differs from Gemini CLI. Gemini's shell metadata parser (`Process Group PGID`, trailing metadata) cannot be reused. `tests_passed` remains explicitly unsupported until serialized hook output payloads are captured and verified separately.
  - `usage_exhausted`: Claude Code does not emit a dedicated billing/quota-zero hook event. Remained explicitly unsupported per product invariants.
  - `StopFailure`: Classification schema (`rate_limit`, `overloaded`, `authentication_failed`, `oauth_org_not_allowed`, `billing_error`, `invalid_request`, `model_not_found`, `server_error`, `max_output_tokens`, `unknown`) is source- and fixture-tested against Claude Code 2.1.162 binary definitions; live API failure testing remains unconfirmed (never deliberately induce real quota/billing outages).
  - Global playback gate & cooldown: The scheduler's shared cooldown (`1200ms`) and non-preemption gate apply globally across all agents. Arriving events during active playback or cooldown are dropped according to priority rules.

## Future Authorized Tester Guide: Claude Code Integration & Verification Protocol

When an authorized tester with active Claude Code subscription/authenticated access is ready to perform live integration testing, follow this exact procedure:

### 1. State Snapshot (Pre-Installation)
Create a unique timestamped snapshot directory and record initial file existence. Do not suppress errors:
```bash
SNAPSHOT_DIR="$HOME/.gemini/tmp/AgentSFX/snapshots/claude-preinstall-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$SNAPSHOT_DIR"

# Record manifest of original state
cat << 'EOF' > "$SNAPSHOT_DIR/manifest.json"
{
  "project_claude_existed": false,
  "user_settings_existed": true,
  "user_known_marketplaces_existed": true,
  "user_installed_plugins_existed": true
}
EOF

# Copy existing state files (abort immediately if any copy fails)
cp "$HOME/.claude/settings.json" "$SNAPSHOT_DIR/user_settings.json"
cp "$HOME/.claude/plugins/known_marketplaces.json" "$SNAPSHOT_DIR/user_known_marketplaces.json"
cp "$HOME/.claude/plugins/installed_plugins.json" "$SNAPSHOT_DIR/user_installed_plugins.json"
```

### 2. Project-Scoped Installation Commands
Install within the target project scope to isolate changes to `.claude/settings.json`:
```bash
# Register local marketplace catalog
claude plugin marketplace add "./npm/claude" --scope project

# Install plugin into project
claude plugin install agent-sfx@agent-sfx-local --scope project
```
*Note on Scope*: `--scope project` records declarations in `.claude/settings.json`, but Claude Code also records catalog metadata in `~/.claude/plugins/known_marketplaces.json` and installation entries in `~/.claude/plugins/installed_plugins.json`.

### 3. Surgical Rollback Procedure
Do NOT perform blind `rm -rf` or overwrite unrelated user settings:
```bash
# 1. Uninstall plugin from project scope
claude plugin uninstall agent-sfx@agent-sfx-local --scope project

# 2. Remove marketplace registration from project scope
claude plugin marketplace remove agent-sfx-local --scope project

# 3. Clean project settings:
# If .claude/settings.json only contained agent-sfx entries, remove it.
# If unrelated project settings were added afterward, surgically remove only
# extraKnownMarketplaces.agent-sfx-local and enabledPlugins["agent-sfx@agent-sfx-local"].

# 4. Verify user state:
# Confirm ~/.claude/plugins/known_marketplaces.json and ~/.claude/plugins/installed_plugins.json
# no longer contain agent-sfx entries.
```

### 4. Live Test Sequence & Critical Timing Invariants
**Crucial Timing Rule**: Waiting between user prompts does NOT prevent the turn's own `task_started` sound from playing at prompt submission. If an action or question is invoked immediately, `task_started` playback (up to 15s) and cooldown (1200ms) will block the subsequent sound. **In-turn clearance is mandatory**: the prompt must direct Claude to perform a controlled timed delay (e.g. `sleep 22` via Bash) before triggering the target action.

- **Test A: Turn Start & Delayed Turn Completion**
  - Prompt: `"Please run: sleep 22; echo 'Ready'"`
  - *Observation*: `task_started` plays at turn submission (`UserPromptSubmit`). After the 22-second delay, turn concludes and `task_finished` plays distinctly (`Stop`).
- **Test B: Permission Request**
  - Verify tester's active permission mode first (`claude config get permissionMode` or check `.claude/settings.json`). Do not assume commands like `date` require permission if already allowlisted; select an un-allowlisted command that genuinely triggers the interactive permission dialog.
  - Prompt: `"Please run: <unapproved-command>"`
  - *Observation*: `permission_requested` plays before the permission dialog is displayed (`PermissionRequest`).
- **Test C: Explicit AskUserQuestion & Collision Suppression**
  - Prompt: `"First run: sleep 22. After that command finishes, use AskUserQuestion to ask me to choose between Option A and Option B."`
  - *Observation*: The 22-second in-turn delay allows `task_started` playback to complete. Then `waiting_for_user` plays on `PreToolUse`. `permission_requested` is suppressed (`details.type == "ask_user"`), verifying collision avoidance.
- **Test D: Intentional Harmless Tool Failure**
  - Prompt: `"First run: sleep 22. After that finishes, run this command in Bash to verify error detection: ls /nonexistent-path-for-agent-sfx-test"`
  - *Observation*: The in-turn delay clears `task_started`. Then the non-zero exit triggers `PostToolUseFailure` and `error` plays.
- **Test E: Sound Controls (Direct & Skill-Operated)**
  - Direct check: `node npm/claude/bin/run.js status`, `off`, `on`.
  - Skill check in Claude: `"Turn sound effects off"`, `"Check sound status"`, `"Turn sound effects on"`.
  - Verify that `enabled: true` and active `sounds_dir` are preserved in `~/Library/Application Support/agent-sfx/config.json`.
- **Observation Recording Rule**: Always record actual observed sounds and exit codes; never mark expected behavior as verified without empirical test runs.



