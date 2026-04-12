package memory

import (
	"context"
	"encoding/json"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type memorySummarizeArgs struct {
	WingName string `json:"wing_name"`
	RoomName string `json:"room_name"`
}

type MemorySummarizeTool struct {
	store *Store
}

func NewMemorySummarizeTool(s *Store) *MemorySummarizeTool {
	return &MemorySummarizeTool{store: s}
}

func (t *MemorySummarizeTool) Name() string { return "memory_summarize" }
func (t *MemorySummarizeTool) Description() string {
	return "Auto-summarize a room's contents into a closet"
}
func (t *MemorySummarizeTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"wing_name":{"type":"string","description":"Name of the wing containing the room"},"room_name":{"type":"string","description":"Name of the room to summarize"}},"required":["wing_name","room_name"]}`)
}

func (t *MemorySummarizeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a memorySummarizeArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}

	wing, err := t.store.GetWingByName(ctx, a.WingName)
	if err != nil {
		return "", fmt.Errorf("finding wing: %w", err)
	}
	if wing == nil {
		return "", fmt.Errorf("wing not found: %s", a.WingName)
	}

	room, err := t.store.GetRoomByName(ctx, wing.ID, a.RoomName)
	if err != nil {
		return "", fmt.Errorf("finding room: %w", err)
	}
	if room == nil {
		return "", fmt.Errorf("room not found: %s", a.RoomName)
	}

	if err := t.store.AutoSummarizeRoom(ctx, wing.ID, room.ID); err != nil {
		return "", fmt.Errorf("summarizing room: %w", err)
	}

	return fmt.Sprintf("Summarized room %s/%s", a.WingName, a.RoomName), nil
}

var _ cobot.Tool = (*MemorySummarizeTool)(nil)
