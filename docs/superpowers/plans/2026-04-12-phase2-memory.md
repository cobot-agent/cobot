# Phase 2: Memory System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the complete memory subsystem — BadgerDB-backed palace storage (wings, rooms, closets, drawers), Bleve full-text search, L0-L3 memory stack, knowledge graph, and memory tools — enabling the agent to store, recall, and search conversation data.

**Architecture:** BadgerDB stores all palace data with key prefixes (wing:, room:, drawer:, closet:, kg:). Bleve provides full-text search over drawer contents. The memory stack layers load context at different depths (L0 identity, L1 facts, L2 recent, L3 deep search). Knowledge graph stores temporal entity-relation triples.

**Tech Stack:** BadgerDB v4 (embedded KV), Bleve v2 (full-text search), Go 1.26

**Design Spec:** `docs/specs/2026-04-12-cobot-agent-system-design.md` Section 4

---

## File Structure

```
internal/memory/
├── store.go              # MemoryStore interface + Store struct (BadgerDB + Bleve)
├── store_test.go         # Integration tests for full store
├── badger.go             # BadgerDB open/close, key prefix helpers
├── wings.go              # Wing CRUD operations
├── rooms.go              # Room CRUD operations
├── drawers.go            # Drawer CRUD operations
├── closets.go            # Closet CRUD operations
├── search.go             # Bleve index setup, search queries
├── layers.go             # L0-L3 memory stack (WakeUp)
├── knowledge.go          # Knowledge graph (temporal triples)
├── knowledge_test.go     # Knowledge graph tests
├── miner.go              # Mine conversations into memory
├── memory_tool.go        # Built-in tools: memory_search, memory_store
├── memory_tool_test.go   # Tool tests
pkg/
├── types.go              # ADD: Wing, Room, Drawer, Closet, Triple, SearchQuery, etc.
├── interfaces.go         # ADD: MemoryStore, KnowledgeGraph interfaces
```

---

### Task 1: Memory Types in pkg/types.go

**Files:**
- Modify: `pkg/types.go`
- Modify: `pkg/interfaces.go`

- [ ] **Step 1: Add memory types to pkg/types.go**

Append after the existing types:

```go
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
	DrawerID  string  `json:"drawer_id"`
	Content   string  `json:"content"`
	WingID    string  `json:"wing_id"`
	RoomID    string  `json:"room_id"`
	Score     float64 `json:"score"`
}
```

Add `"time"` to imports.

- [ ] **Step 2: Add MemoryStore and KnowledgeGraph interfaces to pkg/interfaces.go**

```go
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
```

Add `"time"` to imports.

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add pkg/types.go pkg/interfaces.go
git commit -m "feat: add memory system types and interfaces"
```

---

### Task 2: BadgerDB Store Foundation

**Files:**
- Create: `internal/memory/badger.go`
- Create: `internal/memory/store.go`
- Test: `internal/memory/store_test.go`

This task creates the Store struct that holds BadgerDB + Bleve, with Open/Close lifecycle and key prefix helpers.

- [ ] **Step 1: Create internal/memory/badger.go**

BadgerDB helpers: key prefix constants, open/close, ID generation.

```go
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
)

const (
	prefixWing    = "wing:"
	prefixRoom    = "room:"
	prefixDrawer  = "drawer:"
	prefixCloset  = "closet:"
	prefixKG      = "kg:"
)

func openBadger(dir string) (*badger.DB, error) {
	if err := mkdirAll(dir); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil
	return badger.Open(opts)
}

func mkdirAll(dir string) error {
	return mkdirAllImpl(dir)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func badgerPath(baseDir string) string {
	return filepath.Join(baseDir, "badger")
}
```

Actually, use the stdlib `os.MkdirAll` directly and keep it simple. The full implementation:

```go
package memory

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
)

const (
	prefixWing   = "wing:"
	prefixRoom   = "room:"
	prefixDrawer = "drawer:"
	prefixCloset = "closet:"
	prefixKG     = "kg:"
)

func openBadger(dir string) (*badger.DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil
	return badger.Open(opts)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func badgerPath(base string) string {
	return filepath.Join(base, "badger")
}
```

- [ ] **Step 2: Create internal/memory/store.go**

```go
package memory

import (
	"context"
	"os"
	"path/filepath"

	cobot "github.com/cobot-agent/cobot/pkg"
	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	db      *badger.DB
	bleveDir string
}

func OpenStore(memoryDir string) (*Store, error) {
	db, err := openBadger(badgerPath(memoryDir))
	if err != nil {
		return nil, err
	}
	bleveDir := filepath.Join(memoryDir, "bleve")
	os.MkdirAll(bleveDir, 0755)
	return &Store{db: db, bleveDir: bleveDir}, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *badger.DB {
	return s.db
}

func (s *Store) GetWings(ctx context.Context) ([]*cobot.Wing, error) {
	var wings []*cobot.Wing
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		prefix := []byte(prefixWing)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var w cobot.Wing
			if err := item.Value(func(val []byte) error {
				return jsonUnmarshal(val, &w)
			}); err != nil {
				return err
			}
			wings = append(wings, &w)
		}
		return nil
	})
	return wings, err
}
```

- [ ] **Step 3: Create internal/memory/json.go** (JSON helpers to avoid import in multiple files)

```go
package memory

import "encoding/json"

func jsonMarshal(v any) ([]byte, error)    { return json.Marshal(v) }
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
```

- [ ] **Step 4: Write basic store test**

```go
package memory

import (
	"testing"
)

func TestOpenCloseStore(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewID(t *testing.T) {
	id1 := newID()
	id2 := newID()
	if id1 == id2 {
		t.Error("expected different IDs")
	}
	if len(id1) != 16 {
		t.Errorf("expected 16-char ID, got %d", len(id1))
	}
}
```

- [ ] **Step 5: Add badger dependency**

```bash
go get github.com/dgraph-io/badger/v4@latest
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/memory/ -v`

- [ ] **Step 7: Commit**

```bash
git add internal/memory/
git commit -m "feat: add memory store with BadgerDB foundation"
```

---

### Task 3: Wing and Room CRUD

**Files:**
- Create: `internal/memory/wings.go`
- Create: `internal/memory/rooms.go`
- Test: `internal/memory/wings_test.go`

- [ ] **Implement wings.go** with CreateWing, GetWing, GetWings using BadgerDB prefix scan on `wing:` keys.

- [ ] **Implement rooms.go** with CreateRoom, GetRoom, GetRooms(wingID) using prefix scan on `room:<wingID>:` keys.

- [ ] **Test:** CreateWing + GetWings roundtrip, CreateRoom + GetRooms filtered by wing.

- [ ] **Commit:** `git commit -m "feat: add wing and room CRUD operations"`

---

### Task 4: Drawer and Closet CRUD

**Files:**
- Create: `internal/memory/drawers.go`
- Create: `internal/memory/closets.go`
- Test: `internal/memory/drawers_test.go`

- [ ] **Implement drawers.go** with AddDrawer (generates ID, stores content under `drawer:<id>`), GetDrawer.

- [ ] **Implement closets.go** with CreateCloset (summary + drawer IDs under `closet:<roomID>:<id>`).

- [ ] **Test:** AddDrawer + GetDrawer roundtrip, CreateCloset with drawer references.

- [ ] **Commit:** `git commit -m "feat: add drawer and closet CRUD operations"`

---

### Task 5: Bleve Search Index

**Files:**
- Create: `internal/memory/search.go`
- Test: `internal/memory/search_test.go`

- [ ] **Implement search.go:**
  - `openBleveIndex(bleveDir)` — creates/opens Bleve index with field mappings (content:text, wing_id:keyword, room_id:keyword, hall_type:keyword, created_at:datetime)
  - `indexDrawer(bleveIndex, drawerDoc)` — indexes a DrawerDocument
  - `searchDrawers(bleveIndex, query)` — full-text + filter query, returns ranked results
  - DrawerDocument struct for Bleve: {ID, Content, WingID, RoomID, HallType, CreatedAt}

- [ ] **Add bleve dependency:** `go get github.com/blevesearch/bleve/v2@latest`

- [ ] **Test:** Index drawers, search with text query, search with wing filter.

- [ ] **Commit:** `git commit -m "feat: add Bleve full-text search for memory drawers"`

---

### Task 6: Memory Stack (L0-L3)

**Files:**
- Create: `internal/memory/layers.go`
- Test: `internal/memory/layers_test.go`

- [ ] **Implement layers.go:**

```go
func (s *Store) WakeUp(ctx context.Context) (string, error)
```

WakeUp builds the L0+L1 context:
- L0: Load personality from workspace AGENTS.md (if set)
- L1: Load all closet summaries from rooms with hall_type="facts"
- Concatenate into a single context string (~170 tokens max)

- [ ] **Test:** WakeUp returns concatenated identity + facts from stored closets.

- [ ] **Commit:** `git commit -m "feat: add L0-L3 memory stack with WakeUp"`

---

### Task 7: Knowledge Graph

**Files:**
- Create: `internal/memory/knowledge.go`
- Test: `internal/memory/knowledge_test.go`

- [ ] **Implement knowledge.go:**

Key scheme: `kg:<subject>:<predicate>:<object>` → JSON-encoded Triple

- `AddTriple(ctx, triple)` — Set key with triple data
- `Invalidate(ctx, subject, predicate, object, ended)` — Set ValidTo on matching triple
- `Query(ctx, entity, asOf)` — Prefix scan `kg:<entity>:`, filter by asOf date
- `Timeline(ctx, entity)` — All triples for entity sorted by ValidFrom

- [ ] **Test:** AddTriple, Query finds it, Invalidate sets ValidTo, Query respects time.

- [ ] **Commit:** `git commit -m "feat: add knowledge graph with temporal triples"`

---

### Task 8: Full Store Integration + Search Wire-up

**Files:**
- Modify: `internal/memory/store.go` — wire Store() method that creates wing/room/closet/drawer + indexes
- Test: `internal/memory/store_test.go`

- [ ] **Implement Store.Store()** — high-level method that takes content, wingID, roomID:
  1. AddDrawer with the raw content
  2. Index the drawer in Bleve
  3. Return drawer ID

- [ ] **Implement Store.Search()** — queries Bleve, returns SearchResults with content from BadgerDB

- [ ] **Integration test:** Store content → Search → find it → verify ranking.

- [ ] **Commit:** `git commit -m "feat: wire full store with search integration"`

---

### Task 9: Memory Built-in Tools

**Files:**
- Create: `internal/memory/memory_tool.go`
- Test: `internal/memory/memory_tool_test.go`

- [ ] **Implement memory_search tool:**

```go
type MemorySearchTool struct { Store *Store }
func (t *MemorySearchTool) Name() string { return "memory_search" }
// Parameters: query (string), wing_id (optional), limit (optional, default 10)
// Execute: calls Store.Search, returns formatted results
```

- [ ] **Implement memory_store tool:**

```go
type MemoryStoreTool struct { Store *Store }
func (t *MemoryStoreTool) Name() string { return "memory_store" }
// Parameters: content (string), wing_name (string), room_name (string), hall_type (string)
// Execute: auto-creates wing/room if needed, calls Store.Store
```

- [ ] **Test:** Register tools with tool registry, execute via mock ToolCall.

- [ ] **Commit:** `git commit -m "feat: add memory_search and memory_store built-in tools"`

---

### Task 10: Final Verification

- [ ] **Step 1:** `go build ./...`
- [ ] **Step 2:** `go test ./... -count=1`
- [ ] **Step 3:** `go vet ./...`
- [ ] **Step 4:** Commit if any fixes needed
