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
// After trimming whitespace and normalizing CRLF, content must start with "---".
// The function finds the closing "---" delimiter. Handles both LF and CRLF line endings.
func ParseFrontMatter(content string) (FrontMatter, string, error) {
	var fm FrontMatter

	// Normalize CRLF to LF for consistent parsing.
	// Only allocate a copy when CRLFs are actually present.
	var trimmed string
	if strings.Contains(content, "\r\n") {
		trimmed = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	} else {
		trimmed = strings.TrimSpace(content)
	}
	if !strings.HasPrefix(trimmed, "---") {
		return fm, "", fmt.Errorf("content does not start with frontmatter delimiter")
	}

	// Skip the opening "---" and the following newline.
	afterOpen := trimmed[3:]
	if len(afterOpen) > 0 && afterOpen[0] == '\n' {
		afterOpen = afterOpen[1:]
	}

	// Find closing "---" preceded by a newline.
	closeIdx := strings.Index(afterOpen, "\n---\n")
	if closeIdx < 0 {
		// Check for empty YAML: closing "---" immediately after opening.
		if strings.HasPrefix(afterOpen, "---\n") {
			closeIdx = 0
		} else if afterOpen == "---" {
			// Closing "---" with no trailing newline (e.g., "---\n---" after TrimSpace).
			closeIdx = 0
		} else if strings.HasSuffix(afterOpen, "\n---") && len(afterOpen) >= 4 {
			// Closing "---" at end of string (no trailing newline after body).
			closeIdx = len(afterOpen) - 4
		}
	}
	if closeIdx < 0 {
		return fm, "", fmt.Errorf("missing closing frontmatter delimiter")
	}

	// Extract YAML between delimiters.
	yamlContent := afterOpen[:closeIdx]

	// Body starts after the closing --- and following newline.
	bodyStart := closeIdx + 5 // skip \n---\n
	// For empty YAML case, the closing --- is right at start without leading \n.
	if closeIdx == 0 && strings.HasPrefix(afterOpen, "---\n") {
		bodyStart = 4 // skip ---\n
	}
	body := ""
	if bodyStart < len(afterOpen) {
		body = afterOpen[bodyStart:]
	}

	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return fm, "", fmt.Errorf("parse frontmatter yaml: %w", err)
	}

	return fm, body, nil
}
