package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncFiles_WritesNormalizedSkillWithoutFrontmatter(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeTestFile(t, filepath.Join(sourceDir, "Docker Rules.md"), "# Docker Rules\nUse small images.\n")

	res, err := SyncFiles(&Config{
		Sources: []string{sourceDir},
		Targets: []string{targetDir},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 1 || res.TotalSynced != 1 || res.Created != 1 || res.Updated != 0 || res.FailedCount != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	content := readTestFile(t, filepath.Join(targetDir, "docker-rules", "SKILL.md"))
	assertContains(t, content, "name: docker-rules")
	assertContains(t, content, `description: "Trigger: docker-rules, docker rules"`)
	assertContains(t, content, "# Docker Rules")

	manifest := readTestFile(t, filepath.Join(targetDir, manifestFileName))
	assertContains(t, manifest, `"docker-rules"`)
}

func TestSyncFiles_UpdatesExistingSkillContent(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}}); err != nil {
		t.Fatalf("SyncFiles() first run error = %v", err)
	}

	writeTestFile(t, sourceFile, "# Docker Rules\nUse distroless images.\n")
	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}})
	if err != nil {
		t.Fatalf("SyncFiles() second run error = %v", err)
	}
	if res.Created != 0 || res.Updated != 1 {
		t.Fatalf("unexpected create/update counts: %+v", res)
	}

	content := readTestFile(t, filepath.Join(targetDir, "docker-rules", "SKILL.md"))
	assertContains(t, content, "Use distroless images.")
}

func TestSyncFiles_PrunesOnlyPreviouslyManagedSkills(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true}); err != nil {
		t.Fatalf("SyncFiles() first run error = %v", err)
	}

	unmanagedFile := filepath.Join(targetDir, "manual-skill", "SKILL.md")
	writeTestFile(t, unmanagedFile, "# Manual\n")

	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true})
	if err != nil {
		t.Fatalf("SyncFiles() second run error = %v", err)
	}
	if res.Pruned != 1 {
		t.Fatalf("Pruned = %d, want 1", res.Pruned)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules")); !os.IsNotExist(err) {
		t.Fatalf("managed skill directory exists or stat failed unexpectedly: %v", err)
	}

	if _, err := os.Stat(unmanagedFile); err != nil {
		t.Fatalf("unmanaged skill should remain, stat error = %v", err)
	}
}

func TestSyncFiles_DoesNotPruneByDefault(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}}); err != nil {
		t.Fatalf("SyncFiles() first run error = %v", err)
	}

	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}}); err != nil {
		t.Fatalf("SyncFiles() second run error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules", "SKILL.md")); err != nil {
		t.Fatalf("managed skill should remain when prune is disabled, stat error = %v", err)
	}
}

func TestSyncFiles_DryRunPlansCreateWithoutWriting(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeTestFile(t, filepath.Join(sourceDir, "Docker Rules.md"), "# Docker Rules\nUse small images.\n")

	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, DryRun: true})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if len(res.PlannedCreates) != 1 {
		t.Fatalf("PlannedCreates = %#v, want one create", res.PlannedCreates)
	}
	assertContains(t, res.PlannedCreates[0], filepath.Join("docker-rules", "SKILL.md"))

	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote target or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, manifestFileName)); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote manifest or stat failed unexpectedly: %v", err)
	}
}

func TestSyncFiles_DryRunPlansUpdateWithoutWriting(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")
	targetFile := filepath.Join(targetDir, "docker-rules", "SKILL.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	writeTestFile(t, targetFile, "old content")

	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, DryRun: true})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if len(res.PlannedUpdates) != 1 {
		t.Fatalf("PlannedUpdates = %#v, want one update", res.PlannedUpdates)
	}
	assertContains(t, res.PlannedUpdates[0], filepath.Join("docker-rules", "SKILL.md"))

	content := readTestFile(t, targetFile)
	if content != "old content" {
		t.Fatalf("dry-run changed target content = %q", content)
	}
}

func TestSyncFiles_DryRunPlansPruneWithoutDeleting(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true}); err != nil {
		t.Fatalf("SyncFiles() first run error = %v", err)
	}

	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true, DryRun: true})
	if err != nil {
		t.Fatalf("SyncFiles() dry run error = %v", err)
	}

	if len(res.PlannedPrunes) != 1 {
		t.Fatalf("PlannedPrunes = %#v, want one prune", res.PlannedPrunes)
	}
	assertContains(t, res.PlannedPrunes[0], "docker-rules")

	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules", "SKILL.md")); err != nil {
		t.Fatalf("dry-run should not delete managed skill, stat error = %v", err)
	}
}

func TestSyncFiles_WritesNormalizedSkillWithIncompleteFrontmatter(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeTestFile(t, filepath.Join(sourceDir, "React.md"), "---\nname: custom-react\n---\n# React Rules\nPrefer composition.\n")

	res, err := SyncFiles(&Config{
		Sources: []string{sourceDir},
		Targets: []string{targetDir},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 1 || res.TotalSynced != 1 || res.FailedCount != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	content := readTestFile(t, filepath.Join(targetDir, "custom-react", "SKILL.md"))
	assertContains(t, content, "name: custom-react")
	assertContains(t, content, `description: "Trigger: custom-react, react rules"`)
}

func TestSyncFiles_ReturnsErrorWhenNoTargets(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "Skill.md"), "# Skill\n")

	_, err := SyncFiles(&Config{Sources: []string{sourceDir}})
	if err == nil {
		t.Fatal("SyncFiles() error = nil, want error")
	}
	assertContains(t, err.Error(), "no target directories configured or discovered")
}

func TestSyncFiles_SkipsMissingSource(t *testing.T) {
	targetDir := t.TempDir()
	missingSource := filepath.Join(t.TempDir(), "missing")

	res, err := SyncFiles(&Config{
		Sources: []string{missingSource},
		Targets: []string{targetDir},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 0 || res.TotalSynced != 0 || res.Skipped != 1 || res.FailedCount != 0 || len(res.Errors) != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSyncFiles_SkipsPruneWhenSourceIsMissing(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	sourceFile := filepath.Join(sourceDir, "Docker Rules.md")

	writeTestFile(t, sourceFile, "# Docker Rules\nUse small images.\n")
	if _, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true}); err != nil {
		t.Fatalf("SyncFiles() first run error = %v", err)
	}

	if err := os.Remove(sourceFile); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := os.Remove(sourceDir); err != nil {
		t.Fatalf("Remove source dir error = %v", err)
	}

	res, err := SyncFiles(&Config{Sources: []string{sourceDir}, Targets: []string{targetDir}, PruneRemoved: true})
	if err != nil {
		t.Fatalf("SyncFiles() second run error = %v", err)
	}
	if len(res.Errors) == 0 || res.Skipped != 2 {
		t.Fatalf("expected missing source warning and skipped prune, got: %+v", res)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "docker-rules", "SKILL.md")); err != nil {
		t.Fatalf("managed skill should remain when source scan fails, stat error = %v", err)
	}
}

func TestSyncFiles_SkipsHiddenDirectories(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	writeTestFile(t, filepath.Join(sourceDir, "Visible.md"), "# Visible\n")
	writeTestFile(t, filepath.Join(sourceDir, ".obsidian", "Hidden.md"), "# Hidden\n")

	res, err := SyncFiles(&Config{
		Sources: []string{sourceDir},
		Targets: []string{targetDir},
	})
	if err != nil {
		t.Fatalf("SyncFiles() error = %v", err)
	}

	if res.TotalProcessed != 1 || res.TotalSynced != 1 || res.Created != 1 || res.FailedCount != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "hidden", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("hidden skill exists or stat failed unexpectedly: %v", err)
	}
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
