package openai

import (
	"encoding/json"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type chatMessage struct {
	Role         string         `json:"role"`
	Content      string         `json:"content"`
	ToolCalls    []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID   string         `json:"tool_call_id,omitempty"`
	Name         string         `json:"name,omitempty"`
	FunctionCall *chatFuncCall  `json:"function_call,omitempty"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFuncCall `json:"function"`
}

type chatFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatToolFunc `json:"function"`
}

type chatToolFunc struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Tools       []chatTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

type chatChoice struct {
	Index        int         `json:"index"`
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamChunk struct {
	ID      string         `json:"id"`
	Choices []streamChoice `json:"choices"`
}

type streamChoice struct {
	Index        int         `json:"index"`
	Delta        streamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type streamDelta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []toolCallDelta `json:"tool_calls,omitempty"`
}

type toolCallDelta struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function *funcCallDelta `json:"function,omitempty"`
}

type funcCallDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func toProviderResponse(resp *chatResponse) *cobot.ProviderResponse {
	if len(resp.Choices) == 0 {
		return &cobot.ProviderResponse{}
	}

	choice := resp.Choices[0]
	result := &cobot.ProviderResponse{
		Content: choice.Message.Content,
		Usage: cobot.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	switch choice.FinishReason {
	case "stop":
		result.StopReason = cobot.StopEndTurn
	case "length":
		result.StopReason = cobot.StopMaxTokens
	case "tool_calls":
		result.StopReason = cobot.StopEndTurn
	default:
		result.StopReason = cobot.StopEndTurn
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, cobot.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return result
}

func fromProviderMessages(msgs []cobot.Message) []chatMessage {
	result := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		cm := chatMessage{
			Role:    string(m.Role),
			Content: m.Content,
		}

		for _, tc := range m.ToolCalls {
			cm.ToolCalls = append(cm.ToolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: chatFuncCall{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}

		if m.ToolResult != nil {
			cm.ToolCallID = m.ToolResult.CallID
			cm.Content = m.ToolResult.Output
			if m.ToolResult.Error != "" {
				cm.Content = m.ToolResult.Error
			}
		}

		result = append(result, cm)
	}
	return result
}

func fromProviderTools(tools []cobot.ToolDef) []chatTool {
	result := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		result = append(result, chatTool{
			Type: "function",
			Function: chatToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}
