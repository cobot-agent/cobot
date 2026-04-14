package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cobot-agent/cobot/internal/debug"
	cobot "github.com/cobot-agent/cobot/pkg"
)

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

var _ cobot.Provider = (*Provider)(nil)

func NewProvider(apiKey, baseURL string) *Provider {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (p *Provider) Name() string { return "anthropic" }

func (p *Provider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	body := p.buildRequest(req, false)
	respBody, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var resp messagesResponse
	if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}
	return p.toProviderResponse(&resp), nil
}

func (p *Provider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	body := p.buildRequest(req, true)
	respBody, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	ch := make(chan cobot.ProviderChunk, 64)
	go func() {
		defer close(ch)
		defer respBody.Close()
		p.readStream(respBody, ch)
	}()
	return ch, nil
}

func (p *Provider) buildRequest(req *cobot.ProviderRequest, stream bool) messagesRequest {
	var system string
	var msgs []message
	for _, m := range req.Messages {
		if m.Role == cobot.RoleSystem {
			system = m.Content
			continue
		}
		content, _ := json.Marshal(textBlock{Type: "text", Text: m.Content})
		msgs = append(msgs, message{Role: string(m.Role), Content: content})
	}

	var tools []toolDef
	for _, t := range req.Tools {
		tools = append(tools, toolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	return messagesRequest{
		Model:     strings.TrimPrefix(req.Model, "anthropic:"),
		MaxTokens: maxTokens,
		Messages:  msgs,
		System:    system,
		Tools:     tools,
		Stream:    stream,
	}
}

func (p *Provider) doRequest(ctx context.Context, body messagesRequest) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	debug.Request("anthropic", "POST", url, len(jsonBody))

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request failed: %w", err)
	}

	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		debug.Response("anthropic", resp.StatusCode, len(data), elapsed)
		return nil, fmt.Errorf("anthropic: API error %d: %s", resp.StatusCode, string(data))
	}

	debug.Response("anthropic", resp.StatusCode, 0, elapsed)
	return resp.Body, nil
}

func (p *Provider) toProviderResponse(resp *messagesResponse) *cobot.ProviderResponse {
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}
	stopReason := cobot.StopEndTurn
	if resp.StopReason == "max_tokens" {
		stopReason = cobot.StopMaxTokens
	}
	return &cobot.ProviderResponse{
		Content:    content,
		StopReason: stopReason,
		Usage: cobot.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

const maxScannerBuffer = 256 * 1024 // 256KB

// pendingToolCall tracks incremental assembly of a tool_use block across stream events.
type pendingToolCall struct {
	ID   string
	Name string
	Args strings.Builder
}

func (p *Provider) readStream(body io.ReadCloser, ch chan<- cobot.ProviderChunk) {
	pending := make(map[int]*pendingToolCall)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 4096), maxScannerBuffer)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var evt streamEvent
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "content_block_start":
			if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
				pending[evt.Index] = &pendingToolCall{
					ID:   evt.ContentBlock.ID,
					Name: evt.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			if evt.Delta != nil {
				if evt.Delta.Text != "" {
					ch <- cobot.ProviderChunk{Content: evt.Delta.Text}
				}
				if evt.Delta.PartialJSON != "" {
					if ptc, ok := pending[evt.Index]; ok {
						ptc.Args.WriteString(evt.Delta.PartialJSON)
					}
				}
			}

		case "message_delta":
			if evt.MessageDelta != nil {
				if evt.MessageDelta.StopReason == "tool_use" {
					// Emit assembled tool calls in index order.
					indices := sortedMapKeys(pending)
					for _, idx := range indices {
						ptc := pending[idx]
						ch <- cobot.ProviderChunk{
							ToolCall: &cobot.ToolCall{
								ID:        ptc.ID,
								Name:      ptc.Name,
								Arguments: json.RawMessage(ptc.Args.String()),
							},
						}
					}
				}
				ch <- cobot.ProviderChunk{Done: true}
			}

		case "message_stop":
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- cobot.ProviderChunk{
			Content: fmt.Sprintf("[stream error: %v]", err),
			Done:    true,
		}
	}
}

func sortedMapKeys(m map[int]*pendingToolCall) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
