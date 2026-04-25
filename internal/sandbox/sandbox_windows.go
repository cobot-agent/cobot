//go:build windows

package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procCreateRestrictedToken = windows.NewLazyDLL("advapi32.dll").NewProc("CreateRestrictedToken")
)

const (
	disableMaxPrivilege = 0x0001
	luaToken            = 0x0004
	writeRestricted     = 0x0008
)

// sidAndAttributes matches the Windows SID_AND_ATTRIBUTES structure.
type sidAndAttributes struct {
	Sid        *windows.SID
	Attributes uint32
}

// restrictedTokenLaunch runs a command under a restricted Windows token.
// The child process can only write to directories where the capability SID
// is explicitly granted write access in the DACL. All other writes are denied
// by the kernel's WRITE_RESTRICTED token check.
//
// This is inspired by OpenAI Codex's "Legacy/Unelevated" Windows sandbox mode,
// simplified for cobot's use case.
func restrictedTokenLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req.Config == nil {
		slog.Warn("sandbox: no config provided, running without Restricted Token enforcement", "command", req.Command)
		return hostExec(ctx, req)
	}

	// 1. Generate a random capability SID for this sandbox invocation.
	capSid, sidStr, err := generateCapabilitySID()
	if err != nil {
		slog.Warn("sandbox: failed to generate capability SID, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}
	defer windows.FreeSid(capSid)

	// 2. Grant the SID write access to workspace + TEMP + allowed paths.
	writeDirs := buildWriteDirs(req.Config)
	for _, dir := range writeDirs {
		if err := grantWriteACL(dir, sidStr); err != nil {
			slog.Warn("sandbox: failed to grant write ACL", "dir", dir, "error", err)
		}
	}
	// Cleanup: remove ACEs after command completes.
	defer func() {
		for _, dir := range writeDirs {
			if err := revokeACL(dir, sidStr); err != nil {
				slog.Debug("sandbox: failed to revoke ACL (non-fatal)", "dir", dir, "error", err)
			}
		}
	}()

	// 3. Create restricted token with the capability SID as a restricting SID.
	rtoken, err := createRestrictedToken(capSid)
	if err != nil {
		slog.Warn("sandbox: failed to create restricted token, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}
	defer rtoken.Close()

	// 4. Launch command with the restricted token.
	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(rtoken),
	}
	return cmd.CombinedOutput()
}

// generateCapabilitySID creates a random SID (S-1-5-21-{4 random uint32})
// suitable for use as a restricting SID in a WRITE_RESTRICTED token.
func generateCapabilitySID() (*windows.SID, string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return nil, "", fmt.Errorf("generate random bytes: %w", err)
	}
	a := binary.LittleEndian.Uint32(buf[0:4])
	b := binary.LittleEndian.Uint32(buf[4:8])
	c := binary.LittleEndian.Uint32(buf[8:12])
	d := binary.LittleEndian.Uint32(buf[12:16])

	sidStr := fmt.Sprintf("S-1-5-21-%d-%d-%d-%d", a, b, c, d)
	sid, err := windows.StringToSid(sidStr)
	if err != nil {
		return nil, "", fmt.Errorf("StringToSid(%s): %w", sidStr, err)
	}
	return sid, sidStr, nil
}

// buildWriteDirs returns the list of directories the sandboxed process
// should be able to write to: workspace root, system TEMP, and any
// explicitly allowed paths from the config.
func buildWriteDirs(cfg *SandboxConfig) []string {
	dirs := make([]string, 0, 2+len(cfg.AllowPaths))
	if cfg.Root != "" {
		dirs = append(dirs, cfg.Root)
	}
	if tempDir := os.Getenv("TEMP"); tempDir != "" {
		dirs = append(dirs, tempDir)
	}
	dirs = append(dirs, cfg.AllowPaths...)
	return dirs
}

// grantWriteACL uses icacls to grant the SID full control (with inheritance)
// on the directory. The (OI)(CI)F flags grant Full Control with Object and
// Container Inheritance, so new files/subdirs automatically inherit the ACE.
func grantWriteACL(dir string, sidStr string) error {
	// icacls <dir> /grant *<SID>:(OI)(CI)F
	grantStr := fmt.Sprintf("*%s:(OI)(CI)F", sidStr)
	out, err := exec.Command("icacls", dir, "/grant", grantStr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls grant failed: %w, output: %s", err, string(out))
	}
	return nil
}

// revokeACL uses icacls to remove all ACEs for the SID from the directory.
func revokeACL(dir string, sidStr string) error {
	out, err := exec.Command("icacls", dir, "/remove", fmt.Sprintf("*%s", sidStr)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls remove failed: %w, output: %s", err, string(out))
	}
	return nil
}

// createRestrictedToken creates a restricted token from the current process token.
//
// Flags:
//   - DISABLE_MAX_PRIVILEGE: strips all privileges from the token
//   - LUA_TOKEN: creates a standard-user (UAC-split) token
//   - WRITE_RESTRICTED: write operations are only allowed where a restricting
//     SID is explicitly granted access in the object's DACL
//
// The capability SID is added as the sole restricting SID. After this, the
// token can only write to objects whose DACL includes an allow-ACE for this SID.
func createRestrictedToken(capSid *windows.SID) (windows.Token, error) {
	var procToken windows.Token
	err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_DUPLICATE|windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY,
		&procToken,
	)
	if err != nil {
		return 0, fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer procToken.Close()

	// Build restricting SID list (just our capability SID).
	sidAttrs := [1]sidAndAttributes{
		{Sid: capSid, Attributes: 0},
	}

	var newToken windows.Token
	// BOOL CreateRestrictedToken(
	//   HANDLE ExistingTokenHandle,
	//   DWORD Flags,
	//   DWORD DisableSidCount, PSID_AND_ATTRIBUTES SidsToDisable,
	//   DWORD DeletePrivilegeCount, PLUID_AND_ATTRIBUTES PrivilegesToDelete,
	//   DWORD RestrictedSidCount, PSID_AND_ATTRIBUTES SidsToRestrict,
	//   PHANDLE NewToken
	// );
	r1, _, e1 := procCreateRestrictedToken.Call(
		uintptr(procToken),                                // ExistingTokenHandle
		uintptr(disableMaxPrivilege|luaToken|writeRestricted), // Flags
		0,                                                 // DisableSidCount
		0,                                                 // SidsToDisable (nil)
		0,                                                 // DeletePrivilegeCount
		0,                                                 // PrivilegesToDelete (nil)
		1,                                                 // RestrictedSidCount
		uintptr(unsafe.Pointer(&sidAttrs[0])),             // SidsToRestrict
		uintptr(unsafe.Pointer(&newToken)),                // NewToken
	)
	if r1 == 0 {
		return 0, fmt.Errorf("CreateRestrictedToken: %w", e1)
	}
	return newToken, nil
}
