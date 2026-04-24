package sandbox

// HandleSandboxChildMode is a no-op placeholder.
// In a future implementation it will detect sandbox child-mode re-execution
// and enter Linux namespaces before exec'ing the target command.
func HandleSandboxChildMode() bool {
	return false
}
