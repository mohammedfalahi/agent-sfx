# Set up the Agent SFX project

This ZIP contains context files, not a working sound add-on. Keep the files at your new project's root. Gemini CLI uses GEMINI.md as its default context entry point; it imports agent.md, architecture.md and build-plan.md.

## 1. Create the project folder

macOS/Linux (adjust the downloaded ZIP path):
```bash
mkdir agent-sfx
unzip ~/Downloads/agent-sfx-context.zip -d agent-sfx
cd agent-sfx
git init
```

Windows PowerShell (adjust the downloaded ZIP path):
```powershell
New-Item -ItemType Directory -Path agent-sfx
Expand-Archive -Path "$HOME\Downloads\agent-sfx-context.zip" -DestinationPath agent-sfx
Set-Location agent-sfx
git init
```

Expected layout:
```text
agent-sfx/
  GEMINI.md
  agent.md
  architecture.md
  build-plan.md
  progress.md
  SETUP.md
  .gitignore
  docs/research.md
```

Do not move these files to your global ~/.gemini folder. They belong to this project. Do not download or execute peon-ping's installer; its code was research, not a required dependency.

## 2. Check tools
```text
git --version
go version
gemini --version
```
You already use Gemini CLI. Install a supported stable Go toolchain from https://go.dev/dl/ if missing, then reopen the terminal and check go version. No Docker, Python, jq, Redis or database needed. Node may already be required by Gemini CLI itself; our planned executable does not require it.

If Gemini cannot authenticate or serve requests, resolve your account's supported access before coding. Do not change agent targets or account settings based on assumptions.

## 3. Launch Gemini from this directory
```bash
gemini
```
Inside Gemini:
```text
/memory show
```
Confirm the output includes both "Agent implementation instructions" and "Agent SFX architecture", plus the build plan. If not:
- Confirm current directory and exact case-sensitive filename GEMINI.md.
- Run /memory reload if supported by your version, then /memory show.
- If the subcommand is unavailable, check /help or restart Gemini from the project root.
- If you customized context.fileName, merge GEMINI.md into the existing list instead of overwriting other settings.
- Review loaded global instructions for conflicting technology/project rules.

Imports are expanded into context; modular files improve organization but do not reduce token usage. The huge peon-ping function audit is intentionally NOT imported. Consult the small docs/research.md only when needed.

After editing context files, reload or restart. Context provides model instructions; it is not a security boundary or a guarantee of perfect compliance.

## 4. First prompt — build only M0
Paste:
```text
Read the loaded project context and progress.md, and inspect this repository.
Implement only milestone M0 from build-plan.md.
First check the installed Go version and audio backend. Scaffold a small Go CLI
with preview and doctor, the seven-event enum, JSON config, validated 16-bit PCM
WAV playback with volume control, random selection without immediate repeats,
and fake-player unit tests. Use original software-generated starter sounds with
recorded provenance. Do not install Gemini hooks, build the IPC worker, enable
telemetry, implement Claude, or add guessed detection.
Run formatting, go test ./..., go vet ./..., and a build. Report actual results
and any audio/platform tests you could not run. Update progress.md and explain
how I can manually preview a sound. Stop after M0.
```

## 5. Test the first implementation
These commands are expected AFTER Gemini implements M0; they do not work with the context pack alone.

macOS/Linux:
```bash
go test ./...
go vet ./...
go build -o bin/agent-sfx ./cmd/agent-sfx
./bin/agent-sfx doctor
./bin/agent-sfx preview task_finished
```

Windows PowerShell:
```powershell
go test ./...
go vet ./...
go build -o bin/agent-sfx.exe ./cmd/agent-sfx
.\bin\agent-sfx.exe doctor
.\bin\agent-sfx.exe preview task_finished
```

No sound? Start with doctor. Missing Linux audio programs/devices, remote shells and containers are legitimate limitations; don't ask Gemini to install audio software or alter system settings without reviewing the change.

## 6. Subsequent prompts
After M0 actually passes:
```text
Read progress.md and implement only M1. Keep transport and scheduling testable,
make the Gemini hook receiver neutral/fail-open, and do not install hooks yet.
Run tests, update progress.md, and stop at the milestone boundary.
```
Then:
```text
Read progress.md and docs/research.md. Implement only M2 against my installed
Gemini CLI version. Verify hook schemas, add the four supported mappings and
installer dry-run/tests. Ask before capturing real payloads or installing any
hooks. Preserve existing settings. Keep the other events explicitly unsupported.
```
Only after reviewing its dry-run should you approve project-scope hook installation. Add tests-pass detection next; investigate authoritative API/quota signals later. No need to complete all seven detections before a useful first release.

## Sources
- https://geminicli.com/docs/cli/gemini-md
- https://geminicli.com/docs/hooks/reference/
- https://go.dev/doc/install
