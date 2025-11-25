package sessions

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"sigs.k8s.io/yaml"
)

type filesystemStore struct {
	basePath string
}

func newFilesystemStore(basePath string) Store {
	return &filesystemStore{basePath: basePath}
}

func (f *filesystemStore) GetSession(id string) (*api.Session, error) {
	sessionPath := filepath.Join(f.basePath, id)
	metadataPath := filepath.Join(sessionPath, "metadata.yaml")

	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	var meta Metadata
	if err := yaml.Unmarshal(metadataBytes, &meta); err != nil {
		return nil, err
	}

	chatStore := NewFileChatMessageStore(sessionPath)
	return &api.Session{
		ID:               id,
		ProviderID:       meta.ProviderID,
		ModelID:          meta.ModelID,
		AgentState:       api.AgentStateIdle,
		CreatedAt:        meta.CreatedAt,
		LastModified:     meta.LastAccessed,
		ChatMessageStore: chatStore,
	}, nil
}

func (f *filesystemStore) CreateSession(session *api.Session) error {
	sessionPath := filepath.Join(f.basePath, session.ID)
	if err := os.MkdirAll(sessionPath, 0o755); err != nil {
		return err
	}

	chatStore := NewFileChatMessageStore(sessionPath)
	session.ChatMessageStore = chatStore

	meta := Metadata{
		ProviderID:   session.ProviderID,
		ModelID:      session.ModelID,
		CreatedAt:    session.CreatedAt,
		LastAccessed: session.LastModified,
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(sessionPath, "metadata.yaml"), data, 0o644)
}

func (f *filesystemStore) UpdateSession(session *api.Session) error {
	sessionPath := filepath.Join(f.basePath, session.ID)
	metadataPath := filepath.Join(sessionPath, "metadata.yaml")

	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("session not found")
		}
		return err
	}

	var meta Metadata
	if err := yaml.Unmarshal(metadataBytes, &meta); err != nil {
		return err
	}

	meta.ProviderID = session.ProviderID
	meta.ModelID = session.ModelID
	meta.LastAccessed = session.LastModified

	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0o644)
}

func (f *filesystemStore) ListSessions() ([]*api.Session, error) {
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []*api.Session{}, nil
		}
		return nil, err
	}

	sessions := make([]*api.Session, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		session, err := f.GetSession(entry.Name())
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastModified.After(sessions[j].LastModified)
	})

	return sessions, nil
}

func (f *filesystemStore) DeleteSession(id string) error {
	sessionPath := filepath.Join(f.basePath, id)
	return os.RemoveAll(sessionPath)
}

// FileChatMessageStore implements api.ChatMessageStore by persisting history to disk.
type FileChatMessageStore struct {
	Path string
	mu   sync.Mutex
}

// NewFileChatMessageStore creates a new file-backed chat message store.
func NewFileChatMessageStore(path string) *FileChatMessageStore {
	return &FileChatMessageStore{Path: path}
}

// HistoryPath returns the location of the history file for this session.
func (s *FileChatMessageStore) HistoryPath() string {
	return filepath.Join(s.Path, "history.json")
}

// AddChatMessage appends a message to the existing history on disk.
func (s *FileChatMessageStore) AddChatMessage(record *api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages, err := s.readMessages()
	if err != nil {
		return err
	}

	messages = append(messages, record)
	return s.writeMessages(messages)
}

// SetChatMessages replaces the history file with the provided messages.
func (s *FileChatMessageStore) SetChatMessages(newHistory []*api.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeMessages(newHistory)
}

// ChatMessages returns all persisted chat messages.
func (s *FileChatMessageStore) ChatMessages() []*api.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	messages, err := s.readMessages()
	if err != nil {
		return []*api.Message{}
	}
	return messages
}

// ClearChatMessages truncates the history file, leaving an empty array.
func (s *FileChatMessageStore) ClearChatMessages() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeMessages([]*api.Message{})
}

func (s *FileChatMessageStore) readMessages() ([]*api.Message, error) {
	path := s.HistoryPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []*api.Message{}, nil
	}
	if err != nil {
		return nil, err
	}

	var messages []*api.Message
	if len(data) == 0 {
		return []*api.Message{}, nil
	}

	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (s *FileChatMessageStore) writeMessages(messages []*api.Message) error {
	if err := os.MkdirAll(s.Path, 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return os.WriteFile(s.HistoryPath(), data, 0o644)
}
