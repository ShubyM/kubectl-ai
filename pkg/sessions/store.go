package sessions

import (
	"fmt"
	"os"
	"path/filepath"

	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
)

const sessionsDirName = "sessions"

type Metadata struct {
	ProviderID   string    `json:"providerID"`
	ModelID      string    `json:"modelID"`
	CreatedAt    time.Time `json:"createdAt"`
	LastAccessed time.Time `json:"lastAccessed"`
}

var defaultMemoryStore Store = newMemoryStore()

type Store interface {
	GetSession(id string) (*api.Session, error)
	CreateSession(session *api.Session) error
	UpdateSession(session *api.Session) error
	ListSessions() ([]*api.Session, error)
	DeleteSession(id string) error
}

func NewStore(backend string) (Store, error) {
	switch backend {
	case "memory":
		return defaultMemoryStore, nil
	case "filesystem":
		basePath, err := defaultFilesystemBasePath()
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(basePath, 0o755); err != nil {
			return nil, err
		}
		return newFilesystemStore(basePath), nil
	default:
		return nil, fmt.Errorf("unsupported sessions backend: %s", backend)
	}
}

func defaultFilesystemBasePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kubectl-ai", sessionsDirName), nil
}
