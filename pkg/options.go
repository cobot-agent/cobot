package cobot

type Config struct {
	ConfigPath   string                    `yaml:"config_path"`
	Workspace    string                    `yaml:"workspace"`
	Model        string                    `yaml:"model"`
	MaxTurns     int                       `yaml:"max_turns"`
	SystemPrompt string                    `yaml:"system_prompt"`
	Verbose      bool                      `yaml:"verbose"`
	APIKeys      map[string]string         `yaml:"api_keys"`
	Providers    map[string]ProviderConfig `yaml:"providers"`
	Memory       MemoryConfig              `yaml:"memory"`
	Tools        ToolsConfig               `yaml:"tools"`
}

type MemoryConfig struct {
	Enabled    bool
	BadgerPath string
	BlevePath  string
}

type ProviderConfig struct {
	BaseURL string            `yaml:"base_url"`
	Headers map[string]string `yaml:"headers"`
}

type ToolsConfig struct {
	Builtin    []string                   `yaml:"builtin"`
	MCPServers map[string]MCPServerConfig `yaml:"mcp_servers"`
}

type MCPServerConfig struct {
	Transport string            `yaml:"transport"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	Env       map[string]string `yaml:"env"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: 50,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
	}
}
