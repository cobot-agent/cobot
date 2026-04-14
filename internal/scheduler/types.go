package scheduler

type Task struct {
	Name       string `yaml:"name" json:"name"`
	Schedule   string `yaml:"schedule" json:"schedule"`
	Prompt     string `yaml:"prompt" json:"prompt"`
	Output     string `yaml:"output" json:"output"`
	OutputPath string `yaml:"output_path,omitempty" json:"output_path,omitempty"`
	Enabled    bool   `yaml:"enabled" json:"enabled"`
}

type TaskResult struct {
	Name      string `yaml:"name" json:"name"`
	Success   bool   `yaml:"success" json:"success"`
	Error     string `yaml:"error,omitempty" json:"error,omitempty"`
	StartedAt string `yaml:"started_at" json:"started_at"`
	FinishedAt string `yaml:"finished_at" json:"finished_at"`
}
