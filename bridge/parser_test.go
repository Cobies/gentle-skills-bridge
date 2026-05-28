package bridge

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"React-Testing.md", "react-testing"},
		{"Docker Rules!!!.md", "docker-rules"},
		{"  Clean Architecture  ", "clean-architecture"},
		{"nestjs_best_practices.MD", "nestjs-best-practices"},
	}

	for _, tt := range tests {
		actual := Slugify(tt.input)
		if actual != tt.expected {
			t.Errorf("Slugify(%q) = %q; want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestParseMarkdown_WithFrontmatter(t *testing.T) {
	input := `---
name: custom-react
description: "Trigger: component test, react render"
---
# React Guidelines
Content goes here.`

	info, err := ParseMarkdown("React-Testing.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if info.Name != "custom-react" {
		t.Errorf("Expected name 'custom-react', got %q", info.Name)
	}

	if info.Description != "Trigger: component test, react render" {
		t.Errorf("Expected description 'Trigger: component test, react render', got %q", info.Description)
	}

	if !strings.Contains(info.Content, "name: custom-react") {
		t.Errorf("Expected Content to contain frontmatter name, got:\n%s", info.Content)
	}

	if info.Normalized {
		t.Errorf("Expected complete frontmatter to not require normalization")
	}
}

func TestParseMarkdown_WithoutFrontmatter(t *testing.T) {
	input := `# NestJS Rules
Always structure code in modules.`

	info, err := ParseMarkdown("NestJS-Guide.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if info.Name != "nestjs-guide" {
		t.Errorf("Expected inferred name 'nestjs-guide', got %q", info.Name)
	}

	expectedDesc := "Trigger: nestjs-guide, nestjs rules"
	if info.Description != expectedDesc {
		t.Errorf("Expected inferred description %q, got %q", expectedDesc, info.Description)
	}

	if !info.Normalized {
		t.Errorf("Expected missing frontmatter to require normalization")
	}
}

func TestParseMarkdown_WithIncompleteFrontmatterRequiresNormalization(t *testing.T) {
	input := `---
name: custom-react
---
# React Guidelines
Content goes here.`

	info, err := ParseMarkdown("React-Testing.md", input)
	if err != nil {
		t.Fatalf("ParseMarkdown failed: %v", err)
	}

	if !info.Normalized {
		t.Errorf("Expected incomplete frontmatter to require normalization")
	}

	if !strings.Contains(info.Content, `description: "Trigger: custom-react, react guidelines"`) {
		t.Errorf("Expected Content to contain generated description, got:\n%s", info.Content)
	}
}
