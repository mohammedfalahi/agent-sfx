# Agent implementation instructions

## Mission
Build Agent SFX: a free, open-source, local CLI that plays short randomly selected sound effects for exactly seven terminal-agent moments. Gemini CLI is the first implemented and tested adapter; Claude Code is the next major target. The developer uses Gemini CLI to build this project; that does not require this add-on to be installed while development is in progress.

Product events:
1. permission_requested
2. task_started
3. task_finished
4. waiting_for_user
5. tests_passed
6. usage_exhausted
7. error

No additional automatic sounds: no startup greeting, compaction, rapid-prompt annoyance, workout reminders, subagent chatter, or periodic nagging. Session hooks may do silent lifecycle work. Preview is an explicit diagnostic, not a new event.

## Working procedure
1. Read `architecture.md`, `build-plan.md`, and current `progress.md`; inspect code and existing tests.
2. Identify the current requested milestone. Explain a short implementation plan, then implement only that milestone unless asked otherwise.
3. Check installed agent version and authoritative hook schemas before implementing detection. Use sanitized real fixtures when available. Missing evidence means unsupported, not guessed.
4. Prefer small targeted changes; preserve user files, configuration, and hooks.
5. Run relevant tests and format/vet checks. Report exactly what passed, failed, or could not run.
6. Update `progress.md` with changes, commands/results, limitations, and the next step. Stop at the milestone boundary.

## Technology and code rules
- Go, one CLI executable, shared core plus thin agent adapters. No web backend, database, Docker, LLM calls for detection, terminal wrapper, or PTY scraping.
- Prefer the Go standard library. The planned Windows named-pipe transport may use `github.com/Microsoft/go-winio` behind Windows build tags after verifying a compatible released version and license. Record and pin any dependency; do not add packages speculatively.
- Use the developer's supported stable Go toolchain; set go.mod to the version actually selected. Do not require an unreleased Go version.
- Small packages with explicit interfaces. Inject clock, random source, player, and transport for tests; avoid mutable package-global state.
- Use OS argument arrays for playback, never shell interpolation of user-controlled paths. No bash/jq/Python runtime dependency for the final Go add-on.
- Use JSON configuration and support short 16-bit PCM WAV assets first. Document unsupported formats; never silently promise MP3/OGG compatibility.
- Errors belong to the add-on's diagnostics, not the agent's control flow. No log output on hook stdout.

## Detection invariants
- A successful tool call is NOT task completion or tests passing.
- BeforeTool is NOT proof a permission dialog appeared.
- AfterAgent means the response/turn ended, not proof the requested goal succeeded.
- Compaction is NOT quota exhaustion. Context remaining is NOT subscription remaining.
- A 429 alone is temporary throttling or an ambiguous limit, not proof exhausted allowance.
- Ordinary assistant prose is not reliable proof of tests or quotas. Do not match arbitrary conversation text.
- Do not infer general waiting-for-user from silence or a question mark.
- Unknown hook events, malformed data, and unsupported schemas produce no sound.
- Prefer structured signals. Limited text recognition is allowed only inside verified tool/API error results, with positive and negative fixtures.
- Avoid duplicate classification: a permission request is not also waiting; an exhaustion event is not also a generic error for the same occurrence.
- No automatic sounds for the add-on's own failures; prevent recursive error noise.

## Safety and privacy
- Hook receiver always returns neutral `{}` and success for Gemini, even if parsing, IPC, config, or playback fails. Never return deny/block, exit 2, continue=false, rewritten tool input, added context, or retry instructions.
- Fail open: keep all hook work bounded; do not wait for playback, worker readiness, network access, or an interactive prompt.
- Hook startup has unavoidable overhead. Measure it; never claim zero latency.
- No automatic account login, credential extraction, usage-API reverse engineering, provider probing, telemetry upload, or transcript retention.
- Real payload capture must be explicit and redact prompts, paths, code, tokens, credentials, and tool content before committing fixtures. Prefer synthetic minimal fixtures.
- Do not install/uninstall global agent hooks, change telemetry, kill agent processes, or execute downloaded installers without explicit developer approval.
- Merge configuration atomically with a backup, preserve unrelated entries, and support reversible installation. Invalid existing JSON must be left untouched.
- Handle missing players/devices/assets gracefully. Linux playback is best-effort with existing audio programs, not guaranteed on every distribution, container, SSH session, or WSL setup.

## Licensing
Plan MIT for project code. Reusing MIT code requires retaining applicable notices; do not copy a whole implementation merely because it is available. Default sounds must be original generated sounds or verified CC0 assets with recorded provenance. Do not bundle game/movie/meme clips or assume an MIT repository license covers its sound assets. Users may add their own local files at their responsibility.

## Completion criteria
A milestone is complete only when its acceptance tests pass and the support matrix matches actual evidence. Cross-compilation is not proof of audible playback. Never claim Windows/macOS/Linux validation without running it on the respective environment. Leave unsupported product events explicitly documented instead of faking coverage.
