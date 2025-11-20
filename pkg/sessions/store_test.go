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
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore(t *testing.T) {
	store, err := NewStore("memory")
	require.NoError(t, err)

	testStore(t, store)
}

func TestFilesystemStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "session-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := newFilesystemStore(tempDir)
	testStore(t, store)
}

func testStore(t *testing.T, store Store) {
	// Create a session
	sessionID := uuid.New().String()
	session := &api.Session{
		ID:         sessionID,
		CreatedAt:  time.Now(),
		AgentState: api.AgentStateIdle,
	}

	err := store.CreateSession(session)
	require.NoError(t, err)

	// Verify session exists
	retrievedSession, err := store.GetSession(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, retrievedSession.ID)
	assert.Equal(t, api.AgentStateIdle, retrievedSession.AgentState)

	// Verify ChatMessageStore is initialized
	require.NotNil(t, retrievedSession.ChatMessageStore)

	// Add messages
	msg := &api.Message{
		ID:        uuid.New().String(),
		Source:    api.MessageSourceUser,
		Type:      api.MessageTypeText,
		Payload:   "hello",
		Timestamp: time.Now(),
	}
	err = retrievedSession.ChatMessageStore.AddChatMessage(msg)
	require.NoError(t, err)

	// Verify messages are stored
	messages := retrievedSession.ChatMessageStore.ChatMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "hello", messages[0].Payload)

	// Update session metadata
	retrievedSession.AgentState = api.AgentStateRunning
	retrievedSession.LastModified = time.Now()
	err = store.UpdateSession(retrievedSession)
	require.NoError(t, err)

	// Verify update
	updatedSession, err := store.GetSession(sessionID)
	require.NoError(t, err)
	assert.Equal(t, api.AgentStateRunning, updatedSession.AgentState)

	// Verify messages persist after reload (for FS store especially)
	messages = updatedSession.ChatMessageStore.ChatMessages()
	assert.Len(t, messages, 1)

	// List sessions
	sessions, err := store.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, sessionID, sessions[0].ID)

	// Delete session
	err = store.DeleteSession(sessionID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.GetSession(sessionID)
	assert.Error(t, err)

	sessions, err = store.ListSessions()
	require.NoError(t, err)
	assert.Len(t, sessions, 0)
}
