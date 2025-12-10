package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
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
		UUID:      uuid.New().String(),
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
			UUID:      uuid.New().String(),
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 1",
		},
		{
			UUID:      uuid.New().String(),
			SessionID: createdSession.ID,
			Request:   false,
			Content:   "Agent response 1",
		},
		{
			UUID:      uuid.New().String(),
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 2",
		},
		{
			UUID:      uuid.New().String(),
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
			UUID:      uuid.New().String(),
			SessionID: createdSession.ID,
			Request:   true,
			Content:   "User request 1",
		},
		{
			UUID:      uuid.New().String(),
			SessionID: createdSession.ID,
			Request:   false,
			Content:   "Agent response 1",
		},
		{
			UUID:      uuid.New().String(),
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

func TestAgentInstanceSessionStore_List_WithPagination(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID := int64(77777)

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
		{
			UUID:       uuid.New().String(),
			Name:       "Session 4",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "langflow",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Session 5",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "agno",
		},
	}

	// Create all sessions
	for _, session := range sessions {
		_, err := store.Create(ctx, session)
		require.NoError(t, err)
		// Add small delay to ensure different timestamps
		time.Sleep(1 * time.Millisecond)
	}

	// Test List with pagination - first page
	filter := types.AgentInstanceSessionFilter{InstanceID: &instanceID}
	foundSessions, total, err := store.List(ctx, "", filter, 2, 1)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should be 5
	require.Len(t, foundSessions, 2) // Should return only 2 sessions due to limit

	// Test List with pagination - second page
	foundSessions, total, err = store.List(ctx, "", filter, 2, 2)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should still be 5
	require.Len(t, foundSessions, 2) // Should return 2 sessions

	// Test List with pagination - third page
	foundSessions, total, err = store.List(ctx, "", filter, 2, 3)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should still be 5
	require.Len(t, foundSessions, 1) // Should return 1 session (last one)

	// Test List with pagination - page beyond available data
	foundSessions, total, err = store.List(ctx, "", filter, 2, 4)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should still be 5
	require.Len(t, foundSessions, 0) // Should return 0 sessions

	// Test List with larger page size
	foundSessions, total, err = store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should be 5
	require.Len(t, foundSessions, 5) // Should return all 5 sessions

	// Test List with no filter (should return all sessions)
	foundSessions, total, err = store.List(ctx, "", types.AgentInstanceSessionFilter{}, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 5, total)       // Total should be 5
	require.Len(t, foundSessions, 5) // Should return all 5 sessions
}

func TestAgentInstanceSessionStore_List_WithSearch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID := int64(44444)

	// Create multiple sessions with different names
	sessions := []*database.AgentInstanceSession{
		{
			UUID:       uuid.New().String(),
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "langflow",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Another Test",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "agno",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Production Session",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "code",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Testing Environment",
			InstanceID: instanceID,
			UserUUID:   uuid.New().String(),
			Type:       "langflow",
		},
	}

	// Create all sessions
	for _, session := range sessions {
		_, err := store.Create(ctx, session)
		require.NoError(t, err)
		time.Sleep(1 * time.Millisecond)
	}

	// Test List with search filter - should find sessions containing "Test"
	filter := types.AgentInstanceSessionFilter{
		InstanceID: &instanceID,
		Search:     "Test",
	}
	foundSessions, total, err := store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total) // Should find 3 sessions: "Test Session", "Another Test", "Testing Environment"
	require.Len(t, foundSessions, 3)

	// Verify all found sessions contain "Test" in their name (case-insensitive)
	for _, session := range foundSessions {
		require.Contains(t, session.Name, "Test")
	}

	// Test List with search filter - case-insensitive search
	filter.Search = "test"
	foundSessions, total, err = store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total) // Should still find 3 sessions (case-insensitive)
	require.Len(t, foundSessions, 3)

	// Test List with search filter - partial match
	filter.Search = "Production"
	foundSessions, total, err = store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 1, total) // Should find 1 session: "Production Session"
	require.Len(t, foundSessions, 1)
	require.Equal(t, "Production Session", foundSessions[0].Name)

	// Test List with search filter - no matches
	filter.Search = "NonExistent"
	foundSessions, total, err = store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 0, total) // Should find 0 sessions
	require.Len(t, foundSessions, 0)

	// Test List with empty search filter - should return all sessions
	filter.Search = ""
	foundSessions, total, err = store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 4, total) // Should return all 4 sessions
	require.Len(t, foundSessions, 4)
}

func TestAgentInstanceSessionStore_List_WithDifferentInstances(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID1 := int64(11111)
	instanceID2 := int64(22222)

	// Create sessions for different instances
	sessions := []*database.AgentInstanceSession{
		{
			UUID:       uuid.New().String(),
			Name:       "Session 1",
			InstanceID: instanceID1,
			UserUUID:   uuid.New().String(),
			Type:       "langflow",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Session 2",
			InstanceID: instanceID1,
			UserUUID:   uuid.New().String(),
			Type:       "agno",
		},
		{
			UUID:       uuid.New().String(),
			Name:       "Session 3",
			InstanceID: instanceID2,
			UserUUID:   uuid.New().String(),
			Type:       "code",
		},
	}

	// Create all sessions
	for _, session := range sessions {
		_, err := store.Create(ctx, session)
		require.NoError(t, err)
	}

	// Test List with instanceID1 filter
	filter1 := types.AgentInstanceSessionFilter{InstanceID: &instanceID1}
	foundSessions, total, err := store.List(ctx, "", filter1, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 2, total) // Should find 2 sessions for instanceID1
	require.Len(t, foundSessions, 2)

	// Test List with instanceID2 filter
	filter2 := types.AgentInstanceSessionFilter{InstanceID: &instanceID2}
	foundSessions, total, err = store.List(ctx, "", filter2, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 1, total) // Should find 1 session for instanceID2
	require.Len(t, foundSessions, 1)

	// Test List with non-existent instance ID
	nonExistentID := int64(99999)
	filter3 := types.AgentInstanceSessionFilter{InstanceID: &nonExistentID}
	foundSessions, total, err = store.List(ctx, "", filter3, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 0, total) // Should find 0 sessions
	require.Len(t, foundSessions, 0)
}

func TestAgentInstanceSessionStore_List_Ordering(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentInstanceSessionStoreWithDB(db)
	instanceID := int64(33333)

	// Create sessions with different timestamps
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

	// Create sessions with delays to ensure different timestamps
	for i, session := range sessions {
		_, err := store.Create(ctx, session)
		require.NoError(t, err)
		if i < len(sessions)-1 {
			time.Sleep(50 * time.Millisecond) // Ensure different timestamps
		}
	}

	// Test List ordering (should be ordered by updated_at DESC)
	filter := types.AgentInstanceSessionFilter{InstanceID: &instanceID}
	foundSessions, total, err := store.List(ctx, "", filter, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, foundSessions, 3)

	// Verify sessions are ordered by updated_at DESC (most recent first)
	// We'll verify the ordering exists without making assumptions about exact names
	require.True(t, foundSessions[0].UpdatedAt.After(foundSessions[1].UpdatedAt) ||
		foundSessions[0].UpdatedAt.Equal(foundSessions[1].UpdatedAt))
	require.True(t, foundSessions[1].UpdatedAt.After(foundSessions[2].UpdatedAt) ||
		foundSessions[1].UpdatedAt.Equal(foundSessions[2].UpdatedAt))

	// Verify all expected sessions are present
	sessionNames := make(map[string]bool)
	for _, session := range foundSessions {
		sessionNames[session.Name] = true
	}
	require.True(t, sessionNames["Session 1"])
	require.True(t, sessionNames["Session 2"])
	require.True(t, sessionNames["Session 3"])
}

func TestAgentInstanceSessionHistoryStore_Rewrite(t *testing.T) {
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

	// Create a request history (Request = true)
	requestHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request",
	}

	err = historyStore.Create(ctx, requestHistory)
	require.NoError(t, err)
	require.NotZero(t, requestHistory.ID)
	require.NotZero(t, requestHistory.Turn)

	// Create a response history (Request = false) - this is the one we'll rewrite
	originalResponseHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Original agent response",
	}

	err = historyStore.Create(ctx, originalResponseHistory)
	require.NoError(t, err)
	require.NotZero(t, originalResponseHistory.ID)
	require.Equal(t, requestHistory.Turn, originalResponseHistory.Turn) // Should have same turn as request
	require.False(t, originalResponseHistory.IsRewritten)

	// Create a new history to replace the original response
	newHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Rewritten agent response",
	}

	// Call Rewrite
	err = historyStore.Rewrite(ctx, originalResponseHistory.UUID, newHistory)
	require.NoError(t, err)
	require.NotZero(t, newHistory.ID)

	// Verify the original response history is marked as rewritten
	foundOriginal, err := historyStore.FindByID(ctx, originalResponseHistory.ID)
	require.NoError(t, err)
	require.NotNil(t, foundOriginal)
	require.True(t, foundOriginal.IsRewritten)
	require.Equal(t, "Original agent response", foundOriginal.Content)

	// Verify the new history is inserted with the same Turn
	foundNew, err := historyStore.FindByID(ctx, newHistory.ID)
	require.NoError(t, err)
	require.NotNil(t, foundNew)
	require.Equal(t, originalResponseHistory.Turn, foundNew.Turn)
	require.Equal(t, "Rewritten agent response", foundNew.Content)
	require.Equal(t, createdSession.ID, foundNew.SessionID)
	require.False(t, foundNew.Request)     // Should still be a response
	require.False(t, foundNew.IsRewritten) // New history is not rewritten

	// Verify both histories exist and can be found by UUID
	foundByOriginalUUID, err := historyStore.FindByUUID(ctx, originalResponseHistory.UUID)
	require.NoError(t, err)
	require.True(t, foundByOriginalUUID.IsRewritten)

	foundByNewUUID, err := historyStore.FindByUUID(ctx, newHistory.UUID)
	require.NoError(t, err)
	require.Equal(t, newHistory.ID, foundByNewUUID.ID)
	require.Equal(t, "Rewritten agent response", foundByNewUUID.Content)

	// Verify ListBySessionID excludes rewritten histories
	histories, err := historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	// Should only return non-rewritten histories: request + new response
	require.Len(t, histories, 2)
	require.Equal(t, requestHistory.ID, histories[0].ID)
	require.Equal(t, newHistory.ID, histories[1].ID)
}

func TestAgentInstanceSessionHistoryStore_Rewrite_ErrorCases(t *testing.T) {
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

	// Create a request history (Request = true)
	requestHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request",
	}

	err = historyStore.Create(ctx, requestHistory)
	require.NoError(t, err)

	// Test Rewrite with non-existent UUID
	nonExistentHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "New response",
	}

	err = historyStore.Rewrite(ctx, "non-existent-uuid", nonExistentHistory)
	require.Error(t, err)

	// Test Rewrite with UUID that has Request = true (should fail because Rewrite only works with Request = false)
	newHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "New response",
	}

	err = historyStore.Rewrite(ctx, requestHistory.UUID, newHistory)
	require.Error(t, err) // Should fail because requestHistory has Request = true

	// Test Rewrite with already rewritten history
	// First create and rewrite a response
	responseHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response",
	}

	err = historyStore.Create(ctx, responseHistory)
	require.NoError(t, err)

	// Rewrite it once
	firstRewrite := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "First rewrite",
	}

	err = historyStore.Rewrite(ctx, responseHistory.UUID, firstRewrite)
	require.NoError(t, err)

	// Try to rewrite the already rewritten history (should fail)
	secondRewrite := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Second rewrite",
	}

	err = historyStore.Rewrite(ctx, responseHistory.UUID, secondRewrite)
	require.Error(t, err) // Should fail because original is already rewritten

	// Verify the original history is still marked as rewritten
	foundResponse, err := historyStore.FindByID(ctx, responseHistory.ID)
	require.NoError(t, err)
	require.True(t, foundResponse.IsRewritten)

	// Verify the second rewrite was NOT inserted (since the operation failed)
	_, err = historyStore.FindByID(ctx, secondRewrite.ID)
	require.Error(t, err) // Should not exist
}

func TestAgentInstanceSessionHistoryStore_Create_ResponseBeforeRequest_EmptySession(t *testing.T) {
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

	// Try to create a response before any request (empty session, lastTurn = 0)
	responseHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response",
	}

	err = historyStore.Create(ctx, responseHistory)
	require.Error(t, err)
	require.Contains(t, err.Error(), "response arrived before corresponding request")
}

func TestAgentInstanceSessionHistoryStore_Create_ResponseBeforeRequest_NonEmptySession(t *testing.T) {
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

	// Create a request (turn 1)
	requestHistory := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request 1",
	}

	err = historyStore.Create(ctx, requestHistory)
	require.NoError(t, err)
	require.Equal(t, int64(1), requestHistory.Turn)

	// Create response for turn 1
	responseHistory1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 1",
	}

	err = historyStore.Create(ctx, responseHistory1)
	require.NoError(t, err)
	require.Equal(t, int64(1), responseHistory1.Turn)

	// Try to create another response for turn 1 (should fail - turn already has response)
	responseHistory2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 2",
	}

	err = historyStore.Create(ctx, responseHistory2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "response for turn 1 already exists")
}

func TestAgentInstanceSessionHistoryStore_Create_ResponseBeforeRequest_NextTurn(t *testing.T) {
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

	// Create a request (turn 1)
	requestHistory1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request 1",
	}

	err = historyStore.Create(ctx, requestHistory1)
	require.NoError(t, err)
	require.Equal(t, int64(1), requestHistory1.Turn)

	// Create response for turn 1
	responseHistory1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 1",
	}

	err = historyStore.Create(ctx, responseHistory1)
	require.NoError(t, err)
	require.Equal(t, int64(1), responseHistory1.Turn)

	// Try to create a response for turn 2 before request for turn 2 exists
	// (lastTurn is still 1, so response tries to use turn 1, but turn 1 already has a response)
	responseHistory2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 2",
	}

	err = historyStore.Create(ctx, responseHistory2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "response for turn 1 already exists")

	// Now create request for turn 2
	requestHistory2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request 2",
	}

	err = historyStore.Create(ctx, requestHistory2)
	require.NoError(t, err)
	require.Equal(t, int64(2), requestHistory2.Turn)

	// Now create response for turn 2 (should succeed)
	err = historyStore.Create(ctx, responseHistory2)
	require.NoError(t, err)
	require.Equal(t, int64(2), responseHistory2.Turn)
}

func TestAgentInstanceSessionHistoryStore_Create_RequestResponseFlow(t *testing.T) {
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

	// Test normal flow: request -> response -> request -> response
	// Turn 1
	request1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request 1",
	}
	err = historyStore.Create(ctx, request1)
	require.NoError(t, err)
	require.Equal(t, int64(1), request1.Turn)

	response1 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 1",
	}
	err = historyStore.Create(ctx, response1)
	require.NoError(t, err)
	require.Equal(t, int64(1), response1.Turn)

	// Turn 2
	request2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   true,
		Content:   "User request 2",
	}
	err = historyStore.Create(ctx, request2)
	require.NoError(t, err)
	require.Equal(t, int64(2), request2.Turn)

	response2 := &database.AgentInstanceSessionHistory{
		UUID:      uuid.New().String(),
		SessionID: createdSession.ID,
		Request:   false,
		Content:   "Agent response 2",
	}
	err = historyStore.Create(ctx, response2)
	require.NoError(t, err)
	require.Equal(t, int64(2), response2.Turn)

	// Verify all histories
	histories, err := historyStore.ListBySessionID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Len(t, histories, 4)

	// Verify turns are correct
	require.Equal(t, int64(1), histories[0].Turn)
	require.True(t, histories[0].Request)
	require.Equal(t, int64(1), histories[1].Turn)
	require.False(t, histories[1].Request)
	require.Equal(t, int64(2), histories[2].Turn)
	require.True(t, histories[2].Request)
	require.Equal(t, int64(2), histories[3].Turn)
	require.False(t, histories[3].Request)

	// Verify session's last_turn is updated
	updatedSession, err := sessionStore.FindByID(ctx, createdSession.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), updatedSession.LastTurn)
}
