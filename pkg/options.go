package cobot

type Config struct {
	ConfigPath  string  `yaml:"config_path"`
	Workspace   string  `yaml:"workspace"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature" json:"temperature,omitempty"`
	// Migration-friendly: new default tool set; old configs may use `tools`.
	DefaultTools ToolsConfig               `yaml:"default_tools" json:"default_tools,omitempty"`
	Tools        ToolsConfig               `yaml:"tools,omitempty" json:"tools,omitempty"`
	MaxTurns     int                       `yaml:"max_turns"`
	SystemPrompt string                    `yaml:"system_prompt"`
	Verbose      bool                      `yaml:"verbose"`
	APIKeys      map[string]string         `yaml:"api_keys"`
	Providers    map[string]ProviderConfig `yaml:"providers"`
	Memory       MemoryConfig              `yaml:"memory"`
}

// migrateTools preserves backward compatibility by migrating legacy
// 'Tools' data into the new 'DefaultTools' field when the new field is empty.
func (c *Config) migrateTools() {
	// If there is no data in DefaultTools, but there is data in Tools, copy it over.
	if len(c.DefaultTools.Builtin) == 0 && len(c.DefaultTools.MCPServers) == 0 {
		if len(c.Tools.Builtin) > 0 || len(c.Tools.MCPServers) > 0 {
			c.DefaultTools = c.Tools
		}
	}
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
