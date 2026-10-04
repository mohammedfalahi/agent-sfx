package doctor_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"agent-sfx/internal/doctor"
)

func TestGenerateReport(t *testing.T) {
	soundsDir := filepath.Join("..", "..", "sounds")
	rep := doctor.GenerateReport("", soundsDir)

	if rep.Version == "" {
		t.Errorf("expected version to be set")
	}
	if rep.OS == "" || rep.Arch == "" {
		t.Errorf("expected OS and Arch to be populated")
	}
	if len(rep.SoundAssets) != 7 {
		t.Errorf("expected 7 sound asset entries, got %d", len(rep.SoundAssets))
	}
	if rep.Adapters["gemini"] == "" {
		t.Errorf("expected gemini adapter status in report")
	}

	human := rep.FormatHuman()
	if !strings.Contains(human, "Agent SFX Doctor Report") {
		t.Errorf("unexpected human output: %s", human)
	}

	jsonStr, err := rep.FormatJSON()
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}
