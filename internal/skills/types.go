package skills

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillFile is the canonical filename for new-format skills.
const SkillFile = "SKILL.md"

// frontmatterDelimiter is the YAML frontmatter boundary marker.
const frontmatterDelimiter = "---"

// yamlSkill is the on-disk representation for .yaml skill files (legacy compat).
type yamlSkill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

// Skill represents a loaded skill with full metadata.
type Skill struct {
	Name        string            // from frontmatter
	Description string            // from frontmatter
	Category    string            // parent directory name (empty if no category)
	Content     string            // markdown body (after frontmatter)
	Source      string            // "global" or "workspace"
	Dir         string            // absolute path to skill directory
	Metadata    map[string]string // optional frontmatter metadata
}

// frontMatter holds parsed YAML frontmatter from SKILL.md.
type frontMatter struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// parseFrontMatter splits content into frontmatter and body.
// Content must start with frontmatter delimiter ("---") after trimming whitespace (CRLF normalized to LF).
func parseFrontMatter(content string) (frontMatter, string, error) {
	var fm frontMatter
	content = strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return fm, "", errors.New("content does not start with frontmatter delimiter")
	}

	// Skip opening delimiter and its newline.
	rest := content[len(frontmatterDelimiter):]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	// Find closing delimiter on its own line.
	lines := strings.Split(rest, "\n")
	closeIdx := -1
	for i, line := range lines {
		if line == frontmatterDelimiter {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return fm, "", errors.New("missing closing frontmatter delimiter")
	}

	yamlContent := strings.Join(lines[:closeIdx], "\n")
	body := strings.Join(lines[closeIdx+1:], "\n")

	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return fm, "", fmt.Errorf("parse frontmatter yaml: %w", err)
	}
	return fm, body, nil
}

// extractDescription returns the first non-empty line of a markdown file,
// with an optional leading markdown heading (one or more '#' followed by space) stripped.
func extractDescription(content string) string {
	for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, frontmatterDelimiter) {
			continue
		}
		if stripped := strings.TrimLeft(line, "#"); len(stripped) < len(line) && strings.HasPrefix(stripped, " ") {
			line = stripped[1:]
		}
		return line
	}
	return ""
}
