package skills

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a loaded skill definition.
type Skill struct {
	Name        string
	Description string
	Content     string // full markdown content
	Source      string // "global" or "workspace"
}

// yamlSkill is the on-disk representation for .yaml skill files.
type yamlSkill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

// sourceLabel maps a directory index to a human-readable source label.
// dirIndex 0 → "global", everything else → "workspace".
func sourceLabel(dirIndex int) string {
	if dirIndex == 0 {
		return "global"
	}
	return "workspace"
}

// LoadSkills loads skills from multiple directories, merging by name.
// Later dirs override earlier ones (workspace overrides global).
// If enabledFilter is non-empty, only skills with matching names are returned.
func LoadSkills(ctx context.Context, dirs []string, enabledFilter []string) ([]Skill, error) {
	merged := make(map[string]Skill)

	for i, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read skills dir %s: %w", dir, err)
		}

		src := sourceLabel(i)

		for _, ent := range ents {
			if ent.IsDir() {
				continue
			}

			name := ""
			var sk Skill

			switch {
			case strings.HasSuffix(ent.Name(), ".md"):
				name = strings.TrimSuffix(ent.Name(), ".md")
				data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
				if err != nil {
					continue
				}
				content := string(data)
				sk = Skill{
					Name:    name,
					Content: content,
					Source:  src,
				}
				// Extract description from first non-empty line,
				// stripping leading "# " if present.
				sk.Description = extractDescription(content)

			case strings.HasSuffix(ent.Name(), ".yaml") || strings.HasSuffix(ent.Name(), ".yml"):
				ext := filepath.Ext(ent.Name())
				name = strings.TrimSuffix(ent.Name(), ext)
				data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
				if err != nil {
					continue
				}
				var ys yamlSkill
				if err := yaml.Unmarshal(data, &ys); err != nil {
					continue
				}
				if ys.Name != "" {
					name = ys.Name
				}
				sk = Skill{
					Name:        name,
					Description: ys.Description,
					Content:     ys.Content,
					Source:      src,
				}

			default:
				continue
			}

			if name == "" {
				continue
			}
			merged[name] = sk
		}
	}

	// Apply filter.
	filterSet := make(map[string]struct{}, len(enabledFilter))
	for _, n := range enabledFilter {
		filterSet[n] = struct{}{}
	}

	result := make([]Skill, 0, len(merged))
	for _, sk := range merged {
		if len(filterSet) > 0 {
			if _, ok := filterSet[sk.Name]; !ok {
				continue
			}
		}
		result = append(result, sk)
	}

	return result, nil
}

// extractDescription returns the first non-empty line of a markdown file,
// with an optional leading "# " prefix stripped.
func extractDescription(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		line = strings.TrimPrefix(line, "# ")
		line = strings.TrimPrefix(line, "## ")
		return line
	}
	return ""
}

// SkillsToPrompt formats loaded skills into a system prompt section.
func SkillsToPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Skills\n")
	for _, sk := range skills {
		b.WriteString(fmt.Sprintf("\n### %s (%s)\n", sk.Name, sk.Source))
		if sk.Description != "" {
			b.WriteString(fmt.Sprintf("> %s\n", sk.Description))
		}
		b.WriteString(sk.Content)
		// Ensure trailing newline for clean separation.
		if !strings.HasSuffix(sk.Content, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
