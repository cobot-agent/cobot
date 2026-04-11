package scheduler

type Task struct {
	Name       string `yaml:"name" json:"name"`
	Schedule   string `yaml:"schedule" json:"schedule"`
	Prompt     string `yaml:"prompt" json:"prompt"`
	Output     string `yaml:"output" json:"output"`
	OutputPath string `yaml:"output_path,omitempty" json:"output_path,omitempty"`
}
