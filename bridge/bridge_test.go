package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncFiles_ScansSources(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "Docker Rules.md"), "# Docker Rules\nUse small images.\n")

	res, err := SyncFiles(&Config{
		Sources:      []string{sourceDir},
		SyncToEngram: false, // Don't run engram save CLI command during tests
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 1 || res.TotalSynced != 1 || res.FailedCount != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSyncFiles_DryRunPlansUpdate(t *testing.T) {
	sourceDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")
	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")

	res, err := SyncFiles(&Config{
		Sources: []string{sourceDir},
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if len(res.PlannedUpdates) != 1 {
		t.Fatalf("PlannedUpdates = %#v, want one update", res.PlannedUpdates)
	}
	if filepath.Clean(res.PlannedUpdates[0]) != filepath.Clean(sourceFile) {
		t.Fatalf("expected planned update path %q, got %q", sourceFile, res.PlannedUpdates[0])
	}
}

func TestSyncFiles_SkipsMissingSource(t *testing.T) {
	missingSource := filepath.Join(t.TempDir(), "missing")

	res, err := SyncFiles(&Config{
		Sources: []string{missingSource},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 0 || res.TotalSynced != 0 || res.Skipped != 1 || res.FailedCount != 0 || len(res.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSyncFiles_SkipsHiddenDirectories(t *testing.T) {
	sourceDir := t.TempDir()

	writeTestFile(t, filepath.Join(sourceDir, "Visible.md"), "# Visible\n")
	writeTestFile(t, filepath.Join(sourceDir, ".obsidian", "Hidden.md"), "# Hidden\n")

	res, err := SyncFiles(&Config{
		Sources: []string{sourceDir},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 1 || res.TotalSynced != 1 || res.FailedCount != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestBootstrapRouter(t *testing.T) {
	targetDir := t.TempDir()

	cfg := &Config{
		Targets: []string{targetDir},
	}

	err := BootstrapRouter(cfg)
	if err != nil {
		t.Fatalf("BootstrapRouter() error = %v", err)
	}

	content := readTestFile(t, filepath.Join(targetDir, "mcp-skills-router", "SKILL.md"))
	assertContains(t, content, "name: mcp-skills-router")
	assertContains(t, content, "search_skills")
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(content)
}

func assertContains(t *testing.T, value string, expected string) {
	t.Helper()
	if !strings.Contains(value, expected) {
		t.Fatalf("expected %q to contain %q", value, expected)
	}
}
