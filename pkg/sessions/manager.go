package sessions

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
)

type SessionManager struct {
	store Store
}

func NewSessionManager(backend string) (*SessionManager, error) {
	if backend == "" {
		backend = "memory"
	}

	store, err := NewStore(backend)
	if err != nil {
		return nil, err
	}
	return &SessionManager{store: store}, nil
}

func (sm *SessionManager) NewSession(meta Metadata) (*api.Session, error) {
	suffix := fmt.Sprintf("%04d", rand.Intn(10000))
	sessionID := time.Now().Format("20060102") + suffix

	now := time.Now()
	session := &api.Session{
		ID:           sessionID,
		ProviderID:   meta.ProviderID,
		ModelID:      meta.ModelID,
		AgentState:   api.AgentStateIdle,
		CreatedAt:    now,
		LastModified: now,
	}

	if err := sm.store.CreateSession(session); err != nil {
		return nil, err
	}

	return session, nil
}

func (sm *SessionManager) ListSessions() ([]*api.Session, error) {
	return sm.store.ListSessions()
}

func (sm *SessionManager) FindSessionByID(id string) (*api.Session, error) {
	return sm.store.GetSession(id)
}

func (sm *SessionManager) DeleteSession(id string) error {
	return sm.store.DeleteSession(id)
}

func (sm *SessionManager) GetLatestSession() (*api.Session, error) {
	sessions, err := sm.store.ListSessions()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, nil
	}

	latest := sessions[0]
	for _, session := range sessions[1:] {
		if session.LastModified.After(latest.LastModified) {
			latest = session
		}
	}

	return latest, nil
}

func (sm *SessionManager) UpdateLastAccessed(session *api.Session) error {
	session.LastModified = time.Now()
	return sm.store.UpdateSession(session)
}
