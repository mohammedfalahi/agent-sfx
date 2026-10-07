# Security Policy

Agent SFX is a local, sound-only accessory for terminal coding agents. This document describes the project's security architecture, threat model, and vulnerability reporting procedures.

---

## Reporting a Vulnerability

If you discover a potential security vulnerability in Agent SFX, please do not open a public GitHub issue.

- **Reporting Channel**: Please report vulnerabilities privately by emailing the maintainers or opening a private security advisory on GitHub at [https://github.com/mohammedfalahi/AgentSFX/security/advisories](https://github.com/mohammedfalahi/AgentSFX/security/advisories).
- **Information to Include**:
  - Detailed description of the vulnerability and attack vector.
  - Minimal reproducible example or proof-of-concept payload.
  - Affected versions and target operating system(s).
  - Any potential impact on agent execution, local files, or system stability.
- **Response Timeline**: Maintainers will acknowledge reports within 72 hours and provide an initial assessment and remediation timeline.

---

## Threat Model and Security Architecture

Agent SFX is designed to run locally alongside terminal coding agents (such as Gemini CLI and Claude Code) without compromising agent workflows, authentication, or user privacy.

### 1. Local-Only Execution Boundary
- **No Network Listeners**: Agent SFX binds exclusively to local IPC mechanisms (Unix domain sockets on macOS/Linux, local named pipes on Windows). It never binds to TCP/UDP ports and does not listen on network interfaces.
- **No Outbound Telemetry**: The application transmits zero telemetry, usage analytics, or error reports to external services.
- **No Credential Access**: Agent SFX does not inspect, read, or require API keys, authentication tokens, credentials, or agent transcripts.
- **Privacy by Design**: Hook receivers process agent metadata in memory to classify events and discard raw prompt text, command strings, and file paths.

### 2. IPC Isolation & Permissions
- **macOS / Linux**:
  - Worker IPC uses a Unix domain socket located in `os.UserConfigDir()/agent-sfx/worker.sock` (or system temporary directory).
  - Socket directories are created with owner-only permissions (`0700`), and sockets are restricted to the local user (`0600`).
- **Windows**:
  - Worker IPC uses a local named pipe scoped to the user's Security Identifier (`\\.\pipe\agent-sfx-<SID>`).
  - Named pipe listeners are configured with a strict Discretionary Access Control List (DACL: `D:P(A;;GA;;;<SID>)`) that rejects inheritance and permits access solely to the current user.
  - Remote client access is explicitly rejected (`FILE_PIPE_REJECT_REMOTE_CLIENTS`).

### 3. Safe Child Process Execution
- **Argument Arrays (No Shell Interpolation)**: All child processes (audio players, background daemons) are invoked directly via operating system argument arrays (`exec.Command` in Go, `spawnSync` in Node launchers). No commands or file paths are evaluated through arbitrary shell string interpolation.
- **Windows PowerShell Playback**: Windows audio playback uses a constant, non-interactive script executed via `powershell.exe`. The target audio path is supplied strictly through a child-process environment variable (`AGENT_SFX_PLAY_PATH`) and evaluated using `-LiteralPath`, preventing script injection, bracket/special character syntax errors, and wildcard expansion.
- **Window Hiding**: On Windows, player and background worker processes set `CREATE_NO_WINDOW` and `CREATE_NEW_PROCESS_GROUP` to prevent visible terminal window flashes and isolate process signals.

### 4. Audio Asset & Resource Bounds
- **Format Restrictions**: Supports only uncompressed 16-bit signed PCM WAV files. Compressed formats (e.g., MP3, OGG, AAC) are not parsed or executed.
- **File Size Ceiling**: Audio files are defensively capped at 25 MiB.
- **Duration Ceiling**: Playback duration is bounded by a hard maximum of 15,000 ms (15 seconds). Any clip exceeding the configured duration limit is rejected before playback.
- **Playback Timeout**: Process execution timeouts grant actual audio duration plus a fixed 5-second headroom to absorb OS audio subsystem latency, terminating unresponsive child players.

### 5. Fail-Open Hook Receiver Contract
- **Non-Blocking Operation**: The hook receiver reads stdin up to a defensive maximum of 1 MiB and uses a strict 25ms IPC deadline to communicate with the background worker.
- **Neutral Output**: The hook receiver unconditionally outputs `{}` on stdout and exits with code `0`. It never returns error exits, cancellation directives, or modified tool calls to the host agent, ensuring that a sound accessory failure cannot interrupt or corrupt active agent tasks.

### 6. Atomic Configuration & Hook Management
- **Safe State Updates**: Configuration files (`config.json`) and agent settings (`settings.json`) are updated atomically using temporary files and filesystem replace semantics, accompanied by `.bak` backup preservation.
- **Ownership Verification**: Hook installer and uninstaller commands touch only owned hook entries matching the specific binary signature. Unrelated hooks and agent settings are preserved.

---

## Known Limitations and Residual Risks

1. **User Boundary Trust**: Agent SFX assumes the local operating system user account is trusted. Any malicious software running under the same user privileges can interact with the user's local socket or named pipe.
2. **Audio Backend Reliance**: Agent SFX relies on native operating system tools (`afplay` on macOS, `powershell.exe` on Windows, `pw-play`/`paplay`/`aplay` on Linux). Vulnerabilities in these OS-provided utilities are outside the scope of this project.
3. **Filesystem Atomicity on Windows**: While configuration replacement uses Windows `MoveFileEx(..., MOVEFILE_REPLACE_EXISTING)`, NTFS does not provide transactional power-loss atomicity equivalent to POSIX renames.
4. **Third-Party Sound Files**: Users may place arbitrary WAV files in their local sounds directory. While Agent SFX rigorously validates WAV headers, chunk sizes, and PCM sample depths, users should only add audio assets from trusted sources.
