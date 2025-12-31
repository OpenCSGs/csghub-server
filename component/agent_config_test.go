//go:build ee || saas

package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// testAgentConfigWithMocks represents the test structure for AgentConfigComponent
type testAgentConfigWithMocks struct {
	AgentConfigComponent
	mocks *agentConfigMocks
}

// agentConfigMocks contains all the mocks needed for AgentConfigComponent testing
type agentConfigMocks struct {
	agentConfigStore *mockdatabase.MockAgentConfigStore
}

// initializeTestAgentConfigComponent creates a test AgentConfigComponent with mocks
func initializeTestAgentConfigComponent(_ context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testAgentConfigWithMocks {
	// Create mocks
	agentConfigStore := mockdatabase.NewMockAgentConfigStore(t)

	// Create mocks struct
	mocks := &agentConfigMocks{
		agentConfigStore: agentConfigStore,
	}

	// Create component implementation
	component := &agentConfigComponentImpl{
		agentConfigStore: agentConfigStore,
	}

	return &testAgentConfigWithMocks{
		AgentConfigComponent: component,
		mocks:                mocks,
	}
}

func TestAgentConfigComponent_GetByName(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		dbConfig := &database.AgentConfig{
			ID:   1,
			Name: "instance",
			Config: map[string]any{
				"code_instance_quota_per_user":     5,
				"langflow_instance_quota_per_user": 10,
			},
		}
		// Set CreatedAt and UpdatedAt directly (times is embedded)
		dbConfig.CreatedAt = now
		dbConfig.UpdatedAt = now

		ac.mocks.agentConfigStore.EXPECT().GetByName(ctx, "instance").Return(dbConfig, nil)

		result, err := ac.GetByName(ctx, "instance")

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, int64(1), result.ID)
		require.Equal(t, "instance", result.Name)
		// Handle both float64 (from JSON) and int types
		codeQuota := result.Config["code_instance_quota_per_user"]
		if quotaFloat, ok := codeQuota.(float64); ok {
			require.Equal(t, 5, int(quotaFloat))
		} else if quotaInt, ok := codeQuota.(int); ok {
			require.Equal(t, 5, quotaInt)
		} else {
			t.Fatalf("unexpected type for code_instance_quota_per_user: %T", codeQuota)
		}
		langflowQuota := result.Config["langflow_instance_quota_per_user"]
		if quotaFloat, ok := langflowQuota.(float64); ok {
			require.Equal(t, 10, int(quotaFloat))
		} else if quotaInt, ok := langflowQuota.(int); ok {
			require.Equal(t, 10, quotaInt)
		} else {
			t.Fatalf("unexpected type for langflow_instance_quota_per_user: %T", langflowQuota)
		}
		require.Equal(t, dbConfig.CreatedAt, result.CreatedAt)
		require.Equal(t, dbConfig.UpdatedAt, result.UpdatedAt)
	})

	t.Run("database error", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		ac.mocks.agentConfigStore.EXPECT().GetByName(ctx, "instance").Return(nil, errors.New("database error"))

		result, err := ac.GetByName(ctx, "instance")

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to get agent config by name")
	})

	t.Run("config not found", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		ac.mocks.agentConfigStore.EXPECT().GetByName(ctx, "instance").Return(nil, nil)

		result, err := ac.GetByName(ctx, "instance")

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "agent config not found")
	})
}

func TestAgentConfigComponent_Update(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		existingConfig := &database.AgentConfig{
			ID:   1,
			Name: "instance",
			Config: map[string]any{
				"code_instance_quota_per_user":     5,
				"langflow_instance_quota_per_user": 10,
			},
		}
		existingConfig.CreatedAt = now
		existingConfig.UpdatedAt = now

		newConfig := map[string]any{
			"code_instance_quota_per_user":     15,
			"langflow_instance_quota_per_user": 20,
			"new_setting":                      "new_value",
		}

		req := &types.UpdateAgentConfigReq{
			Config: &newConfig,
		}

		// Mock GetByID to return existing config
		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(existingConfig, nil)

		// Mock Update - the config should be replaced entirely
		ac.mocks.agentConfigStore.EXPECT().Update(ctx, mock.MatchedBy(func(c *database.AgentConfig) bool {
			return c.ID == existingConfig.ID &&
				c.Config["code_instance_quota_per_user"] == 15 &&
				c.Config["langflow_instance_quota_per_user"] == 20 &&
				c.Config["new_setting"] == "new_value" &&
				len(c.Config) == 3 // Should have exactly 3 keys (old keys replaced)
		})).Return(nil)

		result, err := ac.Update(ctx, 1, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, int64(1), result.ID)
		// Handle both float64 (from JSON) and int types
		codeQuota := result.Config["code_instance_quota_per_user"]
		if quotaFloat, ok := codeQuota.(float64); ok {
			require.Equal(t, 15, int(quotaFloat))
		} else if quotaInt, ok := codeQuota.(int); ok {
			require.Equal(t, 15, quotaInt)
		} else {
			t.Fatalf("unexpected type for code_instance_quota_per_user: %T", codeQuota)
		}
		langflowQuota := result.Config["langflow_instance_quota_per_user"]
		if quotaFloat, ok := langflowQuota.(float64); ok {
			require.Equal(t, 20, int(quotaFloat))
		} else if quotaInt, ok := langflowQuota.(int); ok {
			require.Equal(t, 20, quotaInt)
		} else {
			t.Fatalf("unexpected type for langflow_instance_quota_per_user: %T", langflowQuota)
		}
		require.Equal(t, "new_value", result.Config["new_setting"])
	})

	t.Run("get error", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		config := map[string]any{
			"code_instance_quota_per_user": 15,
		}
		req := &types.UpdateAgentConfigReq{
			Config: &config,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(nil, errors.New("database error"))

		result, err := ac.Update(ctx, 1, req)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to get agent config")
	})

	t.Run("config not found", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		config := map[string]any{
			"code_instance_quota_per_user": 15,
		}
		req := &types.UpdateAgentConfigReq{
			Config: &config,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(nil, nil)

		result, err := ac.Update(ctx, 1, req)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "agent config not found")
	})

	t.Run("update error", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		existingConfig := &database.AgentConfig{
			ID:   1,
			Name: "instance",
			Config: map[string]any{
				"code_instance_quota_per_user": 5,
			},
		}
		existingConfig.CreatedAt = now
		existingConfig.UpdatedAt = now

		config := map[string]any{
			"code_instance_quota_per_user": 15,
		}
		req := &types.UpdateAgentConfigReq{
			Config: &config,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(existingConfig, nil)
		ac.mocks.agentConfigStore.EXPECT().Update(ctx, mock.Anything).Return(errors.New("update error"))

		result, err := ac.Update(ctx, 1, req)

		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "failed to update agent config")
	})

	t.Run("replace entire config", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		existingConfig := &database.AgentConfig{
			ID:   1,
			Name: "old-name",
			Config: map[string]any{
				"code_instance_quota_per_user":     5,
				"langflow_instance_quota_per_user": 10,
				"old_setting":                      "old_value",
			},
		}
		existingConfig.CreatedAt = now
		existingConfig.UpdatedAt = now

		// New config only has one key - old keys should be removed
		newConfig := map[string]any{
			"code_instance_quota_per_user": 15,
		}

		req := &types.UpdateAgentConfigReq{
			Config: &newConfig,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(existingConfig, nil)
		ac.mocks.agentConfigStore.EXPECT().Update(ctx, mock.MatchedBy(func(c *database.AgentConfig) bool {
			// Verify that old keys are removed and only new key exists, name unchanged
			_, hasOldKey := c.Config["langflow_instance_quota_per_user"]
			_, hasOldSetting := c.Config["old_setting"]
			_, hasNewKey := c.Config["code_instance_quota_per_user"]
			return !hasOldKey && !hasOldSetting && hasNewKey && len(c.Config) == 1 && c.Name == "old-name"
		})).Return(nil)

		result, err := ac.Update(ctx, 1, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		// Handle both float64 (from JSON) and int types
		codeQuota := result.Config["code_instance_quota_per_user"]
		if quotaFloat, ok := codeQuota.(float64); ok {
			require.Equal(t, 15, int(quotaFloat))
		} else if quotaInt, ok := codeQuota.(int); ok {
			require.Equal(t, 15, quotaInt)
		} else {
			t.Fatalf("unexpected type for code_instance_quota_per_user: %T", codeQuota)
		}
		_, exists := result.Config["langflow_instance_quota_per_user"]
		require.False(t, exists, "old key should be removed")
	})

	t.Run("update name only", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		existingConfig := &database.AgentConfig{
			ID:   1,
			Name: "old-name",
			Config: map[string]any{
				"code_instance_quota_per_user": 5,
			},
		}
		existingConfig.CreatedAt = now
		existingConfig.UpdatedAt = now

		name := "new-name"
		req := &types.UpdateAgentConfigReq{
			Name: &name,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(existingConfig, nil)
		ac.mocks.agentConfigStore.EXPECT().Update(ctx, mock.MatchedBy(func(c *database.AgentConfig) bool {
			return c.Name == "new-name" && c.Config["code_instance_quota_per_user"] == 5
		})).Return(nil)

		result, err := ac.Update(ctx, 1, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "new-name", result.Name)
		codeQuota := result.Config["code_instance_quota_per_user"]
		if quotaFloat, ok := codeQuota.(float64); ok {
			require.Equal(t, 5, int(quotaFloat))
		} else if quotaInt, ok := codeQuota.(int); ok {
			require.Equal(t, 5, quotaInt)
		} else {
			t.Fatalf("unexpected type for code_instance_quota_per_user: %T", codeQuota)
		}
	})

	t.Run("update name and config", func(t *testing.T) {
		ac := initializeTestAgentConfigComponent(ctx, t)

		now := time.Now()
		existingConfig := &database.AgentConfig{
			ID:   1,
			Name: "old-name",
			Config: map[string]any{
				"code_instance_quota_per_user": 5,
			},
		}
		existingConfig.CreatedAt = now
		existingConfig.UpdatedAt = now

		name := "new-name"
		newConfig := map[string]any{
			"code_instance_quota_per_user": 10,
		}
		req := &types.UpdateAgentConfigReq{
			Name:   &name,
			Config: &newConfig,
		}

		ac.mocks.agentConfigStore.EXPECT().GetByID(ctx, int64(1)).Return(existingConfig, nil)
		ac.mocks.agentConfigStore.EXPECT().Update(ctx, mock.MatchedBy(func(c *database.AgentConfig) bool {
			return c.Name == "new-name" && c.Config["code_instance_quota_per_user"] == 10
		})).Return(nil)

		result, err := ac.Update(ctx, 1, req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "new-name", result.Name)
		codeQuota := result.Config["code_instance_quota_per_user"]
		if quotaFloat, ok := codeQuota.(float64); ok {
			require.Equal(t, 10, int(quotaFloat))
		} else if quotaInt, ok := codeQuota.(int); ok {
			require.Equal(t, 10, quotaInt)
		} else {
			t.Fatalf("unexpected type for code_instance_quota_per_user: %T", codeQuota)
		}
	})
}
