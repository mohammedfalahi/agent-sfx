package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OwnedHookNames defines the exact hook names owned by Agent SFX
var OwnedHookNames = map[string]string{
	"SessionStart": "agent-sfx-session-start",
	"BeforeAgent":  "agent-sfx-before-agent",
	"AfterAgent":   "agent-sfx-after-agent",
	"Notification": "agent-sfx-notification",
	"BeforeTool":   "agent-sfx-before-tool-ask-user",
	"AfterTool":    "agent-sfx-after-tool",
}

// CommandRunner abstracts execution of CLI commands for testing
type CommandRunner func(name string, args ...string) ([]byte, error)

// HookInspection represents inspection of manual hooks in a settings.json file
type HookInspection struct {
	Scope           string   `json:"scope"`
	Path            string   `json:"path"`
	Exists          bool     `json:"exists"`
	OwnedHooksFound []string `json:"owned_hooks_found"`
	Conflicts       []string `json:"conflicts"`
	HasCustomized   bool     `json:"has_customized"`
}

// ExtensionInspection represents inspection of Gemini extensions
type ExtensionInspection struct {
	Installed       bool   `json:"installed"`
	Linked          bool   `json:"linked"`
	ExtensionPath   string `json:"extension_path"`
	Source          string `json:"source,omitempty"`
	Version         string `json:"version,omitempty"`
	InspectionError string `json:"inspection_error,omitempty"`
}

// Plan represents the complete dry-run inspection and migration plan
type Plan struct {
	GeminiCliFound     bool                `json:"gemini_cli_found"`
	GeminiVersion      string              `json:"gemini_version,omitempty"`
	PackageDir         string              `json:"package_dir"`
	IsEphemeralNpx     bool                `json:"is_ephemeral_npx"`
	ProjectHooks       HookInspection      `json:"project_hooks"`
	UserHooks          HookInspection      `json:"user_hooks"`
	Extension          ExtensionInspection `json:"extension"`
	ProposedCommands   []string            `json:"proposed_commands"`
	ProposedRemovals   map[string][]string `json:"proposed_removals"`
	MigrationAbort     bool                `json:"migration_abort"`
	AbortReason        string              `json:"abort_reason,omitempty"`
	ConflictDirections []string            `json:"conflict_directions"`
}

// InspectProject inspects settings.json in the specified workspace or user directory
func InspectHooks(scope, settingsPath string) HookInspection {
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

				expectedName, isKnownEvent := OwnedHookNames[eventName]
				isOwnedPrefix := strings.HasPrefix(name, "agent-sfx-")
				containsSfxCmd := strings.Contains(cmd, "agent-sfx")

				if isOwnedPrefix || containsSfxCmd {
					if isKnownEvent && name == expectedName && containsSfxCmd {
						// Exact expected owned hook
						insp.OwnedHooksFound = append(insp.OwnedHooksFound, fmt.Sprintf("%s:%s", eventName, name))
					} else {
						// Ambiguous / customized hook under agent-sfx naming
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

// InspectExtension checks whether the Agent SFX extension is installed or linked in Gemini CLI
func InspectExtension(userHomeDir string, runner CommandRunner) ExtensionInspection {
	ext := ExtensionInspection{}
	extDir := filepath.Join(userHomeDir, ".gemini", "extensions", "agent-sfx")
	ext.ExtensionPath = extDir

	if fi, err := os.Lstat(extDir); err == nil {
		ext.Installed = true
		if fi.Mode()&os.ModeSymlink != 0 {
			ext.Linked = true
			if target, err := os.Readlink(extDir); err == nil {
				ext.Source = target
			}
		}
		// Read manifest version if present
		manifestPath := filepath.Join(extDir, "gemini-extension.json")
		if mData, err := os.ReadFile(manifestPath); err == nil {
			var m struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(mData, &m); err == nil {
				ext.Version = m.Version
			}
		}
		return ext
	}

	// Also query CLI via verified flag --output-format json
	if runner != nil {
		out, err := runner("gemini", "extensions", "list", "--output-format", "json")
		if err == nil {
			var list []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Type    string `json:"type"`
				Source  string `json:"source"`
			}
			if err := json.Unmarshal(out, &list); err == nil {
				for _, item := range list {
					if item.Name == "agent-sfx" {
						ext.Installed = true
						ext.Version = item.Version
						ext.Source = item.Source
						if item.Type == "link" {
							ext.Linked = true
						}
						break
					}
				}
			}
		}
	}

	return ext
}

// PlanSetup generates a dry-run migration and setup plan for Gemini CLI
func PlanSetup(workspaceDir, userHomeDir, customPackageDir string, runner CommandRunner) (*Plan, error) {
	plan := &Plan{
		ProposedRemovals: make(map[string][]string),
	}

	// 1. Detect package directory and check for ephemeral npx cache
	pkgDir := customPackageDir
	if pkgDir == "" {
		if exe, err := os.Executable(); err == nil {
			// Walk up from bin/darwin-arm64/agent-sfx to package root
			pkgDir = filepath.Clean(filepath.Join(filepath.Dir(exe), "..", ".."))
		} else {
			pkgDir = "."
		}
	}
	plan.PackageDir = pkgDir

	// Check if package is in an ephemeral cache
	cleanPkg := filepath.Clean(pkgDir)
	if strings.Contains(cleanPkg, "_npx") || strings.Contains(cleanPkg, "npm-cache") || strings.HasPrefix(cleanPkg, os.TempDir()) {
		plan.IsEphemeralNpx = true
	}

	// 2. Check Gemini CLI binary
	if runner != nil {
		verOut, err := runner("gemini", "--version")
		if err == nil {
			plan.GeminiCliFound = true
			plan.GeminiVersion = strings.TrimSpace(string(verOut))
		}
	}

	// 3. Inspect Project and User Manual Hooks
	projectSettingsPath := filepath.Join(workspaceDir, ".gemini", "settings.json")
	userSettingsPath := filepath.Join(userHomeDir, ".gemini", "settings.json")

	plan.ProjectHooks = InspectHooks("project", projectSettingsPath)
	plan.UserHooks = InspectHooks("user", userSettingsPath)

	// 4. Inspect Extension State
	plan.Extension = InspectExtension(userHomeDir, runner)

	// 5. Evaluate Two-Way Conflicts
	// Direction A: Manual Hooks Present -> Extension setup
	if len(plan.ProjectHooks.OwnedHooksFound) > 0 || len(plan.UserHooks.OwnedHooksFound) > 0 {
		plan.ConflictDirections = append(plan.ConflictDirections, "manual_hooks_to_extension")
		if len(plan.ProjectHooks.OwnedHooksFound) > 0 {
			plan.ProposedRemovals[plan.ProjectHooks.Path] = plan.ProjectHooks.OwnedHooksFound
		}
		if len(plan.UserHooks.OwnedHooksFound) > 0 {
			plan.ProposedRemovals[plan.UserHooks.Path] = plan.UserHooks.OwnedHooksFound
		}
	}

	// Direction B: Extension Present -> Manual Hook setup
	if plan.Extension.Installed {
		plan.ConflictDirections = append(plan.ConflictDirections, "extension_to_manual_hooks")
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

	// 7. Formulate Proposed Safe Commands
	if !plan.Extension.Installed {
		// Proposed extension linking command using safe array
		plan.ProposedCommands = append(plan.ProposedCommands,
			fmt.Sprintf("gemini extensions link %q", plan.PackageDir))
	} else if plan.Extension.Linked {
		plan.ProposedCommands = append(plan.ProposedCommands,
			"# Extension is already linked; no extension changes required")
	}

	return plan, nil
}

// FormatDryRun formats the plan into a clean, human-readable terminal report
func (p *Plan) FormatDryRun() string {
	var b strings.Builder
	fmt.Fprintf(&b, "--- Agent SFX Gemini CLI Integration Dry-Run ---\n")

	if p.GeminiCliFound {
		fmt.Fprintf(&b, "Gemini CLI       : detected (version: %s)\n", p.GeminiVersion)
	} else {
		fmt.Fprintf(&b, "Gemini CLI       : not found in PATH (verification via direct file inspection)\n")
	}

	fmt.Fprintf(&b, "Package Location : %s\n", p.PackageDir)
	if p.IsEphemeralNpx {
		fmt.Fprintf(&b, "  ! NOTICE: Invoked from an ephemeral path (e.g. npx cache).\n")
		fmt.Fprintf(&b, "    For permanent integration, install globally via:\n")
		fmt.Fprintf(&b, "    npm install -g @agent-sfx/agent-sfx\n")
	}

	fmt.Fprintf(&b, "\nInspected Integrations:\n")
	// Project hooks
	fmt.Fprintf(&b, "  - Project Hooks (%s):\n", p.ProjectHooks.Path)
	if !p.ProjectHooks.Exists {
		fmt.Fprintf(&b, "      Status: file does not exist (clean)\n")
	} else if len(p.ProjectHooks.OwnedHooksFound) > 0 {
		fmt.Fprintf(&b, "      Status: %d owned hook(s) found (%s)\n",
			len(p.ProjectHooks.OwnedHooksFound), strings.Join(p.ProjectHooks.OwnedHooksFound, ", "))
	} else {
		fmt.Fprintf(&b, "      Status: 0 Agent SFX hooks found (clean)\n")
	}

	// User hooks
	fmt.Fprintf(&b, "  - User Hooks (%s):\n", p.UserHooks.Path)
	if !p.UserHooks.Exists {
		fmt.Fprintf(&b, "      Status: file does not exist (clean)\n")
	} else if len(p.UserHooks.OwnedHooksFound) > 0 {
		fmt.Fprintf(&b, "      Status: %d owned hook(s) found (%s)\n",
			len(p.UserHooks.OwnedHooksFound), strings.Join(p.UserHooks.OwnedHooksFound, ", "))
	} else {
		fmt.Fprintf(&b, "      Status: 0 Agent SFX hooks found (clean)\n")
	}

	// Extension
	fmt.Fprintf(&b, "  - Gemini Extension (%s):\n", p.Extension.ExtensionPath)
	if p.Extension.Installed {
		linkStatus := "installed"
		if p.Extension.Linked {
			linkStatus = fmt.Sprintf("linked -> %s", p.Extension.Source)
		}
		fmt.Fprintf(&b, "      Status: active (%s)\n", linkStatus)
	} else {
		fmt.Fprintf(&b, "      Status: not installed\n")
	}

	// Conflicts & Migration
	fmt.Fprintf(&b, "\nMigration & Duplicate Assessment:\n")
	if len(p.ConflictDirections) > 0 {
		for _, dir := range p.ConflictDirections {
			if dir == "manual_hooks_to_extension" {
				fmt.Fprintf(&b, "  ! Detected manual hooks in settings.json while preparing extension integration.\n")
				fmt.Fprintf(&b, "    Activating both simultaneously would cause duplicate sound triggers.\n")
			}
			if dir == "extension_to_manual_hooks" {
				fmt.Fprintf(&b, "  ! Detected active extension while manual hooks exist or are planned.\n")
			}
		}
	} else {
		fmt.Fprintf(&b, "  ✓ No duplicate integrations or conflicting configurations detected.\n")
	}

	if p.MigrationAbort {
		fmt.Fprintf(&b, "\n[ABORTED] Migration cannot proceed automatically:\n")
		fmt.Fprintf(&b, "  Error: %s\n", p.AbortReason)
		fmt.Fprintf(&b, "  Instructions: Please manually inspect your settings.json and resolve conflicting hook definitions.\n")
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
		fmt.Fprintf(&b, "  2. Execute extension integration command:\n")
		for _, cmd := range p.ProposedCommands {
			fmt.Fprintf(&b, "       %s\n", cmd)
		}
	}

	fmt.Fprintf(&b, "\n[DRY-RUN] No settings were modified and no extensions were linked.\n")
	return b.String()
}
