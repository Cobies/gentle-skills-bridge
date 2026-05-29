package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
			addPath(filepath.Join(homeDir, ".gemini", "antigravity", "skills"))
			addPath(filepath.Join(homeDir, ".gemini", "antigravity-cli", "skills"))
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

// GetInstalledAgents returns a list of installed agents from state.json or falls back to checking config dirs.
func GetInstalledAgents(home string) []string {
	var agents []string
	statePath := filepath.Join(home, ".gentle-ai", "state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var state GentleState
		if err := json.Unmarshal(data, &state); err == nil && len(state.InstalledAgents) > 0 {
			return state.InstalledAgents
		}
	}

	// Fallback detection based on existing directories
	allPossibleAgents := []string{"claude-code", "antigravity", "opencode", "vscode-copilot", "pi", "codex"}
	for _, agent := range allPossibleAgents {
		if isAgentInstalledFallback(home, agent) {
			agents = append(agents, agent)
		}
	}
	return agents
}

func isAgentInstalledFallback(home string, agent string) bool {
	switch agent {
	case "claude-code":
		return pathExists(filepath.Join(home, ".claude")) || pathExists(filepath.Join(home, ".claude.json"))
	case "antigravity":
		return pathExists(filepath.Join(home, ".gemini"))
	case "opencode":
		return pathExists(filepath.Join(home, ".config", "opencode"))
	case "vscode-copilot":
		var path string
		if os.Getenv("APPDATA") != "" {
			path = filepath.Join(os.Getenv("APPDATA"), "Code", "User")
		} else {
			path = filepath.Join(home, "Library", "Application Support", "Code", "User")
			if _, err := os.Stat(path); os.IsNotExist(err) {
				path = filepath.Join(home, ".config", "Code", "User")
			}
		}
		return pathExists(path)
	case "pi":
		return pathExists(filepath.Join(home, ".pi"))
	case "codex":
		return pathExists(filepath.Join(home, ".codex"))
	}
	return false
}

// ConfigureAgentMCP configures the MCP server for a specific agent.
func ConfigureAgentMCP(home string, agent string, execPath string) error {
	switch agent {
	case "claude-code":
		path := filepath.Join(home, ".claude.json")
		value := map[string]interface{}{
			"command": execPath,
			"args":    []string{"mcp"},
		}
		err := updateJSONConfig(path, []string{"mcpServers", "gentle-skills-bridge"}, value)
		if err == nil {
			fmt.Printf("[bootstrap] MCP registrado para Claude Code en: %s\n", path)
		}
		return err

	case "antigravity":
		// 1. Antigravity CLI: ~/.gemini/antigravity-cli/mcp_config.json
		path1 := filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json")
		value1 := map[string]interface{}{
			"command": execPath,
			"args":    []string{"mcp"},
		}
		err1 := updateJSONConfig(path1, []string{"mcpServers", "gentle-skills-bridge"}, value1)
		if err1 == nil {
			fmt.Printf("[bootstrap] MCP registrado para Antigravity CLI en: %s\n", path1)
		}

		// 2. Gemini CLI (Legacy): ~/.gemini/settings.json
		path2 := filepath.Join(home, ".gemini", "settings.json")
		err2 := updateJSONConfig(path2, []string{"mcpServers", "gentle-skills-bridge"}, value1)
		if err2 == nil {
			fmt.Printf("[bootstrap] MCP registrado para Gemini CLI (Legacy) en: %s\n", path2)
		}

		if err1 != nil {
			return err1
		}
		return err2

	case "opencode":
		path := filepath.Join(home, ".config", "opencode", "opencode.json")
		value := map[string]interface{}{
			"type":    "local",
			"command": []string{execPath, "mcp"},
			"enabled": true,
		}
		err := updateJSONConfig(path, []string{"mcp", "gentle-skills-bridge"}, value)
		if err == nil {
			fmt.Printf("[bootstrap] MCP registrado para OpenCode en: %s\n", path)
		}
		return err

	case "vscode-copilot":
		var path string
		if os.Getenv("APPDATA") != "" {
			path = filepath.Join(os.Getenv("APPDATA"), "Code", "User", "mcp.json")
		} else {
			// fallback/Unix
			path = filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json")
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				path = filepath.Join(home, ".config", "Code", "User", "mcp.json")
			}
		}
		value := map[string]interface{}{
			"command": execPath,
			"args":    []string{"mcp"},
		}
		err := updateJSONConfig(path, []string{"mcpServers", "gentle-skills-bridge"}, value)
		if err == nil {
			fmt.Printf("[bootstrap] MCP registrado para VS Code (GitHub Copilot) en: %s\n", path)
		}
		return err

	case "pi":
		path := filepath.Join(home, ".pi", "agent", "mcp.json")
		value := map[string]interface{}{
			"command": execPath,
			"args":    []string{"mcp"},
		}
		err := updateJSONConfig(path, []string{"mcpServers", "gentle-skills-bridge"}, value)
		if err == nil {
			fmt.Printf("[bootstrap] MCP registrado para Pi en: %s\n", path)
		}
		return err

	case "codex":
		path := filepath.Join(home, ".codex", "config.toml")
		err := updateTOMLConfig(path, execPath)
		if err == nil {
			fmt.Printf("[bootstrap] MCP registrado para Codex en: %s\n", path)
		}
		return err
	}
	return nil
}

func updateJSONConfig(filePath string, keyPath []string, value interface{}) error {
	var config map[string]interface{}
	data, err := os.ReadFile(filePath)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			config = make(map[string]interface{})
		}
	} else if os.IsNotExist(err) {
		config = make(map[string]interface{})
	} else {
		return err
	}

	if config == nil {
		config = make(map[string]interface{})
	}

	// Navigate to the target map
	curr := config
	for i := 0; i < len(keyPath)-1; i++ {
		k := keyPath[i]
		sub, ok := curr[k].(map[string]interface{})
		if !ok {
			sub = make(map[string]interface{})
			curr[k] = sub
		}
		curr = sub
	}

	lastKey := keyPath[len(keyPath)-1]
	curr[lastKey] = value

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	newData = append(newData, '\n')

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, newData, 0644)
}

func updateTOMLConfig(filePath string, execPath string) error {
	entry := fmt.Sprintf("\n[mcp_servers.gentle-skills-bridge]\ncommand = %q\nargs = [%q]\n", execPath, "mcp")
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(filePath, []byte(strings.TrimSpace(entry)+"\n"), 0644)
	} else if err != nil {
		return err
	}

	content := string(data)
	if strings.Contains(content, "[mcp_servers.gentle-skills-bridge]") {
		lines := strings.Split(content, "\n")
		var newLines []string
		inSection := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "[mcp_servers.gentle-skills-bridge]" {
				inSection = true
				continue
			}
			if inSection {
				if strings.HasPrefix(trimmed, "[") {
					inSection = false
				} else {
					continue
				}
			}
			newLines = append(newLines, line)
		}

		content = strings.Join(newLines, "\n")
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry
		return os.WriteFile(filePath, []byte(content), 0644)
	} else {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += entry
		return os.WriteFile(filePath, []byte(content), 0644)
	}
}
