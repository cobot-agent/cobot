package acp

import (
	"encoding/json"
	"testing"
)

func TestInitializeRequestJSON(t *testing.T) {
	orig := InitializeRequest{
		ProtocolVersion: 1,
		ClientCapabilities: ClientCapabilities{
			Fs: &FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: false,
			},
			Terminal: true,
		},
		ClientInfo: &Implementation{
			Name:    "test-client",
			Title:   "Test Client",
			Version: "1.0.0",
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got InitializeRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ProtocolVersion != orig.ProtocolVersion {
		t.Errorf("ProtocolVersion = %d, want %d", got.ProtocolVersion, orig.ProtocolVersion)
	}
	if got.ClientCapabilities.Fs == nil || !got.ClientCapabilities.Fs.ReadTextFile {
		t.Error("ClientCapabilities.Fs.ReadTextFile not preserved")
	}
	if !got.ClientCapabilities.Terminal {
		t.Error("ClientCapabilities.Terminal not preserved")
	}
	if got.ClientInfo.Name != "test-client" {
		t.Errorf("ClientInfo.Name = %q, want %q", got.ClientInfo.Name, "test-client")
	}
}

func TestInitializeResponseJSON(t *testing.T) {
	orig := InitializeResponse{
		ProtocolVersion: 1,
		AgentCapabilities: AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: &PromptCapabilities{
				Image:           true,
				Audio:           false,
				EmbeddedContext: true,
			},
		},
		AgentInfo: &Implementation{
			Name:    "test-agent",
			Version: "2.0.0",
		},
		AuthMethods: []AuthMethod{
			{ID: "api-key", Name: "API Key"},
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got InitializeResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.AgentCapabilities.LoadSession {
		t.Error("AgentCapabilities.LoadSession not preserved")
	}
	if got.AgentCapabilities.PromptCapabilities == nil || !got.AgentCapabilities.PromptCapabilities.Image {
		t.Error("PromptCapabilities.Image not preserved")
	}
	if len(got.AuthMethods) != 1 || got.AuthMethods[0].ID != "api-key" {
		t.Error("AuthMethods not preserved")
	}
}

func TestSessionUpdateNotificationJSON(t *testing.T) {
	orig := SessionUpdateNotification{
		SessionID: "sess-123",
		Update: SessionUpdate{
			SessionUpdate: "agent",
			Content: &ContentBlock{
				Type: "text",
				Text: "hello world",
			},
			ToolCallID: "tc-456",
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got SessionUpdateNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-123")
	}
	if got.Update.SessionUpdate != "agent" {
		t.Errorf("Update.SessionUpdate = %q, want %q", got.Update.SessionUpdate, "agent")
	}
	if got.Update.Content == nil || got.Update.Content.Text != "hello world" {
		t.Error("Update.Content.Text not preserved")
	}
	if got.Update.ToolCallID != "tc-456" {
		t.Errorf("Update.ToolCallID = %q, want %q", got.Update.ToolCallID, "tc-456")
	}
}

func TestPromptRequestJSON(t *testing.T) {
	orig := PromptRequest{
		SessionID: "sess-789",
		Prompt: []ContentBlock{
			{Type: "text", Text: "hello"},
			{Type: "resource", Resource: &ResourceContent{URI: "file:///x.txt", MIMEType: "text/plain"}},
			{Type: "image", URI: "https://example.com/img.png"},
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PromptRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.SessionID != "sess-789" {
		t.Errorf("SessionID = %q, want %q", got.SessionID, "sess-789")
	}
	if len(got.Prompt) != 3 {
		t.Fatalf("len(Prompt) = %d, want 3", len(got.Prompt))
	}
	if got.Prompt[0].Text != "hello" {
		t.Errorf("Prompt[0].Text = %q, want %q", got.Prompt[0].Text, "hello")
	}
	if got.Prompt[1].Resource == nil || got.Prompt[1].Resource.URI != "file:///x.txt" {
		t.Error("Prompt[1].Resource not preserved")
	}
	if got.Prompt[2].URI != "https://example.com/img.png" {
		t.Errorf("Prompt[2].URI = %q, want %q", got.Prompt[2].URI, "https://example.com/img.png")
	}
}

func TestMCPServerJSON(t *testing.T) {
	orig := MCPServer{
		Name:    "my-server",
		Command: "npx",
		Args:    []string{"-y", "some-mcp"},
		Env: []EnvVariable{
			{Name: "API_KEY", Value: "secret"},
			{Name: "DEBUG", Value: "1"},
		},
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got MCPServer
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "my-server" {
		t.Errorf("Name = %q, want %q", got.Name, "my-server")
	}
	if got.Command != "npx" {
		t.Errorf("Command = %q, want %q", got.Command, "npx")
	}
	if len(got.Args) != 2 || got.Args[1] != "some-mcp" {
		t.Errorf("Args = %v, want [npx -y some-mcp]", got.Args)
	}
	if len(got.Env) != 2 || got.Env[0].Name != "API_KEY" || got.Env[0].Value != "secret" {
		t.Errorf("Env = %v, not preserved", got.Env)
	}
}
