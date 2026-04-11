package cobot

import (
	"context"
	"encoding/json"
	"time"
)

type Provider interface {
	Name() string
	Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

type MemoryStore interface {
	Store(ctx context.Context, content string, wingID, roomID string) (string, error)
	Search(ctx context.Context, query *SearchQuery) ([]*SearchResult, error)
	GetWings(ctx context.Context) ([]*Wing, error)
	GetRooms(ctx context.Context, wingID string) ([]*Room, error)
	CreateWing(ctx context.Context, wing *Wing) error
	CreateRoom(ctx context.Context, room *Room) error
	AddDrawer(ctx context.Context, wingID, roomID, content string) (string, error)
	GetDrawer(ctx context.Context, id string) (*Drawer, error)
	WakeUp(ctx context.Context) (string, error)
	Close() error
}

type KnowledgeGraph interface {
	AddTriple(ctx context.Context, triple *Triple) error
	Invalidate(ctx context.Context, subject, predicate, object string, ended time.Time) error
	Query(ctx context.Context, entity string, asOf *time.Time) ([]*Triple, error)
	Timeline(ctx context.Context, entity string) ([]*Triple, error)
}
