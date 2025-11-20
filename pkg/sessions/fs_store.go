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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"sigs.k8s.io/yaml"
)

const (
	metadataFileName = "metadata.yaml"
	historyFileName  = "history.json"
)

type filesystemStore struct {
	basePath string
}

func newFilesystemStore(basePath string) *filesystemStore {
	return &filesystemStore{
		basePath: basePath,
	}
}

func (s *filesystemStore) GetSession(id string) (*api.Session, error) {
	sessionPath := filepath.Join(s.basePath, id)
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session %s not found", id)
	}

	// Load metadata
	metaPath := filepath.Join(sessionPath, metadataFileName)
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	var session api.Session
	if err := yaml.Unmarshal(metaBytes, &session); err != nil {
		return nil, fmt.Errorf("unmarshaling metadata: %w", err)
	}
	// Ensure ID is set (it might not be in the yaml if we just stored metadata fields)
	session.ID = id

	// Initialize ChatMessageStore
	fsChatStore := &fsChatMessageStore{
		historyPath: filepath.Join(sessionPath, historyFileName),
	}
	session.ChatMessageStore = fsChatStore

	// Load messages
	session.Messages = fsChatStore.ChatMessages()

	return &session, nil
}

func (s *filesystemStore) CreateSession(session *api.Session) error {
	sessionPath := filepath.Join(s.basePath, session.ID)
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return err
	}

	// Initialize ChatMessageStore if not set (though for FS store we enforce FS storage)
	fsChatStore := &fsChatMessageStore{
		historyPath: filepath.Join(sessionPath, historyFileName),
	}
	session.ChatMessageStore = fsChatStore

	// Save metadata
	if err := s.UpdateSession(session); err != nil {
		return err
	}

	// Save initial messages if any
	if len(session.Messages) > 0 {
		if err := fsChatStore.SetChatMessages(session.Messages); err != nil {
			return err
		}
	}

	return nil
}

func (s *filesystemStore) UpdateSession(session *api.Session) error {
	sessionPath := filepath.Join(s.basePath, session.ID)
	metaPath := filepath.Join(sessionPath, metadataFileName)

	// We only save the session struct fields as metadata, excluding Messages which are stored separately
	// We create a copy to avoid modifying the passed session
	metaSession := *session
	metaSession.Messages = nil
	metaSession.ChatMessageStore = nil // Don't serialize the interface

	b, err := yaml.Marshal(metaSession)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, b, 0644)
}

func (s *filesystemStore) ListSessions() ([]*api.Session, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		return nil, err
	}

	var sessions []*api.Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// For list, we might want to be lightweight and not load all messages.
		// We'll load metadata but skip loading messages.
		id := entry.Name()
		sessionPath := filepath.Join(s.basePath, id)
		metaPath := filepath.Join(sessionPath, metadataFileName)

		metaBytes, err := os.ReadFile(metaPath)
		if err != nil {
			// Skip corrupted sessions or log warning?
			continue
		}

		var session api.Session
		if err := yaml.Unmarshal(metaBytes, &session); err != nil {
			continue
		}
		session.ID = id
		// We leave Messages nil/empty for ListSessions to be lightweight

		sessions = append(sessions, &session)
	}

	// Sort by LastModified descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified.After(sessions[j].LastModified)
	})

	return sessions, nil
}

func (s *filesystemStore) DeleteSession(id string) error {
	sessionPath := filepath.Join(s.basePath, id)
	return os.RemoveAll(sessionPath)
}

// fsChatMessageStore implements api.ChatMessageStore for filesystem storage
type fsChatMessageStore struct {
	historyPath string
	mu          sync.Mutex
}

func (s *fsChatMessageStore) AddChatMessage(record *api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func (s *fsChatMessageStore) SetChatMessages(newHistory []*api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.historyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, msg := range newHistory {
		b, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func (s *fsChatMessageStore) ChatMessages() []*api.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	var messages []*api.Message

	f, err := os.Open(s.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*api.Message{}
		}
		return nil
	}
	defer f.Close()

	scanner := json.NewDecoder(f)
	for scanner.More() {
		var message api.Message
		if err := scanner.Decode(&message); err != nil {
			continue // skip malformed messages
		}
		messages = append(messages, &message)
	}

	return messages
}

func (s *fsChatMessageStore) ClearChatMessages() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.historyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}
