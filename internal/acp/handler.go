package acp

import (
	"context"
	"strings"

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
	id := newSessionID()
	sessCtx, cancel := context.WithCancel(context.Background())
	s.sessions.Put(&Session{
		ID:       id,
		CWD:      req.CWD,
		Ctx:      sessCtx,
		CancelFn: cancel,
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
		return acpapi.PromptResponse{}, jrpc2.Errorf(jrpc2.InvalidParams, "agent error: %v", err)
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
