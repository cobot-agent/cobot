package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRegistry(t *testing.T) {
	dir := t.TempDir()

	cmdYAML := `
name: filesystem
description: Filesystem MCP server
transport: command
command: npx
args:
  - "-y"
  - "@modelcontextprotocol/server-filesystem"
  - "/tmp"
env:
  NODE_ENV: development
`
	if err := os.WriteFile(filepath.Join(dir, "filesystem.yaml"), []byte(cmdYAML), 0644); err != nil {
		t.Fatal(err)
	}

	httpYAML := `
name: weather
description: Weather MCP server
transport: http
url: http://localhost:8080/mcp
headers:
  Authorization: "Bearer test-token"
`
	if err := os.WriteFile(filepath.Join(dir, "weather.yaml"), []byte(httpYAML), 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}

	if len(registry) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(registry))
	}

	fs, ok := registry["filesystem"]
	if !ok {
		t.Fatal("expected 'filesystem' entry")
	}
	if fs.Transport != "command" {
		t.Errorf("expected transport 'command', got %q", fs.Transport)
	}
	if fs.Command != "npx" {
		t.Errorf("expected command 'npx', got %q", fs.Command)
	}
	if len(fs.Args) != 3 || fs.Args[0] != "-y" {
		t.Errorf("unexpected args: %v", fs.Args)
	}
	if fs.Env["NODE_ENV"] != "development" {
		t.Errorf("expected NODE_ENV=development, got %q", fs.Env["NODE_ENV"])
	}

	weather, ok := registry["weather"]
	if !ok {
		t.Fatal("expected 'weather' entry")
	}
	if weather.Transport != "http" {
		t.Errorf("expected transport 'http', got %q", weather.Transport)
	}
	if weather.URL != "http://localhost:8080/mcp" {
		t.Errorf("unexpected url: %q", weather.URL)
	}
	if weather.Headers["Authorization"] != "Bearer test-token" {
		t.Errorf("unexpected Authorization header: %q", weather.Headers["Authorization"])
	}
}

func TestLoadRegistry_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
name: only-me
transport: command
command: echo
`
	if err := os.WriteFile(filepath.Join(dir, "server.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}

	if len(registry) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(registry))
	}
	if _, ok := registry["only-me"]; !ok {
		t.Fatal("expected 'only-me' entry")
	}
}

func TestLoadRegistry_NonexistentDir(t *testing.T) {
	registry, err := LoadRegistry("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("expected no error for nonexistent dir, got: %v", err)
	}
	if len(registry) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(registry))
	}
}

func TestLoadRegistry_EnvExpansion(t *testing.T) {
	os.Setenv("COBOT_TEST_VAR", "expanded-value")
	defer os.Unsetenv("COBOT_TEST_VAR")

	dir := t.TempDir()

	yamlContent := `
name: expand-test
transport: command
command: echo
env:
  MY_VAR: "${COBOT_TEST_VAR}"
headers:
  X-Custom: "${COBOT_TEST_VAR}"
`
	if err := os.WriteFile(filepath.Join(dir, "expand.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}

	entry, ok := registry["expand-test"]
	if !ok {
		t.Fatal("expected 'expand-test' entry")
	}

	if entry.Env["MY_VAR"] != "expanded-value" {
		t.Errorf("expected MY_VAR='expanded-value', got %q", entry.Env["MY_VAR"])
	}
	if entry.Headers["X-Custom"] != "expanded-value" {
		t.Errorf("expected X-Custom='expanded-value', got %q", entry.Headers["X-Custom"])
	}
}

func TestLoadRegistry_NameFromFilename(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
transport: command
command: echo
args:
  - "hello"
`
	if err := os.WriteFile(filepath.Join(dir, "my-server.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	registry, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry returned error: %v", err)
	}

	entry, ok := registry["my-server"]
	if !ok {
		t.Fatal("expected 'my-server' entry derived from filename")
	}
	if entry.Name != "my-server" {
		t.Errorf("expected Name 'my-server', got %q", entry.Name)
	}
}
