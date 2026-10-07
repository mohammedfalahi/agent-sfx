package installer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agent-sfx/internal/installer"
)

func TestInstaller_PlanAndWrite(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".gemini", "settings.json")
	binPath := filepath.Join(tmpDir, "bin", "agent-sfx")

	// 1. Initial install on non-existent settings file
	plan, rootMap, err := installer.PlanInstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("PlanInstall error: %v", err)
	}

	if !plan.HasChanges {
		t.Fatalf("expected initial install to have changes")
	}
	if len(plan.AddedHooks) != len(installer.AllOwnedHooks) {
		t.Fatalf("expected %d added hooks, got %d", len(installer.AllOwnedHooks), len(plan.AddedHooks))
	}

	// Write proposed changes
	if err := installer.WriteAtomicWithBackup(settingsPath, rootMap); err != nil {
		t.Fatalf("WriteAtomicWithBackup error: %v", err)
	}

	// 2. Second install should be idempotent (no changes)
	plan2, _, err := installer.PlanInstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("second PlanInstall error: %v", err)
	}
	if plan2.HasChanges {
		t.Errorf("second install should be idempotent with zero changes")
	}
	if len(plan2.UnchangedHooks) != len(installer.AllOwnedHooks) {
		t.Errorf("expected %d unchanged hooks, got %d", len(installer.AllOwnedHooks), len(plan2.UnchangedHooks))
	}
}

func TestInstaller_PreservesUnrelatedSettings(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	binPath := "/usr/local/bin/agent-sfx"

	initialContent := `{
		"telemetry": {
			"enabled": false
		},
		"hooks": {
			"BeforeAgent": [
				{
					"matcher": "",
					"hooks": [
						{
							"name": "user-custom-hook",
							"type": "command",
							"command": "/usr/local/bin/my-hook.sh"
						}
					]
				}
			]
		}
	}`

	if err := os.WriteFile(settingsPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial settings: %v", err)
	}

	_, rootMap, err := installer.PlanInstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("PlanInstall error: %v", err)
	}

	if err := installer.WriteAtomicWithBackup(settingsPath, rootMap); err != nil {
		t.Fatalf("WriteAtomicWithBackup error: %v", err)
	}

	// Verify telemetry setting is preserved
	data, _ := os.ReadFile(settingsPath)
	var parsed map[string]interface{}
	_ = json.Unmarshal(data, &parsed)

	telemetry, ok := parsed["telemetry"].(map[string]interface{})
	if !ok || telemetry["enabled"] != false {
		t.Errorf("telemetry settings were lost or modified: %+v", parsed)
	}

	// 3. Uninstall should remove ONLY our hooks, preserving user-custom-hook
	uPlan, uRootMap, err := installer.PlanUninstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("PlanUninstall error: %v", err)
	}
	if !uPlan.HasChanges {
		t.Fatalf("expected uninstall to have changes")
	}
	if len(uPlan.RemovedHooks) != len(installer.AllOwnedHooks) {
		t.Errorf("expected %d removed hooks, got %d", len(installer.AllOwnedHooks), len(uPlan.RemovedHooks))
	}

	if err := installer.WriteAtomicWithBackup(settingsPath, uRootMap); err != nil {
		t.Fatalf("write uninstall error: %v", err)
	}

	dataAfter, _ := os.ReadFile(settingsPath)
	var parsedAfter map[string]interface{}
	_ = json.Unmarshal(dataAfter, &parsedAfter)

	hooksAfter := parsedAfter["hooks"].(map[string]interface{})
	beforeAgentList := hooksAfter["BeforeAgent"].([]interface{})
	if len(beforeAgentList) != 1 {
		t.Errorf("expected user-custom-hook to be preserved in BeforeAgent, got: %+v", beforeAgentList)
	}
}

func TestInstaller_ConflictDetection(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	binPath := "/usr/local/bin/agent-sfx"

	// Modified hook with same name but different command
	content := `{
		"hooks": {
			"BeforeAgent": [
				{
					"matcher": "",
					"hooks": [
						{
							"name": "agent-sfx-before-agent",
							"type": "command",
							"command": "/custom/path/agent-sfx hook gemini"
						}
					]
				}
			]
		}
	}`
	_ = os.WriteFile(settingsPath, []byte(content), 0644)

	plan, _, err := installer.PlanInstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("PlanInstall error: %v", err)
	}

	if len(plan.Conflicts) != 1 {
		t.Errorf("expected 1 conflict detected, got %d", len(plan.Conflicts))
	}

	// Test Uninstall preserves modified entry
	uPlan, _, err := installer.PlanUninstall(installer.ScopeProject, settingsPath, binPath)
	if err != nil {
		t.Fatalf("PlanUninstall error: %v", err)
	}
	if len(uPlan.Conflicts) != 1 {
		t.Errorf("expected uninstall to detect and preserve conflict, got %d conflicts", len(uPlan.Conflicts))
	}
	if len(uPlan.RemovedHooks) != 0 {
		t.Errorf("modified hook should not have been removed")
	}
}

func TestInstaller_RejectsInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.json")
	_ = os.WriteFile(badPath, []byte("{bad json content"), 0644)

	_, _, err := installer.PlanInstall(installer.ScopeProject, badPath, "/bin/agent-sfx")
	if err == nil {
		t.Fatalf("expected error on invalid JSON, got nil")
	}
}

func TestInstaller_BackupPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	bakPath := settingsPath + ".bak"

	origContent := `{"initial": "original state"}`
	_ = os.WriteFile(settingsPath, []byte(origContent), 0644)

	// First write creates .bak
	var m1 map[string]interface{}
	_ = json.Unmarshal([]byte(`{"run": 1}`), &m1)
	if err := installer.WriteAtomicWithBackup(settingsPath, m1); err != nil {
		t.Fatalf("first write error: %v", err)
	}

	bakBytes, err := os.ReadFile(bakPath)
	if err != nil || string(bakBytes) != origContent {
		t.Fatalf("bak file should contain original content, got %s", string(bakBytes))
	}

	// Second write must PRESERVE original .bak, not overwrite with run 1
	var m2 map[string]interface{}
	_ = json.Unmarshal([]byte(`{"run": 2}`), &m2)
	if err := installer.WriteAtomicWithBackup(settingsPath, m2); err != nil {
		t.Fatalf("second write error: %v", err)
	}

	bakBytesAfter, _ := os.ReadFile(bakPath)
	if string(bakBytesAfter) != origContent {
		t.Errorf("backup was overwritten! Expected %s, got %s", origContent, string(bakBytesAfter))
	}
}
