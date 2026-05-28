package bridge

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// SkillInfo contains parsed frontmatter and clean content of a skill.
type SkillInfo struct {
	Name        string
	Description string
	Content     string // Full content with frontmatter (normalized)
	RawBody     string // Content without frontmatter
	Normalized  bool   // Whether generated content differs from the original metadata contract.
}

// Slugify normalizes string to be filesystem-friendly for folder names.
func Slugify(s string) string {
	s = strings.ToLower(s)
	// Remove extension if any
	s = strings.TrimSuffix(s, filepath.Ext(s))
	// Replace spaces/special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	s = reg.ReplaceAllString(s, "-")
	// Trim hyphens
	s = strings.Trim(s, "-")
	return s
}

// ParseMarkdown parses a markdown file, extracts/generates frontmatter and body.
func ParseMarkdown(filename string, content string) (*SkillInfo, error) {
	lines := strings.Split(content, "\n")
	baseName := filepath.Base(filename)
	slugName := Slugify(baseName)

	hasFrontmatter := false
	frontmatterLines := []string{}
	bodyLines := []string{}

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		hasFrontmatter = true
		i := 1
		for i < len(lines) {
			line := lines[i]
			if strings.TrimSpace(line) == "---" {
				i++
				break
			}
			frontmatterLines = append(frontmatterLines, line)
			i++
		}
		for i < len(lines) {
			bodyLines = append(bodyLines, lines[i])
			i++
		}
	} else {
		bodyLines = lines
	}

	name := slugName
	description := ""
	hasName := false
	hasDescription := false

	if hasFrontmatter {
		for _, line := range frontmatterLines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.ToLower(strings.TrimSpace(parts[0]))
				val := strings.TrimSpace(parts[1])
				// Strip quotes
				val = strings.Trim(val, `"'`)
				if key == "name" {
					name = Slugify(val)
					hasName = name != ""
				} else if key == "description" {
					description = val
					hasDescription = description != ""
				}
			}
		}
	}

	metadataWasComplete := hasFrontmatter && hasName && hasDescription

	// If description is empty, infer from first header or filename
	if description == "" {
		firstHeader := ""
		for _, line := range bodyLines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "# ") {
				firstHeader = strings.TrimPrefix(trimmed, "# ")
				break
			}
		}
		if firstHeader != "" {
			description = fmt.Sprintf("Trigger: %s, %s", name, strings.ToLower(firstHeader))
		} else {
			description = fmt.Sprintf("Trigger: %s", name)
		}
	}

	// Format description correctly with Trigger prefix if not present
	if !strings.HasPrefix(strings.ToLower(description), "trigger:") {
		description = "Trigger: " + description
	}

	rawBody := strings.Join(bodyLines, "\n")

	// Build standardized content with frontmatter
	var standardizedContent strings.Builder
	standardizedContent.WriteString("---\n")
	standardizedContent.WriteString(fmt.Sprintf("name: %s\n", name))
	standardizedContent.WriteString(fmt.Sprintf("description: %q\n", description))
	standardizedContent.WriteString("---\n\n")
	standardizedContent.WriteString(strings.TrimSpace(rawBody))
	standardizedContent.WriteString("\n")

	return &SkillInfo{
		Name:        name,
		Description: description,
		Content:     standardizedContent.String(),
		RawBody:     rawBody,
		Normalized:  !metadataWasComplete,
	}, nil
}

// ParseTriggers parses triggers from the description field.
func ParseTriggers(description string) []string {
	desc := strings.TrimPrefix(description, "Trigger:")
	desc = strings.TrimPrefix(desc, "trigger:")
	parts := strings.Split(desc, ",")
	var triggers []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			triggers = append(triggers, trimmed)
		}
	}
	return triggers
}
