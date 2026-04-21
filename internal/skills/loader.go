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

// validNameRe matches skill and category names: lowercase alphanumeric + hyphens.
// Requires at least 2 characters. Upper length bound (64) is enforced separately in ValidateSkillName.
var validNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

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

// ValidateSkillName validates a skill or category name against the spec.
// Name must match ^[a-z][a-z0-9-]*[a-z0-9]$ (2-64 chars).
func ValidateSkillName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("invalid name %q: must be 2-64 characters", name)
	}
	if !validNameRe.MatchString(name) {
		return fmt.Errorf("invalid name %q: must match ^[a-z][a-z0-9-]*[a-z0-9]$", name)
	}
	return nil
}

// ValidateSkillNameForView validates a skill name for read-only operations.
// It is more permissive than ValidateSkillName to allow viewing legacy skills
// whose names may not match the strict ^[a-z][a-z0-9-]*[a-z0-9]$ pattern,
// while still blocking path traversal attacks.
func ValidateSkillNameForView(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if len(name) > 128 {
		return fmt.Errorf("invalid name: too long (%d bytes)", len(name))
	}
	if !isValidLegacyName(name) {
		return fmt.Errorf("invalid name %q: path components not allowed", name)
	}
	return nil
}

// isValidLegacyName checks if a name is safe for legacy flat file compat (no path traversal).
func isValidLegacyName(name string) bool {
	return !strings.Contains(name, "/") && !strings.Contains(name, "\\") && !strings.Contains(name, "..")
}

// isValidCategoryName checks if a directory name is a valid category.
// Blocks path traversal components and dotfiles/dot-directories.
func isValidCategoryName(name string) bool {
	if !isValidLegacyName(name) {
		return false
	}
	// Block names starting with dot (e.g. ".", ".." already caught, ".hidden").
	if strings.HasPrefix(name, ".") {
		return false
	}
	return true
}

// LoadCatalog discovers all skills and returns full Skill objects (with tier-1 info populated).
// Scans dirs in order; later dirs override earlier (workspace > global).
// enabledFilter: if non-empty, only include named skills.
func LoadCatalog(ctx context.Context, dirs []string, enabledFilter []string) ([]Skill, error) {
	merged := make(map[string]Skill)

	for i, dir := range dirs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		scanned, err := scanDir(dir, i)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read skills dir %s: %w", dir, err)
		}
		for _, sk := range scanned {
			merged[sk.Name] = sk
		}
	}

	// Apply filter.
	var filterSet map[string]struct{}
	if len(enabledFilter) > 0 {
		filterSet = make(map[string]struct{}, len(enabledFilter))
		for _, n := range enabledFilter {
			filterSet[n] = struct{}{}
		}
	}

	result := make([]Skill, 0, len(merged))
	for _, sk := range merged {
		if filterSet != nil {
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
// Searches all dirs in order; workspace version wins (last-match).
func LoadFull(ctx context.Context, dirs []string, name string) (*Skill, error) {
	// Scan dirs in order; later dirs (workspace) override earlier (global) via last-match.
	var found *Skill
	for i, dir := range dirs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		scanned, err := scanDir(dir, i)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read skills dir %s: %w", dir, err)
		}
		for idx := range scanned {
			if scanned[idx].Name == name {
				s := scanned[idx]
				found = &s
			}
		}
	}
	if found != nil {
		return found, nil
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

// IsPathTraversalSafe returns false if filePath contains traversal patterns.
// Exported for reuse by other packages (e.g., tools).
func IsPathTraversalSafe(filePath string) bool {
	return !strings.Contains(filePath, "..") && !strings.HasPrefix(filePath, "/") && !strings.HasPrefix(filePath, "\\")
}

// VerifyContainment resolves symlinks and checks that resolved path is under baseDir.
// Returns the resolved absolute path on success.
// Exported for reuse by other packages (e.g., tools).
func VerifyContainment(fullPath string, baseDir string) (string, error) {
	resolved, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("resolve full path: %w", err)
	}
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("abs full path: %w", err)
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base path: %w", err)
	}
	absBaseResolved, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		absBaseResolved = absBase
	}
	if !strings.HasPrefix(absResolved, absBaseResolved+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid file path: path traversal detected")
	}
	return absResolved, nil
}

// ReadLinkedFile reads a specific linked file content.
// filePath must be a relative path under one of the linked subdirs.
// The caller should enforce size limits appropriate to the use case.
func ReadLinkedFile(skillDir string, filePath string) (string, error) {
	if !IsPathTraversalSafe(filePath) {
		return "", fmt.Errorf("invalid file path: path traversal detected")
	}

	// Validate the file is under an allowed subdir.
	if err := ValidateLinkedFilePath(filePath); err != nil {
		return "", err
	}

	fullPath := filepath.Join(skillDir, filePath)
	absResolved, err := VerifyContainment(fullPath, skillDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("linked file not found: %s", filePath)
		}
		return "", err
	}

	// Check file size before reading to prevent unbounded memory allocation.
	const maxReadSize = 10 << 20 // 10 MB hard limit
	info, err := os.Stat(absResolved)
	if err != nil {
		return "", fmt.Errorf("stat linked file: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("linked file too large: %d bytes (max %d)", info.Size(), maxReadSize)
	}

	data, err := os.ReadFile(absResolved)
	if err != nil {
		return "", fmt.Errorf("read linked file: %w", err)
	}
	return string(data), nil
}

// ValidateLinkedFilePath ensures a file path is under an allowed linked subdir.
// Returns an error if the path is not under references/, templates/, scripts/, or assets/.
func ValidateLinkedFilePath(filePath string) error {
	for _, subdir := range linkedSubdirs {
		if strings.HasPrefix(filePath, subdir+"/") {
			return nil
		}
	}
	return fmt.Errorf("file path must be under one of: %s", strings.Join(linkedSubdirs, ", "))
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
			if !isValidCategoryName(catName) {
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
	if err := ValidateSkillName(fm.Name); err != nil {
		return Skill{}, fmt.Errorf("invalid frontmatter name: %w", err)
	}
	if fm.Description == "" {
		return Skill{}, fmt.Errorf("skill description is required in frontmatter")
	}

	dirName := filepath.Base(skillDir)
	if fm.Name != dirName {
		return Skill{}, fmt.Errorf("skill name %q does not match directory name %q", fm.Name, dirName)
	}

	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return Skill{}, fmt.Errorf("resolve skill dir: %w", err)
	}

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
// with an optional leading markdown heading (one or more '#' followed by space) stripped.
func extractDescription(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		// Strip markdown heading prefix (e.g., "# ", "## ", "### ").
		i := 0
		for i < len(line) && line[i] == '#' {
			i++
		}
		if i > 0 && i < len(line) && line[i] == ' ' {
			line = line[i+1:]
		}
		return line
	}
	return ""
}

// FindSkillDir searches workspace then global skills dir for a skill by name.
// Returns the skill directory path.
// For legacy flat-file skills, returns the parent directory containing the file.
func FindSkillDir(wsSkillsDir string, globalSkillsDir string, name string) (string, error) {
	// Validate name to prevent path traversal (allow legacy names which may not
	// match the strict validNameRe, but must still block ".." and separators).
	if !isValidLegacyName(name) {
		return "", fmt.Errorf("invalid skill name %q", name)
	}

	// Search workspace skills dir first.
	if path, err := findSkillDirIn(name, wsSkillsDir); err == nil {
		return path, nil
	}
	// Then global skills dir (skip if empty to avoid searching CWD).
	if globalSkillsDir != "" {
		if path, err := findSkillDirIn(name, globalSkillsDir); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("skill not found: %s", name)
}

// FindNewFormatSkillDir searches for a new-format skill (SKILL.md in a named directory).
// Returns an error for legacy flat-file skills. Use this for mutation operations
// where operating on the parent directory of a legacy file would be dangerous.
func FindNewFormatSkillDir(skillsDir string, name string) (string, error) {
	if err := ValidateSkillName(name); err != nil {
		return "", err
	}

	// Check direct: skillsDir/name/SKILL.md
	candidate := filepath.Join(skillsDir, name, "SKILL.md")
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Join(skillsDir, name), nil
	}

	// Check categorized: skillsDir/category/name/SKILL.md
	ents, err := os.ReadDir(skillsDir)
	if err != nil {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		if !isValidCategoryName(ent.Name()) {
			continue
		}
		candidate := filepath.Join(skillsDir, ent.Name(), name, "SKILL.md")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Join(skillsDir, ent.Name(), name), nil
		}
	}

	// Check if it exists as a legacy file (for better error message).
	if isLegacySkill(name, skillsDir) {
		return "", fmt.Errorf("skill %q is in legacy format; migrate to SKILL.md directory format first", name)
	}
	return "", fmt.Errorf("skill not found: %s", name)
}

// isLegacySkill checks whether a skill exists as a legacy flat file in root.
func isLegacySkill(name string, root string) bool {
	for _, ext := range []string{".md", ".yaml", ".yml"} {
		if _, err := os.Stat(filepath.Join(root, name+ext)); err == nil {
			return true
		}
	}
	return false
}

// findSkillDirIn searches a single root for a skill by name.
func findSkillDirIn(name string, root string) (string, error) {
	// Check new format: root/name/SKILL.md
	candidate := filepath.Join(root, name, "SKILL.md")
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Join(root, name), nil
	}

	// Check new format with categories and legacy format.
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("skill not found in %s", root)
	}
	for _, ent := range ents {
		if !ent.IsDir() {
			// Check legacy format: flat .md/.yaml/.yml files.
			for _, ext := range []string{".md", ".yaml", ".yml"} {
				if ent.Name() == name+ext {
					return root, nil
				}
			}
			continue
		}
		// Check if this is a valid category directory containing the skill.
		if !isValidCategoryName(ent.Name()) {
			continue
		}
		candidate := filepath.Join(root, ent.Name(), name, "SKILL.md")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Join(root, ent.Name(), name), nil
		}
	}

	return "", fmt.Errorf("skill not found: %s in %s", name, root)
}
