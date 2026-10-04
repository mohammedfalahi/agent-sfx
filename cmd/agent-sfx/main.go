package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent-sfx/internal/adapters/gemini"
	"agent-sfx/internal/audio"
	"agent-sfx/internal/config"
	"agent-sfx/internal/doctor"
	"agent-sfx/internal/events"
	"agent-sfx/internal/installer"
	"agent-sfx/internal/ipc"
	"agent-sfx/internal/preview"
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

  hook gemini                          Neutral stdin/stdout hook receiver for Gemini CLI (M1/M2)

  install gemini                       Install hooks into .gemini/settings.json (M2)
                                       Flags:
                                         --scope <project|user>  Scope (default: project)
                                         --dry-run               Show exact proposed changes without writing

  uninstall gemini                     Remove owned hooks from .gemini/settings.json (M2)
                                       Flags:
                                         --scope <project|user>  Scope (default: project)
                                         --dry-run               Show exact proposed changes without writing

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
	case "hook":
		runHook(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
	case "uninstall":
		runUninstall(os.Args[2:])
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
	if adapter != "gemini" {
		fmt.Println("{}")
		os.Exit(0)
	}

	socketPath, _ := ipc.ResolveSocketPath()
	exePath, _ := os.Executable()
	resolvedSounds := preview.ResolveSoundsDir("", "")

	receiver := &gemini.Receiver{
		SocketPath: socketPath,
		BinaryPath: exePath,
		SoundsDir:  resolvedSounds,
		TestMode:   false, // Production hook mode
	}

	// ProcessHook always prints "{}\n" to stdout
	_, _ = receiver.ProcessHook(context.Background(), os.Stdin, os.Stdout)
	os.Exit(0)
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
