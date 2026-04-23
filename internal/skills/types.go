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
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSpace(content)
	// Strip UTF-8 BOM if present (some editors prepend it).
	content = strings.TrimPrefix(content, "\xEF\xBB\xBF")
	if !strings.HasPrefix(content, frontmatterDelimiter) {
		return fm, "", errors.New("skill file must start with YAML frontmatter (---)")
	}
	rest := content[len(frontmatterDelimiter):]
	// Skip optional newline after opening delimiter
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	// Find closing delimiter line. Must be the first non-indented line that is exactly "---"
	// (possibly with trailing whitespace). Indented "---" inside YAML block scalars is ignored.
	lines := strings.Split(rest, "\n")
	closeLine := -1
	for i, line := range lines {
		// Require "---" at line start (not indented) to avoid matching inside YAML values.
		if strings.HasPrefix(line, "---") && strings.TrimSpace(line) == "---" {
			closeLine = i
			break
		}
	}
	if closeLine < 0 {
		return fm, "", errors.New("frontmatter closing delimiter not found")
	}
	fmContent := strings.Join(lines[:closeLine], "\n")
	body := strings.TrimRight(strings.Join(lines[closeLine+1:], "\n"), "\n")
	if err := yaml.Unmarshal([]byte(fmContent), &fm); err != nil {
		return fm, "", fmt.Errorf("parse YAML frontmatter: %w", err)
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
			line = strings.TrimLeft(stripped, " ")
			if line == "" {
				continue
			}
		}
		return line
	}
	return ""
}
