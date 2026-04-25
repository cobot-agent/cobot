//go:build windows && !race

package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This file implements the Windows Restricted Token sandbox.
// It is excluded from builds with -race because CreateRestrictedToken +
// go test -race causes STATUS_HEAP_CORRUPTION (0xc0000374) on Windows.

var (
	winOnce                      sync.Once
	winProcCreateRestrictedToken *windows.LazyProc
	winProcGetNamedSecurityInfoW *windows.LazyProc
	winProcSetEntriesInAclW      *windows.LazyProc
	winProcSetNamedSecurityInfoW *windows.LazyProc
	winProcLocalFree             *windows.LazyProc
)

func initWinProcs() {
	winOnce.Do(func() {
		advapi32 := windows.NewLazyDLL("advapi32.dll")
		kernel32 := windows.NewLazyDLL("kernel32.dll")
		winProcCreateRestrictedToken = advapi32.NewProc("CreateRestrictedToken")
		winProcGetNamedSecurityInfoW = advapi32.NewProc("GetNamedSecurityInfoW")
		winProcSetEntriesInAclW = advapi32.NewProc("SetEntriesInAclW")
		winProcSetNamedSecurityInfoW = advapi32.NewProc("SetNamedSecurityInfoW")
		winProcLocalFree = kernel32.NewProc("LocalFree")
	})
}

const (
	disableMaxPrivilege = 0x0001
	luaToken            = 0x0004
	writeRestricted     = 0x0008

	seFileObject            = 1
	daclSecurityInformation = 0x00000004

	grantAccess  = 1
	revokeAccess = 4

	subContainersAndObjectsInherit = 0x03

	trusteeIsSID     = 0
	trusteeIsUnknown = 0

	fileAllAccess = 0x001F01FF
)

type explicitAccessW struct {
	AccessPermissions uint32
	AccessMode        uint32
	Inheritance       uint32
	Trustee           trusteeW
}

type trusteeW struct {
	pMultipleTrustee         uintptr
	MultipleTrusteeOperation uint32
	TrusteeForm              uint32
	TrusteeType              uint32
	PtstrName                uintptr
}

type sidAndAttributes struct {
	Sid        *windows.SID
	Attributes uint32
}

func restrictedTokenLaunch(ctx context.Context, req *LaunchRequest) ([]byte, error) {
	if req.Config == nil {
		slog.Warn("sandbox: no config provided, running without Restricted Token enforcement", "command", req.Command)
		return hostExec(ctx, req)
	}

	initWinProcs()

	capSid, err := generateCapabilitySID()
	if err != nil {
		slog.Warn("sandbox: failed to generate capability SID, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}
	defer windows.FreeSid(capSid)

	writeDirs := buildWriteDirs(req.Config)
	grantedDirs := make([]string, 0, len(writeDirs))
	for _, dir := range writeDirs {
		if err := grantAccessACL(dir, capSid); err != nil {
			slog.Warn("sandbox: failed to grant write ACL", "dir", dir, "error", err)
		} else {
			grantedDirs = append(grantedDirs, dir)
		}
	}
	defer func() {
		for _, dir := range grantedDirs {
			if err := revokeAccessACL(dir, capSid); err != nil {
				slog.Debug("sandbox: failed to revoke ACL (non-fatal)", "dir", dir, "error", err)
			}
		}
	}()

	rtoken, err := createRestrictedToken(capSid)
	if err != nil {
		slog.Warn("sandbox: failed to create restricted token, falling back to unsandboxed execution", "error", err)
		return hostExec(ctx, req)
	}
	defer rtoken.Close()

	cmd := exec.CommandContext(ctx, req.Shell, req.ShellFlag, req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(rtoken),
	}
	return cmd.CombinedOutput()
}

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

func grantAccessACL(dir string, sid *windows.SID) error {
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	var dacl, sd uintptr
	ret, _, e1 := winProcGetNamedSecurityInfoW.Call(
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
	defer winProcLocalFree.Call(sd)

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

	var newAcl uintptr
	ret, _, e1 = winProcSetEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		dacl,
		uintptr(unsafe.Pointer(&newAcl)),
	)
	if ret != 0 {
		return fmt.Errorf("SetEntriesInAcl: %w", e1)
	}
	defer winProcLocalFree.Call(newAcl)

	ret, _, e1 = winProcSetNamedSecurityInfoW.Call(
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

func revokeAccessACL(dir string, sid *windows.SID) error {
	dirPtr, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	var dacl, sd uintptr
	ret, _, e1 := winProcGetNamedSecurityInfoW.Call(
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
	defer winProcLocalFree.Call(sd)

	ea := explicitAccessW{
		AccessMode: revokeAccess,
		Trustee: trusteeW{
			TrusteeForm: trusteeIsSID,
			TrusteeType: trusteeIsUnknown,
			PtstrName:   uintptr(unsafe.Pointer(sid)),
		},
	}

	var newAcl uintptr
	ret, _, e1 = winProcSetEntriesInAclW.Call(
		1,
		uintptr(unsafe.Pointer(&ea)),
		dacl,
		uintptr(unsafe.Pointer(&newAcl)),
	)
	if ret != 0 {
		return fmt.Errorf("SetEntriesInAcl: %w", e1)
	}
	defer winProcLocalFree.Call(newAcl)

	ret, _, e1 = winProcSetNamedSecurityInfoW.Call(
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

	sidAttrs := [1]sidAndAttributes{
		{Sid: capSid, Attributes: 0},
	}

	var newToken windows.Token
	r1, _, e1 := winProcCreateRestrictedToken.Call(
		uintptr(procToken),
		uintptr(disableMaxPrivilege|luaToken|writeRestricted),
		0, 0,
		0, 0,
		1,
		uintptr(unsafe.Pointer(&sidAttrs[0])),
		uintptr(unsafe.Pointer(&newToken)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("CreateRestrictedToken: %w", e1)
	}
	return newToken, nil
}
