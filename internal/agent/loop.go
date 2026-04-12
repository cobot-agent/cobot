package agent

import (
	"context"
	"fmt"

	"github.com/cobot-agent/cobot/internal/debug"
	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	debug.Session("prompt", message)
	a.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		msgs := a.buildMessages(ctx)
		req := &cobot.ProviderRequest{
			Model:    a.config.Model,
			Messages: msgs,
			Tools:    a.tools.ToolDefs(),
		}

		debug.Agent(turn, "request", fmt.Sprintf("model=%s msgs=%d tools=%d", req.Model, len(req.Messages), len(req.Tools)))

		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			debug.Error("provider", err)
			return nil, fmt.Errorf("provider error: %w", err)
		}

		debug.Agent(turn, "response", fmt.Sprintf("content_len=%d tool_calls=%d stop=%s", len(resp.Content), len(resp.ToolCalls), resp.StopReason))

		a.AddMessage(cobot.Message{
			Role:      cobot.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			return resp, nil
		}

		results := a.tools.ExecuteParallel(ctx, resp.ToolCalls)
		for _, tr := range results {
			debug.ToolResult(tr.CallID, len(tr.Output), 0)
			a.AddMessage(cobot.Message{
				Role:       cobot.RoleTool,
				ToolResult: tr,
			})
		}
	}

	return nil, cobot.ErrMaxTurnsExceeded
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan cobot.Event, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	ch := make(chan cobot.Event, 64)

	go func() {
		defer close(ch)
		debug.Session("stream", message)
		a.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

		for turn := 0; turn < a.config.MaxTurns; turn++ {
			msgs := a.buildMessages(ctx)
			req := &cobot.ProviderRequest{
				Model:    a.config.Model,
				Messages: msgs,
				Tools:    a.tools.ToolDefs(),
			}

			debug.Agent(turn, "stream_request", fmt.Sprintf("model=%s msgs=%d tools=%d", req.Model, len(req.Messages), len(req.Tools)))

			streamCh, err := a.provider.Stream(ctx, req)
			if err != nil {
				debug.Error("provider.stream", err)
				ch <- cobot.Event{Type: cobot.EventError, Error: err}
				return
			}

			var content string
			var toolCalls []cobot.ToolCall
			for chunk := range streamCh {
				select {
				case <-ctx.Done():
					ch <- cobot.Event{Type: cobot.EventError, Error: ctx.Err()}
					return
				default:
				}
				if chunk.Content != "" {
					content += chunk.Content
					ch <- cobot.Event{Type: cobot.EventText, Content: chunk.Content}
				}
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
					ch <- cobot.Event{Type: cobot.EventToolCall, ToolCall: chunk.ToolCall}
				}
				if chunk.Done && len(toolCalls) == 0 {
					debug.Agent(turn, "stream_done", fmt.Sprintf("content_len=%d", len(content)))
					a.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content})
					ch <- cobot.Event{Type: cobot.EventDone, Done: true}
					return
				}
			}

			if len(toolCalls) > 0 {
				debug.Agent(turn, "stream_tool_calls", fmt.Sprintf("count=%d", len(toolCalls)))
				a.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content, ToolCalls: toolCalls})
				results := a.tools.ExecuteParallel(ctx, toolCalls)
				for _, tr := range results {
					ch <- cobot.Event{Type: cobot.EventToolResult, Content: tr.Output}
					a.AddMessage(cobot.Message{Role: cobot.RoleTool, ToolResult: tr})
				}
			}
		}

		debug.Log("agent", "max turns exceeded", "turns", a.config.MaxTurns)
		ch <- cobot.Event{Type: cobot.EventError, Error: cobot.ErrMaxTurnsExceeded}
	}()

	return ch, nil
}
