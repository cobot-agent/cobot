package acp

import (
	"context"
	"net"
)

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

// Serve starts a very lightweight ACP server listening on the provided address.
// This is a minimal scaffold used by the SDK. It blocks until the provided
// context is canceled or the listener is closed due to cancellation.
func (s *Server) Serve(ctx context.Context, addr string) error {
	// If no address is provided, just wait for the context cancellation.
	if addr == "" {
		<-ctx.Done()
		return ctx.Err()
	}

	// Start a simple TCP listener. This is a lightweight placeholder
	// and does not implement a full ACP protocol – it satisfies the
	// SDK surface required by ServeACP.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	// Ensure we stop promptly when the context is canceled.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// If the context was canceled or the listener closed, exit gracefully.
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		// Echo-like placeholder: immediately close the connection.
		_ = conn.Close()
	}
}
