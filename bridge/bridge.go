package bridge

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// SyncFiles performs a one-time sync of all source directories to Engram if configured.
func SyncFiles(cfg *Config) (*SyncResult, error) {
	result := &SyncResult{}

	// 1. Scan Sources
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

			// Sync to Engram if configured
			if cfg.SyncToEngram && !cfg.DryRun {
				// Use title as file base name without path
				title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				err := SyncToEngram(skill.Name, title, skill.Content, cfg.EngramProject)
				if err != nil {
					// We don't fail the sync, just log a warning
					fmt.Printf("[warning] Engram sync failed for %s: %v\n", skill.Name, err)
				}
			} else if cfg.DryRun {
				result.PlannedUpdates = append(result.PlannedUpdates, path)
			}

			result.TotalSynced++
			fmt.Printf("[sync] Processed skill for Engram: %s\n", skill.Name)
			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Error walking source directory %s: %v", cleanSource, err))
		}
	}

	// Trigger Gentle-AI registry refresh if gentle-ai CLI is available
	if !cfg.DryRun && len(result.Errors) == 0 {
		triggerGentleRegistryRefresh()
	}

	return result, nil
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// BootstrapRouter deploys the mcp-skills-router.md as a skill to all target directories.
func BootstrapRouter(cfg *Config) error {
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
		return fmt.Errorf("no target directories configured or discovered")
	}

	routerSkillContent := `---
name: mcp-skills-router
description: "Trigger: cuando requieras guías de codificación, mejores prácticas, patrones de diseño o resolver cualquier tarea de desarrollo."
---

# MCP Skills Router

Antes de proceder con cualquier tarea o resolver un problema técnico:
1. Verificá si existe alguna skill específica en tus instrucciones.
2. Si no encontrás una skill local adecuada, llamá de forma OBLIGATORIA a la herramienta del MCP ` + "`search_skills`" + ` pasándole palabras clave relacionadas con la tarea (por ejemplo: "alphafold", "pubmed", "testing", "diseño").
3. Si encontrás una skill relevante en la búsqueda, cargá sus instrucciones y seguilas al pie de la letra.
`

	successCount := 0
	for _, targetDir := range targets {
		routerFolder := filepath.Join(targetDir, "mcp-skills-router")
		targetFile := filepath.Join(routerFolder, "SKILL.md")

		if err := os.MkdirAll(routerFolder, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[error] Falló la creación de carpeta %s: %v\n", routerFolder, err)
			continue
		}

		if err := os.WriteFile(targetFile, []byte(routerSkillContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "[error] Falló la escritura en %s: %v\n", targetFile, err)
			continue
		}

		fmt.Printf("[bootstrap] Ruteador de MCP instalado con éxito en: %s\n", targetFile)
		successCount++
	}

	if successCount == 0 {
		return fmt.Errorf("no se pudo instalar el ruteador de MCP en ningún agente")
	}

	// Configurar automáticamente el servidor MCP en los agentes detectados
	home, err := os.UserHomeDir()
	if err == nil {
		execPath, err := os.Executable()
		if err != nil {
			execPath = "gentle-skills-bridge"
		}
		if absPath, err := filepath.Abs(execPath); err == nil {
			execPath = absPath
		}
		// Si se ejecuta mediante "go run" o en directorio temporal, resolver al binario local compilado o alias de Scoop
		if strings.Contains(execPath, "go-build") || strings.Contains(strings.ToLower(execPath), "appdata\\local\\temp") {
			if _, err := os.Stat("gentle-skills-bridge.exe"); err == nil {
				if absLocal, err := filepath.Abs("gentle-skills-bridge.exe"); err == nil {
					execPath = absLocal
				} else {
					execPath = "gentle-skills-bridge"
				}
			} else {
				execPath = "gentle-skills-bridge"
			}
		}
		agents := GetInstalledAgents(home)
		for _, agent := range agents {
			if err := ConfigureAgentMCP(home, agent, execPath); err != nil {
				fmt.Fprintf(os.Stderr, "[warning] No se pudo configurar el MCP para el agente %s: %v\n", agent, err)
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "[warning] No se pudo obtener la carpeta home para configurar MCP: %v\n", err)
	}

	// Trigger registry refresh
	triggerGentleRegistryRefresh()
	return nil
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
