package acp

import (
	"context"
	"os"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/workspace"
)

type ACPServer struct {
	agent    *agent.Agent
	sessions *SessionStore
	wsMgr    *workspace.Manager
}

func NewACPServer(a *agent.Agent, wsMgr *workspace.Manager) *ACPServer {
	return &ACPServer{
		agent:    a,
		sessions: NewSessionStore(),
		wsMgr:    wsMgr,
	}
}

func (s *ACPServer) Run(ctx context.Context) error {
	assigner := handler.Map{
		"initialize":     handler.New(s.handleInitialize),
		"session/new":    handler.New(s.handleSessionNew),
		"session/prompt": handler.New(s.handleSessionPrompt),
		"session/cancel": handler.New(s.handleSessionCancel),
	}

	srv := jrpc2.NewServer(assigner, &jrpc2.ServerOptions{
		AllowPush:   true,
		Concurrency: 0,
	})

	ch := channel.Line(os.Stdin, os.Stdout)
	srv.Start(ch)

	go func() {
		<-ctx.Done()
		srv.Stop()
	}()

	return srv.Wait()
}

func (s *ACPServer) notify(ctx context.Context, method string, params any) {
	srv := jrpc2.ServerFromContext(ctx)
	if srv != nil {
		srv.Notify(ctx, method, params)
	}
}

func (s *ACPServer) handlerMap() handler.Map {
	return handler.Map{
		"initialize":     handler.New(s.handleInitialize),
		"session/new":    handler.New(s.handleSessionNew),
		"session/prompt": handler.New(s.handleSessionPrompt),
		"session/cancel": handler.New(s.handleSessionCancel),
	}
}
