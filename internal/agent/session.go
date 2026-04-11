package agent

import (
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type Session struct {
	mu       sync.RWMutex
	messages []cobot.Message
}

func NewSession() *Session {
	return &Session{}
}

func (s *Session) Messages() []cobot.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]cobot.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

func (s *Session) AddMessage(m cobot.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, m)
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}
