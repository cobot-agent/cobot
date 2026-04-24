package sandbox

// NamespaceConfig describes Linux namespace isolation settings.
type NamespaceConfig struct {
	MountProc  bool // mount /proc (implies --unshare-pid)
	UnshareNet bool // isolate network namespace (set when AllowNetwork=false)
	MountDev   bool // mount /dev with null, zero, random, urandom
	TmpfsTmp   bool // mount tmpfs on /tmp
	BindRoot   bool // bind-mount Root to VirtualRoot (read-only)
}

// DefaultNamespaceConfig returns a sane default for a sandboxed shell.
// TmpfsTmp is false by default because mounting a fresh /tmp breaks many
// development tools that rely on the host /tmp (e.g. go test uses /tmp for
// test files). Enable it explicitly for tighter isolation.
func DefaultNamespaceConfig() NamespaceConfig {
	return NamespaceConfig{
		MountProc:  true,
		UnshareNet: false, // caller sets based on AllowNetwork
		MountDev:   true,
		TmpfsTmp:   false,
		BindRoot:   true,
	}
}
