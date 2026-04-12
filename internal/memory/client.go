package memory

import (
	"context"

	cobot "github.com/cobot-agent/cobot/pkg"
)

// Client is the interface for interacting with the memory system.
// Both local *Store and remote daemon.RemoteStore implement this interface.
type Client interface {
	// Core operations
	Store(ctx context.Context, content string, wingID, roomID string) (string, error)
	Search(ctx context.Context, query *cobot.SearchQuery) ([]*cobot.SearchResult, error)

	// Wing operations
	GetWings(ctx context.Context) ([]*cobot.Wing, error)
	GetWingByName(ctx context.Context, name string) (*cobot.Wing, error)
	CreateWingIfNotExists(ctx context.Context, name string) (string, error)

	// Room operations
	GetRooms(ctx context.Context, wingID string) ([]*cobot.Room, error)
	GetRoomByName(ctx context.Context, wingID, name string) (*cobot.Room, error)
	CreateRoomIfNotExists(ctx context.Context, wingID, name, hallType string) (string, error)

	// Layer stack
	WakeUp(ctx context.Context) (string, error)

	// Deep search
	L3DeepSearch(ctx context.Context, query string, limit int) ([]*cobot.SearchResult, error)

	// Summarization
	AutoSummarizeRoom(ctx context.Context, wingID, roomID string) error

	// Lifecycle
	Close() error
}

// Verify *Store implements Client at compile time.
var _ Client = (*Store)(nil)
