package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolveAgentSkillsDirs(t *testing.T) {
	home := filepath.Clean("/home/tester")

	tests := []struct {
		name   string
		agents []string
		want   []string
	}{
		{
			name:   "known agents",
			agents: []string{"claude-code", "opencode"},
			want: []string{
				filepath.Join(home, ".claude", "skills"),
				filepath.Join(home, ".config", "opencode", "skills"),
			},
		},
		{
			name:   "deduplicates shared paths",
			agents: []string{"antigravity", "pi"},
			want: []string{
				filepath.Join(home, ".gemini", "config", "skills"),
				filepath.Join(home, ".gemini", "skills"),
				filepath.Join(home, ".gemini", "antigravity", "skills"),
				filepath.Join(home, ".gemini", "antigravity-cli", "skills"),
				filepath.Join(home, ".pi", "skills"),
			},
		},
		{
			name:   "ignores unknown agents",
			agents: []string{"unknown", "opencode"},
			want: []string{
				filepath.Join(home, ".config", "opencode", "skills"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAgentSkillsDirs(home, tt.agents)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ResolveAgentSkillsDirs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDiscoverTargetsFromHome(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".gentle-ai")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(stateDir, "state.json"), `{"installed_agents":["opencode","claude-code"]}`)

	got, err := discoverTargets(home)
	if err != nil {
		t.Fatalf("discoverTargets() error = %v", err)
	}

	want := []string{
		filepath.Join(home, ".config", "opencode", "skills"),
		filepath.Join(home, ".claude", "skills"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("discoverTargets() = %#v, want %#v", got, want)
	}
}

func TestDiscoverTargetsMissingStateReturnsEmpty(t *testing.T) {
	got, err := discoverTargets(t.TempDir())
	if err != nil {
		t.Fatalf("discoverTargets() error = %v", err)
	}
	if got != nil {
		t.Fatalf("discoverTargets() = %#v, want nil", got)
	}
}

func TestDiscoverTargetsInvalidJSON(t *testing.T) {
	home := t.TempDir()
	stateDir := filepath.Join(home, ".gentle-ai")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(stateDir, "state.json"), `{invalid`)

	_, err := discoverTargets(home)
	if err == nil {
		t.Fatal("discoverTargets() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "failed to decode gentle-ai state") {
		t.Fatalf("discoverTargets() error = %q, want decode error", err.Error())
	}

	var syntaxErr *os.PathError
	if errors.As(err, &syntaxErr) {
		t.Fatalf("discoverTargets() error unexpectedly wraps PathError: %v", err)
	}
}

func TestConfigureAgentMCP(t *testing.T) {
	home := t.TempDir()
	execPath := filepath.Join(home, "my-bin")

	// 1. Test Claude Code JSON configuration (creation & merging)
	claudePath := filepath.Join(home, ".claude.json")
	writeTestFile(t, claudePath, `{"other_key": "some_value", "mcpServers": {"old-server": {"command": "node"}}}`)

	if err := ConfigureAgentMCP(home, "claude-code", execPath); err != nil {
		t.Fatalf("ConfigureAgentMCP(claude-code) failed: %v", err)
	}

	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("ReadFile(.claude.json) failed: %v", err)
	}

	var claudeConfig map[string]interface{}
	if err := json.Unmarshal(data, &claudeConfig); err != nil {
		t.Fatalf("Failed to parse .claude.json: %v", err)
	}

	if claudeConfig["other_key"] != "some_value" {
		t.Fatalf("expected other_key to be preserved, got %v", claudeConfig["other_key"])
	}

	mcpServers, ok := claudeConfig["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers is missing or not a map")
	}

	if _, exists := mcpServers["old-server"]; !exists {
		t.Fatal("old-server was deleted during merge")
	}

	bridgeServer, ok := mcpServers["gentle-skills-bridge"].(map[string]interface{})
	if !ok {
		t.Fatal("gentle-skills-bridge server is missing from config")
	}

	if bridgeServer["command"] != execPath {
		t.Fatalf("expected command %q, got %q", execPath, bridgeServer["command"])
	}

	// 2. Test OpenCode JSON configuration
	opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := ConfigureAgentMCP(home, "opencode", execPath); err != nil {
		t.Fatalf("ConfigureAgentMCP(opencode) failed: %v", err)
	}

	data, err = os.ReadFile(opencodePath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) failed: %v", err)
	}

	var opencodeConfig map[string]interface{}
	if err := json.Unmarshal(data, &opencodeConfig); err != nil {
		t.Fatalf("Failed to parse opencode.json: %v", err)
	}

	mcpBlock, ok := opencodeConfig["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp block is missing or not a map in opencode.json")
	}

	bridgeMCP, ok := mcpBlock["gentle-skills-bridge"].(map[string]interface{})
	if !ok {
		t.Fatal("gentle-skills-bridge is missing under mcp in opencode.json")
	}

	if bridgeMCP["type"] != "local" {
		t.Fatalf("expected type local, got %v", bridgeMCP["type"])
	}

	// 3. Test Codex TOML configuration
	codexPath := filepath.Join(home, ".codex", "config.toml")
	writeTestFile(t, codexPath, "[general]\nkey = \"value\"\n\n[mcp_servers.old]\ncommand = \"node\"\n")

	if err := ConfigureAgentMCP(home, "codex", execPath); err != nil {
		t.Fatalf("ConfigureAgentMCP(codex) failed: %v", err)
	}

	tomlData, err := os.ReadFile(codexPath)
	if err != nil {
		t.Fatalf("ReadFile(config.toml) failed: %v", err)
	}

	tomlStr := string(tomlData)
	if !strings.Contains(tomlStr, "[mcp_servers.gentle-skills-bridge]") {
		t.Fatal("config.toml is missing the [mcp_servers.gentle-skills-bridge] section")
	}
	if !strings.Contains(tomlStr, "[mcp_servers.old]") {
		t.Fatal("config.toml deleted the pre-existing [mcp_servers.old] section")
	}
	expectedCommand := fmt.Sprintf("command = %q", execPath)
	if !strings.Contains(tomlStr, expectedCommand) {
		t.Fatalf("config.toml doesn't contain the correct command line %q in:\n%s", expectedCommand, tomlStr)
	}
}

func TestWriteAntigravityToolSchemas(t *testing.T) {
	home := t.TempDir()
	if err := WriteAntigravityToolSchemas(home); err != nil {
		t.Fatalf("WriteAntigravityToolSchemas() failed: %v", err)
	}

	searchSkillsPath := filepath.Join(home, ".gemini", "antigravity-cli", "mcp", "gentle-skills-bridge", "search_skills.json")
	if _, err := os.Stat(searchSkillsPath); os.IsNotExist(err) {
		t.Fatal("search_skills.json schema file was not written")
	}

	getSkillPath := filepath.Join(home, ".gemini", "antigravity-cli", "mcp", "gentle-skills-bridge", "get_skill.json")
	if _, err := os.Stat(getSkillPath); os.IsNotExist(err) {
		t.Fatal("get_skill.json schema file was not written")
	}
}
