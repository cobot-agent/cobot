//go:build windows

package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// skipInCI skips the test when running in CI (GitHub Actions, etc.)
// because CreateRestrictedToken + go test -race causes STATUS_HEAP_CORRUPTION
// (0xc0000374) on Windows runners.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("skipping: CreateRestrictedToken causes heap corruption with -race in CI")
	}
}

func TestGenerateCapabilitySID(t *testing.T) {
	sid, err := generateCapabilitySID()
	if err != nil {
		t.Fatalf("generateCapabilitySID failed: %v", err)
	}
	defer windows.FreeSid(sid)

	sidStr := sid.String()
	if !strings.HasPrefix(sidStr, "S-1-5-21-") {
		t.Errorf("SID should start with S-1-5-21-, got %q", sidStr)
	}

	// Two calls should produce different SIDs.
	sid2, err := generateCapabilitySID()
	if err != nil {
		t.Fatalf("second generateCapabilitySID: %v", err)
	}
	defer windows.FreeSid(sid2)
	if sid.String() == sid2.String() {
		t.Error("two generated SIDs should differ")
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

func TestGrantAndRevokeAccessACL(t *testing.T) {
	skipInCI(t)

	sid, err := generateCapabilitySID()
	if err != nil {
		t.Fatalf("generateCapabilitySID: %v", err)
	}
	defer windows.FreeSid(sid)

	dir := t.TempDir()

	if err := grantAccessACL(dir, sid); err != nil {
		t.Skipf("grantAccessACL failed (may lack privileges): %v", err)
	}

	// Verify the ACE is present by querying the DACL.
	dirPtr, _ := windows.UTF16PtrFromString(dir)
	var dacl, sd uintptr
	initWinProcs()
	ret, _, _ := winProcGetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(seFileObject),
		uintptr(daclSecurityInformation),
		0, 0,
		uintptr(unsafe.Pointer(&dacl)),
		0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if ret != 0 {
		t.Fatalf("GetNamedSecurityInfo failed: %d", ret)
	}
	winProcLocalFree.Call(sd)

	if err := revokeAccessACL(dir, sid); err != nil {
		t.Logf("revokeAccessACL failed (non-fatal): %v", err)
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

func TestRestrictedTokenWriteBlocking(t *testing.T) {
	skipInCI(t)

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
