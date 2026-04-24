//go:build darwin

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSeatbeltWriteBlocking verifies that sandbox-exec blocks writes outside
// the allowed directory.
func TestSeatbeltWriteBlocking(t *testing.T) {
	allowed := t.TempDir()
	blocked := t.TempDir()

	cfg := &SandboxConfig{
		Root:         allowed,
		AllowNetwork: true,
	}
	profile := buildSeatbeltProfile(cfg)
	t.Logf("Profile:\n%s", profile)

	// Write to allowed dir should succeed
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo ok > " + filepath.Join(allowed, "test.txt") + " && cat " + filepath.Join(allowed, "test.txt"),
		Config:    cfg,
	}
	out, err := sandboxExecLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("allowed write failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'ok', got %q", string(out))
	}

	// Write to blocked dir should fail
	req2 := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo blocked > " + filepath.Join(blocked, "blocked.txt"),
		Config:    cfg,
	}
	out2, err2 := sandboxExecLaunch(t.Context(), req2)
	if err2 == nil {
		// sandbox-exec may not return error but the write should fail
		if !strings.Contains(string(out2), "Operation not permitted") &&
			!strings.Contains(string(out2), "Permission denied") {
			t.Fatalf("write to blocked dir should have been denied, got: %s", out2)
		}
	}
	t.Logf("blocked write result (expected failure): %s", out2)
}

// TestSeatbeltNetworkDeny verifies that sandbox-exec blocks network access.
func TestSeatbeltNetworkDeny(t *testing.T) {
	cfg := &SandboxConfig{
		Root:         "/private/tmp",
		AllowNetwork: false,
	}
	profile := buildSeatbeltProfile(cfg)
	t.Logf("Profile:\n%s", profile)

	// curl should fail when network is denied
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "curl -s --max-time 3 https://example.com 2>&1; echo exit:$?",
		Config:    cfg,
	}
	out, err := sandboxExecLaunch(t.Context(), req)
	if err != nil {
		t.Logf("sandboxExecLaunch returned error (acceptable): %v", err)
	}
	output := string(out)
	t.Logf("output: %s", output)
	// curl should fail (exit code != 0) due to network deny
	if strings.Contains(output, "exit:0") {
		t.Fatal("curl should have failed with network denied, but got exit:0")
	}
}

// TestSeatbeltNoConfigFallback verifies that nil config falls back to hostExec.
func TestSeatbeltNoConfigFallback(t *testing.T) {
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo no-config-ok",
	}
	out, err := sandboxExecLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("no-config fallback failed: %v", err)
	}
	if !strings.Contains(string(out), "no-config-ok") {
		t.Fatalf("expected 'no-config-ok', got %q", string(out))
	}
}

// TestBuildSeatbeltProfile tests profile generation.
func TestBuildSeatbeltProfile(t *testing.T) {
	cfg := &SandboxConfig{
		Root:          "/Users/test/workspace",
		AllowPaths:    []string{"/tmp"},
		ReadonlyPaths: []string{"/etc"},
		AllowNetwork:  false,
	}
	profile := buildSeatbeltProfile(cfg)

	if !strings.Contains(profile, "(version 1)") {
		t.Error("profile should contain version")
	}
	if !strings.Contains(profile, "(deny file-write*)") {
		t.Error("profile should deny file-write*")
	}
	if !strings.Contains(profile, "(deny network*)") {
		t.Error("profile should deny network when AllowNetwork=false")
	}
	if !strings.Contains(profile, "file-write*") {
		t.Error("profile should have file-write rules")
	}
	t.Logf("Generated profile:\n%s", profile)
}

// TestSeatbeltSubprocess runs the full sandbox test in a subprocess via
// sandbox-exec to validate the actual Seatbelt enforcement end-to-end.
func TestSeatbeltSubprocess(t *testing.T) {
	allowed := t.TempDir()
	blocked := t.TempDir()

	// Write a marker in allowed dir to verify read access
	marker := filepath.Join(allowed, "marker.txt")
	if err := os.WriteFile(marker, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run helper in subprocess via sandbox-exec
	cfg := &SandboxConfig{
		Root:         allowed,
		AllowNetwork: false,
	}
	profile := buildSeatbeltProfile(cfg)

	cmd := exec.Command("sandbox-exec", "-p", profile, "--",
		os.Args[0], "-test.run=TestSeatbeltHelper")
	cmd.Env = append(os.Environ(),
		"COBOT_DARWIN_SB_TEST=1",
		"COBOT_DARWIN_SB_ALLOWED="+allowed,
		"COBOT_DARWIN_SB_BLOCKED="+blocked,
	)
	out, err := cmd.CombinedOutput()
	t.Logf("helper output:\n%s", out)
	if err != nil {
		t.Fatalf("helper failed: %v", err)
	}
}

// TestSeatbeltHelper is the subprocess that validates Seatbelt enforcement.
func TestSeatbeltHelper(t *testing.T) {
	if os.Getenv("COBOT_DARWIN_SB_TEST") != "1" {
		t.Skip("only runs as subprocess")
	}

	allowed := os.Getenv("COBOT_DARWIN_SB_ALLOWED")
	blocked := os.Getenv("COBOT_DARWIN_SB_BLOCKED")

	// Write to allowed dir should succeed
	allowedFile := filepath.Join(allowed, "sub.txt")
	if err := os.WriteFile(allowedFile, []byte("ok"), 0644); err != nil {
		t.Fatalf("should write to allowed dir: %v", err)
	}

	// Write to blocked dir should fail
	blockedFile := filepath.Join(blocked, "sub.txt")
	err := os.WriteFile(blockedFile, []byte("nope"), 0644)
	if err == nil {
		t.Fatal("write to blocked dir should have been denied")
	}
	t.Logf("blocked write failed as expected: %v", err)
}
