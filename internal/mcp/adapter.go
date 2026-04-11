package mcp

import (
	"context"
	"encoding/json"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type MCPToolAdapter struct {
	name        string
	description string
	schema      json.RawMessage
	callFunc    func(ctx context.Context, args json.RawMessage) (string, error)
}

func (t *MCPToolAdapter) Name() string                { return t.name }
func (t *MCPToolAdapter) Description() string         { return t.description }
func (t *MCPToolAdapter) Parameters() json.RawMessage { return t.schema }
func (t *MCPToolAdapter) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return t.callFunc(ctx, args)
}

var _ cobot.Tool = (*MCPToolAdapter)(nil)
