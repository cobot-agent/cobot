package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ListLinkedFiles returns a map of subdir→filenames for non-empty linked file
// directories (references/, templates/, scripts/, assets/) under skillDir.
func ListLinkedFiles(skillDir string) map[string][]string {
	result := make(map[string][]string)
	for _, subdir := range linkedSubdirs {
		ents, err := os.ReadDir(filepath.Join(skillDir, subdir))
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

// ReadLinkedFile reads a linked file under an allowed subdir with path safety and 10 MB limit.
func ReadLinkedFile(skillDir, filePath string) (string, error) {
	if err := ValidateLinkedFilePath(filePath); err != nil {
		return "", err
	}
	abs, err := VerifyContainment(filepath.Join(skillDir, filePath), skillDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("linked file not found: %q", filePath)
		}
		return "", err
	}
	const maxReadSize = 10 << 20 // 10 MB
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat linked file: %w", err)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("linked file too large: %d bytes (max %d)", info.Size(), maxReadSize)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read linked file: %w", err)
	}
	return string(data), nil
}

// FindSkillDir searches workspace then global skills dir for a skill by name (legacy returns parent dir).
func FindSkillDir(wsSkillsDir, globalSkillsDir, name string) (string, error) {
	if !isValidLegacyName(name) {
		return "", fmt.Errorf("invalid skill name %q", name)
	}
	if p, err := findSkillDirIn(name, wsSkillsDir); err == nil {
		return p, nil
	}
	if globalSkillsDir != "" {
		if p, err := findSkillDirIn(name, globalSkillsDir); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("skill not found: %q", name)
}

// FindNewFormatSkillDir finds a new-format SkillFile skill; errors for legacy flat files.
func FindNewFormatSkillDir(skillsDir, name string) (string, error) {
	if err := ValidateSkillName(name); err != nil {
		return "", err
	}
	path, err := findSkillDirIn(name, skillsDir)
	if err != nil {
		return "", fmt.Errorf("skill not found: %q", name)
	}
	if path == skillsDir {
		return "", fmt.Errorf("skill %q is in legacy format; migrate to %s directory format first", name, SkillFile)
	}
	return path, nil
}

// EnsureContainedDir verifies that parentDir is safely contained under skillDir
// (blocking path traversal) and creates parentDir with mode 0755 if it doesn't exist.
func EnsureContainedDir(parentDir, skillDir string) error {
	checkDir := parentDir
	if r, err := filepath.EvalSymlinks(parentDir); err == nil {
		checkDir = r
	}
	if _, err := VerifyContainment(checkDir, skillDir); err != nil {
		if !os.IsNotExist(err) {
			return ErrPathTraversal
		}
		absCheck, err1 := filepath.Abs(parentDir)
		absBase, err2 := filepath.Abs(skillDir)
		if err1 != nil || err2 != nil || !strings.HasPrefix(absCheck, absBase+string(filepath.Separator)) {
			return ErrPathTraversal
		}
	}
	return os.MkdirAll(parentDir, 0755)
}

// loadNewFormatSkill loads a SkillFile from a skill directory.
func loadNewFormatSkill(skillDir, category, source string) (Skill, error) {
	data, err := os.ReadFile(filepath.Join(skillDir, SkillFile))
	if err != nil {
		return Skill{}, fmt.Errorf("read %s: %w", SkillFile, err)
	}
	fm, body, err := parseFrontMatter(string(data))
	if err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if fm.Name == "" || fm.Description == "" {
		return Skill{}, errors.New("skill name and description are required in frontmatter")
	}
	if err := ValidateSkillName(fm.Name); err != nil {
		return Skill{}, fmt.Errorf("invalid frontmatter name: %w", err)
	}
	if dirName := filepath.Base(skillDir); fm.Name != dirName {
		return Skill{}, fmt.Errorf("skill name %q does not match directory name %q", fm.Name, dirName)
	}
	absDir, _ := filepath.Abs(skillDir)
	return Skill{Name: fm.Name, Description: fm.Description, Category: category, Content: body, Source: source, Dir: absDir, Metadata: fm.Metadata}, nil
}

// loadLegacyFile loads a legacy flat-file skill (.md or .yaml/.yml).
func loadLegacyFile(dir, filename, src string) (Skill, bool) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	if !isValidLegacyName(name) || (ext != ".md" && ext != ".yaml" && ext != ".yml") {
		return Skill{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return Skill{}, false
	}
	if ext == ".md" {
		return Skill{Name: name, Description: extractDescription(string(data)), Content: string(data), Source: src}, true
	}
	var ys yamlSkill
	if err := yaml.Unmarshal(data, &ys); err != nil {
		return Skill{}, false
	}
	if ys.Name != "" {
		name = ys.Name
	}
	return Skill{Name: name, Description: ys.Description, Content: ys.Content, Source: src}, true
}

// findSkillDirIn searches a single root for a skill by name (new or legacy format).
func findSkillDirIn(name, root string) (string, error) {
	dir := filepath.Join(root, name)
	if _, err := os.Stat(filepath.Join(dir, SkillFile)); err == nil {
		return dir, nil
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("skill not found in %s", root)
	}
	for _, ent := range ents {
		if ent.IsDir() && isValidCategoryName(ent.Name()) {
			if _, err := os.Stat(filepath.Join(root, ent.Name(), name, SkillFile)); err == nil {
				return filepath.Join(root, ent.Name(), name), nil
			}
			continue
		}
		if !ent.IsDir() && strings.TrimSuffix(ent.Name(), filepath.Ext(ent.Name())) == name {
			return root, nil
		}
	}
	return "", fmt.Errorf("skill not found: %q in %s", name, root)
}
