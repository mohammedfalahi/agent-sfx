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

## Other optional sources (later, not MVP dependencies)
- CodexBar: https://github.com/steipete/CodexBar/blob/main/docs/configuration.md — external quota_low/quota_reached/quota_reset hooks; provider-specific authentication and polling. macOS/Linux CLI documented; don't promise Windows support.
- Claude Monitor: https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor — machine-readable snapshots/state files with official vs estimated provenance.
- ccusage: https://ccusage.com/ — local token/cost accounting, not authoritative quota or lifecycle event detection.
- Claude hook reference: https://code.claude.com/docs/en/hooks
- Claude statusline: https://code.claude.com/docs/en/statusline — eligible accounts/version-dependent rate_limits, distinct from context usage.

## Gemini access caveat
Google announced consumer Gemini CLI access changes while enterprise/paid API-key access remained supported. Confirm the developer's actual installation still works; don't silently switch the target agent or assume the project makes agent usage free.
https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli
