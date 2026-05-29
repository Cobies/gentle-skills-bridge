package bridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MCP JSON-RPC 2.0 message frames
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Capabilities structure for MCP Server
type serverCapabilities struct {
	Resources map[string]interface{} `json:"resources,omitempty"`
	Tools     map[string]interface{} `json:"tools,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

// Resource structures
type mcpResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	MimeType    string `json:"mimeType,omitempty"`
	Description string `json:"description,omitempty"`
}

type listResourcesResult struct {
	Resources []mcpResource `json:"resources"`
}

type resourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type readResourceResult struct {
	Contents []resourceContent `json:"contents"`
}

// Tool structures
type mcpToolInputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type mcpTool struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	InputSchema mcpToolInputSchema `json:"inputSchema"`
}

type listToolsResult struct {
	Tools []mcpTool `json:"tools"`
}

type toolContentText struct {
	Type string `json:"type"` // always "text"
	Text string `json:"text"`
}

type callToolResult struct {
	Content []toolContentText `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// StartMCPServer starts the stdio-based Model Context Protocol server.
func StartMCPServer(cfg *Config) error {
	return runMCPServer(cfg, os.Stdin, os.Stdout)
}

func runMCPServer(cfg *Config, stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendErrorResponse(stdout, nil, -32700, "Parse error", err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			sendErrorResponse(stdout, req.ID, -32600, "Invalid Request", "jsonrpc version must be '2.0'")
			continue
		}

		handleRequest(cfg, stdout, &req)
	}
	return nil
}

func sendResponse(stdout io.Writer, id interface{}, result interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to marshal JSON-RPC response: %v\n", err)
		return
	}
	// Output to writer must be exactly one single line followed by newline
	fmt.Fprintf(stdout, "%s\n", string(respBytes))
}

func sendErrorResponse(stdout io.Writer, id interface{}, code int, message string, data interface{}) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] failed to marshal JSON-RPC error response: %v\n", err)
		return
	}
	fmt.Fprintf(stdout, "%s\n", string(respBytes))
}

// scanSkills walks all sources and returns parsed SkillInfo mapping.
func scanSkills(cfg *Config) (map[string]*SkillInfo, map[string]string, error) {
	skills := make(map[string]*SkillInfo)
	paths := make(map[string]string)

	for _, sourceDir := range cfg.Sources {
		cleanSource := filepath.Clean(sourceDir)
		if _, err := os.Stat(cleanSource); os.IsNotExist(err) {
			continue
		}

		err := filepath.Walk(cleanSource, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if strings.HasPrefix(info.Name(), ".") && path != cleanSource {
					return filepath.SkipDir
				}
				return nil
			}

			if filepath.Ext(path) != ".md" {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			skill, err := ParseMarkdown(path, string(contentBytes))
			if err != nil {
				return nil // skip unparseable files
			}

			skills[skill.Name] = skill
			paths[skill.Name] = path
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "[warning] error walking source %s: %v\n", cleanSource, err)
		}
	}
	return skills, paths, nil
}

func handleRequest(cfg *Config, stdout io.Writer, req *jsonRPCRequest) {
	isNotification := req.ID == nil

	switch req.Method {
	case "initialize":
		result := initializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: serverCapabilities{
				Resources: map[string]interface{}{},
				Tools:     map[string]interface{}{},
			},
			ServerInfo: serverInfo{
				Name:    "gentle-skills-bridge-mcp",
				Version: "1.0.0",
			},
		}
		sendResponse(stdout, req.ID, result)

	case "notifications/initialized":
		// Handled silently per specification

	case "resources/list":
		skills, _, err := scanSkills(cfg)
		if err != nil {
			sendErrorResponse(stdout, req.ID, -32603, "Internal error", err.Error())
			return
		}

		resources := make([]mcpResource, 0, len(skills))
		for name, skill := range skills {
			resources = append(resources, mcpResource{
				URI:         fmt.Sprintf("skills://%s", name),
				Name:        name,
				MimeType:    "text/markdown",
				Description: skill.Description,
			})
		}

		// Sort resources by name for determinism
		sort.Slice(resources, func(i, j int) bool {
			return resources[i].Name < resources[j].Name
		})

		sendResponse(stdout, req.ID, listResourcesResult{Resources: resources})

	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendErrorResponse(stdout, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		if !strings.HasPrefix(params.URI, "skills://") {
			sendErrorResponse(stdout, req.ID, -32602, "Invalid params", "URI must start with skills://")
			return
		}

		skillName := strings.TrimPrefix(params.URI, "skills://")
		skills, _, err := scanSkills(cfg)
		if err != nil {
			sendErrorResponse(stdout, req.ID, -32603, "Internal error", err.Error())
			return
		}

		skill, ok := skills[skillName]
		if !ok {
			sendErrorResponse(stdout, req.ID, -32602, "Resource not found", fmt.Sprintf("Skill %q not found", skillName))
			return
		}

		sendResponse(stdout, req.ID, readResourceResult{
			Contents: []resourceContent{
				{
					URI:      params.URI,
					MimeType: "text/markdown",
					Text:     skill.Content,
				},
			},
		})

	case "tools/list":
		tools := []mcpTool{
			{
				Name:        "search_skills",
				Description: "Search for available development skills by keywords, matches triggers, or descriptions.",
				InputSchema: mcpToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query (keyword or phrase)",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "get_skill",
				Description: "Retrieve the full instruction Markdown file for a specific development skill.",
				InputSchema: mcpToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "The exact name/slug of the skill (e.g. 'alphafold-database-fetch-and-analyze')",
						},
					},
					Required: []string{"name"},
				},
			},
			{
				Name:        "get_skills_configuration",
				Description: "Retrieve the list of configured source folders where skills are scanned.",
				InputSchema: mcpToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{},
				},
			},
		}
		sendResponse(stdout, req.ID, listToolsResult{Tools: tools})

	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendErrorResponse(stdout, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		skills, paths, err := scanSkills(cfg)
		if err != nil {
			sendErrorResponse(stdout, req.ID, -32603, "Internal error", err.Error())
			return
		}

		if params.Name == "search_skills" {
			var args struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal(params.Arguments, &args); err != nil {
				sendErrorResponse(stdout, req.ID, -32602, "Invalid arguments", err.Error())
				return
			}

			queryLower := strings.ToLower(args.Query)
			var results []string
			
			// Extract and sort skill names for deterministic search results
			var skillNames []string
			for name := range skills {
				skillNames = append(skillNames, name)
			}
			sort.Strings(skillNames)

			for _, name := range skillNames {
				skill := skills[name]
				if strings.Contains(strings.ToLower(skill.Name), queryLower) ||
					strings.Contains(strings.ToLower(skill.Description), queryLower) ||
					strings.Contains(strings.ToLower(skill.RawBody), queryLower) {
					results = append(results, fmt.Sprintf("- **%s**:\n  %s\n", skill.Name, skill.Description))
				}
			}

			var responseText string
			if len(results) == 0 {
				responseText = "No skills found matching the query: " + args.Query
			} else {
				responseText = "Found the following matching skills:\n\n" + strings.Join(results, "\n")
			}

			sendResponse(stdout, req.ID, callToolResult{
				Content: []toolContentText{
					{
						Type: "text",
						Text: responseText,
					},
				},
			})

		} else if params.Name == "get_skill" {
			var args struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(params.Arguments, &args); err != nil {
				sendErrorResponse(stdout, req.ID, -32602, "Invalid arguments", err.Error())
				return
			}

			skill, ok := skills[args.Name]
			if !ok {
				sendResponse(stdout, req.ID, callToolResult{
					Content: []toolContentText{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: Skill %q not found.", args.Name),
						},
					},
					IsError: true,
				})
				return
			}

			path := paths[args.Name]
			responseText := fmt.Sprintf("Path: %s\n\n%s", path, skill.Content)

			sendResponse(stdout, req.ID, callToolResult{
				Content: []toolContentText{
					{
						Type: "text",
						Text: responseText,
					},
				},
			})

		} else if params.Name == "get_skills_configuration" {
			var sourcesText []string
			for _, src := range cfg.Sources {
				sourcesText = append(sourcesText, fmt.Sprintf("- %s", src))
			}
			responseText := "Configured source directories:\n\n" + strings.Join(sourcesText, "\n")
			sendResponse(stdout, req.ID, callToolResult{
				Content: []toolContentText{
					{
						Type: "text",
						Text: responseText,
					},
				},
			})

		} else {
			sendErrorResponse(stdout, req.ID, -32601, "Method not found", fmt.Sprintf("Unknown tool %s", params.Name))
		}

	default:
		if !isNotification {
			sendErrorResponse(stdout, req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method %s", req.Method))
		}
	}
}
