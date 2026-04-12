package acp

import "encoding/json"

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
	Fs       *FileSystemCapabilities `json:"fs,omitempty"`
	Terminal bool                    `json:"terminal,omitempty"`
}

type FileSystemCapabilities struct {
	ReadTextFile  bool `json:"readTextFile"`
	WriteTextFile bool `json:"writeTextFile"`
}

type AgentCapabilities struct {
	LoadSession         bool                `json:"loadSession"`
	PromptCapabilities  *PromptCapabilities `json:"promptCapabilities,omitempty"`
	MCPCapabilities     *MCPCapabilities    `json:"mcpCapabilities,omitempty"`
	SessionCapabilities json.RawMessage     `json:"sessionCapabilities,omitempty"`
}

type PromptCapabilities struct {
	Image           bool `json:"image"`
	Audio           bool `json:"audio"`
	EmbeddedContext bool `json:"embeddedContext"`
}

type MCPCapabilities struct {
	HTTP bool `json:"http"`
	SSE  bool `json:"sse"`
}

type AuthMethod struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type InitializeRequest struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities,omitempty"`
	ClientInfo         *Implementation    `json:"clientInfo,omitempty"`
}

type InitializeResponse struct {
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         *Implementation   `json:"agentInfo,omitempty"`
	AuthMethods       []AuthMethod      `json:"authMethods,omitempty"`
}

type MCPServer struct {
	Type    string        `json:"type,omitempty"`
	Name    string        `json:"name"`
	Command string        `json:"command,omitempty"`
	Args    []string      `json:"args,omitempty"`
	Env     []EnvVariable `json:"env,omitempty"`
	URL     string        `json:"url,omitempty"`
	Headers []HTTPHeader  `json:"headers,omitempty"`
}

type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HTTPHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type NewSessionRequest struct {
	CWD        string      `json:"cwd"`
	Workspace  string      `json:"workspace,omitempty"`
	Agent      string      `json:"agent,omitempty"`
	MCPServers []MCPServer `json:"mcpServers"`
}

type NewSessionResponse struct {
	SessionID     string                `json:"sessionId"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
}

type PromptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type PromptResponse struct {
	StopReason string `json:"stopReason"`
}

type CancelNotification struct {
	SessionID string `json:"sessionId"`
}

type ContentBlock struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
	URI      string           `json:"uri,omitempty"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type SessionUpdateNotification struct {
	SessionID string        `json:"sessionId"`
	Update    SessionUpdate `json:"update"`
}

type SessionUpdate struct {
	SessionUpdate     string                `json:"sessionUpdate"`
	Content           *ContentBlock         `json:"content,omitempty"`
	ToolCallID        string                `json:"toolCallId,omitempty"`
	Title             string                `json:"title,omitempty"`
	Kind              string                `json:"kind,omitempty"`
	Status            string                `json:"status,omitempty"`
	Entries           []PlanEntry           `json:"entries,omitempty"`
	AvailableCommands []AvailableCommand    `json:"availableCommands,omitempty"`
	ModeID            string                `json:"modeId,omitempty"`
	ConfigOptions     []SessionConfigOption `json:"configOptions,omitempty"`
}

type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

type AvailableCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SessionConfigOption struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Values []ConfigValue `json:"values"`
}

type ConfigValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SessionModeState struct {
	AvailableModes []SessionMode `json:"availableModes"`
	CurrentModeID  string        `json:"currentModeId"`
}

type SessionMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type PermissionOption struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RequestPermissionRequest struct {
	SessionID string             `json:"sessionId"`
	ToolCall  ToolCallUpdate     `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type RequestPermissionResponse struct {
	Outcome string `json:"outcome"`
}

type ToolCallUpdate struct {
	ToolCallID string `json:"toolCallId"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
}

type LoadSessionRequest struct {
	SessionID  string      `json:"sessionId"`
	CWD        string      `json:"cwd"`
	MCPServers []MCPServer `json:"mcpServers"`
}

type LoadSessionResponse struct {
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
}
