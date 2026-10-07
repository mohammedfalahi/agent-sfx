package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var (
	ErrWindowsUnsupported  = errors.New("hook installation is not supported on Windows while worker is a stub")
	ErrInvalidExistingJSON = errors.New("existing settings.json contains invalid JSON")
)

// Scope defines the target configuration file level.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

// OwnedHook defines a managed Agent SFX hook definition.
type OwnedHook struct {
	EventName   string
	Matcher     string
	HookName    string
	Description string
}

// AllOwnedHooks lists the hooks managed by Agent SFX.
// SessionStart uses wildcard matcher "" to cover startup, resume, and clear sources.
var AllOwnedHooks = []OwnedHook{
	{
		EventName:   "SessionStart",
		Matcher:     "",
		HookName:    "agent-sfx-session-start",
		Description: "Agent SFX: silent worker launch on startup, resume, or clear",
	},
	{
		EventName:   "BeforeAgent",
		Matcher:     "",
		HookName:    "agent-sfx-before-agent",
		Description: "Agent SFX: task_started sound event",
	},
	{
		EventName:   "AfterAgent",
		Matcher:     "",
		HookName:    "agent-sfx-after-agent",
		Description: "Agent SFX: task_finished sound event",
	},
	{
		EventName:   "Notification",
		Matcher:     "",
		HookName:    "agent-sfx-notification",
		Description: "Agent SFX: permission_requested sound event",
	},
	{
		EventName:   "BeforeTool",
		Matcher:     "^ask_user$",
		HookName:    "agent-sfx-before-tool-ask-user",
		Description: "Agent SFX: waiting_for_user sound event",
	},
	{
		EventName:   "AfterTool",
		Matcher:     "",
		HookName:    "agent-sfx-after-tool",
		Description: "Agent SFX: tool error and test-pass sound events",
	},
}

// DryRunResult contains the analysis and proposed JSON for an install or uninstall operation.
type DryRunResult struct {
	Scope          Scope    `json:"scope"`
	SettingsPath   string   `json:"settings_path"`
	ProposedJSON   string   `json:"proposed_json"`
	AddedHooks     []string `json:"added_hooks,omitempty"`
	UpdatedHooks   []string `json:"updated_hooks,omitempty"`
	RemovedHooks   []string `json:"removed_hooks,omitempty"`
	Conflicts      []string `json:"conflicts,omitempty"`
	UnchangedHooks []string `json:"unchanged_hooks,omitempty"`
	HasChanges     bool     `json:"has_changes"`
}

// ResolveSettingsPath determines the file path based on scope.
func ResolveSettingsPath(scope Scope, projectDir string) (string, error) {
	switch scope {
	case ScopeProject:
		if projectDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("failed to get current directory: %w", err)
			}
			projectDir = cwd
		}
		return filepath.Join(projectDir, ".gemini", "settings.json"), nil

	case ScopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		return filepath.Join(home, ".gemini", "settings.json"), nil

	default:
		return "", fmt.Errorf("unknown scope %q: must be 'project' or 'user'", scope)
	}
}

// BuildExpectedCommand returns the platform-escaped hook command for the given binary path.
func BuildExpectedCommand(binaryPath string) (string, error) {
	if binaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("unable to determine executable path: %w", err)
		}
		binaryPath = exe
	}

	absBin, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", fmt.Errorf("unable to resolve absolute binary path: %w", err)
	}

	return fmt.Sprintf("%s hook gemini", EscapeShellArg(absBin)), nil
}

// PlanInstall computes the proposed settings.json after idempotent merging of owned hooks.
func PlanInstall(scope Scope, settingsPath, binaryPath string) (*DryRunResult, map[string]interface{}, error) {
	if runtime.GOOS == "windows" {
		return nil, nil, ErrWindowsUnsupported
	}

	expectedCmd, err := BuildExpectedCommand(binaryPath)
	if err != nil {
		return nil, nil, err
	}

	rootMap := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &rootMap); err != nil {
			return nil, nil, fmt.Errorf("%w at %s: %v", ErrInvalidExistingJSON, settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	hooksMap, ok := rootMap["hooks"].(map[string]interface{})
	if !ok {
		hooksMap = make(map[string]interface{})
		rootMap["hooks"] = hooksMap
	}

	res := &DryRunResult{
		Scope:        scope,
		SettingsPath: settingsPath,
	}

	for _, def := range AllOwnedHooks {
		existingList, _ := hooksMap[def.EventName].([]interface{})
		var found bool

		for _, matcherEntry := range existingList {
			mMap, ok := matcherEntry.(map[string]interface{})
			if !ok {
				continue
			}
			hList, ok := mMap["hooks"].([]interface{})
			if !ok {
				continue
			}

			for _, hEntry := range hList {
				hMap, ok := hEntry.(map[string]interface{})
				if !ok {
					continue
				}
				if hMap["name"] == def.HookName {
					found = true
					existingCmd, _ := hMap["command"].(string)
					if existingCmd == expectedCmd {
						res.UnchangedHooks = append(res.UnchangedHooks, def.HookName)
					} else {
						// Detected conflict: command modified or pointing elsewhere
						res.Conflicts = append(res.Conflicts,
							fmt.Sprintf("hook %s has custom command %q (expected %q)", def.HookName, existingCmd, expectedCmd))
						hMap["command"] = expectedCmd
						res.UpdatedHooks = append(res.UpdatedHooks, def.HookName)
						res.HasChanges = true
					}
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			// Append new hook entry
			newHook := map[string]interface{}{
				"name":        def.HookName,
				"type":        "command",
				"command":     expectedCmd,
				"description": def.Description,
			}
			newMatcher := map[string]interface{}{
				"matcher": def.Matcher,
				"hooks":   []interface{}{newHook},
			}
			hooksMap[def.EventName] = append(existingList, newMatcher)
			res.AddedHooks = append(res.AddedHooks, def.HookName)
			res.HasChanges = true
		}
	}

	formatted, err := json.MarshalIndent(rootMap, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format json: %w", err)
	}
	res.ProposedJSON = string(formatted)

	return res, rootMap, nil
}

// PlanUninstall computes the proposed settings.json after removing only exact owned hooks.
func PlanUninstall(scope Scope, settingsPath, binaryPath string) (*DryRunResult, map[string]interface{}, error) {
	if runtime.GOOS == "windows" {
		return nil, nil, ErrWindowsUnsupported
	}

	expectedCmd, err := BuildExpectedCommand(binaryPath)
	if err != nil {
		return nil, nil, err
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &DryRunResult{
				Scope:        scope,
				SettingsPath: settingsPath,
				HasChanges:   false,
			}, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	rootMap := make(map[string]interface{})
	if err := json.Unmarshal(data, &rootMap); err != nil {
		return nil, nil, fmt.Errorf("%w at %s: %v", ErrInvalidExistingJSON, settingsPath, err)
	}

	hooksMap, ok := rootMap["hooks"].(map[string]interface{})
	if !ok {
		return &DryRunResult{
			Scope:        scope,
			SettingsPath: settingsPath,
			HasChanges:   false,
		}, rootMap, nil
	}

	res := &DryRunResult{
		Scope:        scope,
		SettingsPath: settingsPath,
	}

	// Exact owned hook names
	ownedMap := make(map[string]bool)
	for _, o := range AllOwnedHooks {
		ownedMap[o.HookName] = true
	}

	for eventName, matcherListRaw := range hooksMap {
		matcherList, ok := matcherListRaw.([]interface{})
		if !ok {
			continue
		}

		var newMatcherList []interface{}
		for _, matcherEntry := range matcherList {
			mMap, ok := matcherEntry.(map[string]interface{})
			if !ok {
				newMatcherList = append(newMatcherList, matcherEntry)
				continue
			}

			hList, ok := mMap["hooks"].([]interface{})
			if !ok {
				newMatcherList = append(newMatcherList, matcherEntry)
				continue
			}

			var newHList []interface{}
			for _, hEntry := range hList {
				hMap, ok := hEntry.(map[string]interface{})
				if !ok {
					newHList = append(newHList, hEntry)
					continue
				}

				name, _ := hMap["name"].(string)
				cmd, _ := hMap["command"].(string)

				if ownedMap[name] {
					if cmd == expectedCmd {
						// Exact match of owned name AND expected command: safely remove
						res.RemovedHooks = append(res.RemovedHooks, name)
						res.HasChanges = true
					} else {
						// Conflict: user modified command on owned hook name, preserve entry
						res.Conflicts = append(res.Conflicts,
							fmt.Sprintf("preserved modified hook %s with command %q", name, cmd))
						newHList = append(newHList, hEntry)
					}
				} else {
					// Unfamiliar / other hook: preserve intact
					newHList = append(newHList, hEntry)
				}
			}

			if len(newHList) > 0 {
				mMap["hooks"] = newHList
				newMatcherList = append(newMatcherList, mMap)
			}
		}

		if len(newMatcherList) > 0 {
			hooksMap[eventName] = newMatcherList
		} else {
			delete(hooksMap, eventName)
		}
	}

	if len(hooksMap) == 0 {
		delete(rootMap, "hooks")
	}

	formatted, err := json.MarshalIndent(rootMap, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format json: %w", err)
	}
	res.ProposedJSON = string(formatted)

	return res, rootMap, nil
}

// WriteAtomicWithBackup writes the given map atomically to settingsPath, preserving
// any existing settings.json.bak from the first installation.
func WriteAtomicWithBackup(settingsPath string, rootMap map[string]interface{}) error {
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create settings directory %s: %w", dir, err)
	}

	// 1. Preserve initial backup if file exists and .bak does not exist yet
	if _, err := os.Stat(settingsPath); err == nil {
		bakPath := settingsPath + ".bak"
		if _, bakErr := os.Stat(bakPath); os.IsNotExist(bakErr) {
			origBytes, err := os.ReadFile(settingsPath)
			if err == nil {
				_ = os.WriteFile(bakPath, origBytes, 0644)
			}
		}
	}

	// 2. Format JSON
	data, err := json.MarshalIndent(rootMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode settings JSON: %w", err)
	}

	// 3. Write to temporary file
	tmpPath := settingsPath + ".tmp"
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write temp settings file: %w", err)
	}

	// 4. Atomic rename
	if err := os.Rename(tmpPath, settingsPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically update %s: %w", settingsPath, err)
	}

	return nil
}
