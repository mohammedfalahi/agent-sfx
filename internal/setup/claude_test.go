package setup_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sfx/internal/setup"
)

// helper to create a valid self-contained Claude plugin directory for tests
func createTestClaudePackage(t *testing.T, baseDir string) string {
	t.Helper()
	claudeDir := filepath.Join(baseDir, "claude")
	claudePluginDir := filepath.Join(claudeDir, ".claude-plugin")
	binDir := filepath.Join(claudeDir, "bin")

	if err := os.MkdirAll(claudePluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}

	pluginJSON := map[string]any{
		"name":        "agent-sfx",
		"version":     "0.1.0",
		"description": "Test plugin",
	}
	pBytes, _ := json.Marshal(pluginJSON)
	if err := os.WriteFile(filepath.Join(claudePluginDir, "plugin.json"), pBytes, 0644); err != nil {
		t.Fatalf("failed to write plugin.json: %v", err)
	}

	marketplaceJSON := map[string]any{
		"name":        "agent-sfx-local",
		"description": "Test marketplace",
		"owner": map[string]string{
			"name": "Agent SFX",
		},
		"plugins": []map[string]any{
			{
				"name":        "agent-sfx",
				"version":     "0.1.0",
				"description": "Test plugin",
				"source":      "./",
			},
		},
	}
	mBytes, _ := json.Marshal(marketplaceJSON)
	if err := os.WriteFile(filepath.Join(claudePluginDir, "marketplace.json"), mBytes, 0644); err != nil {
		t.Fatalf("failed to write marketplace.json: %v", err)
	}

	if err := os.WriteFile(filepath.Join(binDir, "run.js"), []byte("#!/usr/bin/env node\n"), 0755); err != nil {
		t.Fatalf("failed to write run.js: %v", err)
	}

	return baseDir
}

func TestPlanClaudeSetup_RelocatedDirectory(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	disposablePkgDir := t.TempDir()

	createTestClaudePackage(t, disposablePkgDir)

	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "claude" && len(args) == 1 && args[0] == "--version" {
			return []byte("2.1.162 (Claude Code)\n"), nil
		}
		return nil, fmt.Errorf("command not found: %s", name)
	}

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, disposablePkgDir, mockRunner)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	expectedPluginDir := filepath.Join(disposablePkgDir, "claude")
	if plan.PackageDir != disposablePkgDir {
		t.Errorf("expected PackageDir %s, got %s", disposablePkgDir, plan.PackageDir)
	}
	if plan.ClaudePluginDir != expectedPluginDir {
		t.Errorf("expected ClaudePluginDir %s, got %s", expectedPluginDir, plan.ClaudePluginDir)
	}
	if plan.MigrationAbort {
		t.Fatalf("expected MigrationAbort false, got true: %s", plan.AbortReason)
	}

	if len(plan.ProposedCommands) < 2 {
		t.Fatalf("expected at least 2 proposed commands, got %v", plan.ProposedCommands)
	}

	expectedAdd := fmt.Sprintf("claude plugin marketplace add %q", expectedPluginDir)
	if plan.ProposedCommands[0] != expectedAdd {
		t.Errorf("expected first command %q, got %q", expectedAdd, plan.ProposedCommands[0])
	}
	expectedInstall := "claude plugin install agent-sfx@agent-sfx-local"
	if plan.ProposedCommands[1] != expectedInstall {
		t.Errorf("expected second command %q, got %q", expectedInstall, plan.ProposedCommands[1])
	}
}

func TestPlanClaudeSetup_CleanState(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := t.TempDir()

	createTestClaudePackage(t, pkgDir)

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if plan.ProjectHooks.Exists {
		t.Errorf("expected ProjectHooks.Exists false")
	}
	if plan.UserHooks.Exists {
		t.Errorf("expected UserHooks.Exists false")
	}
	if plan.Marketplace.Registered {
		t.Errorf("expected Marketplace.Registered false")
	}
	if plan.Plugin.Installed {
		t.Errorf("expected Plugin.Installed false")
	}
	if plan.MigrationAbort {
		t.Errorf("expected MigrationAbort false")
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "Status: file does not exist (clean)") {
		t.Errorf("expected clean status in output:\n%s", output)
	}
	if !strings.Contains(output, "claude plugin marketplace add") {
		t.Errorf("expected marketplace add in output:\n%s", output)
	}
	if !strings.Contains(output, "claude plugin install agent-sfx@agent-sfx-local") {
		t.Errorf("expected plugin install in output:\n%s", output)
	}
	if !strings.Contains(output, "[DRY-RUN] No settings were modified and no plugins were installed.") {
		t.Errorf("expected dry-run footer in output:\n%s", output)
	}
}

func TestPlanClaudeSetup_ExactHookOwnership(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := t.TempDir()

	createTestClaudePackage(t, pkgDir)

	// Create project settings with owned manual hooks
	projClaudeDir := filepath.Join(wsDir, ".claude")
	if err := os.MkdirAll(projClaudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	settingsJSON := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"name":    "agent-sfx-session-start",
							"command": "agent-sfx hook claude",
						},
					},
				},
			},
			"Stop": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"name":    "agent-sfx-stop",
							"command": "agent-sfx hook claude",
						},
					},
				},
			},
		},
	}
	sBytes, _ := json.Marshal(settingsJSON)
	if err := os.WriteFile(filepath.Join(projClaudeDir, "settings.json"), sBytes, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if !plan.ProjectHooks.Exists {
		t.Errorf("expected ProjectHooks.Exists true")
	}
	if len(plan.ProjectHooks.OwnedHooksFound) != 2 {
		t.Fatalf("expected 2 owned hooks found, got %d: %v",
			len(plan.ProjectHooks.OwnedHooksFound), plan.ProjectHooks.OwnedHooksFound)
	}
	if plan.ProjectHooks.HasCustomized {
		t.Errorf("expected HasCustomized false")
	}
	if plan.MigrationAbort {
		t.Errorf("expected MigrationAbort false")
	}

	removals := plan.ProposedRemovals[plan.ProjectHooks.Path]
	if len(removals) != 2 {
		t.Errorf("expected 2 removals for project settings, got %d", len(removals))
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "Remove 2 manual owned hook(s)") {
		t.Errorf("expected removal count in output:\n%s", output)
	}
	if !strings.Contains(output, "preserves backup at") {
		t.Errorf("expected backup mention in output:\n%s", output)
	}
}

func TestPlanClaudeSetup_CustomizedHookConflict_Aborts(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := t.TempDir()

	createTestClaudePackage(t, pkgDir)

	userClaudeDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(userClaudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Customized command under owned name
	settingsJSON := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{
							"name":    "agent-sfx-stop",
							"command": "custom-sfx-wrapper hook claude --custom",
						},
					},
				},
			},
		},
	}
	sBytes, _ := json.Marshal(settingsJSON)
	if err := os.WriteFile(filepath.Join(userClaudeDir, "settings.json"), sBytes, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if !plan.UserHooks.HasCustomized {
		t.Errorf("expected UserHooks.HasCustomized true")
	}
	if !plan.MigrationAbort {
		t.Errorf("expected MigrationAbort true")
	}
	if !strings.Contains(plan.AbortReason, "customized hook") {
		t.Errorf("expected customized hook mention in AbortReason: %s", plan.AbortReason)
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "[ABORTED] Migration cannot proceed automatically") {
		t.Errorf("expected abort block in output:\n%s", output)
	}
}

func TestPlanClaudeSetup_ScopeAwarePluginState(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := t.TempDir()

	createTestClaudePackage(t, pkgDir)

	userPluginsDir := filepath.Join(homeDir, ".claude", "plugins")
	if err := os.MkdirAll(userPluginsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Installed and enabled in user scope
	installedPlugins := map[string]any{
		"version": 2,
		"plugins": map[string]any{
			"agent-sfx@agent-sfx-local": []map[string]any{
				{
					"scope":       "user",
					"installPath": "/fake/path/agent-sfx",
					"version":     "0.1.0",
				},
			},
		},
	}
	ipBytes, _ := json.Marshal(installedPlugins)
	if err := os.WriteFile(filepath.Join(userPluginsDir, "installed_plugins.json"), ipBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// Known marketplace registered
	knownMarketplaces := map[string]any{
		"agent-sfx-local": map[string]any{
			"installLocation": filepath.Join(pkgDir, "claude"),
		},
	}
	kmBytes, _ := json.Marshal(knownMarketplaces)
	if err := os.WriteFile(filepath.Join(userPluginsDir, "known_marketplaces.json"), kmBytes, 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if !plan.Plugin.Installed {
		t.Errorf("expected Plugin.Installed true")
	}
	if !plan.Plugin.Enabled {
		t.Errorf("expected Plugin.Enabled true")
	}
	if plan.Plugin.Scope != "user" {
		t.Errorf("expected Plugin.Scope 'user', got %q", plan.Plugin.Scope)
	}
	if !plan.Marketplace.Registered {
		t.Errorf("expected Marketplace.Registered true")
	}
	if len(plan.ProposedCommands) != 1 || !strings.Contains(plan.ProposedCommands[0], "already installed and enabled") {
		t.Errorf("expected no-op command when already installed & enabled, got %v", plan.ProposedCommands)
	}

	// 2. Installed but disabled via settings.json
	userSettings := map[string]any{
		"enabledPlugins": map[string]bool{
			"agent-sfx@agent-sfx-local": false,
		},
	}
	usBytes, _ := json.Marshal(userSettings)
	if err := os.WriteFile(filepath.Join(homeDir, ".claude", "settings.json"), usBytes, 0644); err != nil {
		t.Fatal(err)
	}

	planDisabled, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if !planDisabled.Plugin.Installed {
		t.Errorf("expected Plugin.Installed true")
	}
	if planDisabled.Plugin.Enabled {
		t.Errorf("expected Plugin.Enabled false")
	}
	if len(planDisabled.ProposedCommands) != 1 || planDisabled.ProposedCommands[0] != "claude plugin enable agent-sfx" {
		t.Errorf("expected enable command when disabled, got %v", planDisabled.ProposedCommands)
	}
}

func TestPlanClaudeSetup_UncertainStateAborts(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()

	// 1. Missing plugin manifest
	emptyPkgDir := t.TempDir()
	plan1, err := setup.PlanClaudeSetup(wsDir, homeDir, emptyPkgDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan1.MigrationAbort || !strings.Contains(plan1.AbortReason, "manifest not found") {
		t.Errorf("expected abort on missing manifest, got abort=%v reason=%s", plan1.MigrationAbort, plan1.AbortReason)
	}

	// 2. Missing marketplace manifest
	pkgDirNoMkt := t.TempDir()
	claudeDir := filepath.Join(pkgDirNoMkt, "claude", ".claude-plugin")
	_ = os.MkdirAll(claudeDir, 0755)
	_ = os.WriteFile(filepath.Join(claudeDir, "plugin.json"), []byte("{}"), 0644)
	plan2, err := setup.PlanClaudeSetup(wsDir, homeDir, pkgDirNoMkt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan2.MigrationAbort || !strings.Contains(plan2.AbortReason, "marketplace manifest not found") {
		t.Errorf("expected abort on missing marketplace manifest, got abort=%v reason=%s", plan2.MigrationAbort, plan2.AbortReason)
	}

	// 3. Corrupted JSON in settings.json
	validPkgDir := t.TempDir()
	createTestClaudePackage(t, validPkgDir)

	badSettingsDir := filepath.Join(wsDir, ".claude")
	_ = os.MkdirAll(badSettingsDir, 0755)
	_ = os.WriteFile(filepath.Join(badSettingsDir, "settings.json"), []byte("{ not-valid-json"), 0644)

	plan3, err := setup.PlanClaudeSetup(wsDir, homeDir, validPkgDir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan3.MigrationAbort || !strings.Contains(plan3.AbortReason, "invalid JSON") {
		t.Errorf("expected abort on invalid JSON, got abort=%v reason=%s", plan3.MigrationAbort, plan3.AbortReason)
	}
}

func TestPlanClaudeSetup_EphemeralNpxDetection(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	npxPkgDir := filepath.Join(t.TempDir(), "_npx", "pkg")
	_ = os.MkdirAll(npxPkgDir, 0755)

	createTestClaudePackage(t, npxPkgDir)

	plan, err := setup.PlanClaudeSetup(wsDir, homeDir, npxPkgDir, nil)
	if err != nil {
		t.Fatalf("PlanClaudeSetup failed: %v", err)
	}

	if !plan.IsEphemeralNpx {
		t.Errorf("expected IsEphemeralNpx true for _npx path")
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "ephemeral path (e.g. npx cache)") {
		t.Errorf("expected ephemeral notice in output:\n%s", output)
	}
}
