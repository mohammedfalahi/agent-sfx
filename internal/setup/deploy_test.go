package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployPackages_DryRunAndActual(t *testing.T) {
	tempDir := t.TempDir()
	sourceNpm := filepath.Join(tempDir, "source-npm")
	userAppDir := filepath.Join(tempDir, "user-app")

	// Create simulated source layout
	if err := os.MkdirAll(filepath.Join(sourceNpm, "hooks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(sourceNpm, "claude", ".claude-plugin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceNpm, "gemini-extension.json"), []byte(`{"name":"agent-sfx"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceNpm, "hooks", "hooks.json"), []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceNpm, "claude", ".claude-plugin", "plugin.json"), []byte(`{"name":"agent-sfx"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// 1. Test Dry Run
	dryReport, err := DeployPackages(sourceNpm, userAppDir, true)
	if err != nil {
		t.Fatalf("Dry run failed: %v", err)
	}
	if len(dryReport.DeployedFiles) == 0 {
		t.Fatalf("Expected planned files in dry run report, got 0")
	}
	// Verify no files written in dry run
	if _, err := os.Stat(filepath.Join(userAppDir, "gemini-extension")); !os.IsNotExist(err) {
		t.Fatalf("Expected destination not to exist after dry run")
	}

	// 2. Test Actual Deployment
	actReport, err := DeployPackages(sourceNpm, userAppDir, false)
	if err != nil {
		t.Fatalf("Actual deployment failed: %v", err)
	}
	if len(actReport.DeployedFiles) == 0 {
		t.Fatalf("Expected deployed files in actual report, got 0")
	}

	// Verify target files exist
	targetGeminiExt := filepath.Join(userAppDir, "gemini-extension", "gemini-extension.json")
	if _, err := os.Stat(targetGeminiExt); err != nil {
		t.Errorf("Expected deployed gemini-extension.json to exist at %s: %v", targetGeminiExt, err)
	}

	targetClaudePlugin := filepath.Join(userAppDir, "claude-plugin", ".claude-plugin", "plugin.json")
	if _, err := os.Stat(targetClaudePlugin); err != nil {
		t.Errorf("Expected deployed claude plugin to exist at %s: %v", targetClaudePlugin, err)
	}
}
