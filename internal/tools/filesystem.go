package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed embed_filesystem_read_params.json
var filesystemReadParamsJSON []byte

//go:embed embed_filesystem_write_params.json
var filesystemWriteParamsJSON []byte

type readFileArgs struct {
	Path string `json:"path"`
}

type ReadFileTool struct {
	sandbox *cobot.SandboxConfig
}

type ReadFileToolOption func(*ReadFileTool)

func WithReadSandbox(s *cobot.SandboxConfig) ReadFileToolOption {
	return func(t *ReadFileTool) { t.sandbox = s }
}

func NewReadFileTool(opts ...ReadFileToolOption) *ReadFileTool {
	t := &ReadFileTool{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *ReadFileTool) Name() string {
	return "filesystem_read"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file at the given path"
}

func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemReadParamsJSON)
}

func (t *ReadFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a readFileArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if t.sandbox != nil && !t.sandbox.IsAllowed(a.Path, false) {
		return "", fmt.Errorf("path %q is outside allowed workspace paths", a.Path)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type WriteFileTool struct {
	sandbox *cobot.SandboxConfig
}

type WriteFileToolOption func(*WriteFileTool)

func WithWriteSandbox(s *cobot.SandboxConfig) WriteFileToolOption {
	return func(t *WriteFileTool) { t.sandbox = s }
}

func NewWriteFileTool(opts ...WriteFileToolOption) *WriteFileTool {
	t := &WriteFileTool{}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *WriteFileTool) Name() string {
	return "filesystem_write"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file at the given path"
}

func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(filesystemWriteParamsJSON)
}

func (t *WriteFileTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a writeFileArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}
	if t.sandbox != nil && !t.sandbox.IsAllowed(a.Path, true) {
		return "", fmt.Errorf("path %q is outside allowed workspace paths", a.Path)
	}
	if err := os.WriteFile(a.Path, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	return "ok", nil
}

var (
	_ cobot.Tool = (*ReadFileTool)(nil)
	_ cobot.Tool = (*WriteFileTool)(nil)
)
