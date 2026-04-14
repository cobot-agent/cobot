package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cobot-agent/cobot/internal/config"
)

func TestLoadRegistryMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	yaml1 := []byte(`name: filesystem
transport: command
command: npx
args:
  - "-y"
  - "@modelcontextprotocol/server-filesystem"
  - "/tmp"
`)
	if err := os.WriteFile(filepath.Join(dir, "filesystem.yaml"), yaml1, 0644); err != nil {
		t.Fatal(err)
	}

	yaml2 := []byte(`name: github
transport: command
command: npx
args:
  - "-y"
  - "@modelcontextprotocol/server-github"
env:
  GITHUB_TOKEN: "ghp_test123"
`)
	if err := os.WriteFile(filepath.Join(dir, "github.yaml"), yaml2, 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(registry) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(registry))
	}

	fs, ok := registry["filesystem"]
	if !ok {
		t.Fatal("expected 'filesystem' entry")
	}
	if fs.Transport != "command" {
		t.Fatalf("expected transport 'command', got %q", fs.Transport)
	}
	if fs.Command != "npx" {
		t.Fatalf("expected command 'npx', got %q", fs.Command)
	}
	if len(fs.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(fs.Args))
	}

	gh, ok := registry["github"]
	if !ok {
		t.Fatal("expected 'github' entry")
	}
	if gh.Env["GITHUB_TOKEN"] != "ghp_test123" {
		t.Fatalf("expected GITHUB_TOKEN 'ghp_test123', got %q", gh.Env["GITHUB_TOKEN"])
	}
}

func TestLoadRegistrySkipsNonYAML(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	yamlData := []byte(`name: test
transport: command
command: echo
`)
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), yamlData, 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(registry) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(registry))
	}
	if _, ok := registry["test"]; !ok {
		t.Fatal("expected 'test' entry")
	}
}

func TestLoadRegistryEnvExpansion(t *testing.T) {
	t.Setenv("MY_SECRET", "supersecret")
	t.Setenv("API_HOST", "api.example.com")

	dir := t.TempDir()

	yamlData := []byte(`name: myserver
transport: command
command: npx
env:
  API_KEY: "${MY_SECRET}"
  HOST: "${API_HOST}"
headers:
  Authorization: "Bearer ${MY_SECRET}"
`)
	if err := os.WriteFile(filepath.Join(dir, "myserver.yaml"), yamlData, 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, ok := registry["myserver"]
	if !ok {
		t.Fatal("expected 'myserver' entry")
	}

	if entry.Env["API_KEY"] != "supersecret" {
		t.Fatalf("expected expanded 'supersecret', got %q", entry.Env["API_KEY"])
	}
	if entry.Env["HOST"] != "api.example.com" {
		t.Fatalf("expected expanded 'api.example.com', got %q", entry.Env["HOST"])
	}
	if entry.Headers["Authorization"] != "Bearer supersecret" {
		t.Fatalf("expected expanded 'Bearer supersecret', got %q", entry.Headers["Authorization"])
	}
}

func TestLoadRegistryMissingDir(t *testing.T) {
	registry, err := LoadRegistry("/nonexistent/path/mcp")
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if len(registry) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(registry))
	}
}

func TestLoadRegistryNameFallbackToFilename(t *testing.T) {
	dir := t.TempDir()

	yamlData := []byte(`transport: command
command: echo
`)
	if err := os.WriteFile(filepath.Join(dir, "my-server.yaml"), yamlData, 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := registry["my-server"]; !ok {
		t.Fatal("expected entry keyed by filename 'my-server'")
	}
}

func TestExpandEnvNoVars(t *testing.T) {
	result := config.ExpandEnvVars("plain string")
	if result != "plain string" {
		t.Fatalf("expected 'plain string', got %q", result)
	}
}

func TestExpandEnvUndefinedVar(t *testing.T) {
	result := config.ExpandEnvVars("${UNDEFINED_VAR_12345}")
	if result != "${UNDEFINED_VAR_12345}" {
		t.Fatalf("expected original placeholder for undefined var, got %q", result)
	}
}
