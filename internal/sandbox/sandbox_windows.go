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
	modadvapi32 = windows.NewLazyDLL("advapi32.dll")
	modkernel32 = windows.NewLazyDLL("kernel32.dll")

	procCreateRestrictedToken = modadvapi32.NewProc("CreateRestrictedToken")
	procGetNamedSecurityInfoW = modadvapi32.NewProc("GetNamedSecurityInfoW")
	procSetEntriesInAclW      = modadvapi32.NewProc("SetEntriesInAclW")
	procSetNamedSecurityInfoW = modadvapi32.NewProc("SetNamedSecurityInfoW")
	procLocalFree             = modkernel32.NewProc("LocalFree")
)

const (
	disableMaxPrivilege = 0x0001
	luaToken            = 0x0004
	writeRestricted     = 0x0008

	seFileObject            = 1 // SE_FILE_OBJECT
	daclSecurityInformation = 0x00000004

	grantAccess  = 1
	revokeAccess = 4

	subContainersAndObjectsInherit = 0x03 // OBJECT_INHERIT_ACE | CONTAINER_INHERIT_ACE

	trusteeIsSID     = 0
	trusteeIsUnknown = 0

	fileAllAccess = 0x001F01FF
)

// explicitAccessW matches the Windows EXPLICIT_ACCESS_W structure.
type explicitAccessW struct {
	AccessPermissions uint32
	AccessMode        uint32
	Inheritance       uint32
	Trustee           trusteeW
}

// trusteeW matches the Windows TRUSTEE_W structure.
type trusteeW struct {
	pMultipleTrustee         uintptr
	MultipleTrusteeOperation uint32
	TrusteeForm              uint32
	TrusteeType              uint32
	PtstrName                uintptr
}

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
	capSid, err := generateCapabilitySID()
	if err != nil {
		slog.Warn("sandbox: failed to generate capability SID, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}
	defer windows.FreeSid(capSid)

	// 2. Grant the SID write access to workspace + TEMP + allowed paths.
	writeDirs := buildWriteDirs(req.Config)
	grantedDirs := make([]string, 0, len(writeDirs))
	for _, dir := range writeDirs {
		if err := grantAccessACL(dir, capSid); err != nil {
			slog.Warn("sandbox: failed to grant write ACL", "dir", dir, "error", err)
		} else {
			grantedDirs = append(grantedDirs, dir)
		}
	}
	// Cleanup: remove ACEs after command completes.
	defer func() {
		for _, dir := range grantedDirs {
			if err := revokeAccessACL(dir, capSid); err != nil {
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
func generateCapabilitySID() (*windows.SID, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	a := binary.LittleEndian.Uint32(buf[0:4])
	b := binary.LittleEndian.Uint32(buf[4:8])
	c := binary.LittleEndian.Uint32(buf[8:12])
	d := binary.LittleEndian.Uint32(buf[12:16])

	sidStr := fmt.Sprintf("S-1-5-21-%d-%d-%d-%d", a, b, c, d)
	sid, err := windows.StringToSid(sidStr)
	if err != nil {
		return nil, fmt.Errorf("StringToSid(%s): %w", sidStr, err)
	}
	return sid, nil
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

// grantAccessACL adds a Full Control ACE for the SID to the directory's DACL
// with container and object inheritance, using raw Win32 ACL APIs.
func grantAccessACL(dir string, sid *windows.SID) error {
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	// Get current DACL.
	var dacl, sd uintptr
	ret, _, e1 := procGetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(seFileObject),
		uintptr(daclSecurityInformation),
		0, 0, // owner, group
		uintptr(unsafe.Pointer(&dacl)),
		0, // SACL
		uintptr(unsafe.Pointer(&sd)),
	)
	if ret != 0 {
		return fmt.Errorf("GetNamedSecurityInfo: %w", e1)
	}
	defer procLocalFree.Call(sd)

	// Build explicit access entry: grant FILE_ALL_ACCESS with inheritance.
	ea := explicitAccessW{
		AccessPermissions: fileAllAccess,
		AccessMode:        grantAccess,
		Inheritance:       subContainersAndObjectsInherit,
		Trustee: trusteeW{
			TrusteeForm: trusteeIsSID,
			TrusteeType: trusteeIsUnknown,
			PtstrName:   uintptr(unsafe.Pointer(sid)),
		},
	}

	// Merge with existing DACL.
	var newAcl uintptr
	ret, _, e1 = procSetEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		dacl,
		uintptr(unsafe.Pointer(&newAcl)),
	)
	if ret != 0 {
		return fmt.Errorf("SetEntriesInAcl: %w", e1)
	}
	defer procLocalFree.Call(newAcl)

	// Set the merged DACL.
	ret, _, e1 = procSetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(seFileObject),
		uintptr(daclSecurityInformation),
		0, 0, // owner, group
		newAcl,
		0, // SACL
	)
	if ret != 0 {
		return fmt.Errorf("SetNamedSecurityInfo: %w", e1)
	}
	return nil
}

// revokeAccessACL removes all ACEs for the SID from the directory's DACL.
func revokeAccessACL(dir string, sid *windows.SID) error {
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	// Get current DACL.
	var dacl, sd uintptr
	ret, _, e1 := procGetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(seFileObject),
		uintptr(daclSecurityInformation),
		0, 0,
		uintptr(unsafe.Pointer(&dacl)),
		0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if ret != 0 {
		return fmt.Errorf("GetNamedSecurityInfo: %w", e1)
	}
	defer procLocalFree.Call(sd)

	// Build revoke entry: remove all ACEs for this SID.
	ea := explicitAccessW{
		AccessMode: revokeAccess,
		Trustee: trusteeW{
			TrusteeForm: trusteeIsSID,
			TrusteeType: trusteeIsUnknown,
			PtstrName:   uintptr(unsafe.Pointer(sid)),
		},
	}

	// Remove the SID's ACEs from existing DACL.
	var newAcl uintptr
	ret, _, e1 = procSetEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		dacl,
		uintptr(unsafe.Pointer(&newAcl)),
	)
	if ret != 0 {
		return fmt.Errorf("SetEntriesInAcl: %w", e1)
	}
	defer procLocalFree.Call(newAcl)

	// Set the cleaned DACL.
	ret, _, e1 = procSetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(seFileObject),
		uintptr(daclSecurityInformation),
		0, 0,
		newAcl,
		0,
	)
	if ret != 0 {
		return fmt.Errorf("SetNamedSecurityInfo: %w", e1)
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
	//   DWORD DisableSidCount,      PSID_AND_ATTRIBUTES SidsToDisable,
	//   DWORD DeletePrivilegeCount, PLUID_AND_ATTRIBUTES PrivilegesToDelete,
	//   DWORD RestrictedSidCount,   PSID_AND_ATTRIBUTES SidsToRestrict,
	//   PHANDLE NewToken
	// );
	r1, _, e1 := procCreateRestrictedToken.Call(
		uintptr(procToken),                                    // ExistingTokenHandle
		uintptr(disableMaxPrivilege|luaToken|writeRestricted), // Flags
		0, 0, // no SIDs to disable
		0, 0, // no privileges to delete
		1,                                     // RestrictedSidCount
		uintptr(unsafe.Pointer(&sidAttrs[0])), // SidsToRestrict
		uintptr(unsafe.Pointer(&newToken)),    // NewToken
	)
	if r1 == 0 {
		return 0, fmt.Errorf("CreateRestrictedToken: %w", e1)
	}
	return newToken, nil
}
