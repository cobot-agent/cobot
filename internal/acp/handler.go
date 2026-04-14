package acp

import (
	"context"
	"strings"
	"time"

	"github.com/creachadair/jrpc2"

	acpapi "github.com/cobot-agent/cobot/api/acp"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func (s *ACPServer) handleInitialize(ctx context.Context, req acpapi.InitializeRequest) (acpapi.InitializeResponse, error) {
	return acpapi.InitializeResponse{
		ProtocolVersion: req.ProtocolVersion,
		AgentCapabilities: acpapi.AgentCapabilities{
			LoadSession: false,
		},
		AgentInfo: &acpapi.Implementation{
			Name:    "cobot",
			Title:   "Cobot Agent",
			Version: "0.1.0",
		},
		AuthMethods: []acpapi.AuthMethod{},
	}, nil
}

func (s *ACPServer) handleSessionNew(ctx context.Context, req acpapi.NewSessionRequest) (acpapi.NewSessionResponse, error) {
	if req.Workspace != "" && s.wsMgr != nil {
		if _, err := s.wsMgr.Resolve(req.Workspace); err != nil {
			return acpapi.NewSessionResponse{}, jrpc2.Errorf(jrpc2.InvalidParams, "workspace not found: %s: %v", req.Workspace, err)
		}
	}

	id := newSessionID()
	sessCtx, cancel := context.WithCancel(ctx)
	s.sessions.Put(&Session{
		ID:        id,
		CWD:       req.CWD,
		Workspace: req.Workspace,
		Agent:     req.Agent,
		Ctx:       sessCtx,
		CancelFn:  cancel,
	})
	return acpapi.NewSessionResponse{
		SessionID: id,
	}, nil
}

func (s *ACPServer) handleSessionPrompt(ctx context.Context, req acpapi.PromptRequest) (acpapi.PromptResponse, error) {
	sess, ok := s.sessions.Get(req.SessionID)
	if !ok {
		return acpapi.PromptResponse{}, jrpc2.Errorf(jrpc2.InvalidParams, "session not found: %s", req.SessionID)
	}
	sess.LastActive = time.Now()

	var parts []string
	for _, block := range req.Prompt {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	text := strings.Join(parts, "\n")

	s.notify(ctx, "session/update", acpapi.SessionUpdateNotification{
		SessionID: sess.ID,
		Update: acpapi.SessionUpdate{
			SessionUpdate: "agent_message_chunk",
		},
	})

	promptCtx := sess.Ctx
	if promptCtx == nil {
		promptCtx = ctx
	}
	_, err := s.agent.Prompt(promptCtx, text)
	if err != nil {
		return acpapi.PromptResponse{}, jrpc2.Errorf(jrpc2.InternalError, "agent error: %v", err)
	}

	s.notify(ctx, "session/update", acpapi.SessionUpdateNotification{
		SessionID: sess.ID,
		Update: acpapi.SessionUpdate{
			SessionUpdate: "agent_message_chunk",
		},
	})

	return acpapi.PromptResponse{
		StopReason: string(cobot.StopEndTurn),
	}, nil
}

func (s *ACPServer) handleSessionCancel(ctx context.Context, req acpapi.CancelNotification) (any, error) {
	sess, ok := s.sessions.Get(req.SessionID)
	if ok && sess.CancelFn != nil {
		sess.CancelFn()
	}
	return nil, nil
}

func (s *ACPServer) handleSessionDelete(ctx context.Context, req acpapi.CancelNotification) (any, error) {
	sess, ok := s.sessions.Get(req.SessionID)
	if !ok {
		return nil, jrpc2.Errorf(jrpc2.InvalidParams, "session not found: %s", req.SessionID)
	}
	if sess.CancelFn != nil {
		sess.CancelFn()
	}
	s.sessions.Delete(req.SessionID)
	return nil, nil
}
