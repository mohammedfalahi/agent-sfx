package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeOwnedHookNames defines the exact hook names owned by Agent SFX under Claude Code
var ClaudeOwnedHookNames = map[string]string{
	"SessionStart":       "agent-sfx-session-start",
	"UserPromptSubmit":   "agent-sfx-user-prompt-submit",
	"Stop":               "agent-sfx-stop",
	"PermissionRequest":  "agent-sfx-permission-request",
	"PreToolUse":         "agent-sfx-pre-tool-use",
	"PostToolUseFailure": "agent-sfx-post-tool-use-failure",
	"StopFailure":        "agent-sfx-stop-failure",
}

// ClaudeMarketplaceInspection represents inspection of Claude Code marketplaces
type ClaudeMarketplaceInspection struct {
	Registered      bool   `json:"registered"`
	Name            string `json:"name,omitempty"`
	InstallLocation string `json:"install_location,omitempty"`
}

// ClaudePluginInspection represents inspection of Claude Code plugins
type ClaudePluginInspection struct {
	Installed       bool   `json:"installed"`
	Enabled         bool   `json:"enabled"`
	PluginPath      string `json:"plugin_path,omitempty"`
	Version         string `json:"version,omitempty"`
	Scope           string `json:"scope,omitempty"`
	InspectionError string `json:"inspection_error,omitempty"`
}

// ClaudePlan represents the dry-run inspection and setup plan for Claude Code
type ClaudePlan struct {
	ClaudeCliFound     bool                        `json:"claude_cli_found"`
	ClaudeVersion      string                      `json:"claude_version,omitempty"`
	PackageDir         string                      `json:"package_dir"`
	ClaudePluginDir    string                      `json:"claude_plugin_dir"`
	IsEphemeralNpx     bool                        `json:"is_ephemeral_npx"`
	ProjectHooks       HookInspection              `json:"project_hooks"`
	UserHooks          HookInspection              `json:"user_hooks"`
	Marketplace        ClaudeMarketplaceInspection `json:"marketplace"`
	Plugin             ClaudePluginInspection      `json:"plugin"`
	ProposedCommands   []string                    `json:"proposed_commands"`
	ProposedRemovals   map[string][]string         `json:"proposed_removals"`
	MigrationAbort     bool                        `json:"migration_abort"`
	AbortReason        string                      `json:"abort_reason,omitempty"`
	ConflictDirections []string                    `json:"conflict_directions"`
}

// InspectClaudeHooks inspects settings.json for Claude Code hook definitions
func InspectClaudeHooks(scope, settingsPath string) HookInspection {
	insp := HookInspection{
		Scope: scope,
		Path:  settingsPath,
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			insp.Exists = false
			return insp
		}
		insp.Conflicts = append(insp.Conflicts, fmt.Sprintf("unreadable settings file: %v", err))
		insp.HasCustomized = true
		return insp
	}
	insp.Exists = true

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		insp.Conflicts = append(insp.Conflicts, fmt.Sprintf("invalid JSON in %s: %v", settingsPath, err))
		insp.HasCustomized = true
		return insp
	}

	rawHooks, ok := root["hooks"]
	if !ok || rawHooks == nil {
		return insp
	}

	hookGroups, ok := rawHooks.(map[string]any)
	if !ok {
		insp.Conflicts = append(insp.Conflicts, "hooks field is not an object")
		insp.HasCustomized = true
		return insp
	}

	for eventName, rawGroup := range hookGroups {
		groupsSlice, ok := rawGroup.([]any)
		if !ok {
			continue
		}

		for _, item := range groupsSlice {
			groupMap, ok := item.(map[string]any)
			if !ok {
				continue
			}

			innerHooks, ok := groupMap["hooks"].([]any)
			if !ok {
				continue
			}

			for _, h := range innerHooks {
				hMap, ok := h.(map[string]any)
				if !ok {
					continue
				}

				name, _ := hMap["name"].(string)
				cmd, _ := hMap["command"].(string)

				expectedName, isKnownEvent := ClaudeOwnedHookNames[eventName]
				isOwnedPrefix := strings.HasPrefix(name, "agent-sfx-")
				containsSfxCmd := strings.Contains(cmd, "agent-sfx")

				if isOwnedPrefix || containsSfxCmd {
					if isKnownEvent && name == expectedName && containsSfxCmd {
						insp.OwnedHooksFound = append(insp.OwnedHooksFound, fmt.Sprintf("%s:%s", eventName, name))
					} else {
						insp.Conflicts = append(insp.Conflicts,
							fmt.Sprintf("customized hook %s (%s) under %s: %q", name, eventName, settingsPath, cmd))
						insp.HasCustomized = true
					}
				}
			}
		}
	}

	return insp
}

// InspectClaudeMarketplaces inspects known_marketplaces.json for Agent SFX marketplace registration
func InspectClaudeMarketplaces(userHomeDir, targetPluginDir string, runner CommandRunner) ClaudeMarketplaceInspection {
	insp := ClaudeMarketplaceInspection{
		Name: "agent-sfx-local",
	}

	mktPath := filepath.Join(userHomeDir, ".claude", "plugins", "known_marketplaces.json")
	if data, err := os.ReadFile(mktPath); err == nil {
		var mkts map[string]struct {
			Source struct {
				Source string `json:"source"`
				Path   string `json:"path"`
			} `json:"source"`
			InstallLocation string `json:"installLocation"`
		}
		if err := json.Unmarshal(data, &mkts); err == nil {
			for name, entry := range mkts {
				if name == "agent-sfx-local" {
					insp.Registered = true
					insp.Name = name
					insp.InstallLocation = entry.InstallLocation
					return insp
				}
				if targetPluginDir != "" {
					cleanTarget := filepath.Clean(targetPluginDir)
					if filepath.Clean(entry.InstallLocation) == cleanTarget || filepath.Clean(entry.Source.Path) == cleanTarget {
						insp.Registered = true
						insp.Name = name
						insp.InstallLocation = entry.InstallLocation
						return insp
					}
				}
			}
		}
	}

	if runner != nil {
		if out, err := runner("claude", "plugin", "marketplace", "list", "--json"); err == nil {
			var mktList []struct {
				Name            string `json:"name"`
				InstallLocation string `json:"installLocation"`
			}
			if err := json.Unmarshal(out, &mktList); err == nil {
				for _, m := range mktList {
					if m.Name == "agent-sfx-local" {
						insp.Registered = true
						insp.Name = m.Name
						insp.InstallLocation = m.InstallLocation
						return insp
					}
				}
			}
		}
	}

	return insp
}

// InspectClaudePlugin checks whether the Agent SFX plugin is installed and enabled in Claude Code
func InspectClaudePlugin(userHomeDir, workspaceDir string, runner CommandRunner) ClaudePluginInspection {
	insp := ClaudePluginInspection{}

	// 1. Inspect installed_plugins.json
	installedPath := filepath.Join(userHomeDir, ".claude", "plugins", "installed_plugins.json")
	if data, err := os.ReadFile(installedPath); err == nil {
		var manifest struct {
			Plugins map[string][]struct {
				Scope       string `json:"scope"`
				InstallPath string `json:"installPath"`
				Version     string `json:"version"`
			} `json:"plugins"`
		}
		if err := json.Unmarshal(data, &manifest); err == nil {
			for pluginID, entries := range manifest.Plugins {
				baseName := strings.Split(pluginID, "@")[0]
				if baseName == "agent-sfx" && len(entries) > 0 {
					insp.Installed = true
					insp.PluginPath = entries[0].InstallPath
					insp.Version = entries[0].Version
					insp.Scope = entries[0].Scope
					insp.Enabled = true // default enabled unless overridden in settings.json
					break
				}
			}
		}
	}

	// 2. Inspect user settings for enabledPlugins override
	userSettingsPath := filepath.Join(userHomeDir, ".claude", "settings.json")
	if data, err := os.ReadFile(userSettingsPath); err == nil {
		var uSettings struct {
			EnabledPlugins map[string]bool `json:"enabledPlugins"`
		}
		if err := json.Unmarshal(data, &uSettings); err == nil && uSettings.EnabledPlugins != nil {
			for pluginID, enabled := range uSettings.EnabledPlugins {
				baseName := strings.Split(pluginID, "@")[0]
				if baseName == "agent-sfx" {
					insp.Installed = true
					insp.Enabled = enabled
					break
				}
			}
		}
	}

	// 3. Inspect project settings for enabledPlugins override
	if workspaceDir != "" {
		projSettingsPath := filepath.Join(workspaceDir, ".claude", "settings.json")
		if data, err := os.ReadFile(projSettingsPath); err == nil {
			var pSettings struct {
				EnabledPlugins map[string]bool `json:"enabledPlugins"`
			}
			if err := json.Unmarshal(data, &pSettings); err == nil && pSettings.EnabledPlugins != nil {
				for pluginID, enabled := range pSettings.EnabledPlugins {
					baseName := strings.Split(pluginID, "@")[0]
					if baseName == "agent-sfx" {
						insp.Installed = true
						insp.Enabled = enabled
						insp.Scope = "project"
						break
					}
				}
			}
		}
	}

	// 4. Fallback/Corroboration: Query CLI via claude plugin list
	if runner != nil {
		out, err := runner("claude", "plugin", "list")
		if err == nil {
			outStr := string(out)
			if strings.Contains(outStr, "agent-sfx") {
				insp.Installed = true
				if strings.Contains(outStr, "enabled") {
					insp.Enabled = true
				} else if strings.Contains(outStr, "disabled") {
					insp.Enabled = false
				}
			}
		}
	}

	return insp
}

// PlanClaudeSetup generates a dry-run migration and setup plan for Claude Code
func PlanClaudeSetup(workspaceDir, userHomeDir, customPackageDir string, runner CommandRunner) (*ClaudePlan, error) {
	plan := &ClaudePlan{
		ProposedRemovals: make(map[string][]string),
	}

	// 1. Detect package directory and claude plugin directory
	var pkgDir, claudePluginDir string

	if customPackageDir != "" {
		pkgDir = customPackageDir
		if _, err := os.Stat(filepath.Join(pkgDir, ".claude-plugin", "plugin.json")); err == nil {
			claudePluginDir = pkgDir
		} else {
			claudePluginDir = filepath.Join(pkgDir, "claude")
		}
	} else {
		// Auto-discover package directory
		candidates := []string{}
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			candidates = append(candidates,
				filepath.Clean(filepath.Join(exeDir, "..", "..")),  // npm installed layout: bin/darwin-arm64/ -> pkg
				filepath.Clean(filepath.Join(exeDir, "..", "npm")), // repo root build layout: bin/ -> npm
				filepath.Clean(filepath.Join(exeDir, "..")),        // repo root layout
			)
		}
		if cwd, err := os.Getwd(); err == nil {
			candidates = append(candidates,
				filepath.Join(cwd, "npm"),
				cwd,
			)
		}

		found := false
		for _, cand := range candidates {
			if _, err := os.Stat(filepath.Join(cand, "claude", ".claude-plugin", "plugin.json")); err == nil {
				pkgDir = cand
				claudePluginDir = filepath.Join(cand, "claude")
				found = true
				break
			}
			if _, err := os.Stat(filepath.Join(cand, ".claude-plugin", "plugin.json")); err == nil {
				pkgDir = filepath.Dir(cand)
				claudePluginDir = cand
				found = true
				break
			}
		}

		if !found {
			if exe, err := os.Executable(); err == nil {
				pkgDir = filepath.Clean(filepath.Join(filepath.Dir(exe), "..", ".."))
			} else {
				pkgDir = "."
			}
			claudePluginDir = filepath.Join(pkgDir, "claude")
		}
	}

	plan.PackageDir = pkgDir
	if abs, err := filepath.Abs(claudePluginDir); err == nil {
		claudePluginDir = abs
	}
	plan.ClaudePluginDir = claudePluginDir

	// Verify plugin manifest exists in target package directory
	manifestPath := filepath.Join(claudePluginDir, ".claude-plugin", "plugin.json")
	if _, err := os.Stat(manifestPath); err != nil {
		plan.MigrationAbort = true
		plan.AbortReason = fmt.Sprintf("claude plugin manifest not found at %s", manifestPath)
		return plan, nil
	}

	// Verify marketplace manifest exists in target package directory
	marketplacePath := filepath.Join(claudePluginDir, ".claude-plugin", "marketplace.json")
	if _, err := os.Stat(marketplacePath); err != nil {
		plan.MigrationAbort = true
		plan.AbortReason = fmt.Sprintf("claude marketplace manifest not found at %s", marketplacePath)
		return plan, nil
	}

	// Verify self-contained launcher exists
	launcherPath := filepath.Join(claudePluginDir, "bin", "run.js")
	if _, err := os.Stat(launcherPath); err != nil {
		plan.MigrationAbort = true
		plan.AbortReason = fmt.Sprintf("claude plugin self-contained launcher not found at %s", launcherPath)
		return plan, nil
	}

	// Check if package is in an ephemeral cache
	cleanPkg := filepath.Clean(pkgDir)
	if strings.Contains(cleanPkg, "_npx") || strings.Contains(cleanPkg, "npm-cache") || strings.HasPrefix(cleanPkg, os.TempDir()) {
		plan.IsEphemeralNpx = true
	}

	// 2. Check Claude Code CLI binary
	if runner != nil {
		verOut, err := runner("claude", "--version")
		if err == nil {
			plan.ClaudeCliFound = true
			plan.ClaudeVersion = strings.TrimSpace(string(verOut))
		}
	}

	// 3. Inspect Project and User Manual Hooks
	projectSettingsPath := filepath.Join(workspaceDir, ".claude", "settings.json")
	userSettingsPath := filepath.Join(userHomeDir, ".claude", "settings.json")

	plan.ProjectHooks = InspectClaudeHooks("project", projectSettingsPath)
	plan.UserHooks = InspectClaudeHooks("user", userSettingsPath)

	// 4. Inspect Marketplace and Plugin State
	plan.Marketplace = InspectClaudeMarketplaces(userHomeDir, plan.ClaudePluginDir, runner)
	plan.Plugin = InspectClaudePlugin(userHomeDir, workspaceDir, runner)

	// 5. Evaluate Conflicts & Duplicates
	if len(plan.ProjectHooks.OwnedHooksFound) > 0 || len(plan.UserHooks.OwnedHooksFound) > 0 {
		plan.ConflictDirections = append(plan.ConflictDirections, "manual_hooks_to_plugin")
		if len(plan.ProjectHooks.OwnedHooksFound) > 0 {
			plan.ProposedRemovals[plan.ProjectHooks.Path] = plan.ProjectHooks.OwnedHooksFound
		}
		if len(plan.UserHooks.OwnedHooksFound) > 0 {
			plan.ProposedRemovals[plan.UserHooks.Path] = plan.UserHooks.OwnedHooksFound
		}
	}

	if plan.Plugin.Installed {
		plan.ConflictDirections = append(plan.ConflictDirections, "plugin_installed")
	}

	// 6. Check for unresolved conflicts that require aborting
	if plan.ProjectHooks.HasCustomized {
		plan.MigrationAbort = true
		plan.AbortReason = fmt.Sprintf("unresolved customized hook conflicts in project settings: %s",
			strings.Join(plan.ProjectHooks.Conflicts, "; "))
		return plan, nil
	}
	if plan.UserHooks.HasCustomized {
		plan.MigrationAbort = true
		plan.AbortReason = fmt.Sprintf("unresolved customized hook conflicts in user settings: %s",
			strings.Join(plan.UserHooks.Conflicts, "; "))
		return plan, nil
	}

	// 7. Formulate Proposed Safe Commands using explicit marketplace layout
	if !plan.Marketplace.Registered {
		plan.ProposedCommands = append(plan.ProposedCommands,
			fmt.Sprintf("claude plugin marketplace add %q", plan.ClaudePluginDir))
	}

	mktName := plan.Marketplace.Name
	if mktName == "" {
		mktName = "agent-sfx-local"
	}

	if !plan.Plugin.Installed {
		plan.ProposedCommands = append(plan.ProposedCommands,
			fmt.Sprintf("claude plugin install agent-sfx@%s", mktName))
	} else if !plan.Plugin.Enabled {
		plan.ProposedCommands = append(plan.ProposedCommands,
			"claude plugin enable agent-sfx")
	} else if plan.Marketplace.Registered {
		plan.ProposedCommands = append(plan.ProposedCommands,
			"# Plugin is already installed and enabled; no changes required")
	}

	return plan, nil
}

// FormatDryRun formats the Claude setup plan into a clean, human-readable terminal report
func (p *ClaudePlan) FormatDryRun() string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- Agent SFX Claude Code Integration Dry-Run ---\n")

	if p.ClaudeCliFound {
		fmt.Fprintf(&b, "Claude CLI       : detected (version: %s)\n", p.ClaudeVersion)
	} else {
		fmt.Fprintf(&b, "Claude CLI       : not found in PATH (verification via direct file inspection)\n")
	}

	fmt.Fprintf(&b, "Package Location : %s\n", p.PackageDir)
	fmt.Fprintf(&b, "Plugin Location  : %s\n", p.ClaudePluginDir)
	if p.IsEphemeralNpx {
		fmt.Fprintf(&b, "  ! NOTICE: Invoked from an ephemeral path (e.g. npx cache).\n")
		fmt.Fprintf(&b, "    For permanent user-wide integration, deploy self-contained packages via:\n")
		fmt.Fprintf(&b, "    agent-sfx setup deploy\n")
	}

	fmt.Fprintf(&b, "\nInspected Integrations:\n")
	// Project hooks
	fmt.Fprintf(&b, "  - Project Settings (%s):\n", p.ProjectHooks.Path)
	if !p.ProjectHooks.Exists {
		fmt.Fprintf(&b, "      Status: file does not exist (clean)\n")
	} else if len(p.ProjectHooks.OwnedHooksFound) > 0 {
		fmt.Fprintf(&b, "      Status: %d manual hook(s) found (%s)\n",
			len(p.ProjectHooks.OwnedHooksFound), strings.Join(p.ProjectHooks.OwnedHooksFound, ", "))
	} else {
		fmt.Fprintf(&b, "      Status: 0 Agent SFX hooks found (clean)\n")
	}

	// User hooks
	fmt.Fprintf(&b, "  - User Settings (%s):\n", p.UserHooks.Path)
	if !p.UserHooks.Exists {
		fmt.Fprintf(&b, "      Status: file does not exist (clean)\n")
	} else if len(p.UserHooks.OwnedHooksFound) > 0 {
		fmt.Fprintf(&b, "      Status: %d manual hook(s) found (%s)\n",
			len(p.UserHooks.OwnedHooksFound), strings.Join(p.UserHooks.OwnedHooksFound, ", "))
	} else {
		fmt.Fprintf(&b, "      Status: 0 Agent SFX hooks found (clean)\n")
	}

	// Marketplace state
	fmt.Fprintf(&b, "  - Claude Marketplace (agent-sfx-local):\n")
	if p.Marketplace.Registered {
		fmt.Fprintf(&b, "      Status: registered (%s)\n", p.Marketplace.InstallLocation)
	} else {
		fmt.Fprintf(&b, "      Status: not registered\n")
	}

	// Plugin state
	fmt.Fprintf(&b, "  - Claude Plugin (agent-sfx):\n")
	if p.Plugin.Installed {
		status := "installed"
		if p.Plugin.Enabled {
			status = "installed & enabled"
		} else {
			status = "installed (disabled)"
		}
		if p.Plugin.Scope != "" {
			status += fmt.Sprintf(" [scope: %s]", p.Plugin.Scope)
		}
		if p.Plugin.PluginPath != "" {
			status += fmt.Sprintf(" at %s", p.Plugin.PluginPath)
		}
		fmt.Fprintf(&b, "      Status: %s\n", status)
	} else {
		fmt.Fprintf(&b, "      Status: not installed\n")
	}

	// Conflicts & Migration
	fmt.Fprintf(&b, "\nMigration & Duplicate Assessment:\n")
	hasManualAndPlugin := (len(p.ProjectHooks.OwnedHooksFound) > 0 || len(p.UserHooks.OwnedHooksFound) > 0) && p.Plugin.Installed
	if hasManualAndPlugin {
		fmt.Fprintf(&b, "  ! Detected manual hooks in settings.json while plugin is installed.\n")
		fmt.Fprintf(&b, "    Activating both simultaneously would cause duplicate sound triggers.\n")
	} else if len(p.ProjectHooks.OwnedHooksFound) > 0 || len(p.UserHooks.OwnedHooksFound) > 0 {
		fmt.Fprintf(&b, "  ! Detected manual hooks in settings.json; preparing migration to plugin.\n")
	} else {
		fmt.Fprintf(&b, "  ✓ No duplicate integrations or conflicting configurations detected.\n")
	}

	if p.MigrationAbort {
		fmt.Fprintf(&b, "\n[ABORTED] Migration cannot proceed automatically:\n")
		fmt.Fprintf(&b, "  Error: %s\n", p.AbortReason)
		fmt.Fprintf(&b, "  Instructions: Please manually inspect your Claude settings.json and resolve conflicting hook definitions.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\nProposed Changes (Dry-Run):\n")
	if len(p.ProposedRemovals) > 0 {
		for target, hooks := range p.ProposedRemovals {
			fmt.Fprintf(&b, "  1. Remove %d manual owned hook(s) from %s (preserves backup at %s.bak):\n",
				len(hooks), target, target)
			for _, h := range hooks {
				fmt.Fprintf(&b, "       - %s\n", h)
			}
		}
	}

	if len(p.ProposedCommands) > 0 {
		stepNum := 1
		if len(p.ProposedRemovals) > 0 {
			stepNum = 2
		}
		fmt.Fprintf(&b, "  %d. Proposed plugin command(s):\n", stepNum)
		for _, cmd := range p.ProposedCommands {
			fmt.Fprintf(&b, "       %s\n", cmd)
		}
	}

	fmt.Fprintf(&b, "\n[DRY-RUN] No settings were modified and no plugins were installed.\n")
	return b.String()
}
