package subagent

import (
	"sync"
	"time"
)

type Config struct {
	Task        string
	Model       string
	ToolNames   []string
	MaxTurns    int
	Timeout     time.Duration
	ShareMemory bool
}

type Result struct {
	ID        string
	Output    string
	Error     string
	Duration  time.Duration
	ToolCalls int
}

type SubAgent struct {
	ID     string
	config *Config
	result *Result
	done   chan struct{}
	once   sync.Once
}

func (s *SubAgent) Done() <-chan struct{} { return s.done }
func (s *SubAgent) Result() *Result       { return s.result }

func (s *SubAgent) close() {
	s.once.Do(func() { close(s.done) })
}
