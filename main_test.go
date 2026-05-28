package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gentle-skills-bridge/bridge"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0", code)
	}

	if !strings.Contains(stdout.String(), "v"+version) {
		t.Fatalf("stdout = %q, want version", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunMissingCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() exit code = 0, want failure")
	}

	if !strings.Contains(stdout.String(), "Uso:") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"unknown"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() exit code = 0, want failure")
	}

	if !strings.Contains(stderr.String(), "Comando desconocido") {
		t.Fatalf("stderr = %q, want unknown command error", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Uso:") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
}

func TestRunInvalidConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	configPath := filepath.Join(t.TempDir(), "config.json")

	if err := os.WriteFile(configPath, []byte(`{"targets":["target"]}`), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	code := run([]string{"-config", configPath, "sync"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() exit code = 0, want failure")
	}

	if !strings.Contains(stderr.String(), "configuración inválida") {
		t.Fatalf("stderr = %q, want invalid config error", stderr.String())
	}
}

func TestRunSyncDryRunAfterCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	targetDir := filepath.Join(tempDir, "target")
	configPath := filepath.Join(tempDir, "config.json")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "Docker Rules.md"), []byte("# Docker Rules\n"), 0644); err != nil {
		t.Fatalf("WriteFile() source error = %v", err)
	}
	config := `{"sources":["` + filepath.ToSlash(sourceDir) + `"],"targets":["` + filepath.ToSlash(targetDir) + `"],"watch_interval_ms":1000}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("WriteFile() config error = %v", err)
	}

	code := run([]string{"-config", configPath, "sync", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Dry-run") && !strings.Contains(stdout.String(), "dry-run") {
		t.Fatalf("stdout = %q, want dry-run output", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote target or stat failed unexpectedly: %v", err)
	}
}

func TestRunRemove(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	configPath := filepath.Join(tempDir, "config.json")

	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	config := `{"sources":["` + filepath.ToSlash(sourceDir) + `"],"targets":[],"watch_interval_ms":1000}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("WriteFile() config error = %v", err)
	}

	// Remove sourceDir
	code := run([]string{"-config", configPath, "remove", sourceDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	// Read config back to verify it was removed
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() config error = %v", err)
	}

	if strings.Contains(string(content), filepath.ToSlash(sourceDir)) {
		t.Fatalf("config still contains removed source: %s", string(content))
	}
}

func TestGetChoices(t *testing.T) {
	cfg := &bridge.Config{SyncToEngram: true}
	choices := getChoices(cfg)
	if len(choices) != 6 {
		t.Fatalf("expected 6 choices, got %d", len(choices))
	}
	if !strings.Contains(choices[3], "[ACTIVO]") {
		t.Fatalf("expected choices[3] to contain [ACTIVO], got %q", choices[3])
	}

	cfg.SyncToEngram = false
	choices = getChoices(cfg)
	if !strings.Contains(choices[3], "[INACTIVO]") {
		t.Fatalf("expected choices[3] to contain [INACTIVO], got %q", choices[3])
	}
}

func TestTuiModelToggleSync(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	cfg := &bridge.Config{
		Sources:      []string{"some-source"},
		SyncToEngram: true,
	}

	// Write initial config file so saveConfig works
	if err := saveConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	m := initialModel(configPath, cfg, configPath)
	m.cursor = 3 // Index for toggle sync

	// Simulate pressing enter
	rawModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel := rawModel.(tuiModel)

	if updatedModel.cfg.SyncToEngram {
		t.Fatalf("expected SyncToEngram to be toggled to false, got true")
	}

	if updatedModel.state != "success" {
		t.Fatalf("expected state to be success, got %q", updatedModel.state)
	}

	if !strings.Contains(updatedModel.infoMessage, "Sincronización con Engram desactivada") {
		t.Fatalf("expected infoMessage to notify deactivation, got %q", updatedModel.infoMessage)
	}

	// Now check if it toggles back to active
	updatedModel.state = "menu"
	updatedModel.cursor = 3

	rawModel2, _ := updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel2 := rawModel2.(tuiModel)

	if !updatedModel2.cfg.SyncToEngram {
		t.Fatalf("expected SyncToEngram to be toggled back to true, got false")
	}

	if !strings.Contains(updatedModel2.infoMessage, "Sincronización con Engram activada") {
		t.Fatalf("expected infoMessage to notify activation, got %q", updatedModel2.infoMessage)
	}
}
