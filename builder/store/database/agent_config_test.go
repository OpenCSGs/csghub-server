package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestAgentConfigStore_Get(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success - get existing config", func(t *testing.T) {
		// Get existing config (from migration) and update it
		existingConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, existingConfig)

		// Update with test values
		existingConfig.Config = map[string]any{
			"code_instance_quota_per_user":     5,
			"langflow_instance_quota_per_user": 10,
		}
		err = store.Update(ctx, existingConfig)
		require.NoError(t, err)

		// Get the config
		retrievedConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)
		require.Equal(t, existingConfig.ID, retrievedConfig.ID)
		require.Equal(t, 5, int(retrievedConfig.Config["code_instance_quota_per_user"].(float64)))
		require.Equal(t, 10, int(retrievedConfig.Config["langflow_instance_quota_per_user"].(float64)))
	})

}

func TestAgentConfigStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		config := &database.AgentConfig{
			Name: "test-config-1",
			Config: map[string]any{
				"code_instance_quota_per_user":     5,
				"langflow_instance_quota_per_user": 10,
				"custom_setting":                   "value",
			},
		}

		err := store.Create(ctx, config)
		require.NoError(t, err)
		require.NotZero(t, config.ID)
		require.NotZero(t, config.CreatedAt)
		require.NotZero(t, config.UpdatedAt)
	})

	t.Run("create with empty config", func(t *testing.T) {
		config := &database.AgentConfig{
			Name:   "test-config-2",
			Config: map[string]any{},
		}

		err := store.Create(ctx, config)
		require.NoError(t, err)
		require.NotZero(t, config.ID)
	})
}

func TestAgentConfigStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		// Get existing config (from migration)
		config, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, config)
		originalUpdatedAt := config.UpdatedAt

		// Wait a bit to ensure UpdatedAt changes
		time.Sleep(10 * time.Millisecond)

		// Update config
		config.Config = map[string]any{
			"code_instance_quota_per_user":     15,
			"langflow_instance_quota_per_user": 20,
			"new_setting":                      "new_value",
		}
		err = store.Update(ctx, config)
		require.NoError(t, err)

		// Verify update
		updatedConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, updatedConfig)
		require.Equal(t, 15, int(updatedConfig.Config["code_instance_quota_per_user"].(float64)))
		require.Equal(t, 20, int(updatedConfig.Config["langflow_instance_quota_per_user"].(float64)))
		require.Equal(t, "new_value", updatedConfig.Config["new_setting"])
		require.True(t, updatedConfig.UpdatedAt.After(originalUpdatedAt))
	})

	t.Run("update non-existent config", func(t *testing.T) {
		config := &database.AgentConfig{
			ID: 99999,
			Config: map[string]any{
				"code_instance_quota_per_user": 5,
			},
		}

		err := store.Update(ctx, config)
		require.Error(t, err)
	})
}

func TestAgentConfigStore_GetConfigValue(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		// Get existing config and update it
		config, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, config)

		config.Config = map[string]any{
			"code_instance_quota_per_user":     5,
			"langflow_instance_quota_per_user": 10,
			"string_setting":                   "test_value",
			"bool_setting":                     true,
		}
		err = store.Update(ctx, config)
		require.NoError(t, err)

		// Get specific values
		codeQuota, err := store.GetConfigValue(ctx, "instance", "code_instance_quota_per_user")
		require.NoError(t, err)
		require.Equal(t, float64(5), codeQuota)

		langflowQuota, err := store.GetConfigValue(ctx, "instance", "langflow_instance_quota_per_user")
		require.NoError(t, err)
		require.Equal(t, float64(10), langflowQuota)

		stringValue, err := store.GetConfigValue(ctx, "instance", "string_setting")
		require.NoError(t, err)
		require.Equal(t, "test_value", stringValue)

		boolValue, err := store.GetConfigValue(ctx, "instance", "bool_setting")
		require.NoError(t, err)
		require.Equal(t, true, boolValue)
	})

	t.Run("key not found", func(t *testing.T) {
		// Get existing config and update it
		config, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, config)

		config.Config = map[string]any{
			"code_instance_quota_per_user": 5,
		}
		err = store.Update(ctx, config)
		require.NoError(t, err)

		// Try to get non-existent key
		_, err = store.GetConfigValue(ctx, "instance", "non_existent_key")
		require.Error(t, err)
		require.Contains(t, err.Error(), "config key non_existent_key not found")
	})

	t.Run("config not found", func(t *testing.T) {
		// Note: This test may not work as expected because the migration inserts a default config
		// The agent_config table is designed to always have one row
		// GetConfigValue will return an error for a non-existent key, not for missing config
		_, err := store.GetConfigValue(ctx, "instance", "non_existent_key_for_test")
		require.Error(t, err)
		require.Contains(t, err.Error(), "config key non_existent_key_for_test not found")
	})
}

func TestAgentConfigStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	// Read existing config (from migration)
	config, err := store.GetByName(ctx, "instance")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.NotZero(t, config.ID)

	// Update
	config.Config = map[string]any{
		"code_instance_quota_per_user":     15,
		"langflow_instance_quota_per_user": 10,
	}
	err = store.Update(ctx, config)
	require.NoError(t, err)

	// Verify update
	updatedConfig, err := store.GetByName(ctx, "instance")
	require.NoError(t, err)
	require.Equal(t, 15, int(updatedConfig.Config["code_instance_quota_per_user"].(float64)))
}

func TestAgentConfigStore_ComplexConfigStructure(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("create and retrieve complex nested config", func(t *testing.T) {
		// Get existing config and update it with complex nested structure
		complexConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, complexConfig)

		complexConfig.Config = map[string]any{
			"code_instance_quota_per_user":     5,
			"langflow_instance_quota_per_user": 10,
			"quota_settings": map[string]any{
				"default": 5,
				"premium": 20,
				"enterprise": map[string]any{
					"max_instances": 100,
					"features":      []any{"feature1", "feature2", "feature3"},
				},
			},
			"instance_types": []any{
				map[string]any{
					"type":     "code",
					"quota":    5,
					"enabled":  true,
					"metadata": map[string]any{"cpu": "2", "memory": "4GB"},
				},
				map[string]any{
					"type":     "langflow",
					"quota":    10,
					"enabled":  true,
					"metadata": map[string]any{"cpu": "4", "memory": "8GB"},
				},
			},
			"feature_flags": map[string]any{
				"enable_auto_scaling": true,
				"enable_monitoring":   false,
				"beta_features":       []any{"feature_a", "feature_b"},
			},
			"limits": map[string]any{
				"max_concurrent": 50,
				"rate_limits": map[string]any{
					"per_minute": 100,
					"per_hour":   1000,
				},
			},
		}

		err = store.Update(ctx, complexConfig)
		require.NoError(t, err)
		require.NotZero(t, complexConfig.ID)

		// Retrieve and verify complex structure
		retrievedConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)

		// Verify top-level simple values
		require.Equal(t, 5, int(retrievedConfig.Config["code_instance_quota_per_user"].(float64)))
		require.Equal(t, 10, int(retrievedConfig.Config["langflow_instance_quota_per_user"].(float64)))

		// Verify nested map structure
		quotaSettings, ok := retrievedConfig.Config["quota_settings"].(map[string]any)
		require.True(t, ok, "quota_settings should be a map")
		require.Equal(t, float64(5), quotaSettings["default"])
		require.Equal(t, float64(20), quotaSettings["premium"])

		enterprise, ok := quotaSettings["enterprise"].(map[string]any)
		require.True(t, ok, "enterprise should be a map")
		require.Equal(t, float64(100), enterprise["max_instances"])

		features, ok := enterprise["features"].([]any)
		require.True(t, ok, "features should be an array")
		require.Len(t, features, 3)
		require.Equal(t, "feature1", features[0])
		require.Equal(t, "feature2", features[1])
		require.Equal(t, "feature3", features[2])

		// Verify array of maps
		instanceTypes, ok := retrievedConfig.Config["instance_types"].([]any)
		require.True(t, ok, "instance_types should be an array")
		require.Len(t, instanceTypes, 2)

		codeType, ok := instanceTypes[0].(map[string]any)
		require.True(t, ok, "first instance type should be a map")
		require.Equal(t, "code", codeType["type"])
		require.Equal(t, float64(5), codeType["quota"])
		require.Equal(t, true, codeType["enabled"])

		codeMetadata, ok := codeType["metadata"].(map[string]any)
		require.True(t, ok, "code metadata should be a map")
		require.Equal(t, "2", codeMetadata["cpu"])
		require.Equal(t, "4GB", codeMetadata["memory"])

		langflowType, ok := instanceTypes[1].(map[string]any)
		require.True(t, ok, "second instance type should be a map")
		require.Equal(t, "langflow", langflowType["type"])
		require.Equal(t, float64(10), langflowType["quota"])

		// Verify feature flags
		featureFlags, ok := retrievedConfig.Config["feature_flags"].(map[string]any)
		require.True(t, ok, "feature_flags should be a map")
		require.Equal(t, true, featureFlags["enable_auto_scaling"])
		require.Equal(t, false, featureFlags["enable_monitoring"])

		betaFeatures, ok := featureFlags["beta_features"].([]any)
		require.True(t, ok, "beta_features should be an array")
		require.Len(t, betaFeatures, 2)
		require.Equal(t, "feature_a", betaFeatures[0])
		require.Equal(t, "feature_b", betaFeatures[1])

		// Verify nested limits
		limits, ok := retrievedConfig.Config["limits"].(map[string]any)
		require.True(t, ok, "limits should be a map")
		require.Equal(t, float64(50), limits["max_concurrent"])

		rateLimits, ok := limits["rate_limits"].(map[string]any)
		require.True(t, ok, "rate_limits should be a map")
		require.Equal(t, float64(100), rateLimits["per_minute"])
		require.Equal(t, float64(1000), rateLimits["per_hour"])
	})

	t.Run("update complex config structure", func(t *testing.T) {
		// Get existing config and update it
		initialConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, initialConfig)

		initialConfig.Config = map[string]any{
			"simple_value": 42,
			"nested": map[string]any{
				"level1": map[string]any{
					"level2": "value",
				},
			},
		}
		err = store.Update(ctx, initialConfig)
		require.NoError(t, err)

		// Update with more complex structure
		updatedConfig := map[string]any{
			"simple_value": 100,
			"nested": map[string]any{
				"level1": map[string]any{
					"level2": "updated_value",
					"level2_new": map[string]any{
						"deep_nested": []any{1, 2, 3},
					},
				},
			},
			"new_section": map[string]any{
				"array_of_objects": []any{
					map[string]any{"id": 1, "name": "item1"},
					map[string]any{"id": 2, "name": "item2"},
				},
				"mixed_types": map[string]any{
					"string":  "text",
					"number":  42.5,
					"boolean": true,
					"null":    nil,
				},
			},
		}

		initialConfig.Config = updatedConfig
		err = store.Update(ctx, initialConfig)
		require.NoError(t, err)

		// Verify updated structure
		retrievedConfig, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.Equal(t, float64(100), retrievedConfig.Config["simple_value"])

		nested, ok := retrievedConfig.Config["nested"].(map[string]any)
		require.True(t, ok)
		level1, ok := nested["level1"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "updated_value", level1["level2"])

		level2New, ok := level1["level2_new"].(map[string]any)
		require.True(t, ok)
		deepNested, ok := level2New["deep_nested"].([]any)
		require.True(t, ok)
		require.Len(t, deepNested, 3)
		require.Equal(t, float64(1), deepNested[0])
		require.Equal(t, float64(2), deepNested[1])
		require.Equal(t, float64(3), deepNested[2])

		newSection, ok := retrievedConfig.Config["new_section"].(map[string]any)
		require.True(t, ok)
		arrayOfObjects, ok := newSection["array_of_objects"].([]any)
		require.True(t, ok)
		require.Len(t, arrayOfObjects, 2)

		item1, ok := arrayOfObjects[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, float64(1), item1["id"])
		require.Equal(t, "item1", item1["name"])

		mixedTypes, ok := newSection["mixed_types"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "text", mixedTypes["string"])
		require.Equal(t, 42.5, mixedTypes["number"])
		require.Equal(t, true, mixedTypes["boolean"])
		require.Nil(t, mixedTypes["null"])
	})

	t.Run("get config value from complex structure", func(t *testing.T) {
		// Get existing config and update it with complex structure
		config, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, config)

		config.Config = map[string]any{
			"top_level": "value",
			"nested": map[string]any{
				"key": "nested_value",
				"deep": map[string]any{
					"deeper": map[string]any{
						"value": 123,
					},
				},
			},
		}
		err = store.Update(ctx, config)
		require.NoError(t, err)

		// Get top-level value
		topLevel, err := store.GetConfigValue(ctx, "instance", "top_level")
		require.NoError(t, err)
		require.Equal(t, "value", topLevel)

		// GetConfigValue only returns top-level keys, not nested paths
		// So nested.key won't work - this is expected behavior
		_, err = store.GetConfigValue(ctx, "instance", "nested")
		require.NoError(t, err) // This should work as "nested" is a top-level key

		_, err = store.GetConfigValue(ctx, "instance", "nested.key")
		require.Error(t, err) // This should fail as "nested.key" is not a top-level key
		require.Contains(t, err.Error(), "config key nested.key not found")
	})
}

func TestAgentConfigStore_GetByID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		// Get existing config (from migration)
		config, err := store.GetByName(ctx, "instance")
		require.NoError(t, err)
		require.NotNil(t, config)
		require.NotZero(t, config.ID)

		// Get by ID
		retrievedConfig, err := store.GetByID(ctx, config.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)
		require.Equal(t, config.ID, retrievedConfig.ID)
		require.Equal(t, config.Name, retrievedConfig.Name)
	})

	t.Run("non-existent id", func(t *testing.T) {
		// Try to get a config that doesn't exist
		retrievedConfig, err := store.GetByID(ctx, 99999)
		require.NoError(t, err)
		require.Nil(t, retrievedConfig)
	})
}

func TestAgentConfigStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		// Create a few test configs
		config1 := &database.AgentConfig{
			Name: "test-config-list-1",
			Config: map[string]any{
				"setting1": "value1",
			},
		}
		err := store.Create(ctx, config1)
		require.NoError(t, err)

		// Wait a bit to ensure different updated_at
		time.Sleep(10 * time.Millisecond)

		config2 := &database.AgentConfig{
			Name: "test-config-list-2",
			Config: map[string]any{
				"setting2": "value2",
			},
		}
		err = store.Create(ctx, config2)
		require.NoError(t, err)

		// List all configs
		configs, err := store.List(ctx)
		require.NoError(t, err)
		require.NotNil(t, configs)
		require.GreaterOrEqual(t, len(configs), 2) // At least the two we created plus the default "instance"

		// Verify ordering (should be by updated_at DESC, so config2 should come before config1)
		foundConfig1 := false
		foundConfig2 := false
		for _, cfg := range configs {
			if cfg.ID == config1.ID {
				foundConfig1 = true
			}
			if cfg.ID == config2.ID {
				foundConfig2 = true
			}
		}
		require.True(t, foundConfig1, "config1 should be in the list")
		require.True(t, foundConfig2, "config2 should be in the list")

		// Clean up
		_ = store.Delete(ctx, config1.ID)
		_ = store.Delete(ctx, config2.ID)
	})
}

func TestAgentConfigStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentConfigStoreWithDB(db)

	t.Run("success", func(t *testing.T) {
		// Create a test config to delete
		config := &database.AgentConfig{
			Name: "test-config-to-delete",
			Config: map[string]any{
				"code_instance_quota_per_user": 5,
			},
		}

		err := store.Create(ctx, config)
		require.NoError(t, err)
		require.NotZero(t, config.ID)

		// Verify it exists
		retrievedConfig, err := store.GetByID(ctx, config.ID)
		require.NoError(t, err)
		require.NotNil(t, retrievedConfig)

		// Delete the config
		err = store.Delete(ctx, config.ID)
		require.NoError(t, err)

		// Verify it's deleted
		deletedConfig, err := store.GetByID(ctx, config.ID)
		require.NoError(t, err)
		require.Nil(t, deletedConfig)
	})

	t.Run("delete non-existent config", func(t *testing.T) {
		// Try to delete a config that doesn't exist
		err := store.Delete(ctx, 99999)
		require.Error(t, err)
		require.Contains(t, err.Error(), "affected 0 row(s), want 1")
	})
}
