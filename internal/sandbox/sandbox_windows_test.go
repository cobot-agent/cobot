//go:build windows

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func requireIcacls(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("icacls"); err != nil {
		t.Skip("icacls not available")
	}
}

func TestRestrictedTokenNoConfigFallback(t *testing.T) {
	req := &LaunchRequest{
		Shell: "cmd", ShellFlag: "/C",
		Command: "echo ok",
	}
	out, err := restrictedTokenLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("no-config fallback failed: %v\noutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'ok', got %q", string(out))
	}
}

func TestGenerateCapabilitySID(t *testing.T) {
	sid, sidStr, err := generateCapabilitySID()
	if err != nil {
		t.Fatalf("generateCapabilitySID failed: %v", err)
	}
	defer windows.FreeSid(sid)

	if !strings.HasPrefix(sidStr, "S-1-5-21-") {
		t.Errorf("SID should start with S-1-5-21-, got %q", sidStr)
	}
}

func TestBuildWriteDirs(t *testing.T) {
	cfg := &SandboxConfig{
		Root:       `C:\workspace`,
		AllowPaths: []string{`C:\extra`},
	}
	dirs := buildWriteDirs(cfg)

	if len(dirs) < 2 {
		t.Fatalf("expected at least 2 dirs (root + temp), got %d", len(dirs))
	}
	if dirs[0] != `C:\workspace` {
		t.Errorf("first dir should be root, got %q", dirs[0])
	}
	// TEMP should be included if set.
	if tempDir := os.Getenv("TEMP"); tempDir != "" {
		found := false
		for _, d := range dirs {
			if d == tempDir {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TEMP dir %q not in write dirs", tempDir)
		}
	}
}

func TestGrantAndRevokeACL(t *testing.T) {
	requireIcacls(t)

	sid, sidStr, err := generateCapabilitySID()
	if err != nil {
		t.Fatalf("generateCapabilitySID: %v", err)
	}
	defer windows.FreeSid(sid)

	dir := t.TempDir()

	// Grant write ACL.
	if err := grantWriteACL(dir, sidStr); err != nil {
		t.Fatalf("grantWriteACL: %v", err)
	}

	// Verify the ACE is present by checking icacls output.
	out, err := exec.Command("icacls", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("icacls: %v", err)
	}
	if !strings.Contains(string(out), sidStr) {
		t.Errorf("ACE for %q not found in icacls output:\n%s", sidStr, string(out))
	}

	// Revoke ACL.
	if err := revokeACL(dir, sidStr); err != nil {
		t.Fatalf("revokeACL: %v", err)
	}

	// Verify the ACE is removed.
	out, err = exec.Command("icacls", dir).CombinedOutput()
	if err != nil {
		t.Fatalf("icacls: %v", err)
	}
	if strings.Contains(string(out), sidStr) {
		t.Errorf("ACE for %q still present after revoke:\n%s", sidStr, string(out))
	}
}

func TestRestrictedTokenWriteBlocking(t *testing.T) {
	requireIcacls(t)

	allowed := t.TempDir()
	blocked := t.TempDir()

	cfg := &SandboxConfig{Root: allowed, AllowNetwork: true}

	// Write to allowed dir should succeed.
	req := &LaunchRequest{
		Shell: "cmd", ShellFlag: "/C",
		Command: "echo ok > " + filepath.Join(allowed, "test.txt") + " && type " + filepath.Join(allowed, "test.txt"),
		Config:  cfg,
	}
	out, err := restrictedTokenLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("allowed write failed: %v\noutput: %s", err, string(out))
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'ok', got %q", string(out))
	}

	// Write to blocked dir should fail — the restricted token has no ACE for it.
	blockedFile := filepath.Join(blocked, "blocked.txt")
	req.Command = "echo blocked > " + blockedFile
	out, _ = restrictedTokenLaunch(t.Context(), req)
	t.Logf("blocked write output: %s", out)
	if _, statErr := os.Stat(blockedFile); !os.IsNotExist(statErr) {
		t.Fatalf("write to blocked dir should not create file, got stat err: %v, output: %s", statErr, string(out))
	}
}
