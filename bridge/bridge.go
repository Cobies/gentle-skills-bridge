package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const manifestFileName = ".gentle-skills-bridge-manifest.json"

// Config represents the tool configuration format.
type Config struct {
	Sources            []string `json:"sources"`
	Targets            []string `json:"targets"`
	AutoDiscoverAgents bool     `json:"auto_discover_agents"`
	SyncToEngram       bool     `json:"sync_to_engram"`
	EngramProject      string   `json:"engram_project"`
	WatchIntervalMS    int      `json:"watch_interval_ms"`
	PruneRemoved       bool     `json:"prune_removed"`
	DryRun             bool     `json:"-"`
}

type syncManifest struct {
	Skills []string `json:"skills"`
}

// SyncResult holds the statistics of a sync run.
type SyncResult struct {
	TotalProcessed int
	TotalSynced    int
	Created        int
	Updated        int
	Pruned         int
	Skipped        int
	FailedCount    int
	Errors         []string
	PlannedCreates []string
	PlannedUpdates []string
	PlannedPrunes  []string
}

// SyncFiles performs a one-time sync of all source directories to all target directories.
func SyncFiles(cfg *Config) (*SyncResult, error) {
	result := &SyncResult{}

	// 1. Resolve Targets (including auto-discovery)
	targets := cfg.Targets
	if cfg.AutoDiscoverAgents {
		discovered, err := DiscoverTargets()
		if err != nil {
			fmt.Printf("[warning] Failed to discover agents: %v\n", err)
		} else {
			// Merge discovered targets, avoiding duplicates
			seen := make(map[string]bool)
			for _, t := range targets {
				seen[filepath.Clean(t)] = true
			}
			for _, t := range discovered {
				cleanT := filepath.Clean(t)
				if !seen[cleanT] {
					seen[cleanT] = true
					targets = append(targets, t)
				}
			}
		}
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("no target directories configured or discovered")
	}

	fmt.Printf("[info] Targets resolved: %v\n", targets)
	if cfg.DryRun {
		fmt.Println("[dry-run] Simulating synchronization without writing files")
	}

	syncedByTarget := make(map[string]map[string]bool)
	for _, target := range targets {
		syncedByTarget[filepath.Clean(target)] = make(map[string]bool)
	}

	// 2. Scan Sources
	for _, sourceDir := range cfg.Sources {
		cleanSource := filepath.Clean(sourceDir)
		if _, err := os.Stat(cleanSource); os.IsNotExist(err) {
			fmt.Printf("[warning] Source directory does not exist: %s\n", cleanSource)
			result.Errors = append(result.Errors, fmt.Sprintf("Source directory does not exist: %s", cleanSource))
			result.Skipped++
			continue
		}

		err := filepath.Walk(cleanSource, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				// Don't recurse deep into hidden directories (like .git or .obsidian)
				if strings.HasPrefix(info.Name(), ".") && path != cleanSource {
					return filepath.SkipDir
				}
				return nil
			}

			// Only process Markdown files
			if filepath.Ext(path) != ".md" {
				return nil
			}

			result.TotalProcessed++

			// Read file content
			contentBytes, err := os.ReadFile(path)
			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to read %s: %v", path, err))
				return nil
			}

			// Parse and normalize the skill
			skill, err := ParseMarkdown(path, string(contentBytes))
			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to parse %s: %v", path, err))
				return nil
			}

			// Deploy skill to all targets
			for _, targetDir := range targets {
				cleanTargetDir := filepath.Clean(targetDir)
				skillFolder := filepath.Join(targetDir, skill.Name)
				targetFile := filepath.Join(skillFolder, "SKILL.md")
				targetExists := pathExists(targetFile)

				var isIdentical bool
				if targetExists {
					if !skill.Normalized {
						isIdentical = filesIdentical(path, targetFile)
					} else {
						isIdentical = fileContentIdentical(skill.Content, targetFile)
					}
				}

				if isIdentical {
					result.Skipped++
					syncedByTarget[cleanTargetDir][skill.Name] = true
					continue
				}

				if cfg.DryRun {
					if !targetExists {
						result.PlannedCreates = append(result.PlannedCreates, targetFile)
					} else {
						result.PlannedUpdates = append(result.PlannedUpdates, targetFile)
					}

					syncedByTarget[cleanTargetDir][skill.Name] = true
					continue
				}

				if err := os.MkdirAll(skillFolder, 0755); err != nil {
					result.FailedCount++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to create folder %s: %v", skillFolder, err))
					continue
				}

				var deployErr error
				if !skill.Normalized {
					deployErr = linkOrCopy(path, targetFile)
				} else {
					deployErr = writeContent(skill.Content, targetFile)
				}

				if deployErr != nil {
					result.FailedCount++
					result.Errors = append(result.Errors, fmt.Sprintf("Failed to deploy %s to %s: %v", path, targetFile, deployErr))
					continue
				}

				if targetExists {
					result.Updated++
				} else {
					result.Created++
				}

				syncedByTarget[cleanTargetDir][skill.Name] = true
			}

			// Sync to Engram if configured
			if cfg.SyncToEngram && !cfg.DryRun {
				// Use title as file base name without path
				title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				err := SyncToEngram(skill.Name, title, skill.Content, cfg.EngramProject)
				if err != nil {
					// We don't fail the sync, just log a warning
					fmt.Printf("[warning] Engram sync failed for %s: %v\n", skill.Name, err)
				}
			}

			result.TotalSynced++
			fmt.Printf("[sync] Registered skill: %s\n", skill.Name)
			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Error walking source directory %s: %v", cleanSource, err))
		}
	}

	canPrune := len(result.Errors) == 0

	for _, targetDir := range targets {
		cleanTargetDir := filepath.Clean(targetDir)
		if cfg.PruneRemoved && cfg.DryRun && canPrune {
			plannedPrunes, err := plannedPrunePaths(cleanTargetDir, syncedByTarget[cleanTargetDir])
			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to plan prune for target %s: %v", cleanTargetDir, err))
			} else {
				result.PlannedPrunes = append(result.PlannedPrunes, plannedPrunes...)
			}
		} else if cfg.PruneRemoved && canPrune {
			pruned, err := pruneRemovedSkills(cleanTargetDir, syncedByTarget[cleanTargetDir])
			if err != nil {
				result.FailedCount++
				result.Errors = append(result.Errors, fmt.Sprintf("Failed to prune target %s: %v", cleanTargetDir, err))
			} else {
				result.Pruned += pruned
			}
		} else if cfg.PruneRemoved && !canPrune {
			fmt.Printf("[warning] Prune skipped for %s because source scanning had errors\n", cleanTargetDir)
			result.Skipped++
		}

		if cfg.DryRun {
			continue
		}

		if !canPrune {
			fmt.Printf("[warning] Manifest update skipped for %s because source scanning had errors\n", cleanTargetDir)
			continue
		}

		if err := writeManifest(cleanTargetDir, syncedByTarget[cleanTargetDir]); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to write manifest for %s: %v", cleanTargetDir, err))
		}
	}

	// 4. Trigger Gentle-AI registry refresh if gentle-ai CLI is available
	if !cfg.DryRun && len(result.Errors) == 0 {
		triggerGentleRegistryRefresh()
	}

	return result, nil
}

func plannedPrunePaths(targetDir string, current map[string]bool) ([]string, error) {
	previous, err := readManifest(targetDir)
	if err != nil {
		return nil, err
	}

	planned := make([]string, 0)
	for _, skillName := range previous.Skills {
		if current[skillName] {
			continue
		}
		planned = append(planned, filepath.Join(targetDir, skillName))
	}
	sort.Strings(planned)

	return planned, nil
}

func pruneRemovedSkills(targetDir string, current map[string]bool) (int, error) {
	previous, err := readManifest(targetDir)
	if err != nil {
		return 0, err
	}

	pruned := 0
	for _, skillName := range previous.Skills {
		if current[skillName] {
			continue
		}

		if err := os.RemoveAll(filepath.Join(targetDir, skillName)); err != nil {
			return pruned, err
		}
		pruned++
	}

	return pruned, nil
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func readManifest(targetDir string) (*syncManifest, error) {
	manifestPath := filepath.Join(targetDir, manifestFileName)
	content, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return &syncManifest{}, nil
	}
	if err != nil {
		return nil, err
	}

	var manifest syncManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func writeManifest(targetDir string, current map[string]bool) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	manifest := syncManifest{Skills: make([]string, 0, len(current))}
	for skillName := range current {
		manifest.Skills = append(manifest.Skills, skillName)
	}
	sort.Strings(manifest.Skills)

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	return os.WriteFile(filepath.Join(targetDir, manifestFileName), content, 0644)
}

// linkOrCopy tries to create a symlink, falling back to copy if it fails.
func linkOrCopy(src, dst string) error {
	// Remove existing target if it exists
	if _, err := os.Lstat(dst); err == nil {
		_ = os.Remove(dst)
	}

	// Try symlink first
	err := os.Symlink(src, dst)
	if err == nil {
		return nil
	}

	// Fallback to copying
	return copyFile(src, dst)
}

// copyFile copies content from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}

// writeContent writes raw string content to dst.
func writeContent(content, dst string) error {
	// Remove existing target if it exists
	if _, err := os.Lstat(dst); err == nil {
		_ = os.Remove(dst)
	}
	return os.WriteFile(dst, []byte(content), 0644)
}

// triggerGentleRegistryRefresh executes 'gentle-ai skill-registry refresh' if it exists on PATH.
func triggerGentleRegistryRefresh() {
	_, err := exec.LookPath("gentle-ai")
	if err != nil {
		return // gentle-ai CLI is not installed or not in PATH
	}

	cmd := exec.Command("gentle-ai", "skill-registry", "refresh")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("[warning] Failed to refresh gentle-ai skill registry: %v, stderr: %s\n", err, stderr.String())
	} else {
		fmt.Println("[sync] Refreshed gentle-ai skill registry successfully.")
	}
}

// filesIdentical compares two files byte by byte to check if they are identical.
func filesIdentical(path1, path2 string) bool {
	f1, err := os.Open(path1)
	if err != nil {
		return false
	}
	defer f1.Close()

	f2, err := os.Open(path2)
	if err != nil {
		return false
	}
	defer f2.Close()

	s1, err := f1.Stat()
	if err != nil {
		return false
	}
	s2, err := f2.Stat()
	if err != nil {
		return false
	}
	if s1.Size() != s2.Size() {
		return false
	}

	b1 := make([]byte, 64*1024)
	b2 := make([]byte, 64*1024)
	for {
		n1, err1 := f1.Read(b1)
		n2, err2 := f2.Read(b2)
		if err1 != nil || err2 != nil {
			if err1 == io.EOF && err2 == io.EOF {
				return true
			}
			return false
		}
		if n1 != n2 || !bytes.Equal(b1[:n1], b2[:n2]) {
			return false
		}
	}
}

// fileContentIdentical compares string content with file content to check if they are identical.
func fileContentIdentical(content, path string) bool {
	existingBytes, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return string(existingBytes) == content
}
