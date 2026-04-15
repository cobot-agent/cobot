# Makefile for Cobot Personal Agent
# Provides build, test, and release workflows

.PHONY: all build test clean install lint fmt vet release

# Variables
BINARY_NAME := cobot
MAIN_PACKAGE := ./cmd/cobot
BUILD_DIR := ./build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Default target
all: build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for current platform with optimizations
build-release:
	@echo "Building release binary..."
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Built release: $(BUILD_DIR)/$(BINARY_NAME)"

# Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Quick test (no cache)
test-fresh:
	@echo "Running fresh tests..."
	go test -count=1 ./...

# Install to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

# Install locally to /usr/local/bin (requires sudo)
install-system: build-release
	@echo "Installing to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	go clean
	@echo "Cleaned"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run linter (requires golangci-lint)
lint: fmt vet
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

# Check for issues (fmt + vet + test)
check: fmt vet test
	@echo "All checks passed!"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	@echo "Built all platforms in $(BUILD_DIR)/"

# Create release archives
release: clean build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/release
	cd $(BUILD_DIR) && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 && \
		tar czf release/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	@echo "Release archives created in $(BUILD_DIR)/release/"

# Development build with debug info
dev:
	@echo "Building development version..."
	go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(MAIN_PACKAGE)
	@echo "Built debug version: $(BUILD_DIR)/$(BINARY_NAME)-debug"

# Run the binary
run: build
	$(BUILD_DIR)/$(BINARY_NAME)

# Setup development environment
setup-dev:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@echo "Development environment ready"

# Show help
help:
	@echo "Cobot Personal Agent - Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build          Build the binary (default)"
	@echo "  build-release  Build optimized release binary"
	@echo "  build-all      Build for all platforms"
	@echo "  dev            Build with debug symbols"
	@echo ""
	@echo "Test Targets:"
	@echo "  test           Run all tests"
	@echo "  test-race      Run tests with race detection"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  test-fresh     Run tests without cache"
	@echo ""
	@echo "Quality Targets:"
	@echo "  fmt            Format code"
	@echo "  vet            Run go vet"
	@echo "  lint           Run linter"
	@echo "  check          Run fmt + vet + test"
	@echo ""
	@echo "Install Targets:"
	@echo "  install        Install to \$$GOPATH/bin"
	@echo "  install-system Install to /usr/local/bin"
	@echo ""
	@echo "Release Targets:"
	@echo "  release        Create release archives for all platforms"
	@echo ""
	@echo "Utility Targets:"
	@echo "  clean          Clean build artifacts"
	@echo "  run            Build and run"
	@echo "  setup-dev      Setup development environment"
	@echo "  help           Show this help"
