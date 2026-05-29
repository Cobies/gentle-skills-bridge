package bridge

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testJSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

func TestMCPInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"initialize","id":1,"params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	stdin := bytes.NewBufferString(input)
	stdout := &bytes.Buffer{}

	cfg := &Config{
		Sources: []string{},
	}

	err := runMCPServer(cfg, stdin, stdout)
	if err != nil {
		t.Fatalf("runMCPServer failed: %v", err)
	}

	var resp testJSONRPCResponse
	err = json.Unmarshal(stdout.Bytes(), &resp)
	if err != nil {
		t.Fatalf("failed to parse response: %v, raw output: %s", err, stdout.String())
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc version '2.0', got %q", resp.JSONRPC)
	}

	if resp.ID.(float64) != 1 {
		t.Errorf("expected ID 1, got %v", resp.ID)
	}

	var result map[string]interface{}
	err = json.Unmarshal(resp.Result, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion '2024-11-05', got %v", result["protocolVersion"])
	}
}

func TestMCPResourcesAndTools(t *testing.T) {
	// Create a temp directory for mock skills
	tempDir, err := os.MkdirTemp("", "gentle-mcp-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write a mock skill
	mockSkillContent := "---\nname: test-skill\ndescription: \"Trigger: test-skill, custom test triggers\"\n---\n# Test Title\nThis is a mock skill body."
	mockSkillPath := filepath.Join(tempDir, "test-skill.md")
	if err := os.WriteFile(mockSkillPath, []byte(mockSkillContent), 0644); err != nil {
		t.Fatalf("failed to write mock skill: %v", err)
	}

	cfg := &Config{
		Sources: []string{tempDir},
	}

	// 1. Test resources/list
	inputList := `{"jsonrpc":"2.0","method":"resources/list","id":2}` + "\n"
	stdin := bytes.NewBufferString(inputList)
	stdout := &bytes.Buffer{}

	err = runMCPServer(cfg, stdin, stdout)
	if err != nil {
		t.Fatalf("runMCPServer failed: %v", err)
	}

	var resp testJSONRPCResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal list resources response: %v, raw: %s", err, stdout.String())
	}

	var listResult listResourcesResult
	if err := json.Unmarshal(resp.Result, &listResult); err != nil {
		t.Fatalf("failed to parse list result: %v", err)
	}

	if len(listResult.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(listResult.Resources))
	}

	res := listResult.Resources[0]
	if res.Name != "test-skill" {
		t.Errorf("expected name 'test-skill', got %q", res.Name)
	}
	if res.URI != "skills://test-skill" {
		t.Errorf("expected URI 'skills://test-skill', got %q", res.URI)
	}

	// 2. Test tools/list
	inputTools := `{"jsonrpc":"2.0","method":"tools/list","id":3}` + "\n"
	stdin = bytes.NewBufferString(inputTools)
	stdout.Reset()

	err = runMCPServer(cfg, stdin, stdout)
	if err != nil {
		t.Fatalf("runMCPServer failed: %v", err)
	}

	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal list tools response: %v, raw: %s", err, stdout.String())
	}

	var toolsResult listToolsResult
	if err := json.Unmarshal(resp.Result, &toolsResult); err != nil {
		t.Fatalf("failed to parse tools result: %v", err)
	}

	if len(toolsResult.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(toolsResult.Tools))
	}

	// 3. Test tools/call search_skills
	inputCall := `{"jsonrpc":"2.0","method":"tools/call","id":4,"params":{"name":"search_skills","arguments":{"query":"custom test"}}}` + "\n"
	stdin = bytes.NewBufferString(inputCall)
	stdout.Reset()

	err = runMCPServer(cfg, stdin, stdout)
	if err != nil {
		t.Fatalf("runMCPServer failed: %v", err)
	}

	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal call tool response: %v, raw: %s", err, stdout.String())
	}

	var callRes callToolResult
	if err := json.Unmarshal(resp.Result, &callRes); err != nil {
		t.Fatalf("failed to parse call tool result: %v", err)
	}

	if callRes.IsError {
		t.Errorf("expected callToolResult not to be an error")
	}

	if len(callRes.Content) != 1 {
		t.Fatalf("expected 1 text content, got %d", len(callRes.Content))
	}

	text := callRes.Content[0].Text
	if !strings.Contains(text, "test-skill") {
		t.Errorf("expected search output to mention 'test-skill', got %q", text)
	}
}
