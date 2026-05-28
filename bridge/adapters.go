package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GentleState represents the schema of ~/.gentle-ai/state.json
type GentleState struct {
	InstalledAgents []string `json:"installed_agents"`
}

// ResolveAgentSkillsDirs resolves the correct physical skill folders for installed agents.
func ResolveAgentSkillsDirs(homeDir string, agents []string) []string {
	var paths []string
	seen := make(map[string]bool)

	addPath := func(p string) {
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}

	for _, agent := range agents {
		switch agent {
		case "claude-code":
			addPath(filepath.Join(homeDir, ".claude", "skills"))
		case "opencode":
			addPath(filepath.Join(homeDir, ".config", "opencode", "skills"))
		case "antigravity":
			addPath(filepath.Join(homeDir, ".gemini", "config", "skills"))
			addPath(filepath.Join(homeDir, ".gemini", "skills"))
		case "codex", "vscode-copilot":
			addPath(filepath.Join(homeDir, ".codex", "skills"))
		case "pi":
			addPath(filepath.Join(homeDir, ".pi", "skills"))
			addPath(filepath.Join(homeDir, ".gemini", "skills"))
		}
	}

	return paths
}

// DiscoverTargets reads the gentle-ai state file and returns active agent skill folders.
func DiscoverTargets() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}
	return discoverTargets(home)
}

func discoverTargets(home string) ([]string, error) {
	statePath := filepath.Join(home, ".gentle-ai", "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		// Return empty list if state.json does not exist
		return nil, nil
	}

	file, err := os.Open(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open gentle-ai state file: %w", err)
	}
	defer file.Close()

	var state GentleState
	dec := json.NewDecoder(file)
	if err := dec.Decode(&state); err != nil {
		return nil, fmt.Errorf("failed to decode gentle-ai state: %w", err)
	}

	return ResolveAgentSkillsDirs(home, state.InstalledAgents), nil
}
