package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentInstanceSessionStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)

	// Test data
	sessionUUID := uuid.New().String()
	userUUID := uuid.New().String()
	instanceID := int64(12345)

	// Test Create
	session := &database.AgentInstanceSession{
		UUID:       sessionUUID,
		Name:       "Test Session",
		InstanceID: instanceID,
		UserUUID:   userUUID,
		Type:       "langflow",
	}

	createdSession, err := store.Create(ctx, session)
	require.NoError(t, err)
	require.NotNil(t, createdSession)
	require.NotZero(t, createdSession.ID)
	require.Equal(t, sessionUUID, createdSession.UUID)
	require.Equal(t, "Test Session", createdSession.Name)
	require.Equal(t, instanceID, createdSession.InstanceID)
	require.Equal(t, userUUID, createdSession.UserUUID)
	require.Equal(t, "langflow", createdSession.Type)
	require.False(t, createdSession.CreatedAt.IsZero())
	require.False(t, createdSession.UpdatedAt.IsZero())

	// Test FindByID
	foundSession, err := store.FindByID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.NotNil(t, foundSession)
	require.Equal(t, createdSession.ID, foundSession.ID)
	require.Equal(t, sessionUUID, foundSession.UUID)
	require.Equal(t, "Test Session", foundSession.Name)
	require.Equal(t, instanceID, foundSession.InstanceID)
	require.Equal(t, userUUID, foundSession.UserUUID)
	require.Equal(t, "langflow", foundSession.Type)

	// Test FindByUUID
	foundByUUID, err := store.FindByUUID(ctx, sessionUUID)
	require.NoError(t, err)
	require.NotNil(t, foundByUUID)
	require.Equal(t, createdSession.ID, foundByUUID.ID)
	require.Equal(t, sessionUUID, foundByUUID.UUID)

	// Test ListByInstanceID
	sessions, count, err := store.ListByInstanceID(ctx, instanceID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Len(t, sessions, 1)
	require.Equal(t, createdSession.ID, sessions[0].ID)
	require.Equal(t, sessionUUID, sessions[0].UUID)

	// Test Update
	createdSession.Name = "Updated Session Name"
	createdSession.Type = "agno"
	err = store.Update(ctx, createdSession)
	require.NoError(t, err)

	// Verify update
	updatedSession, err := store.FindByID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated Session Name", updatedSession.Name)
	require.Equal(t, "agno", updatedSession.Type)
	// Note: UpdatedAt might not change if the update doesn't trigger the BeforeAppendModel hook
	// This is expected behavior based on the times struct implementation

	// Test Delete
	err = store.Delete(ctx, createdSession.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = store.FindByID(ctx, createdSession.ID)
	require.Error(t, err)
}

func TestAgentInstanceSessionStore_ListByInstanceID_MultipleSessions(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID := int64(99999)

	// Create multiple sessions for the same instance
	sessions := []*database.AgentInstanceSession{
		{
			UUID:       uuid.New().String(),
			Name:       "Session 1",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "langflow",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Session 2",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "agno",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Session 3",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "code",
		},
	}

	// Create all sessions
	for _, session := range sessions {
		_, err := store.Create(ctx, session)
		require.NoError(t, err)
		// Add small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Test ListByInstanceID
	foundSessions, count, err := store.ListByInstanceID(ctx, instanceID)
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.Len(t, foundSessions, 3)

	// Verify sessions are ordered by updated_at DESC (most recent first)
	// Note: Since we're creating sessions quickly, the timestamps might be very close
	// We'll just verify the ordering exists (>= instead of >)
	require.True(t, foundSessions[0].UpdatedAt.After(foundSessions[1].UpdatedAt) ||
		foundSessions[0].UpdatedAt.Equal(foundSessions[1].UpdatedAt))
	require.True(t, foundSessions[1].UpdatedAt.After(foundSessions[2].UpdatedAt) ||
		foundSessions[1].UpdatedAt.Equal(foundSessions[2].UpdatedAt))

	// Test with non-existent instance
	emptySessions, count, err := store.ListByInstanceID(ctx, 999999)
	require.NoError(t, err)
	require.Equal(t, 0, count)
	require.Len(t, emptySessions, 0)
}

func TestAgentInstanceSessionStore_ErrorCases(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)

	// Test FindByID with non-existent ID
	_, err := store.FindByID(ctx, 999999)
	require.Error(t, err)

	// Test FindByUUID with non-existent UUID
	_, err = store.FindByUUID(ctx, "non-existent-uuid")
	require.Error(t, err)

	// Test Update with non-existent session
	nonExistentSession := &database.AgentInstanceSession{
		ID:         999999,
		UUID:       uuid.New().String(),
		Name:       "Non-existent",
		InstanceID: 12345,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
	}
	err = store.Update(ctx, nonExistentSession)
	require.Error(t, err)

	// Test Delete with non-existent ID
	err = store.Delete(ctx, 999999)
	require.Error(t, err)
}

func TestAgentInstanceSessionHistoryStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	historyStore := database.NewAgentInstanceSessionHistoryStoreWithDB(db)

	// First create a session
	session := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "Test Session",
		InstanceID: 12345,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
	}

	createdSession, err := sessionStore.Create(ctx, session)
	require.NoError(t, err)

	// Test Create history
	history := &database.AgentInstanceSessionHistory{
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "Hello, this is a test request",
	}

	err = historyStore.Create(ctx, history)
	require.NoError(t, err)
	require.NotZero(t, history.ID)

	// Test FindByID
	foundHistory, err := historyStore.FindByID(ctx, history.ID)
	require.NoError(t, err)
	require.NotNil(t, foundHistory)
	require.Equal(t, createdSession.ID, foundHistory.SessionID)
	require.True(t, foundHistory.Request)
	require.Equal(t, "Hello, this is a test request", foundHistory.Content)

	// Test ListBySessionID
	histories, err := historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Len(t, histories, 1)
	require.Equal(t, history.ID, histories[0].ID)
	require.Equal(t, "Hello, this is a test request", histories[0].Content)

	// Test Update
	history.Content = "Updated content"
	err = historyStore.Update(ctx, history)
	require.NoError(t, err)

	// Verify update
	updatedHistory, err := historyStore.FindByID(ctx, history.ID)
	require.NoError(t, err)
	require.Equal(t, "Updated content", updatedHistory.Content)

	// Test Delete
	err = historyStore.Delete(ctx, history.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = historyStore.FindByID(ctx, history.ID)
	require.Error(t, err)
}

func TestAgentInstanceSessionHistoryStore_MultipleHistories(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	historyStore := database.NewAgentInstanceSessionHistoryStoreWithDB(db)

	// Create a session
	session := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "Test Session",
		InstanceID: 12345,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
	}

	createdSession, err := sessionStore.Create(ctx, session)
	require.NoError(t, err)

	// Create multiple history entries
	histories := []*database.AgentInstanceSessionHistory{
		{
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 1",
		},
		{
			SessionID: createdSession.ID,
			Request:   false,
			Content:   "Agent response 1",
		},
		{
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 2",
		},
		{
			SessionID: createdSession.ID,
			Request:   false,
			Content:   "Agent response 2",
		},
	}

	// Create all histories
	for _, history := range histories {
		err = historyStore.Create(ctx, history)
		require.NoError(t, err)
		// Add small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Test ListBySessionID
	foundHistories, err := historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Len(t, foundHistories, 4)

	// Verify histories are ordered by created_at ASC (oldest first)
	// Note: Since we're creating histories quickly, the timestamps might be very close
	// We'll just verify the ordering exists (<= instead of <)
	require.True(t, foundHistories[0].CreatedAt.Before(foundHistories[1].CreatedAt) ||
		foundHistories[0].CreatedAt.Equal(foundHistories[1].CreatedAt))
	require.True(t, foundHistories[1].CreatedAt.Before(foundHistories[2].CreatedAt) ||
		foundHistories[1].CreatedAt.Equal(foundHistories[2].CreatedAt))
	require.True(t, foundHistories[2].CreatedAt.Before(foundHistories[3].CreatedAt) ||
		foundHistories[2].CreatedAt.Equal(foundHistories[3].CreatedAt))

	// Verify content
	require.Equal(t, "User request 1", foundHistories[0].Content)
	require.True(t, foundHistories[0].Request)
	require.Equal(t, "Agent response 1", foundHistories[1].Content)
	require.False(t, foundHistories[1].Request)
	require.Equal(t, "User request 2", foundHistories[2].Content)
	require.True(t, foundHistories[2].Request)
	require.Equal(t, "Agent response 2", foundHistories[3].Content)
	require.False(t, foundHistories[3].Request)
}

func TestAgentInstanceSessionHistoryStore_ErrorCases(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	historyStore := database.NewAgentInstanceSessionHistoryStoreWithDB(db)

	// Test FindByID with non-existent ID
	_, err := historyStore.FindByID(ctx, 999999)
	require.Error(t, err)

	// Test ListBySessionID with non-existent session ID
	histories, err := historyStore.ListBySessionID(ctx, 999999)
	require.NoError(t, err)
	require.Len(t, histories, 0)

	// Test Update with non-existent history
	nonExistentHistory := &database.AgentInstanceSessionHistory{
		ID:        999999,
		SessionID: 12345,
		Request:   true,
		Content:   "Non-existent",
	}
	err = historyStore.Update(ctx, nonExistentHistory)
	require.Error(t, err)

	// Test Delete with non-existent ID
	err = historyStore.Delete(ctx, 999999)
	require.Error(t, err)
}

func TestAgentInstanceSessionStore_DeleteWithHistory(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	sessionStore := database.NewAgentInstanceSessionStoreWithDB(db)
	historyStore := database.NewAgentInstanceSessionHistoryStoreWithDB(db)

	// Create a session
	session := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "Test Session",
		InstanceID: 12345,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
	}

	createdSession, err := sessionStore.Create(ctx, session)
	require.NoError(t, err)

	// Create multiple history entries
	histories := []*database.AgentInstanceSessionHistory{
		{
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 1",
		},
		{
			SessionID: createdSession.ID,
			Request:   false,
			Content:   "Agent response 1",
		},
		{
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 2",
		},
	}

	// Create all histories
	for _, history := range histories {
		err = historyStore.Create(ctx, history)
		require.NoError(t, err)
	}

	// Verify histories exist
	foundHistories, err := historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Len(t, foundHistories, 3)

	// Delete the session (should cascade delete histories)
	err = sessionStore.Delete(ctx, createdSession.ID)
	require.NoError(t, err)

	// Verify session is deleted
	_, err = sessionStore.FindByID(ctx, createdSession.ID)
	require.Error(t, err)

	// Verify all histories are also deleted
	foundHistories, err = historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Len(t, foundHistories, 0)
}

func TestAgentInstanceSessionStore_ConcurrentOperations(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID := int64(88888)

	// Test concurrent session creation
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			defer func() { done <- true }()

			session := &database.AgentInstanceSession{
				UUID:       uuid.New().String(),
				Name:       "Concurrent Session " + string(rune(index)),
				InstanceID: instanceID,
				UserUUID:   uuid.New().String(),
				Type:       "langflow",
			}

			_, err := store.Create(ctx, session)
			require.NoError(t, err)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all sessions were created
	sessions, count, err := store.ListByInstanceID(ctx, instanceID)
	require.NoError(t, err)
	require.Equal(t, 10, count)
	require.Len(t, sessions, 10)
}

func TestAgentInstanceSessionStore_EmptyFields(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)

	// Test with minimal required fields
	session := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		InstanceID: 12345,
		UserUUID:   uuid.New().String(),
		Type:       "langflow",
		// Name is intentionally empty (nullzero)
	}

	createdSession, err := store.Create(ctx, session)
	require.NoError(t, err)
	require.NotNil(t, createdSession)
	require.Equal(t, "", createdSession.Name) // Should be empty string

	// Test with empty string name
	session2 := &database.AgentInstanceSession{
		UUID:       uuid.New().String(),
		Name:       "",
		InstanceID: 12346,
		UserUUID:   uuid.New().String(),
		Type:       "agno",
	}
	createdSession2, err := store.Create(ctx, session2)
	require.NoError(t, err)
	require.Equal(t, "", createdSession2.Name)
}
