package tools

import (
	"github.com/cobot-agent/cobot/internal/sandbox"
)

// sandboxResolvePath resolves and validates a path within the sandbox.
// It delegates to Sandbox.Resolve — the single entry point for virtual→real
// path translation used by all filesystem and shell tools.
func sandboxResolvePath(sb *sandbox.Sandbox, path string, write bool) (string, error) {
	return sb.Resolve(path, write)
}

// sandboxRewriteErr rewrites real filesystem paths in an error back to virtual paths.
// It delegates to Sandbox.RewriteError.
func sandboxRewriteErr(sb *sandbox.Sandbox, err error) error {
	return sb.RewriteError(err)
}

// sandboxTool provides common sandbox functionality for tools.
// All tools use *sandbox.Sandbox instead of raw *sandbox.SandboxConfig,
// ensuring consistent virtual path handling across filesystem and shell operations.
type sandboxTool struct {
	sandbox *sandbox.Sandbox
}

// describeWithSandbox appends the sandbox notice to a tool description.
// It delegates to Sandbox.Describe so all tools show consistent virtual path info.
func (s *sandboxTool) describeWithSandbox(desc string) string {
	return s.sandbox.Describe(desc)
}
