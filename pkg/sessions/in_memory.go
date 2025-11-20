// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sessions

import (
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
)

type memoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*api.Session
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions: make(map[string]*api.Session),
	}
}

func (s *memoryStore) GetSession(id string) (*api.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return session, nil
}

func (s *memoryStore) CreateSession(session *api.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[session.ID]; ok {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	// Initialize the ChatMessageStore if not already set
	if session.ChatMessageStore == nil {
		session.ChatMessageStore = &memoryChatMessageStore{
			messages: session.Messages,
		}
	}

	s.sessions[session.ID] = session
	return nil
}

func (s *memoryStore) UpdateSession(session *api.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[session.ID]; !ok {
		return fmt.Errorf("session %s not found", session.ID)
	}
	s.sessions[session.ID] = session
	return nil
}

func (s *memoryStore) ListSessions() ([]*api.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*api.Session
	for _, session := range s.sessions {
		// Return shallow copy to avoid race conditions on the slice itself if modified elsewhere
		// But typically ListSessions is for metadata.
		// For performance, we might want to strip messages here if they are large,
		// but for memory store it's just a pointer copy.
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *memoryStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return nil
}

// memoryChatMessageStore implements api.ChatMessageStore for in-memory storage
type memoryChatMessageStore struct {
	mu       sync.RWMutex
	messages []*api.Message
}

func (s *memoryChatMessageStore) AddChatMessage(record *api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, record)
	return nil
}

func (s *memoryChatMessageStore) SetChatMessages(newHistory []*api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = newHistory
	return nil
}

func (s *memoryChatMessageStore) ChatMessages() []*api.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to prevent race conditions on the slice.
	messageCopy := make([]*api.Message, len(s.messages))
	copy(messageCopy, s.messages)
	return messageCopy
}

func (s *memoryChatMessageStore) ClearChatMessages() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]*api.Message, 0)
	return nil
}
