package agent

import (
	"sync"

	cobot "github.com/cobot-agent/cobot/pkg"
)

const maxMessages = 1000

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
	if len(s.messages) > maxMessages {
		if len(s.messages) > 0 && s.messages[0].Role == cobot.RoleSystem {
			keep := s.messages[len(s.messages)-(maxMessages-1):]
			kept := make([]cobot.Message, 0, maxMessages)
			kept = append(kept, s.messages[0])
			kept = append(kept, keep...)
			s.messages = kept
		} else {
			s.messages = s.messages[len(s.messages)-maxMessages:]
		}
	}
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}
