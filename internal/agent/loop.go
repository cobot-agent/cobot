package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	cobot "github.com/cobot-agent/cobot/pkg"
)

func (a *Agent) buildRequest(ctx context.Context) *cobot.ProviderRequest {
	return &cobot.ProviderRequest{
		Model:    a.config.Model,
		Messages: a.buildMessages(ctx),
		Tools:    a.tools.ToolDefs(),
	}
}

func (a *Agent) executeToolsAndCollect(ctx context.Context, toolCalls []cobot.ToolCall) []*cobot.ToolResult {
	results := a.tools.ExecuteParallel(ctx, toolCalls)
	for _, tr := range results {
		slog.Debug("tool completed", "call_id", tr.CallID, "result_bytes", len(tr.Output))
		a.AddMessage(cobot.Message{
			Role:       cobot.RoleTool,
			ToolResult: tr,
		})
	}
	return results
}

// runLoop executes the core agent loop shared by Prompt and Stream.
// The executeTurn callback handles mode-specific provider interaction
// (Complete vs Stream) and returns stop=true when the loop should
// terminate (final response received with no tool calls).
func (a *Agent) runLoop(ctx context.Context, prompt, debugLabel string, executeTurn func(ctx context.Context, req *cobot.ProviderRequest, turn int) (stop bool, err error)) error {
	slog.Debug("session", "event", debugLabel, "prompt", prompt)
	a.AddMessage(cobot.Message{Role: cobot.RoleUser, Content: prompt})

	for turn := 0; turn < a.config.MaxTurns; turn++ {
		req := a.buildRequest(ctx)
		slog.Debug("agent turn", "event", debugLabel, "turn", turn, "model", req.Model, "msgs", len(req.Messages), "tools", len(req.Tools))

		stop, err := executeTurn(ctx, req, turn)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}

	return cobot.ErrMaxTurnsExceeded
}

func (a *Agent) Prompt(ctx context.Context, message string) (*cobot.ProviderResponse, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}
	ctx = a.deriveCtx(ctx)

	var result *cobot.ProviderResponse
	err := a.runLoop(ctx, message, "prompt", func(ctx context.Context, req *cobot.ProviderRequest, turn int) (bool, error) {
		resp, err := a.provider.Complete(ctx, req)
		if err != nil {
			slog.Error("provider error", "err", err)
			return false, fmt.Errorf("provider error: %w", err)
		}

		slog.Debug("agent response", "turn", turn, "content_len", len(resp.Content), "tool_calls", len(resp.ToolCalls), "stop", resp.StopReason)
		a.usageTracker.Add(resp.Usage)
		a.AddMessage(cobot.Message{
			Role:      cobot.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		if len(resp.ToolCalls) == 0 {
			result = resp
			return true, nil
		}

		a.executeToolsAndCollect(ctx, resp.ToolCalls)
		return false, nil
	})

	return result, err
}

func (a *Agent) Stream(ctx context.Context, message string) (<-chan cobot.Event, error) {
	if a.provider == nil {
		return nil, cobot.ErrProviderNotConfigured
	}
	ctx = a.deriveCtx(ctx)

	ch := make(chan cobot.Event, 64)

	a.streamWg.Add(1)
	go func() {
		defer a.streamWg.Done()
		defer close(ch)
		a.streamMu.Lock()
		defer a.streamMu.Unlock()

		err := a.runLoop(ctx, message, "stream", func(ctx context.Context, req *cobot.ProviderRequest, turn int) (bool, error) {
			streamCh, err := a.provider.Stream(ctx, req)
			if err != nil {
				slog.Error("provider stream error", "err", err)
				return false, fmt.Errorf("provider stream error: %w", err)
			}

			var content string
			var toolCalls []cobot.ToolCall
			var turnUsage cobot.Usage
			for chunk := range streamCh {
				select {
				case <-ctx.Done():
					return false, ctx.Err()
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
				if chunk.Usage != nil {
					turnUsage.PromptTokens += chunk.Usage.PromptTokens
					turnUsage.CompletionTokens += chunk.Usage.CompletionTokens
					turnUsage.TotalTokens += chunk.Usage.TotalTokens
				}
				if chunk.Done && len(toolCalls) == 0 {
					slog.Debug("stream done", "turn", turn, "content_len", len(content))
					a.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content})
					a.usageTracker.Add(turnUsage)
					ch <- cobot.Event{Type: cobot.EventDone, Done: true, Usage: &turnUsage}
					return true, nil
				}
			}

			if len(toolCalls) > 0 {
				slog.Debug("stream tool calls", "turn", turn, "count", len(toolCalls))
				a.AddMessage(cobot.Message{Role: cobot.RoleAssistant, Content: content, ToolCalls: toolCalls})
				results := a.tools.ExecuteParallel(ctx, toolCalls)
				for _, tr := range results {
					ch <- cobot.Event{Type: cobot.EventToolResult, Content: tr.Output}
					a.AddMessage(cobot.Message{Role: cobot.RoleTool, ToolResult: tr})
				}
			}
			return false, nil
		})

		if err != nil {
			if errors.Is(err, cobot.ErrMaxTurnsExceeded) {
				slog.Debug("max turns exceeded", "turns", a.config.MaxTurns)
			}
			ch <- cobot.Event{Type: cobot.EventError, Error: err.Error()}
		}
	}()

	return ch, nil
}
