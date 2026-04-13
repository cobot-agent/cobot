package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"sync"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerConfig struct {
	Command string
	Args    []string
	Env     []string
}

type MCPManager struct {
	client   *mcp.Client
	mu       sync.Mutex
	sessions map[string]*mcp.ClientSession
	configs  map[string]ServerConfig
}

func NewMCPManager() *MCPManager {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "cobot",
		Version: "0.1.0",
	}, nil)

	return &MCPManager{
		client:   client,
		sessions: make(map[string]*mcp.ClientSession),
		configs:  make(map[string]ServerConfig),
	}
}

func (m *MCPManager) Connect(ctx context.Context, name string, cfg ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[name]; exists {
		return fmt.Errorf("server %q already connected", name)
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}

	transport := &mcp.CommandTransport{Command: cmd}
	session, err := m.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to server %q: %w", name, err)
	}

	m.sessions[name] = session
	m.configs[name] = cfg
	return nil
}

func (m *MCPManager) Disconnect(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[name]
	if !exists {
		return fmt.Errorf("server %q not connected", name)
	}

	err := session.Close()
	delete(m.sessions, name)
	delete(m.configs, name)
	return err
}

func (m *MCPManager) ListTools(ctx context.Context, name string) ([]*mcp.Tool, error) {
	m.mu.Lock()
	session, exists := m.sessions[name]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("server %q not connected", name)
	}

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("list tools from server %q: %w", name, err)
	}

	return result.Tools, nil
}

func (m *MCPManager) CallTool(ctx context.Context, serverName, toolName string, args json.RawMessage) (string, error) {
	m.mu.Lock()
	session, exists := m.sessions[serverName]
	m.mu.Unlock()

	if !exists {
		return "", fmt.Errorf("server %q not connected", serverName)
	}

	var arguments any
	if len(args) > 0 {
		arguments = make(map[string]any)
		if err := json.Unmarshal(args, &arguments); err != nil {
			return "", fmt.Errorf("unmarshal arguments: %w", err)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return "", fmt.Errorf("call tool %q on server %q: %w", toolName, serverName, err)
	}

	if result.IsError {
		return extractText(result.Content), fmt.Errorf("tool %q returned error: %s", toolName, extractText(result.Content))
	}

	return extractText(result.Content), nil
}

func (m *MCPManager) ToolAdapters(ctx context.Context, serverName string) ([]*MCPToolAdapter, error) {
	tools, err := m.ListTools(ctx, serverName)
	if err != nil {
		return nil, err
	}

	adapters := make([]*MCPToolAdapter, 0, len(tools))
	for _, tool := range tools {
		tool := tool
		var schema json.RawMessage
		if tool.InputSchema != nil {
			schema, _ = json.Marshal(tool.InputSchema)
		}

		adapter := &MCPToolAdapter{
			name:        tool.Name,
			description: tool.Description,
			schema:      schema,
			callFunc: func(ctx context.Context, args json.RawMessage) (string, error) {
				return m.CallTool(ctx, serverName, tool.Name, args)
			},
		}
		adapters = append(adapters, adapter)
	}

	return adapters, nil
}

func (m *MCPManager) ConnectSSE(ctx context.Context, name string, entry *RegistryEntry) error {
	httpClient := &http.Client{}
	if len(entry.Headers) > 0 {
		httpClient.Transport = &headerTransport{
			base:    http.DefaultTransport,
			headers: entry.Headers,
		}
	}

	transport := &mcp.SSEClientTransport{Endpoint: entry.URL, HTTPClient: httpClient}
	session, err := m.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to server %q: %w", name, err)
	}

	m.sessions[name] = session
	return nil
}

func (m *MCPManager) ConnectFromRegistry(ctx context.Context, name string, entry *RegistryEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[name]; exists {
		return fmt.Errorf("server %q already connected", name)
	}

	if entry.Transport == "sse" || entry.Transport == "http" {
		return m.ConnectSSE(ctx, name, entry)
	}

	var env []string
	for k, v := range entry.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	cfg := ServerConfig{
		Command: entry.Command,
		Args:    entry.Args,
		Env:     env,
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = cfg.Env
	}

	transport := &mcp.CommandTransport{Command: cmd}
	session, err := m.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to server %q: %w", name, err)
	}

	m.sessions[name] = session
	m.configs[name] = cfg
	return nil
}

func (m *MCPManager) ConnectEnabled(ctx context.Context, registry map[string]*RegistryEntry, enabled []string) error {
	for _, name := range enabled {
		entry, ok := registry[name]
		if !ok {
			return fmt.Errorf("server %q not found in registry", name)
		}
		if err := m.ConnectFromRegistry(ctx, name, entry); err != nil {
			return fmt.Errorf("connect server %q: %w", name, err)
		}
	}
	return nil
}

func (m *MCPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, session := range m.sessions {
		if session != nil {
			session.Close()
		}
		delete(m.sessions, name)
		delete(m.configs, name)
	}
}

func extractText(contents []mcp.Content) string {
	var text string
	for _, content := range contents {
		if tc, ok := content.(*mcp.TextContent); ok {
			if text != "" {
				text += "\n"
			}
			text += tc.Text
		}
	}
	return text
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}
