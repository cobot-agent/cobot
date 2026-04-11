package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNewMCPManager(t *testing.T) {
	m := NewMCPManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.client == nil {
		t.Fatal("expected non-nil client")
	}
	if len(m.sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(m.sessions))
	}
}

func TestMCPToolAdapter(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)

	adapter := &MCPToolAdapter{
		name:        "test-tool",
		description: "A test tool",
		schema:      schema,
		callFunc: func(ctx context.Context, args json.RawMessage) (string, error) {
			var input struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(args, &input); err != nil {
				return "", err
			}
			return "hello " + input.Name, nil
		},
	}

	if adapter.Name() != "test-tool" {
		t.Fatalf("expected name 'test-tool', got %q", adapter.Name())
	}

	if adapter.Description() != "A test tool" {
		t.Fatalf("expected description 'A test tool', got %q", adapter.Description())
	}

	if string(adapter.Parameters()) != string(schema) {
		t.Fatalf("expected schema %s, got %s", schema, adapter.Parameters())
	}

	result, err := adapter.Execute(context.Background(), json.RawMessage(`{"name":"world"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Fatalf("expected 'hello world', got %q", result)
	}
}

func TestMCPToolAdapterExecuteError(t *testing.T) {
	adapter := &MCPToolAdapter{
		name:        "err-tool",
		description: "always fails",
		schema:      nil,
		callFunc: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", errors.New("tool failure")
		},
	}

	_, err := adapter.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "tool failure" {
		t.Fatalf("expected 'tool failure', got %q", err.Error())
	}
}

func TestMCPManagerConnectInvalid(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	ctx := context.Background()
	err := m.Connect(ctx, "bad-server", ServerConfig{
		Command: "this-binary-does-not-exist-12345",
	})
	if err == nil {
		t.Fatal("expected error connecting to nonexistent binary, got nil")
	}
}

func TestMCPManagerDisconnectNonexistent(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	ctx := context.Background()
	err := m.Disconnect(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error disconnecting nonexistent server, got nil")
	}
}

func TestMCPManagerConnectDuplicate(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	m.sessions["dup"] = nil
	m.configs["dup"] = ServerConfig{Command: "echo"}

	err := m.Connect(context.Background(), "dup", ServerConfig{
		Command: "echo",
	})
	if err == nil {
		t.Fatal("expected error for duplicate connection, got nil")
	}
}

func TestMCPManagerListToolsNotConnected(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	_, err := m.ListTools(context.Background(), "no-such-server")
	if err == nil {
		t.Fatal("expected error listing tools from unconnected server, got nil")
	}
}

func TestMCPManagerCallToolNotConnected(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	_, err := m.CallTool(context.Background(), "no-such-server", "some-tool", nil)
	if err == nil {
		t.Fatal("expected error calling tool on unconnected server, got nil")
	}
}

func TestMCPManagerToolAdaptersNotConnected(t *testing.T) {
	m := NewMCPManager()
	defer m.Close()

	_, err := m.ToolAdapters(context.Background(), "no-such-server")
	if err == nil {
		t.Fatal("expected error getting adapters from unconnected server, got nil")
	}
}
