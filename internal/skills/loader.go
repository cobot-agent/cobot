package skills

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlSkill is the on-disk representation for .yaml skill files (legacy compat).
type yamlSkill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
}

// validNameRe matches skill and category names: lowercase alphanumeric + hyphens, 2-64 chars.
var validNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// validSimpleNameRe matches single-char names like "a" (2-char minimum normally, but we allow legacy compat).
var validNameFullRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// linkedSubdirs are the allowed subdirectories for linked files.
var linkedSubdirs = []string{"references", "templates", "scripts", "assets"}

// sourceLabel maps a directory index to a human-readable source label.
// dirIndex 0 → "global", everything else → "workspace".
func sourceLabel(dirIndex int) string {
	if dirIndex == 0 {
		return "global"
	}
	return "workspace"
}

// validateSkillName validates a skill or category name against the spec.
// Name must match ^[a-z][a-z0-9-]*[a-z0-9]$ (2-64 chars).
func validateSkillName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("invalid name %q: must be 2-64 characters", name)
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("invalid name %q: must match ^[a-z][a-z0-9-]*[a-z0-9]$", name)
	}
	return nil
}

// isValidLegacyName checks if a name is safe for legacy flat file compat (no path traversal).
func isValidLegacyName(name string) bool {
	return !strings.Contains(name, "/") && !strings.Contains(name, "\\") && !strings.Contains(name, "..")
}

// LoadCatalog discovers all skills and returns full Skill objects (with tier-1 info populated).
// Scans dirs in order; later dirs override earlier (workspace > global).
// enabledFilter: if non-empty, only include named skills.
func LoadCatalog(ctx context.Context, dirs []string, enabledFilter []string) ([]Skill, error) {
	merged := make(map[string]Skill)

	for i, dir := range dirs {
		skills, err := scanDir(dir, i)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read skills dir %s: %w", dir, err)
		}
		for _, sk := range skills {
			merged[sk.Name] = sk
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

	// Sort by name for deterministic output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// LoadFull loads tier-2 content for a specific skill by name.
// Searches all dirs; workspace version wins.
func LoadFull(ctx context.Context, dirs []string, name string) (*Skill, error) {
	catalog, err := LoadCatalog(ctx, dirs, nil)
	if err != nil {
		return nil, err
	}

	for i := range catalog {
		if catalog[i].Name == name {
			return &catalog[i], nil
		}
	}
	return nil, fmt.Errorf("skill not found: %s", name)
}

// ListLinkedFiles returns files in references/, templates/, scripts/, assets/.
// Returns map[subdir][]filename.
func ListLinkedFiles(skillDir string) map[string][]string {
	result := make(map[string][]string)
	for _, subdir := range linkedSubdirs {
		dirPath := filepath.Join(skillDir, subdir)
		ents, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}
		var files []string
		for _, ent := range ents {
			if !ent.IsDir() {
				files = append(files, ent.Name())
			}
		}
		if len(files) > 0 {
			sort.Strings(files)
			result[subdir] = files
		}
	}
	return result
}

// ReadLinkedFile reads a specific linked file content.
// filePath must be a relative path under one of the linked subdirs.
func ReadLinkedFile(skillDir string, filePath string) (string, error) {
	// Path traversal prevention.
	if strings.Contains(filePath, "..") || strings.HasPrefix(filePath, "/") || strings.HasPrefix(filePath, "\\") {
		return "", fmt.Errorf("invalid file path: path traversal detected")
	}

	// Validate the file is under an allowed subdir.
	allowed := false
	for _, subdir := range linkedSubdirs {
		if strings.HasPrefix(filePath, subdir+"/") || strings.HasPrefix(filePath, subdir+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("file path must be under one of: %s", strings.Join(linkedSubdirs, ", "))
	}

	fullPath := filepath.Join(skillDir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read linked file: %w", err)
	}
	return string(data), nil
}

// SkillsToPrompt formats tier-1 catalog for system prompt injection.
// Format: just name + description per skill, NOT full content.
func SkillsToPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Skills\n")
	for _, sk := range skills {
		if sk.Category != "" {
			b.WriteString(fmt.Sprintf("\n- **%s** (%s): %s", sk.Name, sk.Category, sk.Description))
		} else {
			b.WriteString(fmt.Sprintf("\n- **%s**: %s", sk.Name, sk.Description))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// scanDir scans a single skills directory for both new (SKILL.md) and legacy (.md/.yaml) formats.
func scanDir(dir string, dirIndex int) ([]Skill, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	src := sourceLabel(dirIndex)
	var result []Skill

	for _, ent := range ents {
		if ent.IsDir() {
			// Check if it's a direct skill directory (skills/name/SKILL.md)
			skillMD := filepath.Join(dir, ent.Name(), "SKILL.md")
			if info, err := os.Stat(skillMD); err == nil && !info.IsDir() {
				sk, err := loadNewFormatSkill(filepath.Join(dir, ent.Name()), "", src)
				if err != nil {
					slog.Warn("failed to load skill", "path", skillMD, "error", err)
					continue
				}
				result = append(result, sk)
				continue
			}

			// Check if it's a category directory (skills/category/name/SKILL.md)
			catName := ent.Name()
			if !isValidLegacyName(catName) {
				continue
			}
			catEnts, err := os.ReadDir(filepath.Join(dir, catName))
			if err != nil {
				continue
			}
			for _, catEnt := range catEnts {
				if !catEnt.IsDir() {
					continue
				}
				skillMD := filepath.Join(dir, catName, catEnt.Name(), "SKILL.md")
				if info, err := os.Stat(skillMD); err == nil && !info.IsDir() {
					sk, err := loadNewFormatSkill(filepath.Join(dir, catName, catEnt.Name()), catName, src)
					if err != nil {
						slog.Warn("failed to load skill", "path", skillMD, "error", err)
						continue
					}
					result = append(result, sk)
				}
			}
			continue
		}

		// Legacy flat file formats.
		name := ""
		var sk Skill

		switch {
		case strings.HasSuffix(ent.Name(), ".md"):
			name = strings.TrimSuffix(ent.Name(), ".md")
			if !isValidLegacyName(name) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
			if err != nil {
				continue
			}
			content := string(data)
			sk = Skill{
				Name:    name,
				Content: content,
				Source:  src,
				Dir:     dir,
			}
			sk.Description = extractDescription(content)

		case strings.HasSuffix(ent.Name(), ".yaml") || strings.HasSuffix(ent.Name(), ".yml"):
			ext := filepath.Ext(ent.Name())
			name = strings.TrimSuffix(ent.Name(), ext)
			if !isValidLegacyName(name) {
				continue
			}
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
				Dir:         dir,
			}

		default:
			continue
		}

		if name == "" {
			continue
		}
		result = append(result, sk)
	}

	return result, nil
}

// loadNewFormatSkill loads a SKILL.md from a skill directory.
func loadNewFormatSkill(skillDir string, category string, source string) (Skill, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return Skill{}, fmt.Errorf("read SKILL.md: %w", err)
	}

	content := string(data)
	fm, body, err := ParseFrontMatter(content)
	if err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	if fm.Name == "" {
		return Skill{}, fmt.Errorf("skill name is required in frontmatter")
	}
	if fm.Description == "" {
		return Skill{}, fmt.Errorf("skill description is required in frontmatter")
	}

	absDir, _ := filepath.Abs(skillDir)

	return Skill{
		Name:        fm.Name,
		Description: fm.Description,
		Category:    category,
		Content:     body,
		Source:      source,
		Dir:         absDir,
		Metadata:    fm.Metadata,
	}, nil
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

// FindSkillDir searches workspace then global skills dir for a skill by name.
// Returns the skill directory path.
func FindSkillDir(wsSkillsDir string, globalSkillsDir string, name string) (string, error) {
	// Search workspace skills dir first.
	if path, err := findSkillDirIn(name, wsSkillsDir); err == nil {
		return path, nil
	}
	// Then global skills dir.
	if path, err := findSkillDirIn(name, globalSkillsDir); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("skill not found: %s", name)
}

// findSkillDirIn searches a single root for a skill by name.
func findSkillDirIn(name string, root string) (string, error) {
	// Check new format: root/name/SKILL.md
	candidate := filepath.Join(root, name, "SKILL.md")
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Join(root, name), nil
	}

	// Check new format with categories: root/category/name/SKILL.md
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("skill not found in %s", root)
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			// Check legacy format
			for _, ext := range []string{".md", ".yaml", ".yml"} {
				if ent.Name() == name+ext {
					return root, nil
				}
			}
			continue
		}
		// Check if this is a category containing the skill
		candidate := filepath.Join(root, ent.Name(), name, "SKILL.md")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Join(root, ent.Name(), name), nil
		}
	}

	// Check legacy flat files
	for _, ext := range []string{".md", ".yaml", ".yml"} {
		candidate := filepath.Join(root, name+ext)
		if _, err := os.Stat(candidate); err == nil {
			return root, nil
		}
	}

	return "", fmt.Errorf("skill not found: %s in %s", name, root)
}
