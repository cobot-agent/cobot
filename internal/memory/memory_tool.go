package memory

import (
	"context"
	"encoding/json"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type memorySearchArgs struct {
	Query  string `json:"query"`
	WingID string `json:"wing_id,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type MemorySearchTool struct {
	client Client
}

func NewMemorySearchTool(c Client) *MemorySearchTool {
	return &MemorySearchTool{client: c}
}

func (t *MemorySearchTool) Name() string { return "memory_search" }
func (t *MemorySearchTool) Description() string {
	return "Search the memory palace for relevant information"
}
func (t *MemorySearchTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query text"},"wing_id":{"type":"string","description":"Optional wing ID to filter by"},"limit":{"type":"integer","description":"Max results, default 10"}},"required":["query"]}`)
}

func (t *MemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a memorySearchArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	query := &cobot.SearchQuery{
		Text:   a.Query,
		WingID: a.WingID,
		Limit:  a.Limit,
	}

	results, err := t.client.Search(ctx, query)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "Found 0 results", nil
	}

	out := fmt.Sprintf("Found %d results:\n", len(results))
	for _, r := range results {
		out += fmt.Sprintf("  - [%.4f] %s\n", r.Score, r.Content)
	}
	return out, nil
}

type memoryStoreArgs struct {
	Content  string `json:"content"`
	WingName string `json:"wing_name"`
	RoomName string `json:"room_name"`
	HallType string `json:"hall_type,omitempty"`
}

type MemoryStoreTool struct {
	client Client
}

func NewMemoryStoreTool(c Client) *MemoryStoreTool {
	return &MemoryStoreTool{client: c}
}

func (t *MemoryStoreTool) Name() string        { return "memory_store" }
func (t *MemoryStoreTool) Description() string { return "Store information in the memory palace" }
func (t *MemoryStoreTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"content":{"type":"string","description":"Content to store"},"wing_name":{"type":"string","description":"Name of the wing (auto-created if needed)"},"room_name":{"type":"string","description":"Name of the room (auto-created if needed)"},"hall_type":{"type":"string","description":"Type of room: facts, log, or code"}},"required":["content","wing_name","room_name"]}`)
}

func (t *MemoryStoreTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a memoryStoreArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	hallType := a.HallType
	if hallType == "" {
		hallType = "facts"
	}

	wingID, err := t.findOrCreateWing(ctx, a.WingName)
	if err != nil {
		return "", fmt.Errorf("finding/creating wing: %w", err)
	}

	roomID, err := t.findOrCreateRoom(ctx, wingID, a.RoomName, hallType)
	if err != nil {
		return "", fmt.Errorf("finding/creating room: %w", err)
	}

	drawerID, err := t.client.Store(ctx, a.Content, wingID, roomID)
	if err != nil {
		return "", fmt.Errorf("storing content: %w", err)
	}

	return fmt.Sprintf("Stored in drawer %s (wing: %s, room: %s)", drawerID, a.WingName, a.RoomName), nil
}

func (t *MemoryStoreTool) findOrCreateWing(ctx context.Context, name string) (string, error) {
	return t.client.CreateWingIfNotExists(ctx, name)
}

func (t *MemoryStoreTool) findOrCreateRoom(ctx context.Context, wingID, name, hallType string) (string, error) {
	return t.client.CreateRoomIfNotExists(ctx, wingID, name, hallType)
}

var (
	_ cobot.Tool = (*MemorySearchTool)(nil)
	_ cobot.Tool = (*MemoryStoreTool)(nil)
)
