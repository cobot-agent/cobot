package acp

import (
	"context"
	"testing"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/server"

	acpapi "github.com/cobot-agent/cobot/api/acp"
	agentpkg "github.com/cobot-agent/cobot/internal/agent"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func newTestServer(t *testing.T) (*ACPServer, server.Local) {
	t.Helper()
	a := agentpkg.New(cobot.DefaultConfig())
	s := NewACPServer(a, nil)
	loc := server.NewLocal(s.handlerMap(), nil)
	t.Cleanup(func() { loc.Close() })
	return s, loc
}

func TestHandleInitialize(t *testing.T) {
	_, loc := newTestServer(t)

	var resp acpapi.InitializeResponse
	err := loc.Client.CallResult(context.Background(), "initialize", acpapi.InitializeRequest{
		ProtocolVersion: 1,
	}, &resp)
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	if resp.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", resp.ProtocolVersion)
	}
	if resp.AgentCapabilities.LoadSession {
		t.Error("LoadSession should be false")
	}
	if resp.AgentInfo == nil {
		t.Fatal("AgentInfo is nil")
	}
	if resp.AgentInfo.Name != "cobot" {
		t.Errorf("AgentInfo.Name = %q, want %q", resp.AgentInfo.Name, "cobot")
	}
	if resp.AgentInfo.Title != "Cobot Agent" {
		t.Errorf("AgentInfo.Title = %q, want %q", resp.AgentInfo.Title, "Cobot Agent")
	}
	if resp.AgentInfo.Version != "0.1.0" {
		t.Errorf("AgentInfo.Version = %q, want %q", resp.AgentInfo.Version, "0.1.0")
	}
	if len(resp.AuthMethods) != 0 {
		t.Errorf("AuthMethods = %v, want empty", resp.AuthMethods)
	}
}

func TestHandleSessionNew(t *testing.T) {
	_, loc := newTestServer(t)

	var resp acpapi.NewSessionResponse
	err := loc.Client.CallResult(context.Background(), "session/new", acpapi.NewSessionRequest{
		CWD: "/tmp",
	}, &resp)
	if err != nil {
		t.Fatalf("session/new: %v", err)
	}

	if resp.SessionID == "" {
		t.Error("SessionID is empty")
	}
	if len(resp.SessionID) < 10 {
		t.Errorf("SessionID too short: %q", resp.SessionID)
	}
}

func TestHandleSessionPromptInvalidSession(t *testing.T) {
	_, loc := newTestServer(t)

	var resp acpapi.PromptResponse
	err := loc.Client.CallResult(context.Background(), "session/prompt", acpapi.PromptRequest{
		SessionID: "nonexistent",
		Prompt:    []acpapi.ContentBlock{{Type: "text", Text: "hello"}},
	}, &resp)
	if err == nil {
		t.Fatal("expected error for invalid session, got nil")
	}

	jerr, ok := err.(*jrpc2.Error)
	if !ok {
		t.Fatalf("expected *jrpc2.Error, got %T", err)
	}
	if jerr.Code != jrpc2.InvalidParams {
		t.Errorf("error code = %d, want %d", jerr.Code, jrpc2.InvalidParams)
	}
}

func TestHandleSessionCancelNonexistent(t *testing.T) {
	_, loc := newTestServer(t)

	var result any
	err := loc.Client.CallResult(context.Background(), "session/cancel", acpapi.CancelNotification{
		SessionID: "nonexistent",
	}, &result)
	if err != nil {
		t.Fatalf("session/cancel: %v", err)
	}
}
