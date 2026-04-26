//go:build windows && race

package sandbox

import (
	"strings"
	"testing"
)

func TestRestrictedTokenRaceFallback(t *testing.T) {
	// When built with -race, restrictedTokenLaunch falls back to hostExec.
	// Verify it still works without sandbox enforcement.
	req := &LaunchRequest{
		Shell: "cmd", ShellFlag: "/C",
		Command: "echo race-ok",
	}
	out, err := restrictedTokenLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("race fallback failed: %v\noutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "race-ok") {
		t.Fatalf("expected 'race-ok', got %q", string(out))
	}
}
