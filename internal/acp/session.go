package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Session struct {
	ID       string
	CWD      string
	Ctx      context.Context
	CancelFn context.CancelFunc
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*Session),
	}
}

func (s *SessionStore) Put(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = sess
}

func (s *SessionStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[id]
	return sess, ok
}

func (s *SessionStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
}

func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}
