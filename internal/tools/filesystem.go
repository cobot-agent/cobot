package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// sandboxResolvePath resolves and validates a path within the sandbox.
// If sandbox is nil, the path is returned unchanged.
// If sandbox is active, AutoResolvePath is called to map virtual/relative/absolute
// paths into the sandbox, then ValidatePath ensures the resolved path stays within bounds.
func sandboxResolvePath(sandbox *cobot.SandboxConfig, path string) (string, error) {
	if sandbox == nil {
		return path, nil
	}
	originalPath := path
	resolved, err := sandbox.AutoResolvePath(path)
	if err != nil {
		return "", err
	}
	if err := sandbox.ValidatePath(resolved); err != nil {
		return "", fmt.Errorf("path %q is outside allowed workspace paths", originalPath)
	}
	return resolved, nil
}

// sandboxTool provides common sandbox functionality for filesystem tools.
type sandboxTool struct {
	sandbox *cobot.SandboxConfig
}

// describeWithSandbox appends the sandbox notice to a tool description.
func (s *sandboxTool) describeWithSandbox(desc string) string {
	if s.sandbox != nil && s.sandbox.VirtualRoot != "" {
		return desc + fmt.Sprintf(" Sandbox is active. All file paths are automatically resolved under %q — provide paths starting with %q for best results. Relative paths and other absolute paths are auto-mapped into the sandbox.", s.sandbox.VirtualRoot, s.sandbox.VirtualRoot)
	}
	return desc
}

// sandboxRewriteErr rewrites real paths to virtual paths in error messages.
func sandboxRewriteErr(sandbox *cobot.SandboxConfig, err error) error {
	if sandbox == nil || sandbox.VirtualRoot == "" {
		return err
	}
	return fmt.Errorf("%s", sandbox.RewriteOutputPaths(err.Error()))
}

//go:embed embed_filesystem_read_params.json
var filesystemReadParamsJSON []byte

//go:embed embed_filesystem_write_params.json
var filesystemWriteParamsJSON []byte

type readFileArgs struct {
	Path string `json:"path"`
}

type ReadFileTool struct {
	sandboxTool
}

func NewReadFileTool(sandbox *cobot.SandboxConfig) *ReadFileTool {
	return &ReadFileTool{sandboxTool{sandbox: sandbox}}
}

func (t *ReadFileTool) Name() string {
	return "filesystem_read"
}

func (t *ReadFileTool) Description() string {
	return t.describeWithSandbox("Read the contents of a file at the given path.")
}

func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemReadParamsJSON)
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if resolved, err := sandboxResolvePath(t.sandbox, a.Path); err != nil {
		return "", err
	} else {
		a.Path = resolved
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", sandboxRewriteErr(t.sandbox, err)
	}
	if t.sandbox != nil && t.sandbox.VirtualRoot != "" {
		virtualPath := t.sandbox.RealToVirtual(a.Path)
		return fmt.Sprintf("# %s\n%s", virtualPath, string(data)), nil
	}
	return string(data), nil
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileTool struct {
	sandboxTool
}

func NewWriteFileTool(sandbox *cobot.SandboxConfig) *WriteFileTool {
	return &WriteFileTool{sandboxTool{sandbox: sandbox}}
}

func (t *WriteFileTool) Name() string {
	return "filesystem_write"
}

func (t *WriteFileTool) Description() string {
	return t.describeWithSandbox("Write content to a file at the given path.")
}

func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemWriteParamsJSON)
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if resolved, err := sandboxResolvePath(t.sandbox, a.Path); err != nil {
		return "", err
	} else {
		a.Path = resolved
	}
	// Ensure parent directory exists
	if dir := filepath.Dir(a.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", sandboxRewriteErr(t.sandbox, err)
		}
	}
	if err := os.WriteFile(a.Path, []byte(a.Content), 0644); err != nil {
		return "", sandboxRewriteErr(t.sandbox, err)
	}
	if t.sandbox != nil && t.sandbox.VirtualRoot != "" {
		virtualPath := t.sandbox.RealToVirtual(a.Path)
		return fmt.Sprintf("wrote %s", virtualPath), nil
	}
	return "ok", nil
}

var (
	_ cobot.Tool = (*ReadFileTool)(nil)
	_ cobot.Tool = (*WriteFileTool)(nil)
)

//go:embed embed_filesystem_list_params.json
var filesystemListParamsJSON []byte

//go:embed embed_filesystem_search_params.json
var filesystemSearchParamsJSON []byte

type listDirArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern,omitempty"`
}

type ListDirTool struct {
	sandboxTool
}

func NewListDirTool(sandbox *cobot.SandboxConfig) *ListDirTool {
	return &ListDirTool{sandboxTool{sandbox: sandbox}}
}

func (t *ListDirTool) Name() string { return "filesystem_list" }

func (t *ListDirTool) Description() string {
	return t.describeWithSandbox("List files and directories at the given path.")
}

func (t *ListDirTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemListParamsJSON)
}

func (t *ListDirTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a listDirArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if resolved, err := sandboxResolvePath(t.sandbox, a.Path); err != nil {
		return "", err
	} else {
		a.Path = resolved
	}

	entries, err := os.ReadDir(a.Path)
	if err != nil {
		return "", sandboxRewriteErr(t.sandbox, err)
	}

	// When sandbox is active, compute the virtual path prefix for display
	virtualPrefix := ""
	if t.sandbox != nil && t.sandbox.VirtualRoot != "" {
		virtualPrefix = t.sandbox.RealToVirtual(a.Path)
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		if a.Pattern != "" {
			matched, _ := filepath.Match(a.Pattern, name)
			if !matched {
				continue
			}
		}
		displayName := name
		if virtualPrefix != "" {
			displayName = virtualPrefix + "/" + name
		}
		if entry.IsDir() {
			lines = append(lines, displayName+"/")
		} else {
			info, err := entry.Info()
			if err != nil {
				lines = append(lines, displayName)
			} else {
				lines = append(lines, fmt.Sprintf("%s (%d bytes)", displayName, info.Size()))
			}
		}
	}

	if len(lines) == 0 {
		return "empty directory", nil
	}
	return strings.Join(lines, "\n"), nil
}

type searchFilesArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

type SearchFilesTool struct {
	sandboxTool
}

func NewSearchFilesTool(sandbox *cobot.SandboxConfig) *SearchFilesTool {
	return &SearchFilesTool{sandboxTool{sandbox: sandbox}}
}

func (t *SearchFilesTool) Name() string { return "filesystem_search" }

func (t *SearchFilesTool) Description() string {
	return t.describeWithSandbox("Search for files matching a pattern recursively from a root directory.")
}

func (t *SearchFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemSearchParamsJSON)
}

func (t *SearchFilesTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a searchFilesArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if resolved, err := sandboxResolvePath(t.sandbox, a.Path); err != nil {
		return "", err
	} else {
		a.Path = resolved
	}

	var matches []string
	err := filepath.WalkDir(a.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		matched, _ := filepath.Match(a.Pattern, d.Name())
		if matched {
			displayPath := path
			if t.sandbox != nil && t.sandbox.VirtualRoot != "" {
				displayPath = t.sandbox.RealToVirtual(path)
			}
			if d.IsDir() {
				matches = append(matches, displayPath+"/")
			} else {
				matches = append(matches, displayPath)
			}
		}
		return nil
	})
	if err != nil {
		return "", sandboxRewriteErr(t.sandbox, err)
	}

	if len(matches) == 0 {
		return "no files found matching pattern", nil
	}
	return strings.Join(matches, "\n"), nil
}

var (
	_ cobot.Tool = (*ListDirTool)(nil)
	_ cobot.Tool = (*SearchFilesTool)(nil)
)
