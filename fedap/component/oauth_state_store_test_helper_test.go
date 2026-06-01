//go:build saas

package component

import (
	"context"
	"sync"
	"time"
)

type testOAuthStateStore struct {
	mu       sync.Mutex
	sessions map[string]*authSession
}

func newTestOAuthStateStore() oauthStateStore {
	return &testOAuthStateStore{
		sessions: make(map[string]*authSession),
	}
}

func (s *testOAuthStateStore) Save(_ context.Context, state string, session *authSession, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[state] = session
	return nil
}

func (s *testOAuthStateStore) Load(_ context.Context, state string) (*authSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[state]
	return session, ok, nil
}

func (s *testOAuthStateStore) LoadAndDelete(_ context.Context, state string) (*authSession, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[state]
	if !ok {
		return nil, false, nil
	}
	delete(s.sessions, state)
	return session, true, nil
}
