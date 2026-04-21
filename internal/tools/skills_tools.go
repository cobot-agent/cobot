package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cobot-agent/cobot/internal/skills"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed schemas/embed_skills_list_params.json
var skillsListParamsJSON []byte

//go:embed schemas/embed_skill_view_params.json
var skillViewParamsJSON []byte

//go:embed schemas/embed_skill_manage_params.json
var skillManageParamsJSON []byte

// --- skills_list ---

type SkillsListTool struct {
	workspace *workspace.Workspace
}

func NewSkillsListTool(ws *workspace.Workspace) *SkillsListTool {
	return &SkillsListTool{workspace: ws}
}

func (t *SkillsListTool) Name() string { return "skills_list" }
func (t *SkillsListTool) Description() string {
	return "List all available skills with name, description, and category"
}

func (t *SkillsListTool) Parameters() json.RawMessage {
	return json.RawMessage(skillsListParamsJSON)
}

func (t *SkillsListTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Category string `json:"category"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	skillDirs := []string{workspace.GlobalSkillsDir(), t.workspace.SkillsDir()}
	catalog, err := skills.LoadCatalog(ctx, skillDirs, nil)
	if err != nil {
		return "", fmt.Errorf("load skills catalog: %w", err)
	}

	// Filter by category if provided.
	var filtered []skills.Skill
	for _, sk := range catalog {
		if params.Category != "" && sk.Category != params.Category {
			continue
		}
		filtered = append(filtered, sk)
	}

	summaries := make([]skills.SkillSummary, 0, len(filtered))
	for _, sk := range filtered {
		summaries = append(summaries, skills.SkillSummary{
			Name:        sk.Name,
			Description: sk.Description,
			Category:    sk.Category,
		})
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal skills: %w", err)
	}
	return string(data), nil
}

// --- skill_view ---

type SkillViewTool struct {
	workspace *workspace.Workspace
}

func NewSkillViewTool(ws *workspace.Workspace) *SkillViewTool {
	return &SkillViewTool{workspace: ws}
}

func (t *SkillViewTool) Name() string { return "skill_view" }
func (t *SkillViewTool) Description() string {
	return "View full skill content and list linked files, or read a specific linked file"
}

func (t *SkillViewTool) Parameters() json.RawMessage {
	return json.RawMessage(skillViewParamsJSON)
}

func (t *SkillViewTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Name     string `json:"name"`
		FilePath string `json:"file_path"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	// If a file_path is given, read the linked file.
	if params.FilePath != "" {
		// Path traversal prevention.
		if strings.Contains(params.FilePath, "..") || strings.HasPrefix(params.FilePath, "/") || strings.HasPrefix(params.FilePath, "\\") {
			return "", fmt.Errorf("invalid file path: path traversal detected")
		}

		skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
		if err != nil {
			return "", err
		}
		content, err := skills.ReadLinkedFile(skillDir, params.FilePath)
		if err != nil {
			return "", err
		}
		return content, nil
	}

	// Otherwise, return full SKILL.md content + linked files listing.
	skillDirs := []string{workspace.GlobalSkillsDir(), t.workspace.SkillsDir()}
	sk, err := skills.LoadFull(ctx, skillDirs, params.Name)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(sk.Content)

	linkedFiles := skills.ListLinkedFiles(sk.Dir)
	if len(linkedFiles) > 0 {
		b.WriteString("\n\n## Linked Files\n")
		for subdir, files := range linkedFiles {
			b.WriteString(fmt.Sprintf("\n**%s/**\n", subdir))
			for _, f := range files {
				b.WriteString(fmt.Sprintf("- %s\n", f))
			}
		}
	}

	return b.String(), nil
}

// --- skill_manage ---

type SkillManageTool struct {
	workspace *workspace.Workspace
}

func NewSkillManageTool(ws *workspace.Workspace) *SkillManageTool {
	return &SkillManageTool{workspace: ws}
}

func (t *SkillManageTool) Name() string { return "skill_manage" }
func (t *SkillManageTool) Description() string {
	return "Create, edit, patch, delete skills, or manage linked files"
}

func (t *SkillManageTool) Parameters() json.RawMessage {
	return json.RawMessage(skillManageParamsJSON)
}

func (t *SkillManageTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Action      string `json:"action"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Content     string `json:"content"`
		OldString   string `json:"old_string"`
		NewString   string `json:"new_string"`
		FilePath    string `json:"file_path"`
		FileContent string `json:"file_content"`
	}
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	switch params.Action {
	case "create":
		return t.doCreate(params)
	case "edit":
		return t.doEdit(params)
	case "patch":
		return t.doPatch(params)
	case "delete":
		return t.doDelete(params)
	case "write_file":
		return t.doWriteFile(params)
	case "remove_file":
		return t.doRemoveFile(params)
	default:
		return "", fmt.Errorf("unknown action: %s", params.Action)
	}
}

func (t *SkillManageTool) doCreate(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	if err := validateSkillName(params.Name); err != nil {
		return "", err
	}

	// Validate category if provided.
	if params.Category != "" {
		if err := validateSkillName(params.Category); err != nil {
			return "", fmt.Errorf("invalid category: %w", err)
		}
	}

	// Validate frontmatter in content.
	fm, _, err := skills.ParseFrontMatter(params.Content)
	if err != nil {
		return "", fmt.Errorf("invalid SKILL.md content: %w", err)
	}
	if fm.Name != params.Name {
		return "", fmt.Errorf("frontmatter name %q does not match skill name %q", fm.Name, params.Name)
	}
	if fm.Description == "" {

		return "", fmt.Errorf("frontmatter description is required")
	}

	// Build the skill directory path.
	baseDir := t.workspace.SkillsDir()
	var skillDir string
	if params.Category != "" {
		skillDir = filepath.Join(baseDir, params.Category, params.Name)
	} else {
		skillDir = filepath.Join(baseDir, params.Name)
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", fmt.Errorf("create skill directory: %w", err)
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md: %w", err)
	}

	slog.Info("skill created", "name", params.Name, "category", params.Category)
	return fmt.Sprintf("skill created: %s", params.Name), nil
}

func (t *SkillManageTool) doEdit(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	// Validate frontmatter in content.
	fm, _, err := skills.ParseFrontMatter(params.Content)
	if err != nil {
		return "", fmt.Errorf("invalid SKILL.md content: %w", err)
	}
	if fm.Name != params.Name {
		return "", fmt.Errorf("frontmatter name %q does not match skill name %q", fm.Name, params.Name)
	}

	skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md: %w", err)
	}

	return fmt.Sprintf("skill updated: %s", params.Name), nil
}

func (t *SkillManageTool) doPatch(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	if params.OldString == "" {
		return "", fmt.Errorf("old_string is required for patch action")
	}

	skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return "", fmt.Errorf("read SKILL.md: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, params.OldString) {
		return "", fmt.Errorf("old_string not found in SKILL.md")
	}

	newContent := strings.Replace(content, params.OldString, params.NewString, 1)

	// Validate frontmatter still parses.
	fm, _, err := skills.ParseFrontMatter(newContent)
	if err != nil {
		return "", fmt.Errorf("patched content has invalid frontmatter: %w", err)
	}
	if fm.Name != params.Name {
		return "", fmt.Errorf("patched frontmatter name %q does not match skill name %q", fm.Name, params.Name)
	}

	if err := os.WriteFile(skillMDPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("write patched SKILL.md: %w", err)
	}

	return fmt.Sprintf("skill patched: %s", params.Name), nil
}

func (t *SkillManageTool) doDelete(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return "", fmt.Errorf("remove skill directory: %w", err)
	}

	slog.Info("skill deleted", "name", params.Name)
	return fmt.Sprintf("skill deleted: %s", params.Name), nil
}

func (t *SkillManageTool) doWriteFile(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	if params.FilePath == "" {
		return "", fmt.Errorf("file_path is required for write_file action")
	}

	// Path traversal prevention.
	if strings.Contains(params.FilePath, "..") || strings.HasPrefix(params.FilePath, "/") || strings.HasPrefix(params.FilePath, "\\") {
		return "", fmt.Errorf("invalid file path: path traversal detected")
	}

	// Validate file_path is under an allowed subdir.
	if err := validateLinkedFilePath(params.FilePath); err != nil {
		return "", err
	}

	skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(skillDir, params.FilePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(params.FileContent), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("file written: %s/%s", params.Name, params.FilePath), nil
}

func (t *SkillManageTool) doRemoveFile(params struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}) (string, error) {
	if params.FilePath == "" {
		return "", fmt.Errorf("file_path is required for remove_file action")
	}

	// Path traversal prevention.
	if strings.Contains(params.FilePath, "..") || strings.HasPrefix(params.FilePath, "/") || strings.HasPrefix(params.FilePath, "\\") {
		return "", fmt.Errorf("invalid file path: path traversal detected")
	}

	// Validate file_path is under an allowed subdir.
	if err := validateLinkedFilePath(params.FilePath); err != nil {
		return "", err
	}

	skillDir, err := skills.FindSkillDir(t.workspace.SkillsDir(), workspace.GlobalSkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(skillDir, params.FilePath)
	if err := os.Remove(fullPath); err != nil {
		return "", fmt.Errorf("remove file: %w", err)
	}

	return fmt.Sprintf("file removed: %s/%s", params.Name, params.FilePath), nil
}

// validateSkillName validates a skill or category name against the spec.
func validateSkillName(name string) error {
	if len(name) < 2 || len(name) > 64 {
		return fmt.Errorf("invalid name %q: must be 2-64 characters", name)
	}
	// Must be lowercase alphanumeric + hyphens, start with letter, end with alphanumeric.
	for i, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			if i == 0 {
				return fmt.Errorf("invalid name %q: must start with a letter", name)
			}
			continue
		}
		if ch == '-' {
			if i == 0 || i == len(name)-1 {
				return fmt.Errorf("invalid name %q: must not start or end with hyphen", name)
			}
			continue
		}
		return fmt.Errorf("invalid name %q: must contain only lowercase letters, digits, and hyphens", name)
	}
	return nil
}

// validateLinkedFilePath ensures a file path is under an allowed linked subdir.
func validateLinkedFilePath(filePath string) error {
	allowed := []string{"references", "templates", "scripts", "assets"}
	for _, subdir := range allowed {
		if strings.HasPrefix(filePath, subdir+"/") || strings.HasPrefix(filePath, subdir+string(filepath.Separator)) {
			return nil
		}
	}
	return fmt.Errorf("file path must be under one of: %s", strings.Join(allowed, ", "))
}

// RegisterSkillsTools registers all skills-related tools.
func RegisterSkillsTools(registry cobot.ToolRegistry, ws *workspace.Workspace) {
	registry.Register(NewSkillsListTool(ws))
	registry.Register(NewSkillViewTool(ws))
	registry.Register(NewSkillManageTool(ws))
}
