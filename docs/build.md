# Build and Development Guide

This guide covers the build system, development workflow, and release process for Cobot.

## Build System

Cobot uses a **Makefile-based build system** with Go toolchain.

### Prerequisites

- Go 1.26.2 or later
- Make (GNU Make 3.81+ or BSD Make)
- Git

### Build Targets

| Target | Description |
|--------|-------------|
| `build` | Build development binary |
| `build-release` | Build optimized release binary |
| `build-all` | Build for all platforms (darwin/linux, amd64/arm64) |
| `dev` | Build with debug symbols |
| `test` | Run all tests |
| `test-race` | Run tests with race detection |
| `test-coverage` | Run tests with coverage report |
| `install` | Install to `$GOPATH/bin` |
| `install-system` | Install to `/usr/local/bin` |
| `release` | Create release archives |
| `clean` | Clean build artifacts |
| `check` | Run fmt + vet + test |

### Quick Start

```bash
# Build for development
make build

# Run tests
make test

# Install locally
make install

# Full verification
make check
```

## Development Workflow

### 1. Setup Development Environment

```bash
# Clone repository
git clone https://github.com/cobot-agent/cobot.git
cd cobot

# Download dependencies
make setup-dev

# Verify build
make build
```

### 2. Development Cycle

```bash
# Make changes to code...

# Format code
make fmt

# Run checks
make vet

# Run tests
make test

# Build and test
make check
```

### 3. Debug Build

```bash
# Build with debug symbols
make dev

# Run with debugger
dlv exec ./build/cobot-debug -- chat
```

## Project Structure

```
cobot/
├── cmd/cobot/              # CLI entry point
│   ├── main.go
│   ├── root.go
│   ├── chat.go
│   ├── persona_cmd.go
│   └── ...
├── internal/               # Internal packages
│   ├── agent/              # Core agent loop
│   ├── memory/             # MemPalace storage
│   ├── persona/            # Persona management
│   ├── workspace/          # Workspace utilities
│   └── ...
├── pkg/                    # Public SDK
├── api/                    # API definitions
├── docs/                   # Documentation
├── Makefile               # Build system
└── go.mod                 # Go module
```

## Testing Strategy

### Test Organization

```
internal/
├── agent/
│   ├── loop_test.go       # Unit tests
│   ├── e2e_test.go        # End-to-end tests
│   └── acp_test.go        # Protocol tests
├── memory/
│   ├── store_test.go      # Storage tests
│   ├── race_test.go       # Race condition tests
│   └── layers_test.go     # Memory layer tests
└── ...
```

### Running Tests

```bash
# All tests
make test

# Specific package
go test -v ./internal/memory/...

# Specific test
go test -v -run TestCreateWing ./internal/memory/

# With race detection
make test-race

# Coverage report
make test-coverage
# Opens: coverage.html
```

### Test Patterns

```go
// Unit test
func TestFunction(t *testing.T) {
    input := "test"
    expected := "result"
    
    result := Function(input)
    
    if result != expected {
        t.Errorf("Function(%q) = %q, want %q", input, result, expected)
    }
}

// Table-driven test
func TestFunctionTable(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"short", "hi", "hi"},
        {"long", "hello", "hello"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Function(tt.input)
            if result != tt.expected {
                t.Errorf("got %q, want %q", result, tt.expected)
            }
        })
    }
}

// Integration test
func TestIntegration(t *testing.T) {
    dir := t.TempDir()
    s, err := OpenStore(dir)
    if err != nil {
        t.Fatal(err)
    }
    defer s.Close()
    
    // Test with real storage
}
```

## Release Process

### 1. Version Bump

```bash
# Update version (semantic versioning)
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
```

### 2. Build Release

```bash
# Clean and build
make clean
make release

# Verify artifacts
ls -la build/release/
```

### 3. Test Release

```bash
# Test each platform binary
for binary in build/cobot-*; do
    echo "Testing $binary"
    $binary --version
done
```

### 4. Create Release

```bash
# Create GitHub release (using gh CLI)
gh release create v0.2.0 \
  --title "Cobot v0.2.0" \
  --notes "Release notes..." \
  build/release/*.tar.gz
```

## Platform Support

### Supported Platforms

| OS | Architecture | Status |
|----|--------------|--------|
| macOS | amd64 | ✅ Supported |
| macOS | arm64 (Apple Silicon) | ✅ Supported |
| Linux | amd64 | ✅ Supported |
| Linux | arm64 | ✅ Supported |
| Windows | amd64 | ⚠️ Experimental |

### Cross-Compilation

```bash
# Build for specific platform
GOOS=linux GOARCH=amd64 make build

# Build all platforms
make build-all
```

## Dependencies

### Core Dependencies

```go
// go.mod
require (
    github.com/dgraph-io/badger/v4 v4.9.1    // Storage
    github.com/blevesearch/bleve/v2 v2.5.7   // Search
    github.com/spf13/cobra v1.10.2           // CLI
    github.com/charmbracelet/bubbletea v1.3.10 // TUI
    // ...
)
```

### Updating Dependencies

```bash
# Update all dependencies
go get -u ./...
go mod tidy

# Update specific dependency
go get github.com/some/package@latest
go mod tidy
```

## CI/CD

### GitHub Actions Workflow

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26.2'
      
      - name: Build
        run: make build
      
      - name: Test
        run: make test-race
      
      - name: Coverage
        run: make test-coverage
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26.2'
      
      - name: Build
        run: make build
```

### Release Workflow

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26.2'
      
      - name: Build all platforms
        run: make release
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: build/release/*
```

## Performance Optimization

### Build Optimization

```bash
# Release build (stripped, optimized)
go build -trimpath -ldflags "-s -w" -o cobot ./cmd/cobot

# Smaller binary with upx (optional)
upx --best cobot
```

### Binary Size Analysis

```bash
# Check binary size
ls -lh build/cobot

# Analyze size breakdown
go tool nm -size build/cobot | sort -k2 -n -r | head -20
```

## Troubleshooting

### Build Issues

```bash
# Clean everything
make clean

# Rebuild from scratch
make setup-dev
make build

# Verbose build
go build -v ./...
```

### Test Issues

```bash
# Clear test cache
go clean -testcache

# Run with verbose output
go test -v ./...

# Debug specific test
go test -v -run TestName -count=1 ./package
```

### Module Issues

```bash
# Verify modules
go mod verify

# Tidy modules
go mod tidy

# Download dependencies
go mod download
```

## Best Practices

1. **Always run tests before committing**: `make check`
2. **Use table-driven tests** for multiple test cases
3. **Use `t.TempDir()`** for temporary files (auto-cleanup)
4. **Run race detection** before releasing: `make test-race`
5. **Keep binaries small**: Use release build flags
6. **Tag releases**: Use semantic versioning (v0.1.0)
7. **Write tests for new features**: Maintain coverage
8. **Document public APIs**: Use Go doc comments

## Resources

- [Testing Guide](testing.md)
- [Verification Guide](verification.md)
- [User Guide](user-guide.md)
- [Go Testing](https://golang.org/pkg/testing/)
- [Go Modules](https://golang.org/ref/mod)
