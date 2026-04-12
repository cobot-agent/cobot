package openai

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

const maxScannerBuffer = 256 * 1024 // 256KB

type Provider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewProvider(apiKey, baseURL string) *Provider {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Provider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (p *Provider) Name() string {
	return "openai"
}

func (p *Provider) Complete(ctx context.Context, req *cobot.ProviderRequest) (*cobot.ProviderResponse, error) {
	body := chatRequest{
		Model:       req.Model,
		Messages:    fromProviderMessages(req.Messages),
		Tools:       fromProviderTools(req.Tools),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	respBody, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	var resp chatResponse
	if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	return toProviderResponse(&resp), nil
}

func (p *Provider) Stream(ctx context.Context, req *cobot.ProviderRequest) (<-chan cobot.ProviderChunk, error) {
	body := chatRequest{
		Model:       req.Model,
		Messages:    fromProviderMessages(req.Messages),
		Tools:       fromProviderTools(req.Tools),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

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

func (p *Provider) doRequest(ctx context.Context, body chatRequest) (io.ReadCloser, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	url := p.baseURL + "/chat/completions"
	debug.Request("openai", "POST", url, len(jsonBody))

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai: request failed: %w", err)
	}

	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respData, _ := io.ReadAll(resp.Body)
		debug.Response("openai", resp.StatusCode, len(respData), elapsed)
		return nil, fmt.Errorf("openai: API error %d: %s", resp.StatusCode, string(respData))
	}

	debug.Response("openai", resp.StatusCode, 0, elapsed)
	return resp.Body, nil
}

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
		if data == "[DONE]" {
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			ch <- cobot.ProviderChunk{
				Content: fmt.Sprintf("[stream error: malformed data: %v]", err),
			}
			continue
		}

		for _, choice := range chunk.Choices {
			for _, tc := range choice.Delta.ToolCalls {
				ptc, exists := pending[tc.Index]
				if !exists {
					ptc = &pendingToolCall{}
					pending[tc.Index] = ptc
				}
				if tc.ID != "" {
					ptc.ID = tc.ID
				}
				if tc.Function != nil {
					if tc.Function.Name != "" {
						ptc.Name = tc.Function.Name
					}
					ptc.Args.WriteString(tc.Function.Arguments)
				}
			}

			pc := cobot.ProviderChunk{
				Content: choice.Delta.Content,
			}

			if choice.FinishReason != nil {
				if *choice.FinishReason == "tool_calls" {
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
				pc.Done = true
			}

			ch <- pc
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
