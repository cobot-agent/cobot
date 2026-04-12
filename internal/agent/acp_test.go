package agent

import (
	cobot "github.com/cobot-agent/cobot/pkg"
	"testing"
)

// Ensure ACP scaffolding can be created and getACPServer returns a non-nil server
func TestAgentACPScaffold_GetServer(t *testing.T) {
	cfg := &cobot.Config{}
	a := New(cfg)
	if a == nil {
		t.Fatal("expected non-nil Agent instance from New()")
	}

	// Initially, the acpServer should be nil
	if a.acpServer != nil {
		t.Fatalf("expected acpServer to be nil before first access, got %T", a.acpServer)
	}

	// First call should initialize the ACP server
	srv := a.getACPServer()
	if srv == nil {
		t.Fatal("expected non-nil ACP server from getACPServer()")
	}

	// acpServer field should now be non-nil
	if a.acpServer == nil {
		t.Fatalf("expected acpServer field to be set after getACPServer(), got nil")
	}
}
