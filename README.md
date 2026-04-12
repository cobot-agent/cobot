# Cobot - Personal AI Agent System

[![Go Version](https://img.shields.io/badge/go-1.26.2-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Cobot is a Go-based personal AI agent system with memory, tools, and protocols. It implements the MemPalace architecture for hierarchical memory management and supports ACP (Agent Communication Protocol) and MCP (Model Context Protocol).

## 🌟 Features

### Core Capabilities
- **🧠 Hierarchical Memory (MemPalace)**: L0-L3 memory layers with WakeUp context building
- **🔧 Tool System**: Built-in tools + MCP integration for external tool servers
- **🤖 Multi-Provider**: OpenAI and Anthropic LLM provider support
- **💬 Interactive Modes**: CLI commands, TUI, and ACP server modes
- **📚 Skill System**: YAML-based skill definitions with step execution
- **⏰ Scheduler**: Cron-based task scheduling

### Memory Architecture (MemPalace)
- **Wings**: Top-level organization (projects, domains)
- **Rooms**: Contextual spaces within wings
- **Drawers**: Raw content storage
- **Closets**: Summarized/aggregated content
- **Knowledge Graph**: Temporal triples with inference

### Protocols
- **ACP**: Agent Communication Protocol (JSON-RPC 2.0 over stdio)
- **MCP**: Model Context Protocol for tool integration
- **SubAgent**: Spawn and coordinate sub-agents

## 🚀 Installation

### Prerequisites
- Go 1.26.2 or later
- Git

### From Source
```bash
git clone https://github.com/cobot-agent/cobot.git
cd cobot
go build -o cobot ./cmd/cobot
```

### Install to $GOPATH/bin
```bash
go install github.com/cobot-agent/cobot/cmd/cobot@latest
```

## 📝 Quick Start

### 1. Initialize Workspace
```bash
# Create a new project workspace
cd ~/my-project
cobot workspace init
```

### 2. Configure API Keys
```bash
# Set your OpenAI API key
cobot config set apikey.openai sk-xxx

# Or set environment variable
export OPENAI_API_KEY=sk-xxx
```

### 3. Start Chatting
```bash
# Interactive TUI mode
cobot chat

# One-shot command
cobot chat "What files are in this directory?"

# With specific model
cobot chat --model anthropic:claude-3-opus "Hello!"
```

## 🧪 Testing

Cobot uses Go's standard testing framework with comprehensive test coverage across 29+ test files.

### Test Structure

```
internal/
├── agent/
│   ├── loop_test.go        # Agent loop unit tests
│   ├── e2e_test.go         # End-to-end integration tests
│   └── acp_test.go         # ACP scaffolding tests
├── memory/
│   ├── store_test.go       # Storage CRUD tests
│   ├── layers_test.go      # Memory layer tests (L0-L3)
│   ├── race_test.go        # Race condition tests
│   ├── l3_test.go          # L3 deep search tests
│   └── knowledge_test.go   # Knowledge graph tests
├── llm/
│   ├── openai/
│   │   ├── provider_test.go
│   │   └── stream_test.go  # Streaming tests
│   └── anthropic/
│       └── provider_test.go
└── ... (29 test files total)
```

### Running Tests

#### All Tests
```bash
go test ./...
```

#### Specific Package
```bash
go test ./internal/memory/...
go test ./internal/agent/...
```

#### With Verbose Output
```bash
go test -v ./internal/memory/...
```

#### Race Detection
```bash
go test -race ./internal/memory/...
```

#### Specific Test
```bash
go test -v -run TestCreateWingIfNotExists_RaceCondition ./internal/memory/
```

#### Coverage Report
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Categories

#### 1. Unit Tests
Test individual functions in isolation:
```go
func TestSummarizeContent(t *testing.T) {
    s := &Store{}
    result := s.SummarizeContent("Long content here...")
    // assertions
}
```

#### 2. Integration Tests
Test component interactions:
```go
func TestStoreAndSearch(t *testing.T) {
    dir := t.TempDir()
    s, _ := OpenStore(dir)
    defer s.Close()
    
    // Store content
    s.Store(ctx, "test", wingID, roomID)
    
    // Search and verify
    results, _ := s.Search(ctx, query)
    // assertions
}
```

#### 3. E2E Tests
Test full agent workflows:
```go
func TestE2EToolCallFlow(t *testing.T) {
    a := New(&cobot.Config{MaxTurns: 10})
    a.SetProvider(mockProvider)
    a.ToolRegistry().Register(tool)
    
    resp, err := a.Prompt(ctx, "run echo hello")
    // Verify tool execution + response
}
```

#### 4. Race Condition Tests
Test concurrent access safety:
```go
func TestCreateWingIfNotExists_RaceCondition(t *testing.T) {
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            s.CreateWingIfNotExists(ctx, "test-wing")
        }()
    }
    wg.Wait()
    // Verify only 1 wing created
}
```

### Writing Tests (Best Practices)

1. **Use `t.TempDir()`** for temporary storage:
```go
dir := t.TempDir()
s, err := OpenStore(dir)
defer s.Close()
```

2. **Test table patterns** for multiple cases:
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"short", "hi", "hi"},
    {"long", strings.Repeat("a", 300), strings.Repeat("a", 200) + "..."},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := fn(tt.input)
        if result != tt.expected {
            t.Errorf("got %q, want %q", result, tt.expected)
        }
    })
}
```

3. **Parallel testing** where safe:
```go
t.Parallel()
```

4. **Cleanup with defer**:
```go
dir := t.TempDir()
s, _ := OpenStore(dir)
defer s.Close()
```

## 🏗️ Architecture

```
cobot/
├── api/acp/              # ACP protocol types
├── cmd/cobot/            # CLI commands
│   ├── chat.go
│   ├── config_cmd.go
│   ├── memory_cmd.go
│   └── ...
├── pkg/                  # Public SDK
│   ├── cobot.go          # Agent, AgentCore
│   ├── types.go          # Public types
│   └── interfaces.go     # Provider, Tool interfaces
└── internal/             # Internal implementation
    ├── agent/            # Core agent loop
    ├── memory/           # MemPalace storage
    ├── llm/              # LLM providers
    ├── tools/            # Tool registry
    ├── acp/              # ACP server
    ├── mcp/              # MCP client
    └── subagent/         # Sub-agent coordination
```

## 🔧 Configuration

### Config File Location
- **Global**: `~/.config/cobot/config.yaml`
- **Workspace**: `.cobot/config.yaml`

### Example Config
```yaml
model: openai:gpt-4o
max_turns: 10
system_prompt: "You are a helpful assistant"
temperature: 0.7
apikeys:
  openai: sk-xxx
  anthropic: sk-xxx
memory:
  enabled: true
tools:
  builtin:
    - filesystem_read
    - filesystem_write
    - shell_exec
```

### Environment Variables
```bash
COBOT_MODEL=openai:gpt-4o
COBOT_WORKSPACE=/path/to/workspace
OPENAI_API_KEY=sk-xxx
ANTHROPIC_API_KEY=sk-xxx
```

## 🛠️ Development

### Adding a New Tool

1. Create tool in `internal/tools/builtin/`:
```go
type MyTool struct{}

func (t *MyTool) Name() string { return "my_tool" }
func (t *MyTool) Description() string { return "Does something" }
func (t *MyTool) Parameters() json.RawMessage { ... }
func (t *MyTool) Execute(ctx context.Context, args json.RawMessage) (string, error) { ... }
```

2. Register in agent:
```go
agent.ToolRegistry().Register(&MyTool{})
```

3. Add tests in `*_test.go`

### Running Locally

```bash
# Build
go build -o cobot ./cmd/cobot

# Run with verbose logging
cobot chat --verbose

# Run ACP server
cobot acp serve
```

## 📝 CLI Commands

```bash
cobot chat [message]              # Interactive chat or one-shot
cobot model list                  # List available models
cobot workspace init              # Initialize workspace
cobot config show                 # Show full config
cobot config get <key>            # Get config value
cobot config set <key> <value>    # Set config value
cobot memory search <query>       # Search memory
cobot memory status               # Show memory stats
cobot tools list                  # List available tools
cobot acp serve                   # Start ACP server
cobot tui                         # Start TUI mode
cobot setup                       # First-time setup
cobot doctor                      # Diagnostic check
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for your changes
4. Ensure all tests pass (`go test ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

### Commit Message Format
```
feat: add new feature
fix: resolve race condition
docs: update README
test: add missing tests
refactor: simplify logic
```

## 📄 License

MIT License - see [LICENSE](LICENSE) file

## 🙏 Acknowledgments

- Inspired by MemPalace architecture
- Uses [Bubbletea](https://github.com/charmbracelet/bubbletea) for TUI
- Uses [BadgerDB](https://github.com/dgraph-io/badger) for storage
- Uses [Bleve](https://github.com/blevesearch/bleve) for search

---

**Ultraworked with [Sisyphus](https://github.com/code-yeongyu/oh-my-openagent)**
