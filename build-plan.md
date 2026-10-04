# Incremental build plan

No milestone is implemented merely because it appears here. Consult progress.md.

## M0 — A manual sound works (first coding prompt)
Implement a minimal Go module, CLI preview and doctor, config loader, closed seven-kind enum, deterministic testable sound selection, validated PCM16 WAV gain, and OS player abstraction. Supply seven short original software-generated WAVs with distinct audible patterns and explicit provenance/dedication. Do not copy peon-ping audio.

Acceptance:
- `go test ./...`, `go vet ./...`, and `gofmt` checks pass.
- `go build -o bin/agent-sfx ./cmd/agent-sfx` works (use .exe on Windows).
- `agent-sfx preview task_finished` plays on the current machine or doctor explains missing capability.
- Fake-player tests cover config disabled, missing/invalid WAV, volume bounds and no immediate repeat.
- README clearly distinguishes implemented commands from planned commands.
No installer, agent hooks, IPC worker, quota monitoring, or Claude implementation in M0.

## M1 — Shared worker and neutral hook receiver
Implement local transport, OS-specific endpoint permissions, singleton worker, worker lifecycle CLI, injectable scheduler, queue bounds, cooldown/dedupe and PCM cache cleanup. Implement `hook gemini` as a bounded fail-open receiver with neutral output; use synthetic fixtures only until real schemas are captured.

Acceptance:
- Bad stdin, oversized input, unknown event, dead worker and unavailable player always give hook stdout `{}` and exit 0.
- One player at a time; cooldown and dedupe deterministic under fake clock.
- No stale event queues, IPC waits/retries or agent process modification.
- Platform build checks pass. Report platform runtime tests separately.

## M2 — Verified Gemini MVP
Record installed Gemini CLI version. Confirm authoritative schemas; obtain sanitized real payloads with explicit developer approval when needed. Implement four verified mappings: ToolPermission notification, BeforeAgent, AfterAgent and exposed AfterTool tool errors. Maintain a support matrix. Implement dry-run installer, owned-hook merge/uninstall, backups, and silent worker startup.

Acceptance:
- Permission, turn start, turn end and tool error fixture tests include negative cases.
- No completion on ordinary successful tools; no tests_passed from exit zero alone.
- Preserve notification_type; use verified tool_response fields.
- Existing hooks/settings survive install/reinstall/uninstall and invalid JSON stays unchanged.
- After developer-approved installation, verify sounds in a real Gemini session and measure hook overhead.
- Waiting-for-user, tests_passed, quota exhaustion and unobserved API errors remain explicitly unsupported.

## M3 — Test-result detection
Add at most pytest and Go tests first. Recognize verified runner command/result shapes; require success plus an unambiguous passed summary. Test failure, skipped-only, zero tests, interruptions, quoted prose and compound commands. If results cannot establish success reliably, emit nothing.

## M4 — API errors and exhausted usage investigation
Spike Gemini local telemetry: verify installed-version format, event fields, exporter flushing/delay and privacy before designing a parser. File output is NOT assumed to be JSONL. Keep opt-in, local-only, bounded and content-minimizing. No enabling telemetry or reading credentials without approval.

Only add usage_exhausted with confirmed structured evidence. Do not map every 429, invalid API key, context compaction or estimate to exhaustion. Consider an optional CodexBar command-hook connector, but don't require a dashboard or usage monitor for ordinary sounds. Report unsupported coverage when the spike cannot verify it.

## M5 — Claude Code and release hardening
Implement separate Claude adapter from currently verified hooks, reusing core only. Verify StopFailure version/fields rather than copying outdated mappings. Do not assume shell exit failures always produce a dedicated failure hook.

Run macOS/Linux/Windows manual playback/integration checklist, write beginner-friendly install/uninstall and custom-sound guides, audit code/asset licenses, and add release workflow/checksums. Add no extra audible categories.

## Definition of done for each milestone
Working code, tests with actual results, honest support matrix, relevant docs, and updated progress.md. Stop after the requested milestone and explain how the developer can test it. Never claim future milestones are complete.
