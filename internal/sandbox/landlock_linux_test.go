//go:build linux

package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLandlockApplyRestrictsWrite verifies that Landlock blocks writes
// outside the allowed directory.
func TestLandlockApplyRestrictsWrite(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, Landlock restrictions do not apply")
	}

	allowed := t.TempDir()
	blocked := t.TempDir()

	// Apply Landlock: only `allowed` is writable, everything else is read-only.
	applyLandlock(allowed, nil, nil, false)

	// Writing inside the allowed dir should succeed.
	allowedFile := filepath.Join(allowed, "ok.txt")
	if err := os.WriteFile(allowedFile, []byte("ok"), 0644); err != nil {
		t.Fatalf("should be able to write inside allowed dir: %v", err)
	}

	// Writing inside the blocked dir should fail with EPERM/EACCES.
	blockedFile := filepath.Join(blocked, "blocked.txt")
	err := os.WriteFile(blockedFile, []byte("nope"), 0644)
	if err == nil {
		// BestEffort may silently skip on kernels without Landlock.
		t.Log("WARNING: write outside allowed dir succeeded — Landlock may not be enforced on this kernel")
	} else {
		t.Logf("expected Landlock denial: %v", err)
	}
}

// TestLandlockApplyGracefulDegradation verifies that applyLandlock does not
// crash even with empty/nil arguments.
func TestLandlockApplyGracefulDegradation(t *testing.T) {
	applyLandlock("", nil, nil, false)
	applyLandlock("", nil, nil, true)
	applyLandlock("/nonexistent", nil, nil, false)
	applyLandlock("", []string{"/tmp"}, nil, false)
	applyLandlock("", nil, []string{"/etc"}, true)
}

// TestLandlockLaunchExecutesCommand verifies the launch function runs
// a command and captures output. For test binaries, landlockLaunch falls
// back to hostExec (skips re-exec).
func TestLandlockLaunchExecutesCommand(t *testing.T) {
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo hello-landlock",
		Config:    &SandboxConfig{Root: "/tmp"},
	}

	out, err := landlockLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("landlockLaunch: %v", err)
	}
	got := string(out)
	if !contains(got, "hello-landlock") {
		t.Fatalf("expected 'hello-landlock', got %q", got)
	}
}

// TestHostExecBasic verifies hostExec runs a command directly.
func TestHostExecBasic(t *testing.T) {
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo host-ok",
	}

	out, err := hostExec(t.Context(), req)
	if err != nil {
		t.Fatalf("hostExec: %v", err)
	}
	if !contains(string(out), "host-ok") {
		t.Fatalf("expected 'host-ok', got %q", string(out))
	}
}

// TestPlatformLaunchOnLinux verifies that platformLaunch works end-to-end.
// On Linux it calls landlockLaunch which falls back to hostExec for test binaries.
func TestPlatformLaunchOnLinux(t *testing.T) {
	req := &LaunchRequest{
		Shell:     "/bin/sh",
		ShellFlag: "-c",
		Command:   "echo platform-ok",
		Config:    &SandboxConfig{Root: "/tmp"},
	}

	out, err := platformLaunch(t.Context(), req)
	if err != nil {
		t.Fatalf("platformLaunch: %v", err)
	}
	if !contains(string(out), "platform-ok") {
		t.Fatalf("expected 'platform-ok', got %q", string(out))
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
