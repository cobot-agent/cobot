# Verification Guide

This guide covers how to verify that Cobot Personal Agent is correctly built, configured, and functioning.

## Pre-Flight Checks

### 1. Build Verification

```bash
# Clean build
make clean

# Build binary
make build

# Verify binary exists
ls -la build/cobot

# Check binary can execute
./build/cobot --version
```

### 2. Test Verification

```bash
# Run all tests
make test

# Run with race detection
make test-race

# Run with coverage
make test-coverage
```

Expected: All tests pass, no race conditions detected.

### 3. Code Quality

```bash
# Format check
make fmt

# Vet check
make vet

# Full check
make check
```

## Installation Verification

### Local Install ($GOPATH/bin)

```bash
make install

# Verify in PATH
which cobot
cobot --version
```

### System Install (/usr/local/bin)

```bash
make install-system

# Verify
which cobot
cobot --version
```

## Runtime Verification

### 1. Configuration Check

```bash
cobot doctor
```

Expected output:
```
Cobot Personal Agent Doctor
===========================

Config directory: /Users/you/.config/cobot
  [OK] Directory exists
  [OK] Config file: /Users/you/.config/cobot/config.yaml
  [OK] Model: openai:gpt-4o
  [OK] API keys configured: [openai]

Persona files:
  [OK] SOUL:   /Users/you/.config/cobot/SOUL.md
  [OK] USER:   /Users/you/.config/cobot/USER.md
  [OK] MEMORY: /Users/you/.config/cobot/MEMORY.md

Data directory: /Users/you/.local/share/cobot
  [OK] Directory exists
  [OK] Memory dir: /Users/you/.local/share/cobot/memory

All critical checks passed!
```

### 2. First-Time Setup

```bash
# Run setup (safe to run multiple times)
cobot setup

# Verify files created
ls -la ~/.config/cobot/
ls -la ~/.local/share/cobot/
```

Expected files:
- `~/.config/cobot/config.yaml`
- `~/.config/cobot/SOUL.md`
- `~/.config/cobot/USER.md`
- `~/.config/cobot/MEMORY.md`
- `~/.local/share/cobot/memory/`

### 3. API Key Configuration

```bash
# Set API key
cobot config set apikey.openai sk-your-test-key

# Verify
cobot config get apikey.openai
```

### 4. Basic Functionality

```bash
# Test help
cobot --help

# Test persona commands
cobot persona show soul
cobot persona show user

# Test memory commands
cobot memory status
```

### 5. Chat Test (requires API key)

```bash
# One-shot test (non-interactive)
cobot chat "Say 'Cobot is working' and nothing else"
```

Expected: Response containing "Cobot is working"

## Directory Structure Verification

### XDG Compliance Check

```bash
# Verify config directory
echo $XDG_CONFIG_HOME  # Should be empty or ~/.config
ls -la ~/.config/cobot/

# Verify data directory
echo $XDG_DATA_HOME  # Should be empty or ~/.local/share
ls -la ~/.local/share/cobot/
```

### File Permissions

```bash
# Config files should be readable/writable by user
ls -l ~/.config/cobot/

# Data directories should be accessible
ls -ld ~/.local/share/cobot/
ls -ld ~/.local/share/cobot/memory/
```

## Integration Verification

### 1. Memory Persistence

```bash
# Store something in memory (via chat)
cobot chat "Remember that my favorite color is blue"

# Search for it
cobot memory search "favorite color"
```

Expected: Search returns the conversation about favorite color.

### 2. Cross-Directory Operation

```bash
# Test from different directories
cd /tmp
cobot memory status

cd ~
cobot memory status

cd /var/tmp
cobot memory status
```

Expected: Same memory status from all directories (global memory).

### 3. Persona Persistence

```bash
# Edit SOUL
cobot persona edit soul
# (add "I love testing" to the file)

# Verify it persists
cobot persona show soul | grep "I love testing"
```

## Release Verification

### Multi-Platform Build

```bash
# Build for all platforms
make build-all

# Verify binaries exist
ls -la build/cobot-*

# Check binary sizes (should be reasonable)
du -h build/cobot-*
```

### Release Package

```bash
# Create release
make release

# Verify archives
ls -la build/release/

# Test extraction
cd /tmp
tar xzf /path/to/cobot-v1.0.0-darwin-arm64.tar.gz
./cobot --version
```

## Troubleshooting

### Build Failures

```bash
# Check Go version
go version  # Should be 1.26.2+

# Clean module cache
go clean -modcache
go mod download

# Verbose build
go build -v ./...
```

### Test Failures

```bash
# Run specific failing test
go test -v -run TestName ./package/

# Check for race conditions
go test -race ./...

# Fresh test (no cache)
go test -count=1 ./...
```

### Runtime Issues

```bash
# Check binary dependencies
ldd build/cobot  # Linux
otool -L build/cobot  # macOS

# Check for missing dynamic libraries
./build/cobot 2>&1 | head -20
```

## CI/CD Verification

### GitHub Actions (if configured)

```bash
# Install act for local testing
brew install act

# Run CI locally
act push
```

### Manual CI Steps

```bash
# Simulate CI pipeline
make clean
make check
make test-race
make build-release
make test-coverage
```

## Verification Checklist

Before releasing or deploying:

- [ ] `make build` succeeds
- [ ] `make test` passes (all packages)
- [ ] `make test-race` passes (no race conditions)
- [ ] `make check` passes (fmt + vet)
- [ ] Binary runs: `./build/cobot --version`
- [ ] `cobot doctor` shows all green
- [ ] `cobot setup` creates all required files
- [ ] Persona files are editable and persist
- [ ] Memory commands work
- [ ] Chat works (if API key configured)
- [ ] Multi-platform builds succeed (if releasing)

## Performance Verification

### Binary Size

```bash
# Check binary size
ls -lh build/cobot

# Should be under 50MB for release build
```

### Memory Usage

```bash
# Monitor memory during operation
time ./build/cobot memory status
```

### Startup Time

```bash
# Time startup
time ./build/cobot --help
```

Expected: Sub-second startup time.
