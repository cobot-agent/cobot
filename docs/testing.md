# Testing Guide

## Overview

Cobot uses Go's standard testing framework (`testing` package) with comprehensive coverage across 29+ test files. This guide explains the testing strategy, patterns, and best practices.

## Test Organization

### Directory Structure

```
internal/
├── agent/
│   ├── loop_test.go      # Unit tests for agent loop
│   ├── e2e_test.go       # End-to-end integration tests
│   └── acp_test.go       # ACP scaffolding tests
├── memory/
│   ├── store_test.go     # Storage CRUD operations
│   ├── layers_test.go    # Memory layer tests (L0-L3)
│   ├── race_test.go      # Concurrency/race tests
│   ├── l3_test.go        # L3 deep search tests
│   └── knowledge_test.go # Knowledge graph tests
├── llm/
│   ├── openai/
│   │   ├── provider_test.go
│   │   └── stream_test.go # Streaming tool call assembly
│   └── anthropic/
│       └── provider_test.go
└── ... (other packages)
```

## Test Types

### 1. Unit Tests

Test individual functions/methods in isolation.

**Location**: `*_test.go` alongside source files
**Pattern**:
```go
func TestFunctionName(t *testing.T) {
    // Setup
    input := "test"
    expected := "result"
    
    // Execute
    result := FunctionName(input)
    
    // Assert
    if result != expected {
        t.Errorf("FunctionName(%q) = %q, want %q", input, result, expected)
    }
}
```

**Example**: `TestSummarizeContent` in `l3_test.go`

### 2. Integration Tests

Test component interactions with real dependencies.

**Pattern**:
```go
func TestStoreAndSearch(t *testing.T) {
    // Use real temporary storage
    dir := t.TempDir()
    s, err := OpenStore(dir)
    if err != nil {
        t.Fatal(err)
    }
    defer s.Close()
    
    ctx := context.Background()
    
    // Store content
    id, err := s.Store(ctx, "test content", wingID, roomID)
    if err != nil {
        t.Fatal(err)
    }
    
    // Search and verify
    results, err := s.Search(ctx, &cobot.SearchQuery{Text: "test"})
    if err != nil {
        t.Fatal(err)
    }
    
    if len(results) == 0 {
        t.Error("expected search results")
    }
}
```

**Example**: `TestWakeUpL2` in `layers_test.go`

### 3. E2E Tests

Test complete workflows through the public API.

**Location**: `e2e_test.go` files
**Pattern**:
```go
func TestE2EToolCallFlow(t *testing.T) {
    // Setup agent with mock provider
    a := New(&cobot.Config{MaxTurns: 10})
    a.SetProvider(&mockProvider{
        responses: []*cobot.ProviderResponse{
            {ToolCalls: []cobot.ToolCall{{...}}, StopReason: cobot.StopToolCalls},
            {Content: "Result", StopReason: cobot.StopEndTurn},
        },
    })
    a.ToolRegistry().Register(builtin.NewShellExecTool())
    
    // Execute full workflow
    resp, err := a.Prompt(context.Background(), "run echo hello")
    if err != nil {
        t.Fatal(err)
    }
    
    // Verify complete flow
    if !strings.Contains(resp.Content, "hello") {
        t.Errorf("expected 'hello' in response, got: %s", resp.Content)
    }
}
```

### 4. Race Condition Tests

Test concurrent access safety.

**Location**: `race_test.go`
**Pattern**:
```go
func TestCreateWingIfNotExists_RaceCondition(t *testing.T) {
    dir := t.TempDir()
    s, _ := OpenStore(dir)
    defer s.Close()
    
    ctx := context.Background()
    var wg sync.WaitGroup
    results := make(chan string, 10)
    
    // Spawn 10 concurrent goroutines
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            id, _ := s.CreateWingIfNotExists(ctx, "test-wing")
            results <- id
        }()
    }
    
    wg.Wait()
    close(results)
    
    // Verify all got same ID (no duplicates)
    var firstID string
    for id := range results {
        if firstID == "" {
            firstID = id
        } else if id != firstID {
            t.Error("race condition: got different IDs")
        }
    }
}
```

## Running Tests

### Basic Commands

```bash
# All tests
go test ./...

# Specific package
go test ./internal/memory/...

# Verbose output
go test -v ./internal/agent/...

# Specific test
go test -v -run TestCreateWingIfNotExists_RaceCondition ./internal/memory/

# Race detection
go test -race ./internal/memory/...

# Coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Count (disable cache)
go test -count=1 ./...

# Timeout
go test -timeout 30s ./...

# Parallel (with count)
go test -parallel 4 ./...
```

### CI/CD Pipeline

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.26.2'
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## Testing Best Practices

### 1. Use t.TempDir()

Always use `t.TempDir()` for temporary files (auto-cleanup):
```go
dir := t.TempDir()
s, err := OpenStore(dir)
defer s.Close()
```

### 2. Test Tables

Use table-driven tests for multiple cases:
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"short", "hi", "hi"},
    {"long", strings.Repeat("a", 300), strings.Repeat("a", 200) + "..."},
    {"empty", "", ""},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := SummarizeContent(tt.input)
        if result != tt.expected {
            t.Errorf("got %q, want %q", result, tt.expected)
        }
    })
}
```

### 3. Parallel Tests

Mark safe tests as parallel:
```go
func TestParallel(t *testing.T) {
    t.Parallel()
    // test code
}
```

### 4. Skip Long Tests

Use build tags or env vars for long tests:
```go
func TestLongRunning(t *testing.T) {
    if os.Getenv("LONG_TESTS") != "1" {
        t.Skip("Skipping long test")
    }
    // long test code
}
```

### 5. Mock External Dependencies

Use interfaces for mocking:
```go
type mockProvider struct {
    responses []*cobot.ProviderResponse
    callCount int
}

func (m *mockProvider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
    resp := m.responses[m.callCount]
    m.callCount++
    return resp, nil
}
```

### 6. Assert vs Require

Use `t.Fatal` for setup failures, `t.Error` for assertion failures:
```go
// Fatal - stop test immediately
s, err := OpenStore(dir)
if err != nil {
    t.Fatal(err)  // Can't continue without store
}

// Error - continue with other assertions
result, err := s.Search(ctx, query)
if err != nil {
    t.Error(err)  // Log error but check result too
}
if len(result) != expected {
    t.Errorf("got %d results, want %d", len(result), expected)
}
```

## Test Coverage Goals

| Package | Target Coverage |
|---------|----------------|
| `pkg/` | 90%+ |
| `internal/agent/` | 85%+ |
| `internal/memory/` | 90%+ |
| `internal/llm/` | 80%+ |
| `cmd/cobot/` | 70%+ |

## Debugging Tests

### Verbose Output
```bash
go test -v ./... 2>&1 | grep -A 10 "FAIL"
```

### Run Specific Test with Debug
```bash
go test -v -run TestWakeUpL3 ./internal/memory/
```

### CPU/Memory Profiling
```bash
go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Race Detection
```bash
go test -race ./... 2>&1 | tee race.log
```

## Common Issues

### Issue: Test flakiness due to timing
**Solution**: Use `t.TempDir()` and avoid time.Sleep when possible

### Issue: Database locked (Badger)
**Solution**: Ensure `defer s.Close()` is called

### Issue: Bleve indexing delay
**Solution**: Add `time.Sleep(100 * time.Millisecond)` after Store in tests

### Issue: Port conflicts in ACP tests
**Solution**: Use `:0` for auto-assigned port or mock network layer

## Testing Checklist

Before submitting PR:
- [ ] All tests pass (`go test ./...`)
- [ ] Race tests pass (`go test -race ./...`)
- [ ] Coverage maintained or improved
- [ ] New tests added for new features
- [ ] E2E tests added for user workflows
- [ ] Documentation updated (if needed)
