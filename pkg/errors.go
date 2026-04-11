package cobot

type CobotError struct {
	Code    string
	Message string
	Cause   error
}

func (e *CobotError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *CobotError) Unwrap() error { return e.Cause }

var (
	ErrProviderNotConfigured = &CobotError{Code: "PROVIDER_NOT_CONFIGURED", Message: "LLM provider not configured"}
	ErrWorkspaceNotFound     = &CobotError{Code: "WORKSPACE_NOT_FOUND", Message: "workspace not found"}
	ErrToolNotFound          = &CobotError{Code: "TOOL_NOT_FOUND", Message: "tool not found"}
	ErrMemorySearchFailed    = &CobotError{Code: "MEMORY_SEARCH_FAILED", Message: "memory search failed"}
	ErrMaxTurnsExceeded      = &CobotError{Code: "MAX_TURNS_EXCEEDED", Message: "max turns exceeded"}
	ErrAgentCancelled        = &CobotError{Code: "AGENT_CANCELLED", Message: "agent cancelled"}
)
