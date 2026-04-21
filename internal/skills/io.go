package skills

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxSkillFileSize is the maximum allowed size for a single skill file (1 MB).
// This protects the catalog loader from unbounded memory reads.
const maxSkillFileSize int64 = 1 << 20

// readFileWithLimit reads a file after verifying its size does not exceed maxSize.
// Uses io.LimitReader to protect against TOCTOU races where the file grows between Stat and Read.
func readFileWithLimit(path string, maxSize int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file %s too large: %d bytes (max %d)", path, info.Size(), maxSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("file %s too large (exceeds %d bytes)", path, maxSize)
	}
	return data, nil
}

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
	f, err := os.Open(abs)
	if err != nil {
		return "", fmt.Errorf("open linked file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat linked file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("linked file path is a directory: %q", filePath)
	}
	if info.Size() > maxReadSize {
		return "", fmt.Errorf("linked file too large: %d bytes (max %d)", info.Size(), maxReadSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxReadSize+1))
	if err != nil {
		return "", fmt.Errorf("read linked file: %w", err)
	}
	if len(data) > maxReadSize {
		return "", fmt.Errorf("linked file too large: exceeds %d bytes", maxReadSize)
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
		return "", fmt.Errorf("find skill %q: %w", name, err)
	}
	if path == skillsDir {
		return "", fmt.Errorf("skill %q is in legacy format; migrate to %s directory format first", name, SkillFile)
	}
	return path, nil
}

// EnsureContainedDir verifies that parentDir is safely contained under skillDir
// (blocking path traversal) and creates parentDir with mode 0755 if it doesn't exist.
// Handles symlinks in intermediate path components by resolving the longest
// existing prefix before performing the containment check.
func EnsureContainedDir(parentDir, skillDir string) error {
	checkDir := parentDir
	if r, err := filepath.EvalSymlinks(parentDir); err == nil {
		checkDir = r
	}
	if _, err := VerifyContainment(checkDir, skillDir); err != nil {
		if !os.IsNotExist(err) {
			if errors.Is(err, ErrPathTraversal) {
				return ErrPathTraversal
			}
			return fmt.Errorf("verify containment: %w", err)
		}
		// Target doesn't exist. Resolve longest existing prefix to handle
		// symlinks in intermediate components, then verify full resolved path.
		resolved, err := resolveExistingPrefix(parentDir)
		if err != nil {
			return ErrPathTraversal
		}
		absBase, err := filepath.Abs(skillDir)
		if err != nil {
			return ErrPathTraversal
		}
		absBaseResolved, err := filepath.EvalSymlinks(absBase)
		if err != nil {
			absBaseResolved = absBase
		}
		if !strings.HasPrefix(resolved, absBaseResolved+string(filepath.Separator)) {
			return ErrPathTraversal
		}
	}
	return os.MkdirAll(parentDir, 0755)
}

// resolveExistingPrefix walks up the path to find the longest existing ancestor,
// resolves its symlinks, and appends the non-existing suffix to produce a fully
// resolved path.
func resolveExistingPrefix(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	p := abs
	for {
		r, err := filepath.EvalSymlinks(p)
		if err == nil {
			remaining := strings.TrimPrefix(abs, p)
			return filepath.Clean(r + remaining), nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fmt.Errorf("no existing prefix for %q", path)
		}
		p = parent
	}
}

// loadNewFormatSkill loads a SkillFile from a skill directory.
func loadNewFormatSkill(skillDir, category, source string) (Skill, error) {
	data, err := readFileWithLimit(filepath.Join(skillDir, SkillFile), maxSkillFileSize)
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
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return Skill{}, fmt.Errorf("resolve skill dir: %w", err)
	}
	return Skill{Name: fm.Name, Description: fm.Description, Category: category, Content: body, Source: source, Dir: absDir, Metadata: fm.Metadata}, nil
}

// loadLegacyFile loads a legacy flat-file skill (.md or .yaml/.yml).
// Skips dot-files (e.g., .hidden.md) for consistency with category dir filtering.
func loadLegacyFile(dir, filename, src string) (Skill, bool) {
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)
	if !isValidLegacyName(name) || strings.HasPrefix(name, ".") || (ext != ".md" && ext != ".yaml" && ext != ".yml") {
		return Skill{}, false
	}
	data, err := readFileWithLimit(filepath.Join(dir, filename), maxSkillFileSize)
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
	if !isValidLegacyName(name) {
		return Skill{}, false
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
		return "", fmt.Errorf("read skills dir %s: %w", root, err)
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
