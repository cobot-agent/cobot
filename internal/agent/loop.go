package agent

import (
	"context"
	"fmt"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}

	a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		msgs := a.buildMessages(ctx)
		req := &cobot.ProviderRequest{
			Model:    a.config.Model,
			Messages: msgs,
			Tools:    a.tools.ToolDefs(),
		}

		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("provider error: %w", err)
		}

		a.session.AddMessage(cobot.Message{
			Role:      cobot.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			return resp, nil
		}

		results := a.tools.ExecuteParallel(ctx, resp.ToolCalls)
		for _, tr := range results {
			a.session.AddMessage(cobot.Message{
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
		a.session.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: message})

		for turn := 0; turn < a.config.MaxTurns; turn++ {
			msgs := a.buildMessages(ctx)
			req := &cobot.ProviderRequest{
				Model:    a.config.Model,
				Messages: msgs,
				Tools:    a.tools.ToolDefs(),
			}

			streamCh, err := a.provider.Stream(ctx, req)
			if err != nil {
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
					a.session.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content})
					ch <- cobot.Event{Type: cobot.EventDone, Done: true}
					return
				}
			}

			if len(toolCalls) > 0 {
				a.session.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content, ToolCalls: toolCalls})
				results := a.tools.ExecuteParallel(ctx, toolCalls)
				for _, tr := range results {
					ch <- cobot.Event{Type: cobot.EventToolResult, Content: tr.Output}
					a.session.AddMessage(cobot.Message{Role: cobot.RoleTool, ToolResult: tr})
				}
			}
		}

		ch <- cobot.Event{Type: cobot.EventError, Error: cobot.ErrMaxTurnsExceeded}
	}()

	return ch, nil
}
