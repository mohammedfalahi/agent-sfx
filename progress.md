# Progress

## Current state
- **Open-Source Standardization Milestone**: Completed and verified.
  - Security Policy (`SECURITY.md`) created detailing threat model, local-only IPC boundary, child execution safety without shell interpolation, bounded audio resources (15s ceiling, 25 MiB max), fail-open hook contract, and private vulnerability disclosure.
  - Contributor Guide (`CONTRIBUTING.md`) created with toolchain prerequisites (Go 1.22+, Node 18+), seven-event closed enum invariants, coding standards, race-testing guidelines, and audio synthesis standards.
  - Setup Guide (`SETUP.md`) fully rewritten from obsolete M0 bootstrap notes into an authoritative macOS, Windows, and Linux guide covering source builds, pre-install diagnostics, Gemini CLI hook integration, Claude Code plugin usage, background daemon management, configuration, and uninstallation.
  - Root `README.md` updated with accurate Windows support status, cross-platform commands, daemon controls, diagnostics, and documentation index.
  - Distribution manifests and packages updated: `npm/README.md` updated for Windows x64 support; `npm/claude/README.md` and `npm/claude/SOUND-LICENSES.md` added; `SOUND-LICENSES.md` updated to distinguish bundled CC0 synthesized sounds from user-supplied meme clips.
  - Clean-room verification executed from isolated temporary location (`/tmp/clean-sfx-verify-20261007`): clean build, `doctor`, hook receiver test (outputs `{}` and exit 0), dry-run installation, and 100% pass across all unit/race/vet/fmt checks and Linux/Windows cross-compilation.
- Milestone M0 (manual sound preview and doctor) is implemented and verified.
- Milestone M1 (shared worker, local IPC, scheduler, and neutral Gemini hook receiver) is implemented and verified.
- Milestone M2 (verified Gemini event classification, detached SessionStart spawn, project-scope hook installation, 15-second hard maximum audio architecture, verified tool error and shell exit error detection, and opt-in diagnostics) is implemented and verified. Project-scope hooks installed in `.gemini/settings.json`.
- Milestone M3 (conservative test-pass detection for Go test and pytest) is implemented, verified with comprehensive automated tests, rebuilt, and active in the running worker daemon.
- Milestone M4 (explicit user-question detection via Gemini CLI `ask_user` tool) is implemented, verified with automated race-enabled tests, rebuilt, active in running worker daemon (`PID: 12107`), installed into `.gemini/settings.json`, and live-verified in Gemini CLI 0.62.0.
- Live sound controls (`agent-sfx on`, `agent-sfx off`, `agent-sfx status`) implemented with field-preserving persistence, live playback cancellation, worker queue draining, and execution loop serialization.
- npm distribution scaffold (`@agent-sfx/agent-sfx`) implemented with safe Node launcher, prebuilt macOS binaries (arm64 runtime-tested, x64 build-tested), Windows x64 binary (`windows-amd64/agent-sfx.exe`), pristine CC0 starter sounds (excluding local meme clips), Gemini extension manifest (`gemini-extension.json`), and hydrated hook definitions.
- Read-only setup and migration planner (`agent-sfx setup gemini --dry-run` and `agent-sfx setup claude --dry-run`) implemented with two-way duplicate detection, exact hook ownership matching, customized hook abort protection, and stable path resolution.
- Agent-operated management skill (`npm/skills/sfx/SKILL.md` and `npm/claude/skills/sfx/SKILL.md`) implemented for Gemini CLI and Claude Code bundled skill discovery.
- **Gemini Packaging & Migration Milestone**: Completed and closed. User confirmed that manual extension and control tests worked. Active packaged worker running with custom sounds preserved (`PID: 90651`).
- **Milestone M5 (Claude Code Support) Status**:
  - **Implemented & Automated Verification Complete**: Claude adapter normalization, verified StopFailure allowlist classification (matching installed Claude Code 2.1.162 binary schema enum), malformed PostToolUseFailure protection, CLI subcommand wiring (`agent-sfx hook claude`), silent SessionStart worker startup, self-contained Claude plugin packaging (`npm/claude/`), local marketplace layout (`.claude-plugin/marketplace.json`), read-only setup planner (`agent-sfx setup claude --dry-run`), and comprehensive Go/Node test suites implemented and 100% passing (including `-race`, vet, fmt, and relocated directory execution).
  - **Live Verification Paused & Pending**: Live event detection, audible playback, and management-skill execution under Claude Code are paused and pending future tester authentication (developer currently lacks active Claude subscription or authenticated model access). No live settings were modified, no plugins were installed, no snapshots were written, and no credentials or API limits were perturbed.
  - **Saved Installation & Rollback Protocol**: The complete project-scoped installation, state snapshot manifest, surgical rollback, and corrected live verification protocol have been recorded in `docs/research.md` and `TESTING-CLAUDE-WINDOWS.md` for authorized testing.
  - **Corrected Test Invariants Recorded**:
    1. In-turn clearance is mandatory: waiting between prompts does not prevent the turn's own `task_started` sound from playing at prompt submission and blocking subsequent sounds. Test turns must incorporate explicit controlled timing (`sleep 22`).
    2. Controlled timing: "wait without tools" is not dependable for model execution; use an explicit `sleep 22` command via Bash or PowerShell.
    3. Permission verification: do not assume commands like `date` require permission; check the tester's active permission mode and allowlists without weakening security.
    4. Observation recording: record actual observed sounds, exit codes, and durations; never mark expected behavior as verified without empirical test runs.
- Tested Gemini CLI version: `0.62.0`.
- Tested Claude Code version: `2.1.162`.
- **Windows Release-Readiness (Phases 2A & 2B) Complete**:
  - Phase 2A: Fixed Windows PowerShell audio playback via fixed script and child-process environment variable `AGENT_SFX_PLAY_PATH`, eliminated console flashes via `windows.CREATE_NO_WINDOW`, implemented Windows config locking via `windows.LockFileEx` with finite deadline, updated Unix locking to non-blocking flock with retry loop, and added separate-process lock contention test.
  - Phase 2B: Updated Node launchers for `win32/x64` with explicit Windows ARM64 rejection, included prebuilt Windows binaries in both `npm/bin/` and `npm/claude/bin/`, updated Claude plugin hooks with `node` invocation, added root MIT `LICENSE` with dependency notices, updated `.gitignore` for backup hygiene, and generated tester guide `TESTING-CLAUDE-WINDOWS.md` and standalone tester ZIP bundle.
  - **Runtime Status**: Cross-compilation and automated unit/integration suites pass. Audible playback, live Claude Code integration, and Windows separate-process contention remain explicitly marked as **Pending runtime verification on physical Windows host**.

## Completed (Phase 2B: Windows Packaging & Tester Installation Preparation)
- **Node Launchers Windows Support (`npm/bin/run.js` and `npm/claude/bin/run.js`)**:
  - Added support for `process.platform === 'win32'` and `process.arch === 'x64'`.
  - Maps to `windows-amd64/agent-sfx.exe`.
  - Explicitly rejects Windows ARM64: `Agent SFX: Windows ARM64 is not supported in this release. Supported Windows architecture: x64.`
  - Retains exact macOS `darwin-arm64` and `darwin-x64` resolution and Unix `0755` permissions check.
  - Preserves safe argument forwarding and exit codes using `spawnSync(binaryPath, process.argv.slice(2), { stdio: 'inherit', env })`.
  - Comprehensive launcher tests updated in `npm/test_launcher.js` (9/9 tests passing).
- **Self-Contained Claude Plugin Packaging (`npm/claude/`)**:
  - Populated `npm/claude/bin/windows-amd64/agent-sfx.exe` and `npm/bin/windows-amd64/agent-sfx.exe` from current build (SHA256 checksums verified identical).
  - Updated `npm/claude/hooks/hooks.json` to invoke launcher via `node "${CLAUDE_PLUGIN_ROOT}/bin/run.js" hook claude`.
  - Documented shell behavior and limitation: requires `node` in system PATH.
  - Preserved working Gemini integration paths in `npm/hooks/hooks.json` (`"${extensionPath}/bin/run.js"`).
  - Preserved management skills (`npm/skills/sfx/SKILL.md` and `npm/claude/skills/sfx/SKILL.md`) resolving their local packaged launchers.
- **Package Manifest Updates (`npm/package.json`)**:
  - Updated platform constraints: `"os": ["darwin", "win32"]`, `"cpu": ["arm64", "x64"]`.
  - Added `"LICENSE"` to `files` allowlist.
  - Verified package contents with `npm pack --dry-run`: 34 files, 17.6 MB tarball (33.3 MB unpacked with universal macOS and Windows x64 binaries).
- **Repository Hygiene & Legal Licenses**:
  - Updated `.gitignore` to safely ignore local backup files (`.gemini/*.bak`, `*.bak`) and migration snapshots (`.gemini/migration-snapshot-*/`) without deleting existing local files.
  - Created root `LICENSE`, `npm/LICENSE`, and `npm/claude/LICENSE` matching MIT declaration with copyright attributed to "Agent SFX Contributors" and required third-party notices for `github.com/Microsoft/go-winio` (MIT, Microsoft Corp.) and `golang.org/x/sys` (BSD-3-Clause, The Go Authors).
  - Audio license notices in `SOUND-LICENSES.md` (CC0 1.0 Universal) verified and retained.
- **Windows Claude Tester Guide (`TESTING-CLAUDE-WINDOWS.md`)**:
  - Created portable, copy-pasteable PowerShell testing guide using `$PSScriptRoot` and `Join-Path`.
  - Documented Node >= 18.0.0 and authenticated Claude Code prerequisites.
  - Outlined pre-install verification (`doctor`, manual `preview task_finished`), project-scoped plugin installation via local marketplace, and trust warning handling.
  - Detailed 5 core sound tests with in-turn clearance timing (`Start-Sleep -Seconds 22`): turn start/finish, tool failure, AskUserQuestion choice dialog, direct CLI controls, and agent-operated management skill controls.
  - Documented surgical rollback commands and state verification.

## Completed (Phase 2A: Windows Audio Playback & Config-Lock Fixes)
- **Windows PowerShell Audio Playback (`internal/audio/`)**:
  - Replaced broken inline parameter script with fixed constant script `WindowsPlayScript`:
    `$p = $env:AGENT_SFX_PLAY_PATH; if (-not $p -or -not (Test-Path -LiteralPath $p)) { exit 1 }; (New-Object Media.SoundPlayer $p).PlaySync()`
  - Passes audio path as data via child-process-only environment variable `AGENT_SFX_PLAY_PATH`, completely avoiding PowerShell script evaluation syntax errors.
  - Uses `-LiteralPath` to prevent wildcard expansion on paths with brackets or special characters.
  - Implemented `prepareWindowsPlayerCmd` in `player_windows.go` setting `windows.CREATE_NO_WINDOW` in `cmd.SysProcAttr` to eliminate console window flashes on playback.
  - Audio playback cancellation: context cancellation (`ctx.Done()`) terminates `powershell.exe` child process immediately via `TerminateProcess` without interrupting worker daemon.
  - Audible behavior on Windows is explicitly marked as **Pending runtime verification on physical Windows host**.
- **Windows & Unix Config Locking (`internal/config/`)**:
  - Implemented Windows file locking in `lock_windows.go` using `windows.LockFileEx` (`LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY`) with a finite deadline (2s default timeout, 20ms retry loop).
  - Updated Unix file locking in `lock_unix.go` from blocking `syscall.LOCK_EX` to non-blocking `syscall.LOCK_EX | syscall.LOCK_NB` with finite deadline loop (2s default timeout, 20ms retry loop), preventing indefinite hangs if a lock file is locked.
  - Corrected documentation: `UseNumber()` preserves numeric precision on large numbers and floats without converting to float64; it does not preserve JSON comments (which standard JSON does not support).
  - Config replacement behavior: Note that on Windows, Go's `os.Rename` calls `MoveFileEx(..., MOVEFILE_REPLACE_EXISTING)`. While it replaces existing target files without error, NTFS does not guarantee transactional/all-or-nothing power-loss atomicity for replace operations the way POSIX rename does, and destination files cannot be replaced if open with conflicting sharing modes. Atomicity is therefore bounded by operating system filesystem semantics.
  - Added Windows separate-process contention test `TestWindowsConfigLock_SeparateProcessContention` in `lock_windows_test.go`, marked explicitly as **Pending execution on native Windows host**.

## Completed (Windows IPC & Worker Lifecycle Milestone)
- **Pinned Dependencies (`go.mod`)**:
  - `github.com/Microsoft/go-winio v0.6.2`: MIT License, Go 1.18+ compatible. Provides `ListenPipe` and `DialPipeContext` with local-only remote client rejection (`FILE_PIPE_REJECT_REMOTE_CLIENTS`).
  - `golang.org/x/sys v0.10.0`: BSD-3-Clause License, Go 1.18+ compatible. Provides Windows kernel constants and `LockFileEx` / `UnlockFileEx` APIs.
- **Windows Named Pipe IPC (`internal/ipc/`)**:
  - `paths_windows.go`: Implemented user-scoped path resolution (`\\.\pipe\agent-sfx-<SID>`) using `os/user.Current()` with sanitized SID string and custom `AGENT_SFX_SOCKET_PATH` validation. `SocketDir` returns `%LOCALAPPDATA%\agent-sfx`.
  - `server_windows.go`: Implemented pipe listener with DACL restricted strictly to the current user's SID (`D:P(A;;GA;;;<SID>)`) and protected against inheritance. Preserved 8 KiB framing, double-buffered I/O, and 100ms connection deadlines.
  - `client_windows.go`: Implemented synchronous client using `winio.DialPipeContext`, 8 KiB framing limit, newline delimiters, and 25ms default send timeout.
  - `protocol_test.go`: Platform-neutral request validation and framing tests.
  - `ipc_windows_test.go`: Windows-specific tests for pipe path resolution, custom pipe validation, dead pipe fast timeout, occupied pipe collision rejection, and oversized request protection.
- **Windows Worker Lifecycle & Singleton Lock (`internal/worker/`)**:
  - `lock_windows.go`: Replaced stub with atomic non-blocking exclusive file lock on `%LOCALAPPDATA%\agent-sfx\worker.lock` via `windows.LockFileEx` (`LOCKFILE_EXCLUSIVE_LOCK | LOCKFILE_FAIL_IMMEDIATELY`). Guarantees user-scoped singleton semantics matching the named pipe without administrator privileges. Handle is safely closed and unlocked on release.
  - `lifecycle_windows.go`: Implemented `SpawnDetachedWorker` using named Windows constants (`windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW`), disconnected stdio (`Stdin=nil, Stdout=nil, Stderr=nil`), and immediate process handle release (`cmd.Process.Release()`) so the worker survives parent termination.
  - `worker_windows_test.go`: Windows-specific tests for singleton locking, duplicate acquire rejection (`ErrWorkerAlreadyRunning`), daemon lifecycle over named pipe, live enable/disable (`on`/`off`), and clean exit on stop.
- **Cross-Compilation & Validation**:
  - `GOOS=windows GOARCH=amd64 go build -o bin/agent-sfx.exe ./cmd/agent-sfx`: PE32+ executable (console) x86-64 built successfully.
  - `GOOS=windows GOARCH=amd64 go test -c -o bin/audio-test.exe ./internal/audio`: Test executable compiled successfully.
  - `GOOS=windows GOARCH=amd64 go test -c -o bin/config-test.exe ./internal/config`: Test executable compiled successfully.
  - `GOOS=windows GOARCH=amd64 go test -c -o bin/ipc-test.exe ./internal/ipc`: Test executable compiled successfully.
  - `GOOS=windows GOARCH=amd64 go test -c -o bin/worker-test.exe ./internal/worker`: Test executable compiled successfully.
  - `go test -race ./...`: 100% PASS across all 13 packages on macOS (0 regressions).
  - `gofmt -l .` and `go vet ./...`: 100% clean.
  - Windows runtime tests marked as **Pending** until run on a Windows host.
