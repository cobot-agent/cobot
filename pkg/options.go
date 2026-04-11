package cobot

type Config struct {
	ConfigPath   string
	Workspace    string
	Model        string
	MaxTurns     int
	SystemPrompt string
	Verbose      bool
	APIKeys      map[string]string
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
