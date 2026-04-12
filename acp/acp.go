package acp

// Server is a minimal ACP server scaffold for internal testing.
// This stub does not depend on internal agent types to avoid import cycles.

// Server is a minimal ACP server scaffold for internal testing.
// It is a light-weight placeholder to satisfy the public ServeACP scaffolding.
type Server struct{}

// NewServer creates a new ACP server instance. The input is kept as interface{}
// to avoid importing the internal Agent type and creating an import cycle.
func NewServer(a interface{}) *Server {
	return &Server{}
}
