package skills

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontMatter holds parsed YAML frontmatter from SKILL.md.
type FrontMatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// ParseFrontMatter splits content into frontmatter and body.
// Returns (frontmatter, body, error).
// Content must start with "---\n". The function finds the closing "---\n".
func ParseFrontMatter(content string) (FrontMatter, string, error) {
	var fm FrontMatter

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return fm, content, fmt.Errorf("content does not start with frontmatter delimiter")
	}

	// Find the opening delimiter end (could be "---\n", "---\r\n", or just "---" at end)
	afterOpen := trimmed[3:]
	// Skip whitespace after opening ---
	if len(afterOpen) > 0 && (afterOpen[0] == '\n' || afterOpen[0] == '\r') {
		if afterOpen[0] == '\r' && len(afterOpen) > 1 && afterOpen[1] == '\n' {
			afterOpen = afterOpen[2:]
		} else {
			afterOpen = afterOpen[1:]
		}
	}

	// Find closing "---"
	closeIdx := strings.Index(afterOpen, "\n---")
	if closeIdx < 0 {
		// Maybe the closing --- is at the very end without trailing newline
		if strings.HasSuffix(afterOpen, "---") && len(afterOpen) > 3 {
			// Check character before --- is a newline
			yamlContent := afterOpen[:len(afterOpen)-3]
			if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
				return fm, "", fmt.Errorf("parse frontmatter yaml: %w", err)
			}
			return fm, "", nil
		}
		return fm, content, fmt.Errorf("missing closing frontmatter delimiter")
	}

	// Extract YAML between delimiters
	yamlContent := afterOpen[:closeIdx]

	// Body starts after the closing --- and any following newline
	bodyStart := closeIdx + 4 // skip \n---
	body := ""
	if bodyStart < len(afterOpen) {
		body = afterOpen[bodyStart:]
		// Skip leading newline in body
		if len(body) > 0 && body[0] == '\n' {
			body = body[1:]
		} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
			body = body[2:]
		}
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return fm, "", fmt.Errorf("parse frontmatter yaml: %w", err)
	}

	return fm, body, nil
}
