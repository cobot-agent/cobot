package acp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const SessionTTL = 30 * time.Minute

type Session struct {
	ID         string
	CWD        string
	Workspace  string
	Agent      string
	Ctx        context.Context
	CancelFn   context.CancelFunc
	LastActive time.Time
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
	if sess.LastActive.IsZero() {
		sess.LastActive = time.Now()
	}
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

func (s *SessionStore) RemoveExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, sess := range s.sessions {
		if now.Sub(sess.LastActive) > SessionTTL {
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func (s *SessionStore) RemoveAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions = make(map[string]*Session)
}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return "sess_" + hex.EncodeToString(b)
}
