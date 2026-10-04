# Progress

## Current state
- Milestone M0 (manual sound preview and doctor) is implemented and verified.
- Milestone M1 (shared worker, local IPC, scheduler, and neutral Gemini hook receiver) is implemented and verified.
- Milestone M2 (verified Gemini event classification, detached SessionStart spawn, project-scope hook installation, 15-second hard maximum audio architecture, verified tool error and shell exit error detection, and opt-in diagnostics) is implemented and verified. Project-scope hooks installed in `.gemini/settings.json`.
- Milestone M3 (conservative test-pass detection for Go test and pytest) is implemented, verified with comprehensive automated tests, rebuilt, and active in the running worker daemon.
- Tested Gemini CLI version: `0.62.0`.
- Next milestone: M4 — API errors and exhausted usage investigation.

## Completed (Milestone M3: Conservative Test-Pass Detection)
- **Verified Gemini CLI 0.62.0 Execution Structure**:
  - Inspected installed CLI bundle source (`chunk-72GZSNV7.js`): established that completed synchronous success requires `is_background: false`, presence of `Process Group PGID: <pid>` in trailing metadata, and total absence of `Exit Code:`, `Error:`, `Signal:`, cancellation, or timeout notices.
- **Direct Test Command Tokenization (`tokenizeCommand`)**:
  - Recognizes `go test`, `pytest`, `python -m pytest`, `python3 -m pytest`.
  - Rejects compound commands (`&&`, `||`, `;`, `&`, `|`), subshells (`$()`, backticks), redirections (`>`, `<`), multi-line strings, and background jobs.
  - Leaves leading environment variable assignments unsupported for initial M3.
  - Rejects unsupported output formats like `go test -json` explicitly.
- **Go Test Result Evaluation (`evaluateGoTestOutput`)**:
  - Requires at least one qualifying passed package (`ok <pkg> <duration>`).
  - Excludes individual packages marked `[no test files]` or `[no tests to run]` from qualifying counts.
  - Classifies runs where all qualifying packages are cached as `go_test_cached`; runs with newly executed tests as `go_test_passed`.
  - Strictly rejects runs with `FAIL`, `--- FAIL:`, `panic:`, `[build failed]`, or syntax errors.
- **Pytest Result Evaluation (`evaluatePytestOutput`)**:
  - Parses terminal summary line (`=== N passed ... in ...s ===`).
  - Requires $N \ge 1$ passed tests, 0 failures, 0 errors, and no interruption (`KeyboardInterrupt`, `INTERRUPTED`).
  - Rejects skipped-only runs (`=== 3 skipped ... ===`) and no-test runs (`=== no tests ran ... ===`).
- **Strict Invariants**:
  - Error precedence: tool errors and non-zero exit codes strictly preempt test detection.
  - Non-preemption, fail-open guarantees, and neutral hook output (`"{}\n"`) are preserved.
- **Rebuilt Executable & Daemon**:
  - Rebuilt `./bin/agent-sfx`.
  - Restarted worker daemon (`PID: 28786`).

## Completed (Error Detection Recovery & Opt-In Diagnostics Fix)
- Verified Requirement 1: `tool_response.error` accepts verified `{message, type}` and `{message}` object shapes in `internal/adapters/gemini/normalize.go` (`parseToolErrorObject`).
- Verified Requirement 2: Missing, null, empty objects, empty strings, and unsupported shapes (arrays, numbers, booleans) return non-error in `parseToolErrorObject`.
- Verified Requirement 3: Shell exit detection in `parseShellExitCode` parses only verified generated metadata by scanning upward through trailing metadata prefixes (`Process Group PGID:`, `Background PIDs:`, `Signal:`, `Exit Code:`) and stopping on any output lines, preventing arbitrary regex matches in command output.
- Verified Requirement 4: Successful commands printing `"Exit Code: 7"` do not trigger error events.
- Verified Requirement 5: Successful commands emitting stderr warnings do not trigger error events.
- Verified Requirement 6: Hook receiver always emits neutral `"{}\n"` on stdout with exit 0, even on malformed, oversized, or dead-worker conditions.
- Fixed Requirement 7 (Opt-In Diagnostic Tracing):
  - Tracing is strictly opt-in via `AGENT_SFX_TRACE_LOG`; performs zero file or directory creation when unset or empty.
  - Allowlisted metadata-only schema: `timestamp`, `hook_event_name`, `tool_name`, `has_error_field`, `error_json_type`, `classification`, `ipc_delivery`.
  - Tool names are sanitized via `SanitizeToolName` (alphanumeric allowlist).
  - Classification and IPC delivery use fixed string constants; never leak raw error strings or payloads.
  - New trace files created with owner-only `0600` permissions (and `0700` directories).
  - Logging is best-effort and bounded; unwritable paths or write errors silently preserve `"{}\n"` and exit 0.
  - Added comprehensive tests in `internal/adapters/gemini/trace_test.go`.

## Completed (15-Second Hard Maximum Audio Architecture)
- `DefaultMaxClipMS` = 15000 (15 seconds default and hard maximum).
- Valid configuration range enforced: 1–15000 ms. Config values $\le 0$ or $> 15000$ (e.g. 15001) are strictly rejected with validation errors.
- Audio duration $\le 15000$ ms accepted; any audio longer is rejected without truncation.
- Precise duration comparison: uses ceiling-millisecond and unrounded byte comparison so longer audio is never rounded down.
- Shared player process timeout: `actual clip duration + 5s` overhead (accommodates CoreAudio/ALSA process and buffer teardown latency; overhead does not permit longer audio).
- Increased `MaxWAVSizeBytes` to 25 MiB.
- Preserved non-preemption and fail-open guarantees: all events arriving during active sound playback or cooldown (1200 ms default) are dropped immediately without stale queue buildup.
- Documented how to update older configs with `"max_clip_ms": 3000` to `15000` in `architecture.md` and `README.md`.
- Manually verified user replacement meme clip: `sounds/error/error.wav` (2276 ms) plays to completion with zero errors.

## Completed (Milestone M2 Installation)
- Installed project-scope hooks via `./bin/agent-sfx install gemini`:
  - Target: `.gemini/settings.json` (backup preserved at `.gemini/settings.json.bak`).
  - Five verified hooks installed:
    1. `SessionStart` (`matcher: ""`) $\rightarrow$ `agent-sfx-session-start`: silent detached worker launch across `startup`, `resume`, and `clear`.
    2. `BeforeAgent` (`matcher: ""`) $\rightarrow$ `agent-sfx-before-agent`: `task_started`.
    3. `AfterAgent` (`matcher: ""`) $\rightarrow$ `agent-sfx-after-agent`: `task_finished`.
    4. `Notification` (`matcher: ""`) $\rightarrow$ `agent-sfx-notification`: `permission_requested`.
    5. `AfterTool` (`matcher: ""`) $\rightarrow$ `agent-sfx-after-tool`: `error` and `tests_passed`.
- Verified installer idempotence: `./bin/agent-sfx install gemini --dry-run` reports 0 additions and 5 unchanged hooks.
- User-scope hooks, real payload capture, and telemetry remain uninstalled/disabled.

## Test and Verification Evidence

### Automated Test Suite (Verified)
- `gofmt -s -w .`: Passed with zero diffs.
- `go vet ./...`: 0 warnings.
- `go test -count=1 -race ./...`: 100% passed across all 11 packages:
  - `internal/adapters/gemini`: Verified fixture normalization, native error objects, missing/null/empty/unsupported error shapes, shell non-zero exit parsing, stdout with exit code string rejection, stderr warning rejection, neutral stdout invariant (`"{}\n"`), dead worker timeout, subprocess detached worker execution, default-disabled tracing, opt-in metadata verification, 0600 permissions, sensitive sentinel non-retention, unwritable log degradation, Go test newly executed pass, Go test cached pass, Go test [no test files] exclusion, Go test [no tests to run] exclusion, Go test build failure rejection, pytest pass, pytest mixed pass/skipped, pytest skipped-only rejection, pytest no tests ran rejection, pytest failure/error rejection, misleading summary rejection, truncated output rejection, compound command rejection, pipeline rejection, background command rejection, and error precedence over tests passed.
  - `internal/audio`: Tested exactly 15 seconds accepted, slightly over 15 seconds (15000.02 ms) rejected, 16 seconds rejected, and simplified process timeout (`clipDuration + 5s`).
  - `internal/config`: Tested 15000 ms default, 15000 ms boundary accepted, and 15001 ms rejected.
  - `internal/preview`: Tested 15-second clip playing to completion, 16-second clip rejected, and process timeout cancellation on hung players.
  - `internal/installer`: Verified plan creation, atomic write, backup preservation across runs, conflict detection, invalid JSON rejection, and POSIX shell escaping.
  - `internal/worker`, `internal/scheduler`, `internal/ipc`, `internal/sounds`, `internal/doctor`: All unit tests passed.

### Manual Hardware Playback & Integration Verification
- **Confirmed Live Error Sounds**: User confirmed that both native-tool and shell-error sounds played successfully in a live Gemini CLI session.
- **Confirmed Live Test Sounds (Milestone M3)**: User confirmed hearing audible playback in the live Gemini CLI session across all controlled scenarios:
  1. Passing Go test (`go test -count=1 pass_test.go`): `tests_passed` sound played.
  2. Failing Go test (`go test -count=1 fail_test.go`): `error` sound played (verifying error precedence over test detection).
  3. Passing pytest (`pytest test_pass.py`): `tests_passed` sound played.
  4. Failing pytest (`pytest test_fail.py`): `error` sound played (verifying error precedence for pytest).
  5. Go test with `[no test files]` (`go test main.go`): no sound played (verifying neutral drop on non-qualifying packages).
  - All isolated temporary test fixtures were cleaned up immediately following confirmation.
- Executed `./bin/agent-sfx preview error`: Played replacement clip (`sounds/error/error.wav`, 2276 ms) to completion via `afplay` without timeout errors.
- Active background worker daemon running with updated binary (`PID: 28786`).

## Platform Limitations & Distinctions
- **Test Fixtures vs Real Evidence**: Sanitized fixture tests verify compliance with the documented Gemini CLI 0.62.0 schema.
- **Cross-Compilation vs Runtime**: Cross-compilation verifies that the code builds on Windows and Linux. It does not constitute Windows runtime verification. Windows hook installation remains blocked while the Windows worker is a stub.
