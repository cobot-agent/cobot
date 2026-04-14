package acp

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"

	"github.com/cobot-agent/cobot/internal/agent"
	"github.com/cobot-agent/cobot/internal/workspace"
)

const cleanupInterval = 5 * time.Minute

type ACPServer struct {
	agent    *agent.Agent
	sessions *SessionStore
	wsMgr    *workspace.Manager
	cancel   context.CancelFunc
}

func NewACPServer(a *agent.Agent, wsMgr *workspace.Manager) *ACPServer {
	return &ACPServer{
		agent:    a,
		sessions: NewSessionStore(),
		wsMgr:    wsMgr,
	}
}

func (s *ACPServer) Run(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	go s.cleanupLoop(ctx)

	srv := jrpc2.NewServer(s.handlerMap(), &jrpc2.ServerOptions{
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

func (s *ACPServer) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed := s.sessions.RemoveExpired()
			if removed > 0 {
				slog.Debug("cleaned up expired sessions", "count", removed)
			}
		}
	}
}

func (s *ACPServer) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	s.sessions.RemoveAll()
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
		"session/delete": handler.New(s.handleSessionDelete),
	}
}
