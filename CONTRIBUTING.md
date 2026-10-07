# Contributing to Agent SFX

Thank you for your interest in contributing to Agent SFX! This document provides guidelines for setting up your development environment, adhering to project conventions, and submitting changes.

---

## Mission and Design Principles

Agent SFX is a free, open-source, local-only sound accessory for terminal coding agents. Every contribution must respect these core architectural invariants:

1. **Seven Canonical Events Only**: The product recognizes a closed enumeration of seven moments:
   - `permission_requested`
   - `task_started`
   - `task_finished`
   - `waiting_for_user`
   - `tests_passed`
   - `usage_exhausted`
   - `error`
   No additional unprompted sounds (no startup greetings, periodic reminders, or chatty subagent sounds).
2. **Fail-Open Hook Receiver**: The hook receiver must always emit `{}` on stdout and exit `0` within bounded time (<25ms IPC). It must never crash, hang, block, or modify agent execution.
3. **Local-Only & Private**: No network listeners, no external cloud dependencies, no analytics/telemetry, and no retention of user prompts or transcripts.
4. **Platform Safety**: All child processes must be invoked using OS argument arrays, never shell string interpolation.

---

## Development Prerequisites

- **Go**: Version 1.22 or higher.
- **Node.js**: Version 18.0.0 or higher (for npm launcher tests and package validation).
- **Git**: Modern git client.
- **Supported Platforms**:
  - macOS: Apple Silicon (`arm64`) and Intel (`x64`)
  - Windows: 64-bit (`x64` / `amd64`)
  - Linux: Cross-compilation supported; audio playback is best-effort (`pw-play`, `paplay`, `aplay`).

---

## Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/mohammedfalahi/agent-sfx.git
   cd agent-sfx
   # PowerShell: Set-Location agent-sfx
   ```

2. **Run tests**:
   ```bash
   go test -race ./...
   ```

3. **Verify code formatting and analysis**:
   ```bash
   gofmt -l .
   go vet ./...
   ```

4. **Build the binary**:
   - macOS / Linux:
     ```bash
     go build -o bin/agent-sfx ./cmd/agent-sfx
     ```
   - Windows PowerShell:
     ```powershell
     go build -o bin\agent-sfx.exe .\cmd\agent-sfx
     ```

5. **Test Node launchers and packaging**:
   ```bash
   node npm/test_launcher.js
   cd npm && npm pack --dry-run && cd ..
   ```

---

## Repository Structure

```text
├── cmd/
│   ├── agent-sfx/         # CLI entry point and subcommand routing
│   └── gen-sounds/        # Mathematical synthesizer for CC0 sound assets
├── internal/
│   ├── adapters/          # Agent-specific hook decoders (gemini, claude)
│   ├── audio/             # Audio player abstractions and 16-bit PCM gain
│   ├── config/            # JSON configuration loader with file locking
│   ├── doctor/            # Environment and capability diagnostics
│   ├── events/            # Canonical event types and normalization contracts
│   ├── installer/         # Agent hook merger, uninstaller, and backup manager
│   ├── ipc/               # Unix domain socket & Windows named pipe transports
│   ├── preview/           # Direct manual sound playback
│   ├── scheduler/         # Deduplication, priority coalescing, and cooldown
│   ├── setup/             # Dry-run integration inspectors
│   ├── sounds/            # WAV header validation, asset discovery, no-repeat RNG
│   └── worker/            # Background daemon lifecycle, singleton lock
├── npm/                   # npm distribution package (@agent-sfx/agent-sfx)
│   ├── bin/               # Prebuilt platform binaries and cross-platform launcher
│   ├── claude/            # Self-contained Claude Code plugin
│   ├── hooks/             # Gemini extension hooks definition
│   ├── skills/            # Agent-operated control skill
│   └── sounds/            # Bundled CC0 starter sound assets
├── sounds/                # Active sound asset directory
└── testdata/              # Sanitized minimal JSON hook fixtures
```

---

## Audio Asset Guidelines

- **License Requirements**: All sound assets distributed in the repository or npm packages must be dedicated to the public domain under **Creative Commons CC0 1.0 Universal**.
- **No Third-Party Copyrights**: Never commit movie memes, video game sound effects, or commercial audio clips to the repository.
- **Synthesized Audio**: Default starter sounds are mathematically synthesized using `cmd/gen-sounds/main.go`. If adding or refining default sounds, update the synthesizer script so all audio remains reproducible.
- **Technical Specifications**:
  - Format: Uncompressed 16-bit signed PCM WAV.
  - Channels: 1 (mono) or 2 (stereo).
  - Duration: Maximum 15,000 ms (15 seconds).
  - File Size: Maximum 25 MiB per file.

---

## Submitting Pull Requests

1. **Create a focused branch** for your changes.
2. **Add automated tests**: Every bug fix or new normalization rule must include corresponding unit tests and sanitized minimal fixtures in `testdata/`.
3. **Verify cross-platform compilation**:
   ```bash
   # Test Linux build
   GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/agent-sfx

   # Test Windows build
   GOOS=windows GOARCH=amd64 go build -o /dev/null ./cmd/agent-sfx
   ```
4. **Ensure clean tests and checks**:
   `go test -race ./...`, `go vet ./...`, and `gofmt` must all pass cleanly.
5. **Update documentation**: Keep `README.md`, `SETUP.md`, or architecture documents up to date with your changes.
