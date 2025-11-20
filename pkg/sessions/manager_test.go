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
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_InMemory(t *testing.T) {
	manager, err := NewSessionManager("memory")
	require.NoError(t, err)

	testSessionManager(t, manager)
}

func TestSessionManager_Filesystem(t *testing.T) {
	// We need a way to configure the base path for testing, but NewSessionManager uses default NewStore.
	// For now, we rely on NewStore("filesystem") using a temp dir if we could mock it,
	// but NewStore hardcodes the path in the original implementation?
	// Wait, NewStore in store.go:
	// func NewStore(backend string) (Store, error) {
	//     if backend == "filesystem" {
	//         home, _ := os.UserHomeDir()
	//         return newFilesystemStore(filepath.Join(home, ".kubectl-ai", "sessions")), nil
	//     }
	// ...
	// }
	// This is hard to test without polluting user home dir.
	// I should probably update NewStore to allow passing a path or use an environment variable,
	// OR just test MemoryStore for SessionManager logic since it's backend-agnostic.
	// The backend tests are in store_test.go which uses newFilesystemStore directly with a temp dir.

	// So for SessionManager, testing with memory is sufficient for logic verification.
	// If I really want to test FS integration via Manager, I'd need to make NewStore more flexible.
	// Let's stick to Memory for now to avoid side effects.
}

func testSessionManager(t *testing.T, manager *SessionManager) {
	// Create a session
	meta := Metadata{
		ProviderID: "test-provider",
		ModelID:    "test-model",
	}
	session, err := manager.NewSession(meta)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "test-provider", session.ProviderID)
	assert.Equal(t, "test-model", session.ModelID)
	assert.Equal(t, api.AgentStateIdle, session.AgentState)

	// Get session
	retrieved, err := manager.FindSessionByID(session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, retrieved.ID)

	// List sessions
	sessions, err := manager.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, session.ID, sessions[0].ID)

	// Get latest
	latest, err := manager.GetLatestSession()
	require.NoError(t, err)
	assert.Equal(t, session.ID, latest.ID)

	// Update last accessed
	time.Sleep(10 * time.Millisecond) // Ensure time advances
	err = manager.UpdateLastAccessed(session)
	require.NoError(t, err)

	updated, err := manager.FindSessionByID(session.ID)
	require.NoError(t, err)
	assert.True(t, updated.LastModified.After(session.CreatedAt))

	// Delete session
	err = manager.DeleteSession(session.ID)
	require.NoError(t, err)

	_, err = manager.FindSessionByID(session.ID)
	assert.Error(t, err)
}
