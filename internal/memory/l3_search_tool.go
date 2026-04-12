package memory

import (
	"context"
	"encoding/json"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type l3SearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type L3DeepSearchTool struct {
	client Client
}

func NewL3DeepSearchTool(c Client) *L3DeepSearchTool {
	return &L3DeepSearchTool{client: c}
}

func (t *L3DeepSearchTool) Name() string { return "l3_deep_search" }
func (t *L3DeepSearchTool) Description() string {
	return "Perform deep semantic search across all memory"
}
func (t *L3DeepSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query for deep semantic search"},"limit":{"type":"integer","description":"Max results, default 10"}},"required":["query"]}`)
}

func (t *L3DeepSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a l3SearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	results, err := t.client.L3DeepSearch(ctx, a.Query, a.Limit)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "Deep search found 0 results", nil
	}

	out := fmt.Sprintf("Deep search found %d results:\n", len(results))
	for _, r := range results {
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		out += fmt.Sprintf("  - [%.4f][%s] %s\n", r.Score, r.WingID, content)
	}
	return out, nil
}

var _ cobot.Tool = (*L3DeepSearchTool)(nil)
