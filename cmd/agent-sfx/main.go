package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"agent-sfx/internal/adapters/claude"
	"agent-sfx/internal/adapters/gemini"
	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/doctor"
	"agent-sfx/internal/events"
	"agent-sfx/internal/installer"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/preview"
	"agent-sfx/internal/setup"
	"agent-sfx/internal/sounds"
	"agent-sfx/internal/worker"
)

func printUsage() {
	fmt.Printf(`Agent SFX — local sound accessory for terminal coding agents (v%s)

Usage:
  agent-sfx <command> [arguments]

Implemented Commands:
  preview <event>                      Play a sound effect manually (M0)
                                       Valid events: permission_requested, task_started, task_finished,
                                                     waiting_for_user, tests_passed, usage_exhausted, error
                                       Flags: --config <path>, --sounds-dir <path>

  doctor                               Inspect audio backend, paths, configuration, and worker (M0/M1)
                                       Flags: --json, --config <path>, --sounds-dir <path>

  worker start                         Start the background audio worker daemon (M1)
                                       Flags: --config <path>, --sounds-dir <path>

  worker stop                          Stop the background audio worker daemon (M1)

  worker status                        Check if the worker daemon is running (M1)

  on                                   Enable Agent SFX sounds (persisted + live worker)
                                       Flags: --config <path>

  off                                  Disable Agent SFX sounds (persisted + cancel active/clear pending)
                                       Flags: --config <path>

  status                               Show persisted configuration and live worker status
                                       Flags: --config <path>, --json

  hook gemini                          Neutral stdin/stdout hook receiver for Gemini CLI (M1/M2)

  hook claude                          Neutral stdin/stdout hook receiver for Claude Code (M5)

  install gemini                       Install hooks into .gemini/settings.json (M2)
                                       Flags:
                                         --scope <project|user>  Scope (default: project)
                                         --dry-run               Show exact proposed changes without writing

  uninstall gemini                     Remove owned hooks from .gemini/settings.json (M2)
                                       Flags:
                                         --scope <project|user>  Scope (default: project)
                                         --dry-run               Show exact proposed changes without writing

  setup gemini|claude|deploy           Inspect integrations or deploy self-contained user packages
                                       Subcommands:
                                         gemini --dry-run        Inspect Gemini CLI integrations and conflicts
                                         claude --dry-run        Inspect Claude Code integrations and conflicts
                                         deploy [--dry-run]      Deploy self-contained packages to user location

Planned Commands (Future Milestones):
  Claude Code Adapter                  Dedicated adapter for Claude Code hooks (planned for M5)
`, doctor.AppVersion)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "preview":
		runPreview(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "worker":
		runWorker(os.Args[2:])
	case "on":
		runOn(os.Args[2:])
	case "off":
		runOff(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	case "hook":
		runHook(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runPreview(args []string) {
	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to custom configuration JSON file")
	soundsDir := fs.String("sounds-dir", "", "Path to custom sounds directory")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	positional := fs.Args()
	if len(positional) < 1 {
		fmt.Fprintf(os.Stderr, "Error: missing event name for preview.\n\n")
		fmt.Fprintf(os.Stderr, "Available events:\n")
		for _, k := range events.AllEvents {
			fmt.Fprintf(os.Stderr, "  - %s\n", k)
		}
		os.Exit(1)
	}

	eventStr := positional[0]
	eventKind, err := events.ParseEventKind(eventStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, _, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	player, err := audio.DetectPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Player detection error: %v\n", err)
		os.Exit(1)
	}

	cache, err := audio.NewCache("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cache error: %v\n", err)
		os.Exit(1)
	}

	resolvedSounds := preview.ResolveSoundsDir(*soundsDir, cfg.SoundsDir)

	runner := &preview.Runner{
		Cfg:       cfg,
		Player:    player,
		Cache:     cache,
		Selector:  sounds.NewSelector(nil),
		SoundsDir: resolvedSounds,
	}

	result, err := runner.Preview(context.Background(), eventKind)
	if err != nil {
		if result != nil && result.Skipped {
			fmt.Printf("[SKIPPED] %s: %s\n", eventKind, result.SkipReason)
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "[ERROR] %s: %v\n", eventKind, err)
		os.Exit(1)
	}

	relSound, _ := filepath.Rel(".", result.SoundPath)
	if relSound == "" {
		relSound = result.SoundPath
	}

	fmt.Printf("[OK] Played %s using %s (volume: %.2f, duration: %dms, asset: %s)\n",
		result.Event, result.Player, result.Volume, result.DurationMS, relSound)
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output doctor diagnostics in JSON format")
	configPath := fs.String("config", "", "Path to custom configuration JSON file")
	soundsDir := fs.String("sounds-dir", "", "Path to custom sounds directory")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	rep := doctor.GenerateReport(*configPath, *soundsDir)

	if *jsonOutput {
		jsonStr, err := rep.FormatJSON()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to format JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	} else {
		fmt.Print(rep.FormatHuman())
	}

	if !rep.Audio.Available {
		os.Exit(1)
	}
}

func runWorker(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: agent-sfx worker <start|stop|status>\n")
		os.Exit(1)
	}

	subcommand := args[0]
	subArgs := args[1:]

	socketPath, err := ipc.ResolveSocketPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Worker error: %v\n", err)
		os.Exit(1)
	}

	switch subcommand {
	case "start":
		fs := flag.NewFlagSet("worker start", flag.ExitOnError)
		configPath := fs.String("config", "", "Path to custom configuration file")
		soundsDir := fs.String("sounds-dir", "", "Path to custom sounds directory")
		_ = fs.Parse(subArgs)

		resolvedSounds := preview.ResolveSoundsDir(*soundsDir, "")
		resp, newlyStarted, err := worker.StartDaemon("", socketPath, *configPath, resolvedSounds)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to start worker daemon: %v\n", err)
			os.Exit(1)
		}
		if newlyStarted {
			fmt.Printf("[OK] Worker daemon started (PID: %d, socket: %s)\n", resp.PID, socketPath)
		} else {
			fmt.Printf("[INFO] Worker daemon already running (PID: %d, uptime: %ds, socket: %s)\n", resp.PID, resp.Uptime, socketPath)
		}

	case "stop":
		resp, err := worker.StopDaemon(socketPath)
		if err != nil {
			fmt.Printf("[INFO] Worker daemon is not running\n")
			return
		}
		if resp.OK {
			fmt.Printf("[OK] Worker daemon stopped\n")
		} else {
			fmt.Fprintf(os.Stderr, "[ERROR] Worker stop failed: %s\n", resp.Error)
			os.Exit(1)
		}

	case "status":
		resp, err := worker.StatusDaemon(socketPath)
		if err != nil {
			fmt.Printf("Worker daemon is stopped\n")
			return
		}
		activeStatus := "idle"
		if resp.Playing {
			activeStatus = "playing/cooling down"
		}
		fmt.Printf("Worker daemon is running [PID: %d, uptime: %ds, state: %s, socket: %s]\n",
			resp.PID, resp.Uptime, activeStatus, socketPath)

	case "run":
		// Internal command executed by detached daemon
		fs := flag.NewFlagSet("worker run", flag.ExitOnError)
		configPath := fs.String("config", "", "Path to custom configuration file")
		soundsDir := fs.String("sounds-dir", "", "Path to custom sounds directory")
		_ = fs.Parse(subArgs)

		cfg, _, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
			os.Exit(1)
		}

		resolvedSounds := preview.ResolveSoundsDir(*soundsDir, cfg.SoundsDir)
		daemon, err := worker.NewDaemon(cfg, resolvedSounds, nil, nil, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Worker init error: %v\n", err)
			os.Exit(1)
		}

		if err := daemon.Run(socketPath); err != nil {
			fmt.Fprintf(os.Stderr, "Worker runtime error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown worker subcommand %q. Valid: start, stop, status\n", subcommand)
		os.Exit(1)
	}
}

func runHook(args []string) {
	if len(args) < 1 {
		fmt.Println("{}")
		os.Exit(0)
	}

	adapter := args[0]
	socketPath, _ := ipc.ResolveSocketPath()
	exePath, _ := os.Executable()
	resolvedSounds := preview.ResolveSoundsDir("", "")

	switch adapter {
	case "gemini":
		receiver := &gemini.Receiver{
			SocketPath: socketPath,
			BinaryPath: exePath,
			SoundsDir:  resolvedSounds,
			TestMode:   false,
		}
		_, _ = receiver.ProcessHook(context.Background(), os.Stdin, os.Stdout)
		os.Exit(0)

	case "claude":
		receiver := &claude.Receiver{
			SocketPath: socketPath,
			BinaryPath: exePath,
			SoundsDir:  resolvedSounds,
			TestMode:   false,
		}
		_, _ = receiver.ProcessHook(context.Background(), os.Stdin, os.Stdout)
		os.Exit(0)

	default:
		fmt.Println("{}")
		os.Exit(0)
	}
}

func runInstall(args []string) {
	var filtered []string
	var adapter string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") && adapter == "" {
			adapter = arg
		} else {
			filtered = append(filtered, arg)
		}
	}

	if adapter != "gemini" {
		fmt.Fprintf(os.Stderr, "Usage: agent-sfx install gemini [--scope project|user] [--dry-run]\n")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("install", flag.ExitOnError)
	scopeStr := fs.String("scope", "project", "Configuration scope ('project' or 'user')")
	dryRun := fs.Bool("dry-run", false, "Print proposed changes without writing")
	binaryPath := fs.String("binary", "", "Path to agent-sfx binary (defaults to current executable)")

	if err := fs.Parse(filtered); err != nil {
		os.Exit(1)
	}

	scope := installer.Scope(*scopeStr)
	settingsPath, err := installer.ResolveSettingsPath(scope, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scope resolution error: %v\n", err)
		os.Exit(1)
	}

	plan, rootMap, err := installer.PlanInstall(scope, settingsPath, *binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Install plan failed: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		fmt.Printf("--- Dry-Run: Agent SFX Gemini Hook Installation ---\n")
		fmt.Printf("Scope         : %s\n", plan.Scope)
		fmt.Printf("Target File   : %s\n", plan.SettingsPath)
		fmt.Printf("Hooks to Add  : %v\n", plan.AddedHooks)
		fmt.Printf("Unchanged     : %v\n", plan.UnchangedHooks)
		if len(plan.UpdatedHooks) > 0 {
			fmt.Printf("Hooks to Update: %v\n", plan.UpdatedHooks)
		}
		if len(plan.Conflicts) > 0 {
			fmt.Printf("Conflicts     :\n")
			for _, c := range plan.Conflicts {
				fmt.Printf("  ! %s\n", c)
			}
		}
		fmt.Printf("\nProposed settings.json content:\n%s\n", plan.ProposedJSON)
		fmt.Printf("[DRY-RUN] No files were modified.\n")
		return
	}

	if !plan.HasChanges {
		fmt.Printf("[INFO] Agent SFX hooks are already installed and up-to-date in %s\n", settingsPath)
		return
	}

	if err := installer.WriteAtomicWithBackup(settingsPath, rootMap); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to write settings: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[OK] Successfully installed Agent SFX hooks into %s (backup preserved at %s.bak)\n",
		settingsPath, settingsPath)
}

func runUninstall(args []string) {
	var filtered []string
	var adapter string
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") && adapter == "" {
			adapter = arg
		} else {
			filtered = append(filtered, arg)
		}
	}

	if adapter != "gemini" {
		fmt.Fprintf(os.Stderr, "Usage: agent-sfx uninstall gemini [--scope project|user] [--dry-run]\n")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	scopeStr := fs.String("scope", "project", "Configuration scope ('project' or 'user')")
	dryRun := fs.Bool("dry-run", false, "Print proposed changes without writing")
	binaryPath := fs.String("binary", "", "Path to agent-sfx binary (defaults to current executable)")

	if err := fs.Parse(filtered); err != nil {
		os.Exit(1)
	}

	scope := installer.Scope(*scopeStr)
	settingsPath, err := installer.ResolveSettingsPath(scope, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scope resolution error: %v\n", err)
		os.Exit(1)
	}

	plan, rootMap, err := installer.PlanUninstall(scope, settingsPath, *binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Uninstall plan failed: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		fmt.Printf("--- Dry-Run: Agent SFX Gemini Hook Removal ---\n")
		fmt.Printf("Scope          : %s\n", plan.Scope)
		fmt.Printf("Target File    : %s\n", plan.SettingsPath)
		fmt.Printf("Hooks to Remove: %v\n", plan.RemovedHooks)
		if len(plan.Conflicts) > 0 {
			fmt.Printf("Preserved (Conflicts):\n")
			for _, c := range plan.Conflicts {
				fmt.Printf("  ! %s\n", c)
			}
		}
		fmt.Printf("\nProposed settings.json content:\n%s\n", plan.ProposedJSON)
		fmt.Printf("[DRY-RUN] No files were modified.\n")
		return
	}

	if !plan.HasChanges {
		fmt.Printf("[INFO] No Agent SFX hooks found to remove in %s\n", settingsPath)
		return
	}

	if err := installer.WriteAtomicWithBackup(settingsPath, rootMap); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to update settings: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[OK] Successfully uninstalled Agent SFX hooks from %s (backup preserved at %s.bak)\n",
		settingsPath, settingsPath)
}

func runOn(args []string) {
	fs := flag.NewFlagSet("on", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to custom configuration JSON file")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	targetPath, err := config.SetEnabled(*configPath, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Persisted config : enabled (in %s)\n", targetPath)

	sockPath, err := ipc.ResolveSocketPath()
	if err != nil {
		fmt.Printf("Worker daemon    : unsupported on this platform\n")
		return
	}

	resp, err := worker.SetWorkerEnabled(sockPath, true)
	if err != nil {
		if errors.Is(err, worker.ErrWorkerNotRunning) {
			fmt.Printf("Worker daemon    : not running (setting will take effect on next worker start)\n")
			return
		}
		fmt.Fprintf(os.Stderr, "Worker communication error: %v\n", err)
		os.Exit(1)
	}

	if resp.OK {
		fmt.Printf("Worker daemon    : acknowledged [PID: %d, live audio enabled]\n", resp.PID)
	} else {
		fmt.Fprintf(os.Stderr, "Worker error: %s\n", resp.Error)
		os.Exit(1)
	}
}

func runOff(args []string) {
	fs := flag.NewFlagSet("off", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to custom configuration JSON file")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	targetPath, err := config.SetEnabled(*configPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Persisted config : disabled (in %s)\n", targetPath)

	sockPath, err := ipc.ResolveSocketPath()
	if err != nil {
		fmt.Printf("Worker daemon    : unsupported on this platform\n")
		return
	}

	resp, err := worker.SetWorkerEnabled(sockPath, false)
	if err != nil {
		if errors.Is(err, worker.ErrWorkerNotRunning) {
			fmt.Printf("Worker daemon    : not running (setting will take effect on next worker start)\n")
			return
		}
		fmt.Fprintf(os.Stderr, "Worker communication error: %v\n", err)
		os.Exit(1)
	}

	if resp.OK {
		fmt.Printf("Worker daemon    : acknowledged [PID: %d, playback canceled, live audio muted]\n", resp.PID)
	} else {
		fmt.Fprintf(os.Stderr, "Worker error: %s\n", resp.Error)
		os.Exit(1)
	}
}

type statusReport struct {
	Persisted struct {
		ConfigPath string `json:"config_path"`
		Enabled    bool   `json:"enabled"`
		Valid      bool   `json:"valid"`
		Error      string `json:"error,omitempty"`
	} `json:"persisted"`
	Worker struct {
		Supported bool   `json:"supported"`
		Running   bool   `json:"running"`
		PID       int    `json:"pid,omitempty"`
		UptimeSec int64  `json:"uptime_sec,omitempty"`
		Enabled   bool   `json:"enabled"`
		Playing   bool   `json:"playing"`
		Socket    string `json:"socket,omitempty"`
	} `json:"worker"`
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to custom configuration JSON file")
	jsonOut := fs.Bool("json", false, "Output in JSON format")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	var report statusReport

	cfg, targetPath, err := config.Load(*configPath)
	report.Persisted.ConfigPath = targetPath
	if err != nil {
		report.Persisted.Valid = false
		report.Persisted.Error = err.Error()
	} else {
		report.Persisted.Valid = true
		report.Persisted.Enabled = cfg.Enabled
	}

	sockPath, err := ipc.ResolveSocketPath()
	if err != nil {
		report.Worker.Supported = false
	} else {
		report.Worker.Supported = true
		report.Worker.Socket = sockPath
		resp, err := worker.StatusDaemon(sockPath)
		if err == nil && resp.OK {
			report.Worker.Running = true
			report.Worker.PID = resp.PID
			report.Worker.UptimeSec = resp.Uptime
			report.Worker.Enabled = resp.Enabled
			report.Worker.Playing = resp.Playing
		}
	}

	if *jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "JSON formatting error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Agent SFX Status\n")
	fmt.Printf("----------------\n")
	if !report.Persisted.Valid {
		fmt.Printf("Persisted Config : INVALID (%s) [path: %s]\n", report.Persisted.Error, report.Persisted.ConfigPath)
	} else if report.Persisted.Enabled {
		fmt.Printf("Persisted Config : enabled (in %s)\n", report.Persisted.ConfigPath)
	} else {
		fmt.Printf("Persisted Config : disabled (in %s)\n", report.Persisted.ConfigPath)
	}

	if !report.Worker.Supported {
		fmt.Printf("Worker Daemon    : unsupported on this platform\n")
	} else if report.Worker.Running {
		audioState := "enabled"
		if !report.Worker.Enabled {
			audioState = "muted"
		}
		playingStr := "idle"
		if report.Worker.Playing {
			playingStr = "playing"
		}
		fmt.Printf("Worker Daemon    : running [PID: %d, uptime: %ds, socket: %s]\n",
			report.Worker.PID, report.Worker.UptimeSec, report.Worker.Socket)
		fmt.Printf("Worker State     : live audio %s (%s)\n", audioState, playingStr)
	} else {
		fmt.Printf("Worker Daemon    : not running (socket: %s)\n", report.Worker.Socket)
	}
}

func runSetup(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: agent-sfx setup gemini|claude|deploy [--dry-run]\n")
		os.Exit(1)
	}

	targetAgent := args[0]
	if targetAgent != "gemini" && targetAgent != "claude" && targetAgent != "deploy" {
		fmt.Fprintf(os.Stderr, "Unsupported subcommand %q for setup. Supported: gemini, claude, deploy\n", targetAgent)
		os.Exit(1)
	}

	if targetAgent == "deploy" {
		fs := flag.NewFlagSet("setup deploy", flag.ExitOnError)
		dryRun := fs.Bool("dry-run", false, "Show proposed file deployments without copying files")
		packageDir := fs.String("package-dir", "", "Path to source npm package directory (optional override)")
		targetDir := fs.String("target-dir", "", "Path to destination user app directory (optional override)")
		_ = fs.Parse(args[1:])

		srcDir := *packageDir
		if srcDir == "" {
			if cwd, err := os.Getwd(); err == nil {
				if fi, err := os.Stat(filepath.Join(cwd, "npm", "gemini-extension.json")); err == nil && !fi.IsDir() {
					srcDir = filepath.Join(cwd, "npm")
				}
			}
			if srcDir == "" {
				if exe, err := os.Executable(); err == nil {
					cand := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "npm"))
					if fi, err := os.Stat(filepath.Join(cand, "gemini-extension.json")); err == nil && !fi.IsDir() {
						srcDir = cand
					}
				}
			}
			if srcDir == "" {
				srcDir = "npm"
			}
		}

		report, err := setup.DeployPackages(srcDir, *targetDir, *dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Deployment failed: %v\n", err)
			os.Exit(1)
		}

		if *dryRun {
			fmt.Printf("--- Dry-Run: Agent SFX User-Wide Package Deployment ---\n")
			fmt.Printf("Source Package Dir  : %s\n", srcDir)
			fmt.Printf("Target Base Dir     : %s\n", report.BaseDir)
			fmt.Printf("Gemini Extension Dir: %s\n", report.GeminiExtensionDir)
			fmt.Printf("Claude Plugin Dir   : %s\n", report.ClaudePluginDir)
			fmt.Printf("Planned Files to Deploy (%d files):\n", len(report.DeployedFiles))
			for _, f := range report.DeployedFiles {
				fmt.Printf("  + %s\n", f)
			}
			fmt.Printf("\n[DRY-RUN] No files were deployed.\n")
			return
		}

		fmt.Printf("[OK] Successfully deployed self-contained packages to %s\n", report.BaseDir)
		fmt.Printf("  - Gemini Extension: %s\n", report.GeminiExtensionDir)
		fmt.Printf("  - Claude Plugin   : %s\n", report.ClaudePluginDir)
		fmt.Printf("\nNext Steps for One-Time User-Wide Activation:\n")
		fmt.Printf("  1. Gemini CLI:\n")
		fmt.Printf("       gemini extensions link %q\n", report.GeminiExtensionDir)
		fmt.Printf("  2. Claude Code:\n")
		fmt.Printf("       claude plugin marketplace add %q --scope user\n", report.ClaudePluginDir)
		fmt.Printf("       claude plugin install agent-sfx@agent-sfx-local --scope user\n")
		return
	}

	fs := flag.NewFlagSet("setup "+targetAgent, flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Show proposed integration plan without modifying settings (required)")
	packageDir := fs.String("package-dir", "", "Path to agent-sfx package directory (optional override)")
	projectDir := fs.String("project-dir", "", "Path to project workspace directory (optional override)")
	userDir := fs.String("user-dir", "", "Path to user home directory (optional override)")
	_ = fs.Parse(args[1:])

	if !*dryRun {
		fmt.Fprintf(os.Stderr, "Error: --dry-run is currently required. Live integration modification is pending review.\n")
		fmt.Fprintf(os.Stderr, "Run: agent-sfx setup %s --dry-run\n", targetAgent)
		os.Exit(1)
	}

	workspace := *projectDir
	if workspace == "" {
		if wd, err := os.Getwd(); err == nil {
			workspace = wd
		}
	}

	home := *userDir
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}

	cliRunner := func(name string, cArgs ...string) ([]byte, error) {
		cmd := exec.Command(name, cArgs...)
		return cmd.Output()
	}

	if targetAgent == "claude" {
		plan, err := setup.PlanClaudeSetup(workspace, home, *packageDir, cliRunner)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Claude setup planning failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Print(plan.FormatDryRun())
		if plan.MigrationAbort {
			os.Exit(1)
		}
		return
	}

	plan, err := setup.PlanSetup(workspace, home, *packageDir, cliRunner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Setup planning failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(plan.FormatDryRun())
	if plan.MigrationAbort {
		os.Exit(1)
	}
}
