package memory

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

//go:embed embed_memory_search_params.json
var memorySearchParamsJSON []byte

//go:embed embed_memory_store_params.json
var memoryStoreParamsJSON []byte

//go:embed embed_l3_deep_search_params.json
var l3DeepSearchParamsJSON []byte

func decodeArgs(args json.RawMessage, v any) error {
	return json.Unmarshal(args, v)
}

type memorySearchArgs struct {
	Query string `json:"query"`
	Tier1 string `json:"tier1,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type MemorySearchTool struct {
	store *Store
}

func NewMemorySearchTool(s *Store) *MemorySearchTool {
	return &MemorySearchTool{store: s}
}

func (t *MemorySearchTool) Name() string { return "memory_search" }
func (t *MemorySearchTool) Description() string {
	return "Search the memory palace for relevant information"
}
func (t *MemorySearchTool) Parameters() json.RawMessage {
	return json.RawMessage(memorySearchParamsJSON)
}

func (t *MemorySearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a memorySearchArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}

	query := &cobot.SearchQuery{
		Text:  a.Query,
		Tier1: a.Tier1,
		Limit: a.Limit,
	}

	results, err := t.store.Search(ctx, query)
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
	store *Store
}

func NewMemoryStoreTool(s *Store) *MemoryStoreTool {
	return &MemoryStoreTool{store: s}
}

func (t *MemoryStoreTool) Name() string        { return "memory_store" }
func (t *MemoryStoreTool) Description() string { return "Store information in the memory palace" }
func (t *MemoryStoreTool) Parameters() json.RawMessage {
	return json.RawMessage(memoryStoreParamsJSON)
}

func (t *MemoryStoreTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a memoryStoreArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}

	hallType := a.HallType
	if hallType == "" {
		hallType = cobot.TagFacts
	}

	wingID, err := t.store.CreateWingIfNotExists(ctx, a.WingName)
	if err != nil {
		return "", fmt.Errorf("finding/creating wing: %w", err)
	}

	roomID, err := t.store.CreateRoomIfNotExists(ctx, wingID, a.RoomName, hallType)
	if err != nil {
		return "", fmt.Errorf("finding/creating room: %w", err)
	}

	drawerID, err := t.store.Store(ctx, a.Content, wingID, roomID)
	if err != nil {
		return "", fmt.Errorf("storing content: %w", err)
	}

	return fmt.Sprintf("Stored in drawer %s (wing: %s, room: %s)", drawerID, a.WingName, a.RoomName), nil
}

// --- L3DeepSearchTool ---

type l3SearchArgs struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type L3DeepSearchTool struct {
	store *Store
}

func NewL3DeepSearchTool(s *Store) *L3DeepSearchTool {
	return &L3DeepSearchTool{store: s}
}

func (t *L3DeepSearchTool) Name() string { return "l3_deep_search" }
func (t *L3DeepSearchTool) Description() string {
	return "Perform deep semantic search across all memory"
}
func (t *L3DeepSearchTool) Parameters() json.RawMessage {
	return json.RawMessage(l3DeepSearchParamsJSON)
}

func (t *L3DeepSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a l3SearchArgs
	if err := decodeArgs(args, &a); err != nil {
		return "", err
	}

	results, err := t.store.L3DeepSearch(ctx, a.Query, a.Limit)
	if err != nil {
		return "", err
	}

	if len(results) == 0 {
		return "Deep search found 0 results", nil
	}

	out := fmt.Sprintf("Deep search found %d results:\n", len(results))
	for _, r := range results {
		content := truncate(r.Content, 200)
		out += fmt.Sprintf("  - [%.4f][%s] %s\n", r.Score, r.Tier1, content)
	}
	return out, nil
}

var (
	_ cobot.Tool = (*MemorySearchTool)(nil)
	_ cobot.Tool = (*MemoryStoreTool)(nil)
	_ cobot.Tool = (*L3DeepSearchTool)(nil)
)
