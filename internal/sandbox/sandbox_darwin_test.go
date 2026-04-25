//go:build darwin

package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireSandboxExec(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sandbox-exec"); err != nil {
		t.Skip("sandbox-exec not available")
	}
}

func TestSeatbeltWriteBlocking(t *testing.T) {
	requireSandboxExec(t)
	allowed := t.TempDir()
	blocked := t.TempDir()

	cfg := &SandboxConfig{Root: allowed, AllowNetwork: true}

	// Write to allowed dir should succeed.
	req := &LaunchRequest{
		Shell: "/bin/sh", ShellFlag: "-c",
		Command: "echo ok > " + filepath.Join(allowed, "test.txt") + " && cat " + filepath.Join(allowed, "test.txt"),
		Config:  cfg,
	}
	out, err := sandboxExecLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("allowed write failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'ok', got %q", string(out))
	}

	// Write to blocked dir should fail.
	req.Command = "echo blocked > " + filepath.Join(blocked, "blocked.txt")
	out, _ = sandboxExecLaunch(t.Context(), req)
	if !strings.Contains(string(out), "Operation not permitted") {
		t.Fatalf("write to blocked dir should be denied, got: %s", out)
	}
}

func TestSeatbeltNetworkDeny(t *testing.T) {
	requireSandboxExec(t)
	cfg := &SandboxConfig{Root: "/private/tmp", AllowNetwork: false}
	req := &LaunchRequest{
		Shell: "/bin/sh", ShellFlag: "-c",
		Command: "curl -s --max-time 3 https://example.com 2>&1; echo exit:$?",
		Config:  cfg,
	}
	out, _ := sandboxExecLaunch(t.Context(), req)
	if strings.Contains(string(out), "exit:0") {
		t.Fatal("curl should fail with network denied")
	}
}

func TestSeatbeltNoConfigFallback(t *testing.T) {
	requireSandboxExec(t)
	req := &LaunchRequest{Shell: "/bin/sh", ShellFlag: "-c", Command: "echo ok"}
	out, err := sandboxExecLaunch(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("expected 'ok', got %q", string(out))
	}
}

func TestBuildSeatbeltProfile(t *testing.T) {
	cfg := &SandboxConfig{
		Root:          "/Users/test/ws",
		AllowPaths:    []string{"/tmp"},
		ReadonlyPaths: []string{"/etc"},
		AllowNetwork:  false,
	}
	profile := buildSeatbeltProfile(cfg)
	for _, want := range []string{"(version 1)", "(deny file-write*)", "(deny network*)"} {
		if !strings.Contains(profile, want) {
			t.Errorf("profile missing %q", want)
		}
	}
}

func TestSeatbeltSubprocess(t *testing.T) {
	requireSandboxExec(t)
	allowed := t.TempDir()
	blocked := t.TempDir()

	cfg := &SandboxConfig{Root: allowed, AllowNetwork: false}
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

func TestSeatbeltHelper(t *testing.T) {
	if os.Getenv("COBOT_DARWIN_SB_TEST") != "1" {
		t.Skip("only runs as subprocess")
	}

	allowed := os.Getenv("COBOT_DARWIN_SB_ALLOWED")
	blocked := os.Getenv("COBOT_DARWIN_SB_BLOCKED")

	if err := os.WriteFile(filepath.Join(allowed, "sub.txt"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "sub.txt"), []byte("nope"), 0644); err == nil {
		t.Fatal("write to blocked dir should be denied")
	}
}
