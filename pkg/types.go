package cobot

import (
	"encoding/json"
	"time"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role           `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	ToolResult *ToolResult    `json:"tool_result,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	CallID string `json:"call_id"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type StopReason string

const (
	StopEndTurn         StopReason = "end_turn"
	StopMaxTokens       StopReason = "max_tokens"
	StopMaxTurnRequests StopReason = "max_turn_requests"
	StopCancelled       StopReason = "cancelled"
	StopRefusal         StopReason = "refusal"
)

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ProviderRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []ToolDef `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type ProviderResponse struct {
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason StopReason `json:"stop_reason"`
	Usage      Usage      `json:"usage"`
}

type ProviderChunk struct {
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done"`
	Usage    *Usage    `json:"usage,omitempty"`
}

type Event struct {
	Type     EventType `json:"type"`
	Content  string    `json:"content,omitempty"`
	ToolCall *ToolCall `json:"tool_call,omitempty"`
	Done     bool      `json:"done"`
	Error    string    `json:"error,omitempty"`
}

type EventType string

const (
	EventText       EventType = "text"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventDone       EventType = "done"
	EventError      EventType = "error"
)

type Wing struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Keywords []string `json:"keywords,omitempty"`
}

type Room struct {
	ID       string `json:"id"`
	WingID   string `json:"wing_id"`
	Name     string `json:"name"`
	HallType string `json:"hall_type"`
}

type Drawer struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Closet struct {
	ID        string   `json:"id"`
	RoomID    string   `json:"room_id"`
	DrawerIDs []string `json:"drawer_ids"`
	Summary   string   `json:"summary"`
}

type Triple struct {
	Subject   string     `json:"subject"`
	Predicate string     `json:"predicate"`
	Object    string     `json:"object"`
	ValidFrom time.Time  `json:"valid_from"`
	ValidTo   *time.Time `json:"valid_to,omitempty"`
}

type SearchQuery struct {
	Text     string `json:"text"`
	WingID   string `json:"wing_id,omitempty"`
	RoomID   string `json:"room_id,omitempty"`
	HallType string `json:"hall_type,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type SearchResult struct {
	DrawerID string  `json:"drawer_id"`
	Content  string  `json:"content"`
	WingID   string  `json:"wing_id"`
	RoomID   string  `json:"room_id"`
	Score    float64 `json:"score"`
}
