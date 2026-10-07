package setup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-sfx/internal/setup"
)

func TestPlanSetup_CleanState(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := "/opt/agent-sfx-package"

	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "gemini" && len(args) == 1 && args[0] == "--version" {
			return []byte("0.62.0\n"), nil
		}
		if name == "gemini" && len(args) >= 3 && args[0] == "extensions" && args[1] == "list" {
			return []byte("[]\n"), nil
		}
		return nil, os.ErrNotExist
	}

	plan, err := setup.PlanSetup(wsDir, homeDir, pkgDir, mockRunner)
	if err != nil {
		t.Fatalf("PlanSetup failed: %v", err)
	}

	if !plan.GeminiCliFound {
		t.Errorf("expected GeminiCliFound to be true")
	}
	if plan.GeminiVersion != "0.62.0" {
		t.Errorf("expected GeminiVersion 0.62.0, got %s", plan.GeminiVersion)
	}
	if plan.MigrationAbort {
		t.Errorf("expected clean plan not to abort")
	}
	if len(plan.ConflictDirections) != 0 {
		t.Errorf("expected 0 conflicts, got %v", plan.ConflictDirections)
	}
	if len(plan.ProposedCommands) != 1 || !strings.Contains(plan.ProposedCommands[0], "gemini extensions link") {
		t.Errorf("expected proposed link command, got %v", plan.ProposedCommands)
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "[DRY-RUN]") {
		t.Errorf("expected dry-run notice in formatted output:\n%s", output)
	}
}

func TestPlanSetup_ManualHooksPresent_ExactOwned(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := "/usr/local/lib/node_modules/@agent-sfx/agent-sfx"

	// Create project settings with 2 exact owned hooks
	projectSettingsDir := filepath.Join(wsDir, ".gemini")
	_ = os.MkdirAll(projectSettingsDir, 0755)
	settingsContent := `{
  "hooks": {
    "BeforeAgent": [
      {
        "matcher": "",
        "hooks": [
          {"name": "agent-sfx-before-agent", "type": "command", "command": "/path/bin/agent-sfx hook gemini"}
        ]
      }
    ],
    "AfterAgent": [
      {
        "matcher": "",
        "hooks": [
          {"name": "agent-sfx-after-agent", "type": "command", "command": "/path/bin/agent-sfx hook gemini"}
        ]
      }
    ]
  }
}`
	_ = os.WriteFile(filepath.Join(projectSettingsDir, "settings.json"), []byte(settingsContent), 0644)

	plan, err := setup.PlanSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanSetup failed: %v", err)
	}

	if plan.MigrationAbort {
		t.Errorf("expected clean owned hooks not to trigger abort")
	}
	if len(plan.ProjectHooks.OwnedHooksFound) != 2 {
		t.Fatalf("expected 2 owned hooks found, got %d", len(plan.ProjectHooks.OwnedHooksFound))
	}
	if len(plan.ConflictDirections) != 1 || plan.ConflictDirections[0] != "manual_hooks_to_extension" {
		t.Errorf("expected manual_hooks_to_extension conflict, got %v", plan.ConflictDirections)
	}

	removals := plan.ProposedRemovals[plan.ProjectHooks.Path]
	if len(removals) != 2 {
		t.Errorf("expected 2 proposed removals, got %d", len(removals))
	}
}

func TestPlanSetup_CustomizedHookConflict_Aborts(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := "/opt/agent-sfx"

	// Create settings with customized agent-sfx hook
	projectSettingsDir := filepath.Join(wsDir, ".gemini")
	_ = os.MkdirAll(projectSettingsDir, 0755)
	settingsContent := `{
  "hooks": {
    "BeforeAgent": [
      {
        "matcher": "",
        "hooks": [
          {"name": "agent-sfx-custom-hook", "type": "command", "command": "my-custom-script.sh"}
        ]
      }
    ]
  }
}`
	_ = os.WriteFile(filepath.Join(projectSettingsDir, "settings.json"), []byte(settingsContent), 0644)

	plan, err := setup.PlanSetup(wsDir, homeDir, pkgDir, nil)
	if err != nil {
		t.Fatalf("PlanSetup failed: %v", err)
	}

	if !plan.MigrationAbort {
		t.Errorf("expected plan to abort when customized hook is detected")
	}
	if !strings.Contains(plan.AbortReason, "customized hook") {
		t.Errorf("expected abort reason to mention customized hook, got: %s", plan.AbortReason)
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "[ABORTED]") {
		t.Errorf("expected [ABORTED] in dry-run output:\n%s", output)
	}
}

func TestPlanSetup_ExtensionAlreadyInstalled(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	pkgDir := "/opt/agent-sfx"

	// Mock extensions list returning active agent-sfx extension
	mockRunner := func(name string, args ...string) ([]byte, error) {
		if name == "gemini" && len(args) >= 3 && args[0] == "extensions" && args[1] == "list" {
			list := []map[string]any{
				{
					"name":    "agent-sfx",
					"version": "0.1.0",
					"type":    "link",
					"source":  pkgDir,
				},
			}
			return json.Marshal(list)
		}
		return nil, os.ErrNotExist
	}

	plan, err := setup.PlanSetup(wsDir, homeDir, pkgDir, mockRunner)
	if err != nil {
		t.Fatalf("PlanSetup failed: %v", err)
	}

	if !plan.Extension.Installed {
		t.Errorf("expected extension to be detected as installed")
	}
	if !plan.Extension.Linked {
		t.Errorf("expected extension to be detected as linked")
	}
	if len(plan.ConflictDirections) != 1 || plan.ConflictDirections[0] != "extension_to_manual_hooks" {
		t.Errorf("expected extension_to_manual_hooks conflict, got %v", plan.ConflictDirections)
	}
}

func TestPlanSetup_EphemeralNpxDetection(t *testing.T) {
	wsDir := t.TempDir()
	homeDir := t.TempDir()
	ephemeralPkgDir := "/Users/test/.npm/_npx/12345/node_modules/@agent-sfx/agent-sfx"

	plan, err := setup.PlanSetup(wsDir, homeDir, ephemeralPkgDir, nil)
	if err != nil {
		t.Fatalf("PlanSetup failed: %v", err)
	}

	if !plan.IsEphemeralNpx {
		t.Errorf("expected IsEphemeralNpx to be true for _npx path")
	}

	output := plan.FormatDryRun()
	if !strings.Contains(output, "ephemeral path (e.g. npx cache)") {
		t.Errorf("expected ephemeral notice in output:\n%s", output)
	}
}
