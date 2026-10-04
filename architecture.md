# Agent SFX architecture

Status: design baseline; no application code is shipped in this context pack.

## Product boundaries
A local sound-only accessory for terminal coding agents. Users place sounds in seven event folders; the app selects one randomly, applies volume and short cooldowns, and plays asynchronously. Installation should be quick. Agent execution, permissions, tool results, model context, and authentication must remain unchanged.

Gemini CLI first, Claude Code later. No editor integrations, screen overlays, animated characters, server infrastructure, or cloud subscriptions required by this add-on. The underlying agent may require paid access; verify the developer's Gemini access independently.

## Data flow
```text
Gemini hook JSON                    Future Claude hook JSON
        |                                    |
        v                                    v
Gemini adapter                         Claude adapter
        +------------------+-----------------+
                           v
             normalized event or Ignore
                           |
                bounded local IPC send
                           v
                background sound worker
                           |
     validate -> dedupe -> priority/coalesce -> cooldown
                           |
                 random WAV selection
                           |
                  volume processing
                           v
               OS-specific audio player
```
Hook processes only parse, normalize, attempt a short IPC send, and respond neutrally. Playback/state live in a worker. A silent SessionStart hook may start a detached worker; it must not emit a greeting or wait for readiness. If the worker is absent, drop the current sound. Do not launch a new worker on every tool hook.

M0 builds preview and core behavior first; the worker is introduced in M1, not before a manual sound works.

## Repository shape
```text
GEMINI.md                 # imports agent.md, architecture.md, build-plan.md
agent.md
architecture.md
build-plan.md
progress.md
SETUP.md
docs/research.md
cmd/agent-sfx/            # main and CLI command wiring
internal/
  events/                # canonical seven kinds, validation
  adapters/gemini/       # version-aware Gemini payload decoding/mapping
  adapters/claude/       # future; no placeholder implementation needed now
  config/                # JSON validation and defaults
  worker/                # state, scheduling, bounded playback
  ipc/                   # local transport with OS-specific files
  audio/                 # player selection and PCM WAV gain
  installer/             # owned-hook merge, backup, remove
sounds/
  permission_requested/
  task_started/
  task_finished/
  waiting_for_user/
  tests_passed/
  usage_exhausted/
  error/
testdata/gemini/          # sanitized minimal fixtures
README.md
SOUND-LICENSES.md
```
Do not create empty packages for unimplemented integrations. Add files as milestones need them.

## Canonical event contract
Use a closed EventKind enumeration containing only the seven product names. Ignore is a classification result, not an eighth audible event.

Event fields:
- kind: required canonical event kind.
- agent: e.g. gemini or claude.
- session_id: stable ID from the agent; do not invent a persistent session from each hook PID.
- observed_at: local receipt time for scheduling; agent timestamp may be retained separately.
- dedupe_key: source event ID when available; otherwise a narrowly scoped composite with a short expiry. Do not pretend a synthesized key is a globally unique task ID.
- reason_code: optional coarse enum (tool_failure, quota_exhausted, etc.), never raw prompt/error content.

Adapter interface concept:
`Normalize(payload) -> []Event, diagnostics`
Return no events for unknown or unsupported input. One tool result can contain multiple supported signals; apply specificity rules before scheduling. Keep parsing/classification pure. Transport/playback cannot be called by an adapter.

## Gemini support matrix and semantics
Verify every row against the installed version and fixtures before enabling it.

| Product event | Initial signal | Release claim |
| --- | --- | --- |
| permission_requested | Notification.notification_type == ToolPermission | Documented direct signal; verify actual payload |
| task_started | BeforeAgent | User prompt/turn starts, not session startup |
| task_finished | AfterAgent | Response/turn ends, not guaranteed task success |
| waiting_for_user | No general signal confirmed in the audited hook reference | Unsupported initially; add only a verified explicit input/question event |
| tests_passed | Verified AfterTool result for recognized direct Go test or pytest runner | Direct runner invocation with verified successful completion and positive test summary (M3) |
| usage_exhausted | No dedicated Gemini hook confirmed | Unsupported initially; evaluate local telemetry/API error source later |
| error | AfterTool.tool_response.error when present | Tool errors only initially, not every possible runtime/API error |

Do not copy peon-ping's Gemini top-level exit_code assumption, successful AfterTool -> Stop mapping, or discarded notification subtype. Its documented sound category resource.limit is not authoritative remaining quota.

For tests_passed (M3): supports direct `go test` and `pytest` (`pytest`, `python -m pytest`, `python3 -m pytest`). Requires verified successful command completion (`Process Group PGID: <pid>` present in trailing metadata, no signal, no exit code, no error section, no cancellation, and not backgrounded).
- For Go: requires at least one qualifying passed package (`ok <pkg> <duration>`). Excludes individual packages marked `[no test files]` or `[no tests to run]`. Cached passes (`ok <pkg> (cached)`) are classified with reason code `go_test_cached`; runs with newly executed tests use `go_test_passed`. Rejects `[build failed]`, syntax errors, panics, or any package failure.
- For pytest: requires at least one passed test in the terminal summary line (`=== N passed ... ===`), zero failures, zero errors, and no interruption. Excludes skipped-only or no-test runs.
- Invariants: rejects compound commands (`&&`, `||`, `;`, `|`, `&`), subshells (`$()`, backticks), redirections (`>`, `<`), leading environment variable assignments, unsupported output modes (e.g. `go test -json`), and background execution. Error detection strictly takes precedence over test detection for the same occurrence.

For error: do not treat stderr alone as failure; many successful programs write warnings/progress to stderr. Shell exit detection parses only verified generated metadata (non-zero `Exit Code: X` in the trailing metadata block of `run_shell_command`) rather than arbitrary command output. No new error sound for user cancellation unless explicitly agreed. Diagnostic tracing is strictly opt-in via `AGENT_SFX_TRACE_LOG`, creates no files or directories when unset, retains allowlisted metadata only with fixed reason codes and 0600 permissions, and never logs prompts, paths, commands, error messages, or tool outputs.

For usage_exhausted: a structured provider exhaustion/billing-zero reason can qualify; generic invalid credentials or 429-only responses should be error/unsupported, not guessed exhaustion. Any later provider quota connector must distinguish authoritative/fresh data from estimates and account scope from session scope.

## Worker and transport
- One worker per OS user/install scope, with singleton acquisition. A stale endpoint is not permission to kill an arbitrary process.
- Unix-domain socket on macOS/Linux; Windows named pipe via a narrowly scoped platform dependency if approved. No internet listener and no remote control surface.
- Unix socket directory owner-only; Windows pipe ACL restricted to current user. Validate bounded event messages, event kind and protocol version.
- Initial transport protocol v1: one compact JSON event per bounded message, max 8 KiB. Hook stdin has a separate defensive max (initially 1 MiB); oversized input is ignored, not truncated into misleading JSON.
- IPC deadline initially 25 ms (excludes process startup/JSON decoding). No retries on the hook path. Receiver timeout and bounded queue prevent stalls.
- Worker launch is detached with inherited stdio closed; platform-specific spawn implementation. SessionStart may attempt startup without waiting. For the MVP, keep the worker alive until explicit stop or OS shutdown; do not silently exit during a long-lived agent session. Automatic idle shutdown/session tracking is a later optimization, not required now.
- Worker is optional for manual preview. No install-time background service required.

## Scheduling
- Persistent worker state: last played file per event, scoped dedupe timestamps, active player PID/handle. Only retain content-free state; bounded maps and expiry.
- Defaults: global cooldown 1200 ms, maximum clip duration 15000 ms, at most one player. Valid configuration range is 1 to 15000 ms (15000 ms is both the default and the hard maximum). Don't queue sounds for later after the event becomes stale.
- Process timeout is actual clip duration + 5 seconds headroom to absorb CoreAudio/ALSA device initialization and buffer teardown latency. Process overhead does not permit audio longer than max_clip_ms.
- Audio duration is validated precisely without rounding down longer clips; any audio exceeding the configured limit is rejected without truncation.
- Coalesce near-simultaneous pending events over at most 100 ms with priority: usage_exhausted, error, permission_requested, waiting_for_user, tests_passed, task_finished, task_started. Drop lower-priority events in that window.
- Same-occurrence specificity: permission_requested beats waiting_for_user; usage_exhausted beats generic error; tests_passed may beat immediately adjacent finish for sound scheduling, but is not evidence the whole task succeeded.
- Do not preempt already-playing clips in v1. Drop arriving events during active playback or cooldown. Document this best-effort tradeoff instead of claiming every event will be audible.
- Dedupe per agent/session/kind/key; global playback gate prevents different terminals from stacking sounds.
- Use an injectable monotonic clock for local cooldown and deterministic RNG for tests. Preserve no-repeat behavior when at least two valid sounds are available.

## Audio
- Starter format: RIFF/WAVE, uncompressed signed 16-bit PCM, mono/stereo, bounded file size (25 MiB) and duration (up to max_clip_ms, default and hard maximum 15000 ms). Reject malformed headers, unsafe chunk lengths, unsupported encoding, missing files, or clips exceeding max_clip_ms without crashing or truncating.
- Volume is 0.0..1.0 and applied to PCM samples, not system master volume. Scale samples in a validated temporary/cache WAV keyed by file identity and volume; cap cache size and clean it up.
- macOS backend: afplay.
- Linux: select available pw-play, then paplay, then aplay; bounded execution and real capability checks. If none works, doctor reports unavailable and hook playback stays silent.
- Windows: fixed noninteractive PowerShell command using .NET SoundPlayer; pass WAV path safely as an argument/environment value to a fixed script, never interpolate it into executable source. Launch off the hook path, hide windows where supported.
- On playback timeout, stop only the child player this worker owns. Do not kill unrelated audio/agent processes.
- A doctor command diagnoses missing player/device/asset/config support. No audio software is automatically installed.

## Config and asset paths
Use OS user-config/user-cache locations (os.UserConfigDir/os.UserCacheDir) and make doctor print resolved paths. Optional --config path override. Config v1 example:
```json
{
  "version": 1,
  "enabled": true,
  "volume": 0.6,
  "cooldown_ms": 1200,
  "max_clip_ms": 15000,
  "events": {
    "permission_requested": true,
    "task_started": true,
    "task_finished": true,
    "waiting_for_user": true,
    "tests_passed": true,
    "usage_exhausted": true,
    "error": true
  }
}
```
Enabled does not mean supported by the current adapter. Missing folders mean silent skip, not an error sound. Separate user assets from bundled defaults so upgrades do not overwrite custom sounds. Invalid values receive explicit diagnostics; the hook remains fail-open.

Updating older configs: If an existing config specifies `"max_clip_ms": 3000`, update it to `15000` (the 15-second hard maximum) in `os.UserConfigDir()/agent-sfx/config.json` to allow longer custom meme audio clips to play completely.

## CLI contract (planned, not currently implemented)
- `agent-sfx preview <event>`: explicit manual playback.
- `agent-sfx doctor [--json]`: report version, paths, audio capabilities, and adapter support.
- `agent-sfx hook gemini`: JSON stdin -> bounded handoff -> `{}` stdout + exit 0.
- `agent-sfx worker start|stop|status`: local worker lifecycle. Stop only owned worker.
- `agent-sfx install gemini [--scope project|user] [--dry-run]`: default project; merge owned hooks.
- `agent-sfx uninstall gemini [--scope project|user] [--dry-run]`: remove only owned entries.

## Installer and hook safety
Gemini hooks live in project .gemini/settings.json or user ~/.gemini/settings.json. Preserve the schema's matcher group / hooks array / type command structure; don't assume each event directly contains a command. Verify version-specific schema before writing it. Register only relevant implemented hooks and optional silent lifecycle startup. No BeforeTool sound registration unless a future validated case needs it.

Use distinctive owned hook names, reliable platform quoting and absolute executable paths. Merge idempotently, atomically write with a backup, reject invalid JSON without changing it, and report planned diff via --dry-run. Repeated install must not duplicate hooks; uninstall must retain user hooks and unrelated settings. Capture fixtures is a separate explicit development workflow, not normal production logging.

## Testing and release gates
- Unit: payload decoding, exact mappings, unsupported/malformed input, specificity, clock/RNG, WAV validation/gain, config bounds, installer ownership.
- Integration: hook stdout exactly `{}` plus newline, exit zero on invalid data, dead IPC and player failures; worker singleton, restricted endpoint, dedupe/cooldown, timeout cleanup.
- Test with fake player/transport; no network/credentials/audio hardware required by default tests.
- Benchmark receiver processing and measure real Gemini hook startup separately. Record p50/p95 locally; no zero-overhead claims.
- Cross-compile windows/darwin/linux. Then separately test audible playback and hook integration on real target machines. CI cannot prove audio works.
- README support matrix shows direct, conditional and unsupported events per tested agent version. Specify that completion is turn completion.
- Release via GitHub Actions/GoReleaser after an initial working MVP. Include checksums, license notices and asset provenance.
