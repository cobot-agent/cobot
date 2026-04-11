package cobot

type Config struct {
	ConfigPath   string            `yaml:"config_path"`
	Workspace    string            `yaml:"workspace"`
	Model        string            `yaml:"model"`
	MaxTurns     int               `yaml:"max_turns"`
	SystemPrompt string            `yaml:"system_prompt"`
	Verbose      bool              `yaml:"verbose"`
	APIKeys      map[string]string `yaml:"api_keys"`
}

type MemoryConfig struct {
	Enabled    bool
	BadgerPath string
	BlevePath  string
}

func DefaultConfig() *Config {
	return &Config{
		MaxTurns: 50,
		Model:    "openai:gpt-4o",
		APIKeys:  make(map[string]string),
	}
}
