package component

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockmq "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mq"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// testAgentWithMocks represents the test structure for AgentComponent
type testAgentWithMocks struct {
	*agentComponentImpl
	mocks *agentMocks
}

// agentMocks contains all the mocks needed for AgentComponent testing
type agentMocks struct {
	templateStore          *mockdatabase.MockAgentTemplateStore
	instanceStore          *mockdatabase.MockAgentInstanceStore
	sessionStore           *mockdatabase.MockAgentInstanceSessionStore
	sessionHistoryStore    *mockdatabase.MockAgentInstanceSessionHistoryStore
	agentInstanceTaskStore *mockdatabase.MockAgentInstanceTaskStore
	userStore              *mockdatabase.MockUserStore
	agenthubSvcClient      *mockrpc.MockAgentHubSvcClient
	notificationSvcClient  *mockrpc.MockNotificationSvcClient
	queue                  *mockmq.MockMessageQueue
}

// initializeTestAgentComponent creates a test AgentComponent with mocks
func initializeTestAgentComponent(_ context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testAgentWithMocks {
	// Create mocks
	templateStore := mockdatabase.NewMockAgentTemplateStore(t)
	instanceStore := mockdatabase.NewMockAgentInstanceStore(t)
	sessionStore := mockdatabase.NewMockAgentInstanceSessionStore(t)
	sessionHistoryStore := mockdatabase.NewMockAgentInstanceSessionHistoryStore(t)
	agentInstanceTaskStore := mockdatabase.NewMockAgentInstanceTaskStore(t)
	userStore := mockdatabase.NewMockUserStore(t)
	agenthubSvcClient := mockrpc.NewMockAgentHubSvcClient(t)
	notificationSvcClient := mockrpc.NewMockNotificationSvcClient(t)
	queue := mockmq.NewMockMessageQueue(t)

	// Create config for adapter factory
	config := &config.Config{}
	config.Agent.AgentHubServiceHost = "localhost:8080"
	config.Agent.AgentHubServiceToken = "test-token"
	config.Agent.LangflowInstanceQuotaPerUser = 5
	config.Agent.CodeInstanceQuotaPerUser = 5

	// Create adapter factory with mock client
	adapterFactory := createTestAdapterFactory(agenthubSvcClient)

	// Create mocks struct
	mocks := &agentMocks{
		templateStore:          templateStore,
		instanceStore:          instanceStore,
		sessionStore:           sessionStore,
		sessionHistoryStore:    sessionHistoryStore,
		agentInstanceTaskStore: agentInstanceTaskStore,
		userStore:              userStore,
		agenthubSvcClient:      agenthubSvcClient,
		notificationSvcClient:  notificationSvcClient,
		queue:                  queue,
	}

	// Create component implementation
	component := &agentComponentImpl{
		config:                 config,
		templateStore:          templateStore,
		instanceStore:          instanceStore,
		sessionStore:           sessionStore,
		sessionHistoryStore:    sessionHistoryStore,
		agentInstanceTaskStore: agentInstanceTaskStore,
		userStore:              userStore,
		adapterFactory:         adapterFactory,
		queue:                  queue,
		notificationSvcClient:  notificationSvcClient,
	}

	return &testAgentWithMocks{
		agentComponentImpl: component,
		mocks:              mocks,
	}
}

// createTestAdapterFactory creates an adapter factory with mock client for testing
func createTestAdapterFactory(mockClient rpc.AgentHubSvcClient) *AgentInstanceAdapterFactory {
	factory := NewAgentInstanceAdapterFactory()
	config := &config.Config{}
	config.Agent.LangflowInstanceQuotaPerUser = 5
	config.Agent.CodeInstanceQuotaPerUser = 5

	// Register langflow adapter with mock client
	langflowAdapter := NewLangflowAgentInstanceAdapterWithClient(mockClient, config)
	factory.RegisterAdapter("langflow", langflowAdapter)

	// Register code adapter (this doesn't use the RPC client, so we can use the normal factory)
	codeAdapter, err := NewCodeAgentInstanceAdapter(config)
	if err != nil {
		// If code adapter creation fails, we'll just skip it for testing
	} else {
		factory.RegisterAdapter("code", codeAdapter)
	}

	return factory
}

func TestAgentComponent_CreateTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateType := "langflow"
		userUUID := "test-user"
		content := "test template content"
		name := "test template name"
		description := "test template description"

		template := &types.AgentTemplate{
			Type:        &templateType,
			UserUUID:    &userUUID,
			Content:     &content,
			Name:        &name,
			Description: &description,
			Public:      func() *bool { v := false; return &v }(),
			Metadata:    &map[string]any{"tags": []any{}},
		}

		// Setup mock expectations
		expectedDBTemplate := &database.AgentTemplate{
			ID:          1,
			Type:        templateType,
			UserUUID:    userUUID,
			Content:     content,
			Name:        name,
			Description: description,
			Public:      false,
		}
		ac.mocks.templateStore.EXPECT().Create(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			return dbTemplate.Type == templateType &&
				dbTemplate.UserUUID == userUUID &&
				dbTemplate.Content == content &&
				dbTemplate.Name == name &&
				dbTemplate.Description == description &&
				dbTemplate.Public == false
		})).Return(expectedDBTemplate, nil)

		// Execute
		err := ac.CreateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
		require.Equal(t, int64(1), template.ID)
		require.Equal(t, templateType, *template.Type)
		require.Equal(t, userUUID, *template.UserUUID)
		require.Equal(t, content, *template.Content)
		require.Equal(t, name, *template.Name)
		require.Equal(t, description, *template.Description)
		require.Equal(t, false, *template.Public)
	})

	t.Run("database error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateType := "langflow"
		userUUID := "test-user"
		content := "test template content"
		name := "test template name"
		description := "test template description"

		template := &types.AgentTemplate{
			Type:        &templateType,
			UserUUID:    &userUUID,
			Content:     &content,
			Name:        &name,
			Description: &description,
			Public:      func() *bool { v := false; return &v }(),
			Metadata:    &map[string]any{"tags": []any{}},
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().Create(ctx, mock.Anything).Return(nil, errors.New("database error"))

		// Execute
		err := ac.CreateTemplate(ctx, template)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "database error")
	})
}

func TestAgentComponent_GetTemplateByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success - own template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"
		content := "test template content"

		dbTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     content,
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(dbTemplate, nil)

		// Execute
		result, err := ac.GetTemplateByID(ctx, templateID, userUUID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, templateID, result.ID)
		require.Equal(t, templateType, *result.Type)
		require.Equal(t, userUUID, *result.UserUUID)
		require.Equal(t, content, *result.Content)
		require.Equal(t, false, *result.Public)
	})

	t.Run("success - public template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "other-user"
		templateType := "langflow"
		content := "test template content"

		dbTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    "template-owner",
			Name:        "Public Template",
			Description: "Public template description",
			Content:     content,
			Public:      true,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(dbTemplate, nil)

		// Execute
		result, err := ac.GetTemplateByID(ctx, templateID, userUUID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, templateID, result.ID)
		require.Equal(t, templateType, *result.Type)
		require.Equal(t, "template-owner", *result.UserUUID)
		require.Equal(t, content, *result.Content)
		require.Equal(t, true, *result.Public)
	})

	t.Run("forbidden - private template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "other-user"

		dbTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        "langflow",
			UserUUID:    "template-owner",
			Name:        "Private Template",
			Description: "Private template description",
			Content:     "test content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(dbTemplate, nil)

		// Execute
		result, err := ac.GetTemplateByID(ctx, templateID, userUUID)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("invalid template ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		result, err := ac.GetTemplateByID(ctx, 0, "test-user")

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "invalid template ID")
	})

	t.Run("template not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(999)

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(nil, errors.New("not found"))

		// Execute
		result, err := ac.GetTemplateByID(ctx, templateID, "test-user")

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
	})
}

func TestAgentComponent_ListTemplatesByUserUUID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		dbTemplates := []database.AgentTemplate{
			{
				ID:          1,
				Type:        "langflow",
				UserUUID:    userUUID,
				Name:        "template 1",
				Description: "template 1 description",
				Content:     "template 1",
				Public:      false,
			},
			{
				ID:          2,
				Type:        "agno",
				UserUUID:    "other-user",
				Name:        "template 2",
				Description: "template 2 description",
				Content:     "template 2",
				Public:      true,
			},
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().ListByUserUUID(ctx, userUUID, mock.Anything, 10, 1).Return(dbTemplates, 2, nil)

		// Execute
		result, total, err := ac.ListTemplatesByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 10, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, 2, total)
		require.Equal(t, int64(1), result[0].ID)
		require.Equal(t, "langflow", *result[0].Type)
		require.Equal(t, userUUID, *result[0].UserUUID)
		require.Equal(t, "template 1", *result[0].Name)
		require.Equal(t, "template 1 description", *result[0].Description)
		require.Equal(t, false, *result[0].Public)

		require.Equal(t, int64(2), result[1].ID)
		require.Equal(t, "agno", *result[1].Type)
		require.Equal(t, "other-user", *result[1].UserUUID)
		require.Equal(t, "template 2", *result[1].Name)
		require.Equal(t, "template 2 description", *result[1].Description)
		require.Equal(t, true, *result[1].Public)
	})

	t.Run("empty user UUID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		result, total, err := ac.ListTemplatesByUserUUID(ctx, "", types.AgentTemplateFilter{}, 10, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 0, total)
		require.Contains(t, err.Error(), "user uuid cannot be empty")
	})

	t.Run("database error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().ListByUserUUID(ctx, userUUID, mock.Anything, 10, 1).Return(nil, 0, errors.New("database error"))

		// Execute
		result, total, err := ac.ListTemplatesByUserUUID(ctx, userUUID, types.AgentTemplateFilter{}, 10, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 0, total)
	})
}

func TestAgentComponent_UpdateTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"
		content := "updated content"
		name := "updated name"
		description := "updated description"
		template := &types.AgentTemplate{
			ID:          templateID,
			Type:        &templateType,
			UserUUID:    &userUUID,
			Content:     &content,
			Name:        &name,
			Description: &description,
			Public:      func() *bool { v := true; return &v }(),
		}

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        "old-type",
			UserUUID:    userUUID,
			Name:        "Old Template",
			Description: "Old template description",
			Content:     "old content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			return dbTemplate.ID == templateID &&
				// type should remain unchanged if not provided in update request
				dbTemplate.Type == existingTemplate.Type &&
				dbTemplate.UserUUID == userUUID &&
				dbTemplate.Name == name &&
				dbTemplate.Description == description &&
				dbTemplate.Content == content &&
				dbTemplate.Public == true
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})

	t.Run("nil template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		err := ac.UpdateTemplate(ctx, nil)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "template cannot be nil")
	})

	t.Run("invalid template ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateType := "langflow"
		userUUID := "test-user"
		content := "content"

		template := &types.AgentTemplate{
			ID:       0,
			Type:     &templateType,
			UserUUID: &userUUID,
			Content:  &content,
			Public:   func() *bool { v := false; return &v }(),
		}

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid template ID")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"
		content := "content"

		template := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Content:  &content,
			Public:   func() *bool { v := false; return &v }(),
		}

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        "old-type",
			UserUUID:    "other-user",
			Name:        "Other User Template",
			Description: "Other user template description",
			Content:     "old content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("success - add metadata to template with no existing metadata", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
			Metadata:    nil, // No existing metadata
		}

		newMetadata := map[string]any{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
		template := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			if dbTemplate.Metadata == nil {
				return false
			}
			val1, ok1 := dbTemplate.Metadata["key1"]
			val2, ok2 := dbTemplate.Metadata["key2"]
			val3, ok3 := dbTemplate.Metadata["key3"]
			return ok1 && ok2 && ok3 &&
				val1 == "value1" &&
				val2 == 123 &&
				val3 == true
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})

	t.Run("success - update existing metadata", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
			Metadata: map[string]any{
				"key1": "old-value1",
				"key2": 456,
				"key3": false,
			},
		}

		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": 789,
			"key4": "new-key",
		}
		template := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			if dbTemplate.Metadata == nil {
				return false
			}
			// key1 should be updated
			val1, ok1 := dbTemplate.Metadata["key1"]
			// key2 should be updated
			val2, ok2 := dbTemplate.Metadata["key2"]
			// key3 should remain unchanged
			val3, ok3 := dbTemplate.Metadata["key3"]
			// key4 should be added
			val4, ok4 := dbTemplate.Metadata["key4"]
			return ok1 && ok2 && ok3 && ok4 &&
				val1 == "new-value1" &&
				val2 == 789 &&
				val3 == false &&
				val4 == "new-key"
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})

	t.Run("success - delete metadata keys", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
			Metadata: map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		}

		// Set value to nil to delete the key
		newMetadata := map[string]any{
			"key1": nil,
			"key2": nil,
		}
		template := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			if dbTemplate.Metadata == nil {
				return false
			}
			// key1 should be deleted
			_, ok1 := dbTemplate.Metadata["key1"]
			// key2 should be deleted
			_, ok2 := dbTemplate.Metadata["key2"]
			// key3 should remain
			val3, ok3 := dbTemplate.Metadata["key3"]
			return !ok1 && !ok2 && ok3 && val3 == "value3"
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})

	t.Run("success - mixed metadata operations", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
			Metadata: map[string]any{
				"key1": "old-value1",
				"key2": "value2",
			},
		}

		// Mixed operations: update key1, delete key2, add key3
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": nil,
			"key3": "new-key3",
		}
		template := &types.AgentTemplate{
			ID:       templateID,
			Type:     &templateType,
			UserUUID: &userUUID,
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			if dbTemplate.Metadata == nil {
				return false
			}
			// key1 should be updated
			val1, ok1 := dbTemplate.Metadata["key1"]
			// key2 should be deleted
			_, ok2 := dbTemplate.Metadata["key2"]
			// key3 should be added
			val3, ok3 := dbTemplate.Metadata["key3"]
			return ok1 && !ok2 && ok3 &&
				val1 == "new-value1" &&
				val3 == "new-key3"
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})

	t.Run("success - update name, description and metadata together", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		templateType := "langflow"
		newName := "Updated Template Name"
		newDescription := "Updated description"
		newContent := "Updated content"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        templateType,
			UserUUID:    userUUID,
			Name:        "Old Template Name",
			Description: "Old template description",
			Content:     "old content",
			Public:      false,
			Metadata: map[string]any{
				"old-key": "old-value",
			},
		}

		newMetadata := map[string]any{
			"new-key": "new-value",
		}
		template := &types.AgentTemplate{
			ID:          templateID,
			Type:        &templateType,
			UserUUID:    &userUUID,
			Name:        &newName,
			Description: &newDescription,
			Content:     &newContent,
			Metadata:    &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbTemplate *database.AgentTemplate) bool {
			if dbTemplate.Metadata == nil {
				return false
			}
			val, ok := dbTemplate.Metadata["new-key"]
			return dbTemplate.ID == templateID &&
				dbTemplate.Name == newName &&
				dbTemplate.Description == newDescription &&
				dbTemplate.Content == newContent &&
				dbTemplate.UserUUID == userUUID &&
				ok && val == "new-value"
		})).Return(nil)

		// Execute
		err := ac.UpdateTemplate(ctx, template)

		// Verify
		require.NoError(t, err)
	})
}

func TestAgentComponent_DeleteTemplate(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        "langflow",
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)
		ac.mocks.templateStore.EXPECT().Delete(ctx, templateID).Return(nil)

		// Execute
		err := ac.DeleteTemplate(ctx, templateID, userUUID)

		// Verify
		require.NoError(t, err)
	})

	t.Run("invalid template ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		err := ac.DeleteTemplate(ctx, 0, "test-user")

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid template ID")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"

		existingTemplate := &database.AgentTemplate{
			ID:          templateID,
			Type:        "langflow",
			UserUUID:    "other-user",
			Name:        "Other User Template",
			Description: "Other user template description",
			Content:     "content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(existingTemplate, nil)

		// Execute
		err := ac.DeleteTemplate(ctx, templateID, userUUID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})
}

func TestAgentComponent_CreateInstance(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"
		instanceDesc := "test description"

		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &instanceDesc,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Metadata:    map[string]any{"tags": []any{"CSGHub", "q-a"}},
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(0, nil)
		ac.mocks.agenthubSvcClient.EXPECT().CreateAgentInstance(ctx, userUUID, mock.AnythingOfType("*rpc.CreateAgentInstanceRequest")).Return(&rpc.CreateAgentInstanceResponse{
			ID:          "agenthub-instance-123",
			Name:        instanceName,
			Description: instanceDesc,
		}, nil)
		expectedDBInstance := &database.AgentInstance{
			ID:         1,
			TemplateID: templateID,
			UserUUID:   userUUID,
			Type:       instanceType,
			ContentID:  "agenthub-instance-123",
			Public:     false,
			Metadata:   map[string]any{"template_metadata": template.Metadata},
		}
		expectedMetadata := map[string]any{"template_metadata": template.Metadata}
		ac.mocks.instanceStore.EXPECT().Create(ctx, mock.MatchedBy(func(dbInstance *database.AgentInstance) bool {
			// verify core fields
			if !(dbInstance.TemplateID == templateID &&
				dbInstance.UserUUID == userUUID &&
				dbInstance.Type == instanceType &&
				dbInstance.ContentID == "agenthub-instance-123" &&
				dbInstance.Public == false) {
				return false
			}
			// verify metadata propagation from template -> instance
			return reflect.DeepEqual(expectedMetadata, dbInstance.Metadata)
		})).Return(expectedDBInstance, nil)

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.NoError(t, err)
		require.Equal(t, int64(1), instance.ID)
		require.Equal(t, templateID, *instance.TemplateID)
		require.Equal(t, userUUID, *instance.UserUUID)
		require.Equal(t, instanceType, *instance.Type)
		require.Equal(t, "agenthub-instance-123", *instance.ContentID)
		require.Equal(t, false, *instance.Public)
		// verify instance.Metadata contains template metadata
		require.NotNil(t, instance.Metadata)
		require.Contains(t, *instance.Metadata, "template_metadata")
		require.Equal(t, template.Metadata, (*instance.Metadata)["template_metadata"])
		require.Equal(t, true, instance.Editable) // Owner should be able to edit
	})

	t.Run("template not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(999)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"

		description := "test description"
		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      &public,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(nil, errors.New("not found"))

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find agent template")
	})

	t.Run("forbidden - private template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"

		description := "test description"
		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    "other-user",
			Name:        "Other User Template",
			Description: "Other user template description",
			Content:     "template content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("agenthub service error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"

		description := "test description"
		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &description,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(0, nil)
		ac.mocks.agenthubSvcClient.EXPECT().CreateAgentInstance(ctx, userUUID, mock.AnythingOfType("*rpc.CreateAgentInstanceRequest")).Return(nil, errors.New("failed to create agent instance"))

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create agent instance")
	})

	t.Run("success - no template", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"
		instanceDesc := "test description"

		public := false
		instance := &types.AgentInstance{
			TemplateID:  nil, // No template
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &instanceDesc,
			Public:      &public,
		}

		// Setup mock expectations - no template store call since TemplateID is nil
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(0, nil)
		ac.mocks.agenthubSvcClient.EXPECT().CreateAgentInstance(ctx, userUUID, mock.AnythingOfType("*rpc.CreateAgentInstanceRequest")).Return(&rpc.CreateAgentInstanceResponse{
			ID:          "agenthub-instance-123",
			Name:        instanceName,
			Description: instanceDesc,
		}, nil)
		expectedDBInstance := &database.AgentInstance{
			ID:         1,
			TemplateID: 0, // Should be 0 when no template
			UserUUID:   userUUID,
			Type:       instanceType,
			ContentID:  "agenthub-instance-123",
			Public:     false,
		}
		ac.mocks.instanceStore.EXPECT().Create(ctx, mock.MatchedBy(func(dbInstance *database.AgentInstance) bool {
			return dbInstance.TemplateID == 0 && // Should be 0 when no template
				dbInstance.UserUUID == userUUID &&
				dbInstance.Type == instanceType &&
				dbInstance.ContentID == "agenthub-instance-123" &&
				dbInstance.Public == false
		})).Return(expectedDBInstance, nil)

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.NoError(t, err)
		require.Equal(t, int64(1), instance.ID)
		require.Equal(t, int64(0), *instance.TemplateID) // Should be 0 when no template
		require.Equal(t, userUUID, *instance.UserUUID)
		require.Equal(t, instanceType, *instance.Type)
		require.Equal(t, "agenthub-instance-123", *instance.ContentID)
		require.Equal(t, false, *instance.Public)
		require.Equal(t, true, instance.Editable) // Owner should be able to edit
	})

	t.Run("quota exceeded", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"
		instanceDesc := "test description"

		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &instanceDesc,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)
		// Default quota is 5, so return 6 to exceed quota (instanceCount > quota)
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(5, nil)

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AGENT-ERR-0")
	})

	t.Run("quota check error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"
		instanceDesc := "test description"

		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &instanceDesc,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(0, errors.New("database error"))

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get agent instance count")
	})

	t.Run("quota not exceeded - at limit", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		templateID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		instanceName := "test instance"
		instanceDesc := "test description"

		public := false
		instance := &types.AgentInstance{
			TemplateID:  &templateID,
			UserUUID:    &userUUID,
			Type:        &instanceType,
			Name:        &instanceName,
			Description: &instanceDesc,
			Public:      &public,
		}

		template := &database.AgentTemplate{
			ID:          templateID,
			Type:        instanceType,
			UserUUID:    userUUID,
			Name:        "Test Template",
			Description: "Test template description",
			Content:     "template content",
			Metadata:    map[string]any{"tags": []any{"CSGHub", "q-a"}},
			Public:      false,
		}

		// Setup mock expectations
		ac.mocks.templateStore.EXPECT().FindByID(ctx, templateID).Return(template, nil)
		// Default quota is 5, so return 4 to be within quota (instanceCount <= quota)
		ac.mocks.instanceStore.EXPECT().CountByUserAndType(ctx, userUUID, instanceType).Return(4, nil)
		ac.mocks.agenthubSvcClient.EXPECT().CreateAgentInstance(ctx, userUUID, mock.AnythingOfType("*rpc.CreateAgentInstanceRequest")).Return(&rpc.CreateAgentInstanceResponse{
			ID:          "agenthub-instance-123",
			Name:        instanceName,
			Description: instanceDesc,
		}, nil)
		expectedDBInstance := &database.AgentInstance{
			ID:         1,
			TemplateID: templateID,
			UserUUID:   userUUID,
			Type:       instanceType,
			ContentID:  "agenthub-instance-123",
			Public:     false,
			Metadata:   map[string]any{"template_metadata": template.Metadata},
		}
		expectedMetadata := map[string]any{"template_metadata": template.Metadata}
		ac.mocks.instanceStore.EXPECT().Create(ctx, mock.MatchedBy(func(dbInstance *database.AgentInstance) bool {
			// verify core fields
			if !(dbInstance.TemplateID == templateID &&
				dbInstance.UserUUID == userUUID &&
				dbInstance.Type == instanceType &&
				dbInstance.ContentID == "agenthub-instance-123" &&
				dbInstance.Public == false) {
				return false
			}
			// verify metadata propagation from template -> instance
			return reflect.DeepEqual(expectedMetadata, dbInstance.Metadata)
		})).Return(expectedDBInstance, nil)

		// Execute
		err := ac.CreateInstance(ctx, instance)

		// Verify
		require.NoError(t, err)
		require.Equal(t, int64(1), instance.ID)
	})
}

func TestAgentComponent_GetInstanceByID(t *testing.T) {
	ctx := context.Background()

	t.Run("success - own instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   userUUID,
			Type:       instanceType,
			ContentID:  contentID,
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)

		// Execute
		result, err := ac.GetInstanceByID(ctx, instanceID, userUUID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, instanceID, result.ID)
		require.Equal(t, int64(1), *result.TemplateID)
		require.Equal(t, userUUID, *result.UserUUID)
		require.Equal(t, instanceType, *result.Type)
		require.Equal(t, contentID, *result.ContentID)
		require.Equal(t, false, *result.Public)
		require.Equal(t, true, result.Editable) // Owner should be able to edit
	})

	t.Run("success - public instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "other-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   "instance-owner",
			Type:       instanceType,
			ContentID:  contentID,
			Public:     true,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)

		// Execute
		result, err := ac.GetInstanceByID(ctx, instanceID, userUUID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, instanceID, result.ID)
		require.Equal(t, "instance-owner", *result.UserUUID)
		require.Equal(t, true, *result.Public)
		require.Equal(t, false, result.Editable) // Non-owner should not be able to edit
	})

	t.Run("forbidden - private instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "other-user"

		dbInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   "instance-owner",
			Type:       "langflow",
			ContentID:  "content-123",
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)

		// Execute
		result, err := ac.GetInstanceByID(ctx, instanceID, userUUID)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		result, err := ac.GetInstanceByID(ctx, 0, "test-user")

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "invalid instance ID")
	})

	t.Run("success - built-in instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   userUUID,
			Type:       instanceType,
			ContentID:  contentID,
			Public:     false,
			BuiltIn:    true,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)
		// Note: LangflowAgentInstanceAdapter.IsInstanceRunning always returns true, nil
		// So we don't need to mock any RPC calls

		// Execute
		result, err := ac.GetInstanceByID(ctx, instanceID, userUUID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, instanceID, result.ID)
		require.Equal(t, true, result.BuiltIn)
		require.Equal(t, false, result.Editable) // Built-in instances should not be editable
		require.Equal(t, true, result.IsRunning) // Langflow adapter always returns true
	})
}

func TestAgentComponent_ListInstancesByUserUUID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		dbInstances := []database.AgentInstance{
			{
				ID:          1,
				TemplateID:  1,
				UserUUID:    userUUID,
				Type:        "langflow",
				ContentID:   "content-1",
				Public:      false,
				Name:        "instance 1",
				Description: "description 1",
			},
			{
				ID:          2,
				TemplateID:  2,
				UserUUID:    "other-user",
				Type:        "agno",
				ContentID:   "content-2",
				Public:      true,
				Name:        "instance 2",
				Description: "description 2",
			},
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().ListByUserUUID(ctx, userUUID, mock.Anything, 10, 1).Return(dbInstances, 2, nil)

		// Execute
		result, total, err := ac.ListInstancesByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 10, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, 2, total)
		require.Equal(t, int64(1), result[0].ID)
		require.Equal(t, "instance 1", *result[0].Name)
		require.Equal(t, "description 1", *result[0].Description)
		require.Equal(t, true, result[0].Editable) // Own instance should be editable
		require.Equal(t, int64(2), result[1].ID)
		require.Equal(t, "instance 2", *result[1].Name)
		require.Equal(t, "description 2", *result[1].Description)
		require.Equal(t, false, result[1].Editable) // Other user's instance should not be editable
	})

	t.Run("agenthub service error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		dbInstances := []database.AgentInstance{
			{
				ID:         1,
				TemplateID: 1,
				UserUUID:   userUUID,
				Type:       "langflow",
				ContentID:  "content-1",
				Public:     false,
			},
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().ListByUserUUID(ctx, userUUID, mock.Anything, 10, 1).Return(dbInstances, 1, nil)

		// Execute
		result, total, err := ac.ListInstancesByUserUUID(ctx, userUUID, types.AgentInstanceFilter{}, 10, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, 1, total)
	})
}

func TestAgentComponent_UpdateInstance(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"
		templateID := int64(1)
		existingType := "old-type"
		contentID := "content-123"

		public := true
		instance := &types.AgentInstance{
			ID:         instanceID,
			TemplateID: &templateID,
			UserUUID:   &userUUID,
			Type:       &existingType,
			ContentID:  &contentID,
			Public:     &public,
		}

		existingInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: templateID,
			UserUUID:   userUUID,
			Type:       "old-type",
			ContentID:  "old-content",
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(existingInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(dbInstance *database.AgentInstance) bool {
			return dbInstance.ID == instanceID &&
				dbInstance.TemplateID == templateID &&
				dbInstance.UserUUID == userUUID &&
				dbInstance.Type == existingType &&
				dbInstance.ContentID == contentID &&
				dbInstance.Public == true
		})).Return(nil)

		// Execute
		err := ac.UpdateInstance(ctx, instance)

		// Verify
		require.NoError(t, err)
	})

	t.Run("nil instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		err := ac.UpdateInstance(ctx, nil)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "instance cannot be nil")
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "langflow"
		userUUID := "test-user"
		contentID := "content-123"

		public := false
		instance := &types.AgentInstance{
			ID:         0,
			TemplateID: &[]int64{1}[0],
			UserUUID:   &userUUID,
			Type:       &instanceType,
			ContentID:  &contentID,
			Public:     &public,
		}

		// Execute
		err := ac.UpdateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid instance ID")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"
		templateID := int64(1)
		instanceType := "langflow"
		contentID := "content-123"

		public := false
		instance := &types.AgentInstance{
			ID:         instanceID,
			TemplateID: &templateID,
			UserUUID:   &userUUID,
			Type:       &instanceType,
			ContentID:  &contentID,
			Public:     &public,
		}

		existingInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: templateID,
			UserUUID:   "other-user",
			Type:       "old-type",
			ContentID:  "old-content",
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(existingInstance, nil)

		// Execute
		err := ac.UpdateInstance(ctx, instance)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})
}

func TestAgentComponent_DeleteInstance(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"

		existingInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
			ContentID:  "content-123",
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(existingInstance, nil)
		ac.mocks.instanceStore.EXPECT().Delete(ctx, instanceID).Return(nil)

		var wg sync.WaitGroup
		wg.Add(1)

		ac.mocks.agenthubSvcClient.EXPECT().DeleteAgentInstance(mock.Anything, userUUID, existingInstance.ContentID).Return(nil).Run(func(ctx context.Context, userUUID string, contentID string) {
			wg.Done()
		}).Once()

		// Execute
		err := ac.DeleteInstance(ctx, instanceID, userUUID)

		// Verify
		require.NoError(t, err)

		wg.Wait()
	})

	t.Run("invalid instance ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		err := ac.DeleteInstance(ctx, 0, "test-user")

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid instance ID")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"

		existingInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   "other-user",
			Type:       "langflow",
			ContentID:  "content-123",
			Public:     false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(existingInstance, nil)

		// Execute
		err := ac.DeleteInstance(ctx, instanceID, userUUID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("forbidden - built-in instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceID := int64(1)
		userUUID := "test-user"

		existingInstance := &database.AgentInstance{
			ID:         instanceID,
			TemplateID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
			ContentID:  "content-123",
			Public:     false,
			BuiltIn:    true,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(existingInstance, nil)

		// Execute
		err := ac.DeleteInstance(ctx, instanceID, userUUID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot delete built-in instance")
	})
}

func TestAgentComponent_UpdateInstanceByContentID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Instance Name"
		newDescription := "Updated description"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Old Name",
			Description: "Old description",
			Public:      false,
		}

		updateRequest := types.UpdateAgentInstanceRequest{
			Name:        &newName,
			Description: &newDescription,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			return instance.ID == 1 &&
				instance.Name == newName &&
				instance.Description == newDescription &&
				instance.UserUUID == userUUID
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result) // Method returns (nil, nil) on success

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("instance not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Name"

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: &newName,
		}

		// Setup mock expectations - return nil instance
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(nil, nil)

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "agent instance not found")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		otherUserUUID := "other-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Name"

		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  otherUserUUID,
			Type:      instanceType,
			ContentID: contentID,
		}

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: &newName,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("database error on find", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Name"

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: &newName,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(nil, errors.New("database error"))

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "database error")
	})

	t.Run("database error on update", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Name"

		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  userUUID,
			Type:      instanceType,
			ContentID: contentID,
		}

		updateRequest := types.UpdateAgentInstanceRequest{
			Name: &newName,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.Anything).Return(errors.New("update error"))

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "update error")
	})

	t.Run("success - add metadata to instance with no existing metadata", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Test Instance",
			Description: "Test description",
			Public:      false,
			Metadata:    nil, // No existing metadata
		}

		newMetadata := map[string]any{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
		updateRequest := types.UpdateAgentInstanceRequest{
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			if instance.Metadata == nil {
				return false
			}
			val1, ok1 := instance.Metadata["key1"]
			val2, ok2 := instance.Metadata["key2"]
			val3, ok3 := instance.Metadata["key3"]
			return ok1 && ok2 && ok3 &&
				val1 == "value1" &&
				val2 == 123 &&
				val3 == true
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result)

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("success - update existing metadata", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Test Instance",
			Description: "Test description",
			Public:      false,
			Metadata: map[string]any{
				"key1": "old-value1",
				"key2": 456,
				"key3": false,
			},
		}

		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": 789,
			"key4": "new-key",
		}
		updateRequest := types.UpdateAgentInstanceRequest{
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			if instance.Metadata == nil {
				return false
			}
			// key1 should be updated
			val1, ok1 := instance.Metadata["key1"]
			// key2 should be updated
			val2, ok2 := instance.Metadata["key2"]
			// key3 should remain unchanged
			val3, ok3 := instance.Metadata["key3"]
			// key4 should be added
			val4, ok4 := instance.Metadata["key4"]
			return ok1 && ok2 && ok3 && ok4 &&
				val1 == "new-value1" &&
				val2 == 789 &&
				val3 == false &&
				val4 == "new-key"
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result)

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("success - delete metadata keys", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Test Instance",
			Description: "Test description",
			Public:      false,
			Metadata: map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		}

		// Set value to nil to delete the key
		newMetadata := map[string]any{
			"key1": nil,
			"key2": nil,
		}
		updateRequest := types.UpdateAgentInstanceRequest{
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			if instance.Metadata == nil {
				return false
			}
			// key1 should be deleted
			_, ok1 := instance.Metadata["key1"]
			// key2 should be deleted
			_, ok2 := instance.Metadata["key2"]
			// key3 should remain
			val3, ok3 := instance.Metadata["key3"]
			return !ok1 && !ok2 && ok3 && val3 == "value3"
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result)

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("success - mixed metadata operations", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Test Instance",
			Description: "Test description",
			Public:      false,
			Metadata: map[string]any{
				"key1": "old-value1",
				"key2": "value2",
			},
		}

		// Mixed operations: update key1, delete key2, add key3
		newMetadata := map[string]any{
			"key1": "new-value1",
			"key2": nil,
			"key3": "new-key3",
		}
		updateRequest := types.UpdateAgentInstanceRequest{
			Metadata: &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			if instance.Metadata == nil {
				return false
			}
			// key1 should be updated
			val1, ok1 := instance.Metadata["key1"]
			// key2 should be deleted
			_, ok2 := instance.Metadata["key2"]
			// key3 should be added
			val3, ok3 := instance.Metadata["key3"]
			return ok1 && !ok2 && ok3 &&
				val1 == "new-value1" &&
				val3 == "new-key3"
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result)

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("success - update name, description and metadata together", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"
		newName := "Updated Instance Name"
		newDescription := "Updated description"

		dbInstance := &database.AgentInstance{
			ID:          1,
			TemplateID:  1,
			UserUUID:    userUUID,
			Type:        instanceType,
			ContentID:   contentID,
			Name:        "Old Name",
			Description: "Old description",
			Public:      false,
			Metadata: map[string]any{
				"old-key": "old-value",
			},
		}

		newMetadata := map[string]any{
			"new-key": "new-value",
		}
		updateRequest := types.UpdateAgentInstanceRequest{
			Name:        &newName,
			Description: &newDescription,
			Metadata:    &newMetadata,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Update(ctx, mock.MatchedBy(func(instance *database.AgentInstance) bool {
			if instance.Metadata == nil {
				return false
			}
			val, ok := instance.Metadata["new-key"]
			return instance.ID == 1 &&
				instance.Name == newName &&
				instance.Description == newDescription &&
				instance.UserUUID == userUUID &&
				ok && val == "new-value"
		})).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceUpdated
		})).Return(nil).Once()

		// Execute
		result, err := ac.UpdateInstanceByContentID(ctx, userUUID, instanceType, contentID, updateRequest)

		// Verify
		require.NoError(t, err)
		require.Nil(t, result)

		// Wait for async notification to complete
		wg.Wait()
	})
}

func TestAgentComponent_DeleteInstanceByContentID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  userUUID,
			Type:      instanceType,
			ContentID: contentID,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Delete(ctx, int64(1)).Return(nil)
		// Setup WaitGroup for async notification
		var wg sync.WaitGroup
		wg.Add(1)

		// Mock notification call
		ac.mocks.notificationSvcClient.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req *types.MessageRequest) bool {
			defer wg.Done()
			return req.Scenario == types.MessageScenarioAgentInstanceDeleted
		})).Return(nil).Once()

		// Execute
		err := ac.DeleteInstanceByContentID(ctx, userUUID, instanceType, contentID)

		// Verify
		require.NoError(t, err)

		// Wait for async notification to complete
		wg.Wait()
	})

	t.Run("instance not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		// Setup mock expectations - return nil instance
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(nil, nil)

		// Execute
		err := ac.DeleteInstanceByContentID(ctx, userUUID, instanceType, contentID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "agent instance not found")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		otherUserUUID := "other-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  otherUserUUID,
			Type:      instanceType,
			ContentID: contentID,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)

		// Execute
		err := ac.DeleteInstanceByContentID(ctx, userUUID, instanceType, contentID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("database error on find", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(nil, errors.New("database error"))

		// Execute
		err := ac.DeleteInstanceByContentID(ctx, userUUID, instanceType, contentID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "database error")
	})

	t.Run("database error on delete", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceType := "langflow"
		contentID := "content-123"

		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  userUUID,
			Type:      instanceType,
			ContentID: contentID,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, contentID).Return(dbInstance, nil)
		ac.mocks.instanceStore.EXPECT().Delete(ctx, int64(1)).Return(errors.New("delete error"))

		// Execute
		err := ac.DeleteInstanceByContentID(ctx, userUUID, instanceType, contentID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "delete error")
	})
}

func TestAgentComponent_HelperFunctions(t *testing.T) {
	ctx := context.Background()

	t.Run("isNewSession - new session", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		sessionID := "new-session-123"

		// Setup mock expectations - return ErrDatabaseNoRows (session not found)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionID).Return(nil, errorx.ErrDatabaseNoRows)

		// Execute
		isNew, err := ac.isNewSession(ctx, sessionID)

		// Verify
		require.NoError(t, err)
		require.True(t, isNew)
	})

	t.Run("isNewSession - database error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		sessionID := "session-123"

		// Setup mock expectations - return database error (not ErrDatabaseNoRows)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionID).Return(nil, errors.New("database connection error"))

		// Execute
		isNew, err := ac.isNewSession(ctx, sessionID)

		// Verify
		require.Error(t, err)
		require.False(t, isNew)
		require.Contains(t, err.Error(), "database connection error")
	})

	t.Run("isNewSession - existing session", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		sessionID := "existing-session-123"

		existingSession := &database.AgentInstanceSession{
			ID:         456,
			UUID:       sessionID,
			Name:       "Existing Session",
			InstanceID: 123,
			UserUUID:   "test-user",
			Type:       "langflow",
		}

		// Setup mock expectations - return existing session with no error
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionID).Return(existingSession, nil)

		// Execute
		isNew, err := ac.isNewSession(ctx, sessionID)

		// Verify
		require.NoError(t, err)
		require.False(t, isNew)
	})

	t.Run("extractSessionName - short input", func(t *testing.T) {
		input := "Hello"

		// Execute
		result := extractSessionName(input)

		// Verify
		require.Equal(t, "Hello", result)
	})

	t.Run("extractSessionName - long input", func(t *testing.T) {
		input := "This is a very long input that should be truncated to 20 characters"

		// Execute
		result := extractSessionName(input)

		// Verify
		require.Equal(t, "This is a very long ", result)
		require.Len(t, result, 20)
	})

	t.Run("extractSessionName - input with newline", func(t *testing.T) {
		input := "Hello\nThis is a new line"

		// Execute
		result := extractSessionName(input)

		// Verify
		require.Equal(t, "Hello", result)
	})

	t.Run("extractSessionName - empty input", func(t *testing.T) {
		input := ""

		// Execute
		result := extractSessionName(input)

		// Verify
		require.Equal(t, "", result)
	})
}

func TestAgentComponent_CreateSessionHistories(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		content := `{"role": "user", "content": "帮我构建一个智能体", "file": null, "timestamp": "2025-10-20T07:54:50.025Z"}`
		request := true

		req := &types.CreateSessionHistoryRequest{
			SessionUUID: sessionUUID,
			Messages: []types.SessionHistoryMessage{
				{
					Request: request,
					Content: content,
				},
			},
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.MatchedBy(func(msg types.SessionHistoryMessageEnvelope) bool {
			return msg.MessageType == types.SessionHistoryMessageTypeCreate &&
				msg.SessionID == int64(1) &&
				msg.SessionUUID == sessionUUID &&
				msg.Request == request &&
				msg.Content == content &&
				msg.MsgUUID != ""
		})).Return(nil)

		// Execute
		result, err := ac.CreateSessionHistories(ctx, userUUID, instanceID, req)

		// Verify
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.MsgUUIDs, 1)
		require.NotEmpty(t, result.MsgUUIDs[0])
	})

	t.Run("multiple messages", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		content1 := `{"role": "user", "content": "hello"}`
		content2 := `{"role": "assistant", "content": "hi there"}`

		req := &types.CreateSessionHistoryRequest{
			SessionUUID: sessionUUID,
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content1,
				},
				{
					Request: false,
					Content: content2,
				},
			},
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.MatchedBy(func(msg types.SessionHistoryMessageEnvelope) bool {
			return msg.MessageType == types.SessionHistoryMessageTypeCreate &&
				msg.SessionID == int64(1) &&
				msg.SessionUUID == sessionUUID
		})).Return(nil).Times(2)

		// Execute
		result, err := ac.CreateSessionHistories(ctx, userUUID, instanceID, req)

		// Verify
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.MsgUUIDs, 2)
	})

	t.Run("nil request", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		result, err := ac.CreateSessionHistories(ctx, "test-user", 1, nil)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "create session histories request is nil")
	})

	t.Run("session validation error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "non-existent-session"

		req := &types.CreateSessionHistoryRequest{
			SessionUUID: sessionUUID,
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: "test",
				},
			},
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		result, err := ac.CreateSessionHistories(ctx, userUUID, instanceID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("mq publish error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		content := `{"role": "user", "content": "test"}`
		request := true

		req := &types.CreateSessionHistoryRequest{
			SessionUUID: sessionUUID,
			Messages: []types.SessionHistoryMessage{
				{
					Request: request,
					Content: content,
				},
			},
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.Anything).Return(errors.New("mq publish error"))

		// Execute
		result, err := ac.CreateSessionHistories(ctx, userUUID, instanceID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to create session histories")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		wrongInstanceID := int64(999)
		sessionUUID := "session-uuid-123"
		content := `{"role": "user", "content": "test"}`

		req := &types.CreateSessionHistoryRequest{
			SessionUUID: sessionUUID,
			Messages: []types.SessionHistoryMessage{
				{
					Request: true,
					Content: content,
				},
			},
		}

		// Mock existing session with different instanceID
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		result, err := ac.CreateSessionHistories(ctx, userUUID, wrongInstanceID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_CreateSession(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionType := "langflow"
		instanceID := int64(16)
		sessionName := "Test Session"

		req := &types.CreateAgentInstanceSessionRequest{
			Type:       sessionType,
			InstanceID: &instanceID,
			Name:       &sessionName,
		}

		// Mock instance
		dbInstance := &database.AgentInstance{
			ID:        instanceID,
			UserUUID:  userUUID,
			Type:      sessionType,
			ContentID: "content-123",
			Public:    false,
		}

		// Mock session creation
		expectedSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       "generated-session-uuid",
			Name:       sessionName,
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       sessionType,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)
		ac.mocks.sessionStore.EXPECT().Create(ctx, mock.MatchedBy(func(session *database.AgentInstanceSession) bool {
			return session.InstanceID == instanceID &&
				session.UserUUID == userUUID &&
				session.Type == sessionType &&
				session.Name == sessionName
		})).Return(expectedSession, nil)

		// Execute
		sessionUUID, err := ac.CreateSession(ctx, userUUID, req)

		// Verify
		require.NoError(t, err)
		require.NotEmpty(t, sessionUUID)
	})

	// create session by type and content id
	t.Run("success by type and content id", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionType := "langflow"
		contentID := "content-123"
		sessionName := "Test Session"

		// Mock instance
		dbInstance := &database.AgentInstance{
			ID:        1,
			UserUUID:  userUUID,
			Type:      sessionType,
			ContentID: contentID,
			Public:    false,
		}

		// Mock session creation
		expectedSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       "generated-session-uuid",
			Name:       sessionName,
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       sessionType,
		}

		req := &types.CreateAgentInstanceSessionRequest{
			Type:      sessionType,
			ContentID: &contentID,
			Name:      &sessionName,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, sessionType, contentID).Return(dbInstance, nil)
		ac.mocks.sessionStore.EXPECT().Create(ctx, mock.MatchedBy(func(session *database.AgentInstanceSession) bool {
			return session.InstanceID == 1 &&
				session.UserUUID == userUUID &&
				session.Type == sessionType &&
				session.Name == sessionName
		})).Return(expectedSession, nil)

		// Execute
		sessionUUID, err := ac.CreateSession(ctx, userUUID, req)

		// Verify
		require.NoError(t, err)
		require.NotEmpty(t, sessionUUID)
	})

	t.Run("instance not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionType := "langflow"
		instanceID := int64(999)
		sessionName := "Test Session"

		req := &types.CreateAgentInstanceSessionRequest{
			Type:       sessionType,
			InstanceID: &instanceID,
			Name:       &sessionName,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(nil, errors.New("instance not found"))

		// Execute
		sessionUUID, err := ac.CreateSession(ctx, userUUID, req)

		// Verify
		require.Error(t, err)
		require.Empty(t, sessionUUID)
		require.Contains(t, err.Error(), "instance not found")
	})

	t.Run("forbidden - private instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "other-user"
		sessionType := "langflow"
		instanceID := int64(16)
		sessionName := "Test Session"

		req := &types.CreateAgentInstanceSessionRequest{
			Type:       sessionType,
			InstanceID: &instanceID,
			Name:       &sessionName,
		}

		// Mock private instance owned by different user
		dbInstance := &database.AgentInstance{
			ID:        instanceID,
			UserUUID:  "instance-owner",
			Type:      sessionType,
			ContentID: "content-123",
			Public:    false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(dbInstance, nil)

		// Execute
		sessionUUID, err := ac.CreateSession(ctx, userUUID, req)

		// Verify
		require.Error(t, err)
		require.Empty(t, sessionUUID)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})
}

func TestAgentComponent_ListSessions(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		dbSessions := []database.AgentInstanceSession{
			{
				ID:         1,
				UUID:       "session-1",
				Name:       "Session 1",
				InstanceID: 1,
				UserUUID:   userUUID,
				Type:       "langflow",
			},
			{
				ID:         2,
				UUID:       "session-2",
				Name:       "Session 2",
				InstanceID: 2,
				UserUUID:   userUUID,
				Type:       "agno",
			},
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().List(ctx, userUUID, mock.Anything, 10, 1).Return(dbSessions, 2, nil)

		// Execute
		result, total, err := ac.ListSessions(ctx, userUUID, types.AgentInstanceSessionFilter{}, 10, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, 2, total)
		require.Equal(t, int64(1), result[0].ID)
		require.Equal(t, "session-1", result[0].SessionUUID)
		require.Equal(t, "Session 1", result[0].Name)
		require.Equal(t, int64(1), result[0].InstanceID)
		require.Equal(t, userUUID, result[0].UserUUID)
		require.Equal(t, "langflow", result[0].Type)

		require.Equal(t, int64(2), result[1].ID)
		require.Equal(t, "session-2", result[1].SessionUUID)
		require.Equal(t, "Session 2", result[1].Name)
		require.Equal(t, int64(2), result[1].InstanceID)
		require.Equal(t, userUUID, result[1].UserUUID)
		require.Equal(t, "agno", result[1].Type)
	})

	t.Run("success with search filter", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		dbSessions := []database.AgentInstanceSession{
			{
				ID:         1,
				UUID:       "session-1",
				Name:       "Test Session",
				InstanceID: 1,
				UserUUID:   userUUID,
				Type:       "langflow",
			},
		}

		// Setup mock expectations with search filter
		ac.mocks.sessionStore.EXPECT().List(ctx, userUUID, mock.MatchedBy(func(filter types.AgentInstanceSessionFilter) bool {
			return filter.Search == "Test"
		}), 10, 1).Return(dbSessions, 1, nil)

		// Execute with search filter
		filter := types.AgentInstanceSessionFilter{
			Search: "Test",
		}
		result, total, err := ac.ListSessions(ctx, userUUID, filter, 10, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, 1, total)
		require.Equal(t, int64(1), result[0].ID)
		require.Equal(t, "session-1", result[0].SessionUUID)
		require.Equal(t, "Test Session", result[0].Name)
	})

	t.Run("database error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().List(ctx, userUUID, mock.Anything, 10, 1).Return(nil, 0, errors.New("database error"))

		// Execute
		result, total, err := ac.ListSessions(ctx, userUUID, types.AgentInstanceSessionFilter{}, 10, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, 0, total)
		require.Contains(t, err.Error(), "database error")
	})
}

func TestAgentComponent_GetSessionByUUID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"

		dbSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(dbSession, nil)

		// Execute
		result, err := ac.GetSessionByUUID(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.NoError(t, err)
		require.Equal(t, int64(1), result.ID)
		require.Equal(t, sessionUUID, result.SessionUUID)
		require.Equal(t, "Test Session", result.Name)
		require.Equal(t, int64(1), result.InstanceID)
		require.Equal(t, userUUID, result.UserUUID)
		require.Equal(t, "langflow", result.Type)
	})

	t.Run("session not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "non-existent-session"

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		result, err := ac.GetSessionByUUID(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		wrongInstanceID := int64(999)

		dbSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(dbSession, nil)

		// Execute
		result, err := ac.GetSessionByUUID(ctx, userUUID, sessionUUID, wrongInstanceID)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_DeleteSessionByUUID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.sessionStore.EXPECT().Delete(ctx, int64(1)).Return(nil)

		// Execute
		err := ac.DeleteSessionByUUID(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.NoError(t, err)
	})

	t.Run("session not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "non-existent-session"

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		err := ac.DeleteSessionByUUID(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   "other-user",
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		err := ac.DeleteSessionByUUID(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		wrongInstanceID := int64(999)

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		err := ac.DeleteSessionByUUID(ctx, userUUID, sessionUUID, wrongInstanceID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_UpdateSessionByUUID(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Old Session Name",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.sessionStore.EXPECT().Update(ctx, mock.MatchedBy(func(session *database.AgentInstanceSession) bool {
			return session.ID == 1 &&
				session.UUID == sessionUUID &&
				session.Name == newName &&
				session.UserUUID == userUUID
		})).Return(nil)

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, 1, req)

		// Verify
		require.NoError(t, err)
	})

	t.Run("session not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "non-existent-session"
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, nil)

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, 1, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "agent instance session not found")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Old Session Name",
			InstanceID: 1,
			UserUUID:   "other-user",
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, 1, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("database error on find", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("database error"))

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, 1, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "database error")
	})

	t.Run("database error on update", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Old Session Name",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.sessionStore.EXPECT().Update(ctx, mock.Anything).Return(errors.New("update error"))

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, 1, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "update error")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		wrongInstanceID := int64(999)
		newName := "Updated Session Name"

		req := &types.UpdateAgentInstanceSessionRequest{
			Name: newName,
		}

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Old Session Name",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		err := ac.UpdateSessionByUUID(ctx, userUUID, sessionUUID, wrongInstanceID, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_IsInstanceExistsByContentID(t *testing.T) {
	ctx := context.Background()

	t.Run("success - instance exists", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "langflow"
		instanceContentID := "content-123"

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().IsInstanceExistsByContentID(ctx, instanceType, instanceContentID).Return(true, nil)

		// Execute
		exists, err := ac.IsInstanceExistsByContentID(ctx, instanceType, instanceContentID)

		// Verify
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("success - instance does not exist", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "langflow"
		instanceContentID := "non-existent-content"

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().IsInstanceExistsByContentID(ctx, instanceType, instanceContentID).Return(false, nil)

		// Execute
		exists, err := ac.IsInstanceExistsByContentID(ctx, instanceType, instanceContentID)

		// Verify
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("database error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "langflow"
		instanceContentID := "content-123"

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().IsInstanceExistsByContentID(ctx, instanceType, instanceContentID).Return(false, errors.New("database error"))

		// Execute
		exists, err := ac.IsInstanceExistsByContentID(ctx, instanceType, instanceContentID)

		// Verify
		require.Error(t, err)
		require.False(t, exists)
		require.Contains(t, err.Error(), "database error")
	})
}

func TestAgentComponent_UpdateSessionHistoryFeedback(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		msgUUID := "msg-uuid-123"
		feedback := types.AgentSessionHistoryFeedbackLike

		req := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  msgUUID,
			Feedback: feedback,
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.MatchedBy(func(msg types.SessionHistoryMessageEnvelope) bool {
			return msg.MessageType == types.SessionHistoryMessageTypeUpdateFeedback &&
				msg.MsgUUID == msgUUID &&
				msg.SessionID == int64(1) &&
				msg.SessionUUID == sessionUUID &&
				msg.Feedback != nil &&
				*msg.Feedback == feedback
		})).Return(nil)

		// Execute
		err := ac.UpdateSessionHistoryFeedback(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.NoError(t, err)
	})

	t.Run("nil request", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		err := ac.UpdateSessionHistoryFeedback(ctx, "test-user", 1, "session-uuid", nil)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "feedback request is nil")
	})

	t.Run("session validation error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "non-existent-session"
		msgUUID := "msg-uuid-123"

		req := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  msgUUID,
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		err := ac.UpdateSessionHistoryFeedback(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("mq publish error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		msgUUID := "msg-uuid-123"

		req := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  msgUUID,
			Feedback: types.AgentSessionHistoryFeedbackDislike,
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.Anything).Return(errors.New("mq publish error"))

		// Execute
		err := ac.UpdateSessionHistoryFeedback(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to publish session history message")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		wrongInstanceID := int64(999)
		sessionUUID := "session-uuid-123"
		msgUUID := "msg-uuid-123"

		req := &types.FeedbackSessionHistoryRequest{
			MsgUUID:  msgUUID,
			Feedback: types.AgentSessionHistoryFeedbackLike,
		}

		// Mock existing session with different instanceID
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		err := ac.UpdateSessionHistoryFeedback(ctx, userUUID, wrongInstanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_RewriteSessionHistory(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"
		originalMsgUUID := "original-msg-uuid-123"
		content := `{"role": "assistant", "content": "rewritten response"}`

		req := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: originalMsgUUID,
			Content:         content,
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.MatchedBy(func(msg types.SessionHistoryMessageEnvelope) bool {
			return msg.MessageType == types.SessionHistoryMessageTypeRewrite &&
				msg.OriginalMsgUUID == originalMsgUUID &&
				msg.SessionID == int64(1) &&
				msg.SessionUUID == sessionUUID &&
				msg.Request == false &&
				msg.Content == content &&
				msg.MsgUUID != ""
		})).Return(nil)

		// Execute
		result, err := ac.RewriteSessionHistory(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.MsgUUID)
	})

	t.Run("nil request", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		// Execute
		result, err := ac.RewriteSessionHistory(ctx, "test-user", 1, "session-uuid", nil)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "rewrite session history request is nil")
	})

	t.Run("session validation error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "non-existent-session"

		req := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid",
			Content:         "rewritten content",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		result, err := ac.RewriteSessionHistory(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("mq publish error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		instanceID := int64(1)
		sessionUUID := "session-uuid-123"

		req := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid",
			Content:         "rewritten content",
		}

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: instanceID,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.queue.EXPECT().PublishAgentSessionHistoryMsg(mock.Anything).Return(errors.New("mq publish error"))

		// Execute
		result, err := ac.RewriteSessionHistory(ctx, userUUID, instanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to rewrite session history")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		wrongInstanceID := int64(999)
		sessionUUID := "session-uuid-123"

		req := &types.RewriteSessionHistoryRequest{
			OriginalMsgUUID: "original-msg-uuid",
			Content:         "rewritten content",
		}

		// Mock existing session with different instanceID
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		result, err := ac.RewriteSessionHistory(ctx, userUUID, wrongInstanceID, sessionUUID, req)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_ListSessionHistories(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"

		// Mock existing session
		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		dbHistories := []database.AgentInstanceSessionHistory{
			{
				ID:        1,
				SessionID: 1,
				Request:   true,
				Content:   "User request 1",
			},
			{
				ID:        2,
				SessionID: 1,
				Request:   false,
				Content:   "Agent response 1",
			},
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)
		ac.mocks.sessionHistoryStore.EXPECT().ListBySessionID(ctx, int64(1)).Return(dbHistories, nil)

		// Execute
		result, err := ac.ListSessionHistories(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, int64(1), result[0].ID)
		require.Equal(t, int64(1), result[0].SessionID)
		require.True(t, result[0].Request)
		require.Equal(t, "User request 1", result[0].Content)

		require.Equal(t, int64(2), result[1].ID)
		require.Equal(t, int64(1), result[1].SessionID)
		require.False(t, result[1].Request)
		require.Equal(t, "Agent response 1", result[1].Content)
	})

	t.Run("session not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "non-existent-session"

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		result, err := ac.ListSessionHistories(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "session not found")
	})

	t.Run("forbidden - not owner", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   "other-user",
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		result, err := ac.ListSessionHistories(ctx, userUUID, sessionUUID, 1)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "AUTH-ERR-2")
	})

	t.Run("instance ID mismatch", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "test-user"
		sessionUUID := "session-uuid-123"
		wrongInstanceID := int64(999)

		existingSession := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			Name:       "Test Session",
			InstanceID: 1,
			UserUUID:   userUUID,
			Type:       "langflow",
		}

		// Setup mock expectations
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(existingSession, nil)

		// Execute
		result, err := ac.ListSessionHistories(ctx, userUUID, sessionUUID, wrongInstanceID)

		// Verify
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "does not belong to the specified instance")
	})
}

func TestAgentComponent_CreateTaskIfInstanceExists(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`
		taskType := types.AgentTaskTypeFinetuneJob

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     taskType,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		instance := &database.AgentInstance{
			ID:        1,
			Type:      instanceType,
			ContentID: instanceContentID,
			UserUUID:  userUUID,
			Public:    false,
		}

		session := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			InstanceID: instance.ID,
		}

		expectedTask := &database.AgentInstanceTask{
			ID:          1,
			InstanceID:  instance.ID,
			TaskID:      taskID,
			TaskType:    taskType,
			SessionUUID: sessionUUID,
			UserUUID:    userUUID,
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(instance, nil)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(session, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().Create(ctx, mock.MatchedBy(func(task *database.AgentInstanceTask) bool {
			return task.InstanceID == instance.ID &&
				task.TaskID == taskID &&
				task.TaskType == taskType &&
				task.SessionUUID == sessionUUID &&
				task.UserUUID == userUUID
		})).Return(expectedTask, nil)

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.NoError(t, err)
	})

	t.Run("nil request", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, nil)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "agent instance task request is nil")
	})

	t.Run("invalid agent JSON", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		invalidAgentJSON := `{"type":"code","request_id":"` + sessionUUID + `"}` // missing id

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    invalidAgentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse agent")
	})

	t.Run("empty task ID", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		sessionUUID := "session-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   "",
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "task_id cannot be empty")
	})

	t.Run("instance not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(nil, nil)

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify - should return nil when instance not found (skip creating task)
		require.NoError(t, err)
	})

	t.Run("FindByContentID error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(nil, errors.New("database error"))

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find agent instance")
		require.Contains(t, err.Error(), "database error")
	})

	t.Run("Create task error", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		instance := &database.AgentInstance{
			ID:        1,
			Type:      instanceType,
			ContentID: instanceContentID,
			UserUUID:  userUUID,
			Public:    false,
		}

		session := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			InstanceID: instance.ID,
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(instance, nil)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(session, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().Create(ctx, mock.Anything).Return(nil, errors.New("database error"))

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create agent instance task")
		require.Contains(t, err.Error(), "database error")
	})

	t.Run("invalid agent JSON format", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		invalidAgentJSON := `not a json`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    invalidAgentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse agent")
	})

	t.Run("agent type empty", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		agentJSON := `{"type":"","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse agent")
	})

	t.Run("agent id empty", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		agentJSON := `{"type":"code","id":"","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse agent")
	})

	t.Run("user not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "nonexistent"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(database.User{}, errors.New("user not found"))

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find user by username")
		require.Contains(t, err.Error(), username)
	})

	t.Run("session not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		instance := &database.AgentInstance{
			ID:        1,
			Type:      instanceType,
			ContentID: instanceContentID,
			UserUUID:  userUUID,
			Public:    false,
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(instance, nil)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(nil, errors.New("session not found"))

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to find session by uuid")
		require.Contains(t, err.Error(), sessionUUID)
	})

	t.Run("instance ownership check - user does not own and not public", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		otherUserUUID := "other-user-uuid-456"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		instance := &database.AgentInstance{
			ID:        1,
			Type:      instanceType,
			ContentID: instanceContentID,
			UserUUID:  otherUserUUID, // Different user
			Public:    false,         // Not public
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(instance, nil)

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "agent instance does not belong to the user")
	})

	t.Run("success with public instance", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		instanceType := "code"
		instanceContentID := "123"
		taskID := "task-123"
		sessionUUID := "session-uuid-123"
		username := "testuser"
		userUUID := "user-uuid-123"
		otherUserUUID := "other-user-uuid-456"
		agentJSON := `{"type":"code","id":"123","request_id":"` + sessionUUID + `"}`
		taskType := types.AgentTaskTypeInference

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     taskType,
			Username: username,
		}

		user := database.User{
			ID:       1,
			UUID:     userUUID,
			Username: username,
			Email:    "test@example.com",
		}

		instance := &database.AgentInstance{
			ID:        1,
			Type:      instanceType,
			ContentID: instanceContentID,
			UserUUID:  otherUserUUID, // Different user
			Public:    true,          // Public instance
		}

		session := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			InstanceID: instance.ID,
		}

		expectedTask := &database.AgentInstanceTask{
			ID:          1,
			InstanceID:  instance.ID,
			TaskID:      taskID,
			TaskType:    taskType,
			SessionUUID: sessionUUID,
			UserUUID:    userUUID,
		}

		// Setup mock expectations
		ac.mocks.userStore.EXPECT().FindByUsername(ctx, username).Return(user, nil)
		ac.mocks.instanceStore.EXPECT().FindByContentID(ctx, instanceType, instanceContentID).Return(instance, nil)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(session, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().Create(ctx, mock.MatchedBy(func(task *database.AgentInstanceTask) bool {
			return task.InstanceID == instance.ID &&
				task.TaskID == taskID &&
				task.TaskType == taskType &&
				task.SessionUUID == sessionUUID &&
				task.UserUUID == userUUID
		})).Return(expectedTask, nil)

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.NoError(t, err)
	})

	t.Run("missing request_id in agent JSON", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		taskID := "task-123"
		agentJSON := `{"type":"code","id":"123"}` // Missing request_id

		req := &types.AgentInstanceTaskReq{
			TaskID:   taskID,
			Agent:    agentJSON,
			Type:     types.AgentTaskTypeFinetuneJob,
			Username: "testuser",
		}

		// Execute
		err := ac.CreateTaskIfInstanceExists(ctx, req)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to parse agent")
	})
}

func TestAgentComponent_ListTasks(t *testing.T) {
	ctx := context.Background()

	t.Run("success - no filters", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		filter := types.AgentTaskFilter{}
		per := 10
		page := 1

		expectedTasks := []types.AgentTaskListItem{
			{
				ID:          1,
				TaskID:      "task-1",
				TaskName:    "Task 1",
				TaskType:    types.AgentTaskTypeFinetuneJob,
				TaskStatus:  types.AgentTaskStatusInProgress,
				InstanceID:  1,
				SessionUUID: "session-uuid-1",
			},
		}
		expectedTotal := 1

		// Setup mock expectations
		ac.mocks.agentInstanceTaskStore.EXPECT().ListTasks(ctx, userUUID, filter, per, page).Return(expectedTasks, expectedTotal, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.NoError(t, err)
		require.Equal(t, expectedTotal, total)
		require.Equal(t, expectedTasks, tasks)
	})

	t.Run("success - with instance_id filter", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		instanceID := int64(1)
		filter := types.AgentTaskFilter{
			InstanceID: &instanceID,
		}
		per := 10
		page := 1

		instance := &database.AgentInstance{
			ID:       1,
			UserUUID: userUUID,
			Public:   false,
		}

		expectedTasks := []types.AgentTaskListItem{
			{
				ID:          1,
				TaskID:      "task-1",
				TaskName:    "Task 1",
				TaskType:    types.AgentTaskTypeFinetuneJob,
				TaskStatus:  types.AgentTaskStatusInProgress,
				InstanceID:  1,
				SessionUUID: "session-uuid-1",
			},
		}
		expectedTotal := 1

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(instance, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().ListTasks(ctx, userUUID, filter, per, page).Return(expectedTasks, expectedTotal, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.NoError(t, err)
		require.Equal(t, expectedTotal, total)
		require.Equal(t, expectedTasks, tasks)
	})

	t.Run("error - session_uuid without instance_id", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		filter := types.AgentTaskFilter{
			SessionUUID: "session-uuid-1",
		}
		per := 10
		page := 1

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "instance_id is required when session_uuid is provided")
		require.Nil(t, tasks)
		require.Equal(t, 0, total)
	})

	t.Run("error - instance not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		instanceID := int64(999)
		filter := types.AgentTaskFilter{
			InstanceID: &instanceID,
		}
		per := 10
		page := 1

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(nil, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "instance not found")
		require.Nil(t, tasks)
		require.Equal(t, 0, total)
	})

	t.Run("error - instance access forbidden", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		otherUserUUID := "other-user-uuid"
		instanceID := int64(1)
		filter := types.AgentTaskFilter{
			InstanceID: &instanceID,
		}
		per := 10
		page := 1

		instance := &database.AgentInstance{
			ID:       1,
			UserUUID: otherUserUUID,
			Public:   false,
		}

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(instance, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.Error(t, err)
		require.Equal(t, errorx.ErrForbidden, err)
		require.Nil(t, tasks)
		require.Equal(t, 0, total)
	})

	t.Run("success - public instance access", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		otherUserUUID := "other-user-uuid"
		instanceID := int64(1)
		filter := types.AgentTaskFilter{
			InstanceID: &instanceID,
		}
		per := 10
		page := 1

		instance := &database.AgentInstance{
			ID:       1,
			UserUUID: otherUserUUID,
			Public:   true,
		}

		expectedTasks := []types.AgentTaskListItem{
			{
				ID:          1,
				TaskID:      "task-1",
				TaskName:    "Task 1",
				TaskType:    types.AgentTaskTypeFinetuneJob,
				TaskStatus:  types.AgentTaskStatusInProgress,
				InstanceID:  1,
				SessionUUID: "session-uuid-1",
			},
		}
		expectedTotal := 1

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(instance, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().ListTasks(ctx, userUUID, filter, per, page).Return(expectedTasks, expectedTotal, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.NoError(t, err)
		require.Equal(t, expectedTotal, total)
		require.Equal(t, expectedTasks, tasks)
	})

	t.Run("success - with session_uuid filter", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		instanceID := int64(1)
		sessionUUID := "session-uuid-1"
		filter := types.AgentTaskFilter{
			InstanceID:  &instanceID,
			SessionUUID: sessionUUID,
		}
		per := 10
		page := 1

		instance := &database.AgentInstance{
			ID:       1,
			UserUUID: userUUID,
			Public:   false,
		}

		session := &database.AgentInstanceSession{
			ID:         1,
			UUID:       sessionUUID,
			InstanceID: instanceID,
			UserUUID:   userUUID,
		}

		expectedTasks := []types.AgentTaskListItem{
			{
				ID:          1,
				TaskID:      "task-1",
				TaskName:    "Task 1",
				TaskType:    types.AgentTaskTypeFinetuneJob,
				TaskStatus:  types.AgentTaskStatusInProgress,
				InstanceID:  1,
				SessionUUID: sessionUUID,
			},
		}
		expectedTotal := 1

		// Setup mock expectations
		ac.mocks.instanceStore.EXPECT().FindByID(ctx, instanceID).Return(instance, nil)
		ac.mocks.sessionStore.EXPECT().FindByUUID(ctx, sessionUUID).Return(session, nil)
		ac.mocks.agentInstanceTaskStore.EXPECT().ListTasks(ctx, userUUID, filter, per, page).Return(expectedTasks, expectedTotal, nil)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.NoError(t, err)
		require.Equal(t, expectedTotal, total)
		require.Equal(t, expectedTasks, tasks)
	})

	t.Run("error - store list tasks fails", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		filter := types.AgentTaskFilter{}
		per := 10
		page := 1

		storeErr := errors.New("database error")

		// Setup mock expectations
		ac.mocks.agentInstanceTaskStore.EXPECT().ListTasks(ctx, userUUID, filter, per, page).Return(nil, 0, storeErr)

		// Execute
		tasks, total, err := ac.ListTasks(ctx, userUUID, filter, per, page)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to list agent tasks")
		require.Nil(t, tasks)
		require.Equal(t, 0, total)
	})
}

func TestAgentComponent_GetTaskDetail(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		taskID := int64(1)

		expectedDetail := &types.AgentTaskDetail{
			ID:           1,
			TaskID:       "task-1",
			TaskName:     "Test Task",
			TaskDesc:     "Test description",
			TaskType:     types.AgentTaskTypeFinetuneJob,
			Status:       types.AgentTaskStatusInProgress,
			InstanceID:   1,
			InstanceType: "langflow",
			InstanceName: "Test Instance",
			SessionUUID:  "session-uuid-1",
			SessionName:  "Test Session",
			Username:     "testuser",
			Backend:      "argo_workflow",
		}

		// Setup mock expectations
		ac.mocks.agentInstanceTaskStore.EXPECT().GetTaskByID(ctx, userUUID, taskID).Return(expectedDetail, nil)

		// Execute
		detail, err := ac.GetTaskDetail(ctx, userUUID, taskID)

		// Verify
		require.NoError(t, err)
		require.Equal(t, expectedDetail, detail)
	})

	t.Run("error - task not found", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		taskID := int64(999)

		// Setup mock expectations
		ac.mocks.agentInstanceTaskStore.EXPECT().GetTaskByID(ctx, userUUID, taskID).Return(nil, errorx.ErrDatabaseNoRows)

		// Execute
		detail, err := ac.GetTaskDetail(ctx, userUUID, taskID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get task detail")
		require.Nil(t, detail)
	})

	t.Run("error - store get task fails", func(t *testing.T) {
		ac := initializeTestAgentComponent(ctx, t)
		userUUID := "user-uuid-123"
		taskID := int64(1)

		storeErr := errors.New("database error")

		// Setup mock expectations
		ac.mocks.agentInstanceTaskStore.EXPECT().GetTaskByID(ctx, userUUID, taskID).Return(nil, storeErr)

		// Execute
		detail, err := ac.GetTaskDetail(ctx, userUUID, taskID)

		// Verify
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get task detail")
		require.Nil(t, detail)
	})
}
