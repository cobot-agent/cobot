package cobot

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Name() string
	Complete(ctx context.Context, req *ProviderRequest) (*ProviderResponse, error)
	Stream(ctx context.Context, req *ProviderRequest) (<-chan ProviderChunk, error)
}

// ModelResolver resolves a "provider:model" spec into a Provider and model name.
// Implemented by llm.Registry; used by Agent for multi-provider model switching.
type ModelResolver interface {
	ProviderForModel(modelSpec string) (Provider, string, error)
}

type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolRegistry manages available tools. Implemented by tools.Registry.
type ToolRegistry interface {
	Register(tool Tool)
	ToolDefs() []ToolDef
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)
	ExecuteParallel(ctx context.Context, calls []ToolCall) []*ToolResult
	Clone() ToolRegistry
}

// SubAgent is a minimal interface for agent delegation, allowing tools to
// invoke sub-agents without importing the agent package directly.
type SubAgent interface {
	SetModel(spec string) error
	Prompt(ctx context.Context, message string) (*ProviderResponse, error)
}

// MemoryStore handles persistence: storing content and searching it.
type MemoryStore interface {
	Store(ctx context.Context, content, tier1, tier2 string) (string, error)
	Search(ctx context.Context, query *SearchQuery) ([]*SearchResult, error)
	Close() error
}

// MemoryRecall handles prompt assembly from stored memories. Implementations
// can be swapped independently from the storage backend, allowing different
// recall strategies (e.g. facts-only, deep-search, RAG) without changing
// how content is persisted.
type MemoryRecall interface {
	WakeUp(ctx context.Context) (string, error)
}
