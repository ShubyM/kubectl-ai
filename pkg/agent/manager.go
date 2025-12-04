package agent

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"k8s.io/klog/v2"
)

// Factory is a function that creates a new Agent instance.
type Factory func() *Agent

// SessionManager manages active agent sessions.
type SessionManager struct {
	factory        Factory
	store          *sessions.SessionManager
	agents         map[string]*Agent
	mu             sync.RWMutex
	onAgentCreated func(*Agent)
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(factory Factory, store *sessions.SessionManager) *SessionManager {
	return &SessionManager{
		factory: factory,
		store:   store,
		agents:  make(map[string]*Agent),
	}
}

// SetAgentCreatedCallback sets the callback to be called when a new agent is created.
// It also calls the callback immediately for all currently active agents.
func (sm *SessionManager) SetAgentCreatedCallback(cb func(*Agent)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onAgentCreated = cb
	for _, agent := range sm.agents {
		cb(agent)
	}
}

// CreateSession creates a new session and an associated agent.
func (sm *SessionManager) CreateSession(ctx context.Context) (*Agent, error) {
	meta := sessions.Metadata{
		// We might want to pass these in.
		ProviderID: "gemini",         // Default
		ModelID:    "gemini-2.5-pro", // Default
	}

	// If we can, we should get these from the factory's closure variables?
	// But we can't access them.
	// Maybe we should update CreateSession to take Metadata?
	// For now, let's just use what we have.

	session, err := sm.store.NewSession(meta)
	if err != nil {
		return nil, fmt.Errorf("creating new session: %w", err)
	}

	agent := sm.factory()
	agent.Session = session
	// Ensure SessionBackend is set if not already (though factory should set it)

	if err := agent.Init(ctx); err != nil {
		return nil, fmt.Errorf("initializing agent: %w", err)
	}

	sm.mu.Lock()
	sm.agents[session.ID] = agent
	if sm.onAgentCreated != nil {
		sm.onAgentCreated(agent)
	}
	sm.mu.Unlock()

	return agent, nil
}

// GetAgent returns the agent for the given session ID, loading it if necessary.
func (sm *SessionManager) GetAgent(ctx context.Context, sessionID string) (*Agent, error) {
	sm.mu.RLock()
	agent, ok := sm.agents[sessionID]
	sm.mu.RUnlock()

	if ok {
		return agent, nil
	}

	// Load session from store
	session, err := sm.store.FindSessionByID(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Create new agent
	agent = sm.factory()
	agent.Session = session

	if err := agent.Init(ctx); err != nil {
		return nil, fmt.Errorf("initializing agent: %w", err)
	}

	sm.mu.Lock()
	sm.agents[sessionID] = agent
	if sm.onAgentCreated != nil {
		sm.onAgentCreated(agent)
	}
	sm.mu.Unlock()

	return agent, nil
}

// Close closes all active agents.
func (sm *SessionManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, agent := range sm.agents {
		klog.Infof("Closing agent for session %s", id)
		if err := agent.Close(); err != nil {
			klog.Errorf("Error closing agent %s: %v", id, err)
		}
	}
	// Clear the map
	sm.agents = make(map[string]*Agent)
	return nil
}

// ListSessions delegates to the underlying store.
func (sm *SessionManager) ListSessions() ([]*api.Session, error) {
	return sm.store.ListSessions()
}

// FindSessionByID delegates to the underlying store.
func (sm *SessionManager) FindSessionByID(id string) (*api.Session, error) {
	return sm.store.FindSessionByID(id)
}

// DeleteSession delegates to the underlying store and closes the active agent if any.
func (sm *SessionManager) DeleteSession(id string) error {
	sm.mu.Lock()
	if agent, ok := sm.agents[id]; ok {
		agent.Close()
		delete(sm.agents, id)
	}
	sm.mu.Unlock()
	return sm.store.DeleteSession(id)
}

// UpdateLastAccessed delegates to the underlying store.
func (sm *SessionManager) UpdateLastAccessed(session *api.Session) error {
	return sm.store.UpdateLastAccessed(session)
}
