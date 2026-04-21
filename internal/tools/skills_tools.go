package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cobot-agent/cobot/internal/skills"
	"github.com/cobot-agent/cobot/internal/workspace"
	cobot "github.com/cobot-agent/cobot/pkg"
)

const (
	// maxSkillContentSize limits skill content (SKILL.md) to prevent unbounded memory use.
	maxSkillContentSize = 256 * 1024 // 256 KB

	// maxLinkedFileSize limits linked file content for reads and writes.
	maxLinkedFileSize = 1024 * 1024 // 1 MB
)

//go:embed schemas/embed_skills_list_params.json
var skillsListParamsJSON []byte

//go:embed schemas/embed_skill_view_params.json
var skillViewParamsJSON []byte

//go:embed schemas/embed_skill_manage_params.json
var skillManageParamsJSON []byte

// --- skills_list ---

// SkillsListTool lists all available skills with name, description, and category.
type SkillsListTool struct {
	workspace *workspace.Workspace
}

// NewSkillsListTool creates a SkillsListTool for the given workspace.
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

	// Filter by category and build summaries in a single pass.
	summaries := make([]skills.SkillSummary, 0, len(catalog))
	for _, sk := range catalog {
		if params.Category != "" && sk.Category != params.Category {
			continue
		}
		summaries = append(summaries, skills.SkillSummary{
			Name:        sk.Name,
			Description: sk.Description,
			Category:    sk.Category,
			Source:      sk.Source,
		})
	}

	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal skills: %w", err)
	}
	return string(data), nil
}

// --- skill_view ---

// SkillViewTool views full skill content and linked files, or reads a specific linked file.
type SkillViewTool struct {
	workspace *workspace.Workspace
}

// NewSkillViewTool creates a SkillViewTool for the given workspace.
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

	// Use permissive validation for read operations to allow viewing legacy skills.
	if err := skills.ValidateSkillNameForView(params.Name); err != nil {
		return "", err
	}

	// If a file_path is given, read the linked file.
	// ReadLinkedFile handles path traversal checks, containment verification,
	// and file size limits internally.
	if params.FilePath != "" {
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

	// For new-format skills, read the full SKILL.md from disk (includes frontmatter).
	// sk.Content is body-only for new-format, so we read the file directly.
	if sk.Dir != "" {
		skillMDPath := filepath.Join(sk.Dir, "SKILL.md")
		if data, err := os.ReadFile(skillMDPath); err == nil {
			if len(data) > maxSkillContentSize {
				return "", fmt.Errorf("skill content exceeds maximum size of %d bytes", maxSkillContentSize)
			}
			b.Write(data)
		} else {
			// Fallback to body content.
			b.WriteString(sk.Content)
		}
	} else {
		// Legacy skill: Content is already the full file.
		b.WriteString(sk.Content)
	}

	// Only list linked files for skills with their own directory (new format).
	if sk.Dir != "" {
		linkedFiles := skills.ListLinkedFiles(sk.Dir)
		if len(linkedFiles) > 0 {
			b.WriteString("\n\n## Linked Files\n")
			// Sort subdirectories for deterministic output.
			sortedSubdirs := make([]string, 0, len(linkedFiles))
			for subdir := range linkedFiles {
				sortedSubdirs = append(sortedSubdirs, subdir)
			}
			sort.Strings(sortedSubdirs)
			for _, subdir := range sortedSubdirs {
				b.WriteString(fmt.Sprintf("\n**%s/**\n", subdir))
				for _, f := range linkedFiles[subdir] {
					b.WriteString(fmt.Sprintf("- %s\n", f))
				}
			}
		}
	}

	return b.String(), nil
}

// --- skill_manage ---

// manageParams holds the common parameters for all skill management actions.
type manageParams struct {
	Action      string `json:"action"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Content     string `json:"content"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	FilePath    string `json:"file_path"`
	FileContent string `json:"file_content"`
}

// SkillManageTool creates, edits, patches, and deletes skills, and manages linked files.
type SkillManageTool struct {
	workspace *workspace.Workspace
}

// NewSkillManageTool creates a SkillManageTool for the given workspace.
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
	var params manageParams
	if err := decodeArgs(args, &params); err != nil {
		return "", err
	}

	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Validate name for all actions to prevent path traversal.
	if err := skills.ValidateSkillName(params.Name); err != nil {
		return "", err
	}

	if params.Action == "" {
		return "", fmt.Errorf("action is required (create, edit, patch, delete, write_file, remove_file)")
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

// validateSkillContent parses frontmatter from content and validates that the name
// matches the expected skill name and that a description is present.
func validateSkillContent(content string, expectedName string) error {
	fm, _, err := skills.ParseFrontMatter(content)
	if err != nil {
		return fmt.Errorf("invalid SKILL.md content: %w", err)
	}
	if fm.Name != expectedName {
		return fmt.Errorf("frontmatter name %q does not match skill name %q", fm.Name, expectedName)
	}
	if fm.Description == "" {
		return fmt.Errorf("frontmatter description is required")
	}
	return nil
}

func (t *SkillManageTool) doCreate(params manageParams) (string, error) {
	// Validate category if provided.
	if params.Category != "" {
		if err := skills.ValidateSkillName(params.Category); err != nil {
			return "", fmt.Errorf("invalid category: %w", err)
		}
	}

	// Validate non-empty content.
	if params.Content == "" {
		return "", fmt.Errorf("content is required for create action")
	}

	// Enforce content size limit.
	if len(params.Content) > maxSkillContentSize {
		return "", fmt.Errorf("content exceeds maximum size of %d bytes", maxSkillContentSize)
	}

	// Validate frontmatter in content.
	if err := validateSkillContent(params.Content, params.Name); err != nil {
		return "", err
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

	// Check for existing skill.
	if _, err := os.Stat(skillMDPath); err == nil {
		return "", fmt.Errorf("skill %q already exists; use edit or patch to modify it", params.Name)
	}

	if err := os.WriteFile(skillMDPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md: %w", err)
	}

	slog.Info("skill created", "name", params.Name, "category", params.Category)
	return fmt.Sprintf("skill created: %s", params.Name), nil
}

func (t *SkillManageTool) doEdit(params manageParams) (string, error) {
	if params.Content == "" {
		return "", fmt.Errorf("content is required for edit action")
	}

	// Enforce content size limit.
	if len(params.Content) > maxSkillContentSize {
		return "", fmt.Errorf("content exceeds maximum size of %d bytes", maxSkillContentSize)
	}

	// Validate frontmatter in content.
	if err := validateSkillContent(params.Content, params.Name); err != nil {
		return "", err
	}

	// Workspace-only: do not modify global skills.
	// FindNewFormatSkillDir ensures we only find new-format (SKILL.md) skills.
	skillDir, err := skills.FindNewFormatSkillDir(t.workspace.SkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write SKILL.md: %w", err)
	}

	return fmt.Sprintf("skill updated: %s", params.Name), nil
}

func (t *SkillManageTool) doPatch(params manageParams) (string, error) {
	if params.OldString == "" {
		return "", fmt.Errorf("old_string is required for patch action")
	}

	// Workspace-only: do not modify global skills.
	// FindNewFormatSkillDir ensures we only find new-format (SKILL.md) skills.
	skillDir, err := skills.FindNewFormatSkillDir(t.workspace.SkillsDir(), params.Name)
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

	// Enforce content size limit on patched result.
	if len(newContent) > maxSkillContentSize {
		return "", fmt.Errorf("patched content exceeds maximum size of %d bytes", maxSkillContentSize)
	}

	// Validate frontmatter still parses.
	if err := validateSkillContent(newContent, params.Name); err != nil {
		return "", fmt.Errorf("patched content: %w", err)
	}

	if err := os.WriteFile(skillMDPath, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("write patched SKILL.md: %w", err)
	}

	return fmt.Sprintf("skill patched: %s", params.Name), nil
}

func (t *SkillManageTool) doDelete(params manageParams) (string, error) {
	// Workspace-only: do not modify global skills.
	// FindNewFormatSkillDir ensures we only delete new-format skill directories,
	// never accidentally removing the workspace skills root for legacy skills.
	skillDir, err := skills.FindNewFormatSkillDir(t.workspace.SkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(skillDir); err != nil {
		return "", fmt.Errorf("remove skill directory: %w", err)
	}

	slog.Info("skill deleted", "name", params.Name)
	return fmt.Sprintf("skill deleted: %s", params.Name), nil
}

// validateLinkedFileAction validates file_path for write_file/remove_file actions.
// Returns an error if the path is missing, contains traversal, or is not under an allowed subdir.
func validateLinkedFileAction(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file_path is required")
	}
	if !skills.IsPathTraversalSafe(filePath) {
		return fmt.Errorf("invalid file path: path traversal detected")
	}
	if err := skills.ValidateLinkedFilePath(filePath); err != nil {
		return fmt.Errorf("invalid linked file path: %w", err)
	}
	return nil
}

func (t *SkillManageTool) doWriteFile(params manageParams) (string, error) {
	if err := validateLinkedFileAction(params.FilePath); err != nil {
		return "", err
	}

	// Enforce file content size limit.
	if len(params.FileContent) > maxLinkedFileSize {
		return "", fmt.Errorf("file_content exceeds maximum size of %d bytes", maxLinkedFileSize)
	}

	// Workspace-only: do not modify global skills.
	skillDir, err := skills.FindNewFormatSkillDir(t.workspace.SkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(skillDir, params.FilePath)
	parentDir := filepath.Dir(fullPath)

	// Ensure the parent directory is under the skill dir.
	// For existing dirs, resolve symlinks; for new dirs, check the literal path.
	checkDir := parentDir
	if resolved, err := filepath.EvalSymlinks(parentDir); err == nil {
		checkDir = resolved
	}
	if _, err := skills.VerifyContainment(checkDir, skillDir); err != nil {
		// If the parent doesn't exist yet, VerifyContainment fails.
		// Fall back to a direct prefix check on the literal path.
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("invalid file path: path traversal detected")
		}
		absCheck, errCheck := filepath.Abs(parentDir)
		if errCheck != nil {
			return "", fmt.Errorf("resolve parent path: %w", errCheck)
		}
		absBase, errBase := filepath.Abs(skillDir)
		if errBase != nil {
			return "", fmt.Errorf("resolve skill dir path: %w", errBase)
		}
		if !strings.HasPrefix(absCheck, absBase+string(filepath.Separator)) {
			return "", fmt.Errorf("invalid file path: path traversal detected")
		}
	}

	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(fullPath, []byte(params.FileContent), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("file written: %s/%s", params.Name, params.FilePath), nil
}

func (t *SkillManageTool) doRemoveFile(params manageParams) (string, error) {
	if err := validateLinkedFileAction(params.FilePath); err != nil {
		return "", err
	}

	// Workspace-only: do not modify global skills.
	skillDir, err := skills.FindNewFormatSkillDir(t.workspace.SkillsDir(), params.Name)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(skillDir, params.FilePath)
	if _, err := skills.VerifyContainment(fullPath, skillDir); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", params.FilePath)
		}
		return "", err
	}
	if err := os.Remove(fullPath); err != nil {
		return "", fmt.Errorf("remove file: %w", err)
	}

	return fmt.Sprintf("file removed: %s/%s", params.Name, params.FilePath), nil
}

// RegisterSkillsTools registers all skills-related tools.
func RegisterSkillsTools(registry cobot.ToolRegistry, ws *workspace.Workspace) {
	registry.Register(NewSkillsListTool(ws))
	registry.Register(NewSkillViewTool(ws))
	registry.Register(NewSkillManageTool(ws))
}
