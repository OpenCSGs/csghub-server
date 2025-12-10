package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestAgentMCPServerStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()
	server := &database.AgentMCPServer{
		UserUUID:    userUUID,
		Name:        "Test MCP Server",
		Description: "Test description",
		Protocol:    "sse",
		URL:         "http://localhost:8080",
		Headers: map[string]any{
			"Authorization": "Bearer token123",
		},
		Env: map[string]any{
			"API_KEY": "secret123",
		},
	}

	createdServer, err := store.Create(ctx, server)
	require.NoError(t, err)
	require.NotZero(t, createdServer.ID)
	require.Equal(t, userUUID, createdServer.UserUUID)
	require.Equal(t, "Test MCP Server", createdServer.Name)
	require.Equal(t, "Test description", createdServer.Description)
	require.Equal(t, "sse", createdServer.Protocol)
	require.Equal(t, "http://localhost:8080", createdServer.URL)
	require.NotNil(t, createdServer.Headers)
	require.Equal(t, "Bearer token123", createdServer.Headers["Authorization"])
	require.NotNil(t, createdServer.Env)
	require.Equal(t, "secret123", createdServer.Env["API_KEY"])
}

func TestAgentMCPServerStore_FindByID_UserServer(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()
	server := &database.AgentMCPServer{
		UserUUID:    userUUID,
		Name:        "Test User Server",
		Description: "User server description",
		Protocol:    "streamable",
		URL:         "http://example.com",
		Headers: map[string]any{
			"X-Custom-Header": "value",
		},
		Env: map[string]any{
			"ENV_VAR": "env_value",
		},
	}

	createdServer, err := store.Create(ctx, server)
	require.NoError(t, err)

	// Test FindByID with user: prefix
	id := fmt.Sprintf("user:%d", createdServer.ID)
	detail, err := store.FindByID(ctx, userUUID, id)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, id, detail.ID)
	require.Equal(t, "Test User Server", detail.Name)
	require.Equal(t, "User server description", detail.Description)
	require.Equal(t, "streamable", detail.Protocol)
	require.Equal(t, "http://example.com", detail.URL)
	require.False(t, detail.BuiltIn)
	require.Equal(t, userUUID, detail.UserUUID)
	require.NotNil(t, detail.Headers)
	require.Equal(t, "value", detail.Headers["X-Custom-Header"])
	require.NotNil(t, detail.Env)
	require.Equal(t, "env_value", detail.Env["ENV_VAR"])
}

func TestAgentMCPServerStore_FindByID_BuiltInServer(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Built-in Server', 'Built-in description', 'System', '', 'sse', 'http://builtin.com', '{"X-Header": "value"}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Built-in Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()
	id := fmt.Sprintf("builtin:%d", resourceID)
	detail, err := store.FindByID(ctx, userUUID, id)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, id, detail.ID)
	require.Equal(t, "Built-in Server", detail.Name)
	require.Equal(t, "Built-in description", detail.Description)
	require.Equal(t, "sse", detail.Protocol)
	require.Equal(t, "http://builtin.com", detail.URL)
	require.True(t, detail.BuiltIn)
	require.NotNil(t, detail.Headers)
	require.Equal(t, "value", detail.Headers["X-Header"])
}

func TestAgentMCPServerStore_FindByID_BuiltInServer_WithOverride(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)
	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Built-in With Override', 'Description', 'System', '', 'sse', 'http://builtin.com', '{"Default-Header": "default"}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Built-in With Override").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()

	// Create override config
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Override-Header": "override",
			"Default-Header":  "overridden",
		},
		Env: map[string]any{
			"OVERRIDE_ENV": "override_value",
		},
	}
	_, err = configStore.Create(ctx, config)
	require.NoError(t, err)

	id := fmt.Sprintf("builtin:%d", resourceID)
	detail, err := store.FindByID(ctx, userUUID, id)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, id, detail.ID)
	require.True(t, detail.BuiltIn)

	// Headers should be merged (override takes precedence)
	require.NotNil(t, detail.Headers)
	require.Equal(t, "overridden", detail.Headers["Default-Header"])
	require.Equal(t, "override", detail.Headers["Override-Header"])

	// Env should be overridden
	require.NotNil(t, detail.Env)
	require.Equal(t, "override_value", detail.Env["OVERRIDE_ENV"])
}

func TestAgentMCPServerStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()
	server := &database.AgentMCPServer{
		UserUUID:    userUUID,
		Name:        "Original Name",
		Description: "Original description",
		Protocol:    "sse",
		URL:         "http://original.com",
		Headers: map[string]any{
			"Original-Header": "original",
		},
		Env: map[string]any{
			"ORIGINAL_ENV": "original_value",
		},
	}

	createdServer, err := store.Create(ctx, server)
	require.NoError(t, err)

	// Update the server
	createdServer.Name = "Updated Name"
	createdServer.Description = "Updated description"
	createdServer.Protocol = "streamable"
	createdServer.URL = "http://updated.com"
	createdServer.Headers = map[string]any{
		"Updated-Header": "updated",
	}
	createdServer.Env = map[string]any{
		"UPDATED_ENV": "updated_value",
	}

	err = store.Update(ctx, createdServer)
	require.NoError(t, err)

	// Verify update
	id := fmt.Sprintf("user:%d", createdServer.ID)
	updatedDetail, err := store.FindByID(ctx, userUUID, id)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", updatedDetail.Name)
	require.Equal(t, "Updated description", updatedDetail.Description)
	require.Equal(t, "streamable", updatedDetail.Protocol)
	require.Equal(t, "http://updated.com", updatedDetail.URL)
	require.Equal(t, "updated", updatedDetail.Headers["Updated-Header"])
	require.Equal(t, "updated_value", updatedDetail.Env["UPDATED_ENV"])
}

func TestAgentMCPServerStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()
	server := &database.AgentMCPServer{
		UserUUID: userUUID,
		Name:     "Server To Delete",
		Protocol: "sse",
		URL:      "http://delete.com",
	}

	createdServer, err := store.Create(ctx, server)
	require.NoError(t, err)

	// Delete the server
	err = store.Delete(ctx, createdServer.ID)
	require.NoError(t, err)

	// Verify deletion
	id := fmt.Sprintf("user:%d", createdServer.ID)
	_, err = store.FindByID(ctx, userUUID, id)
	require.Error(t, err)
}

func TestAgentMCPServerStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	// Create servers for user1
	server1 := &database.AgentMCPServer{
		UserUUID: userUUID1,
		Name:     "User1 Server 1",
		Protocol: "sse",
		URL:      "http://user1-1.com",
	}
	_, err := store.Create(ctx, server1)
	require.NoError(t, err)

	server2 := &database.AgentMCPServer{
		UserUUID: userUUID1,
		Name:     "User1 Server 2",
		Protocol: "streamable",
		URL:      "http://user1-2.com",
	}
	_, err = store.Create(ctx, server2)
	require.NoError(t, err)

	// Create server for user2
	server3 := &database.AgentMCPServer{
		UserUUID: userUUID2,
		Name:     "User2 Server",
		Protocol: "sse",
		URL:      "http://user2.com",
	}
	_, err = store.Create(ctx, server3)
	require.NoError(t, err)

	// Test List for user1 - should return user1's servers and built-in servers
	filter := types.AgentMCPServerFilter{
		UserUUID: userUUID1,
	}
	servers, total, err := store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 2) // At least 2 user servers
	require.GreaterOrEqual(t, len(servers), 2)

	// Verify user1's servers are in the results
	foundServer1 := false
	foundServer2 := false
	for _, s := range servers {
		if s.Name == "User1 Server 1" {
			foundServer1 = true
			require.Equal(t, userUUID1, s.UserUUID)
		}
		if s.Name == "User1 Server 2" {
			foundServer2 = true
			require.Equal(t, userUUID1, s.UserUUID)
		}
	}
	require.True(t, foundServer1, "User1 Server 1 should be found")
	require.True(t, foundServer2, "User1 Server 2 should be found")

	// Test List with built_in filter
	builtInTrue := true
	builtInFilter := types.AgentMCPServerFilter{
		UserUUID: userUUID1,
		BuiltIn:  &builtInTrue,
	}
	builtInServers, _, err := store.List(ctx, builtInFilter, 10, 1)
	require.NoError(t, err)
	for _, s := range builtInServers {
		require.True(t, s.BuiltIn, "All servers should be built-in")
	}

	// Test List with protocol filter
	protocolSSE := "sse"
	protocolFilter := types.AgentMCPServerFilter{
		UserUUID: userUUID1,
		Protocol: &protocolSSE,
	}
	protocolServers, _, err := store.List(ctx, protocolFilter, 10, 1)
	require.NoError(t, err)
	for _, s := range protocolServers {
		require.Equal(t, "sse", s.Protocol)
	}

	// Test List with search filter
	searchFilter := types.AgentMCPServerFilter{
		UserUUID: userUUID1,
		Search:   "Server 1",
	}
	searchServers, total, err := store.List(ctx, searchFilter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	found := false
	for _, s := range searchServers {
		if s.Name == "User1 Server 1" {
			found = true
			break
		}
	}
	require.True(t, found, "Search should find User1 Server 1")
}

func TestAgentMCPServerConfigStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Config Test Server', 'Description', 'System', '', 'sse', 'http://config.com', '{}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Config Test Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Config-Header": "config_value",
		},
		Env: map[string]any{
			"CONFIG_ENV": "config_env_value",
		},
	}

	createdConfig, err := configStore.Create(ctx, config)
	require.NoError(t, err)
	require.NotZero(t, createdConfig.ID)
	require.Equal(t, userUUID, createdConfig.UserUUID)
	require.Equal(t, resourceID, createdConfig.ResourceID)
	require.NotNil(t, createdConfig.Headers)
	require.Equal(t, "config_value", createdConfig.Headers["Config-Header"])
	require.NotNil(t, createdConfig.Env)
	require.Equal(t, "config_env_value", createdConfig.Env["CONFIG_ENV"])
}

func TestAgentMCPServerConfigStore_FindByUserUUIDAndResourceID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Find Config Server', 'Description', 'System', '', 'sse', 'http://find.com', '{}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Find Config Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Find-Header": "find_value",
		},
		Env: map[string]any{
			"FIND_ENV": "find_env_value",
		},
	}

	_, err = configStore.Create(ctx, config)
	require.NoError(t, err)

	// Test FindByUserUUIDAndResourceID
	foundConfig, err := configStore.FindByUserUUIDAndResourceID(ctx, userUUID, resourceID)
	require.NoError(t, err)
	require.NotNil(t, foundConfig)
	require.Equal(t, userUUID, foundConfig.UserUUID)
	require.Equal(t, resourceID, foundConfig.ResourceID)
	require.Equal(t, "find_value", foundConfig.Headers["Find-Header"])
	require.Equal(t, "find_env_value", foundConfig.Env["FIND_ENV"])

	// Test FindByUserUUIDAndResourceID with non-existent config
	nonExistentConfig, err := configStore.FindByUserUUIDAndResourceID(ctx, uuid.New().String(), resourceID)
	require.NoError(t, err)
	require.Nil(t, nonExistentConfig)
}

func TestAgentMCPServerConfigStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Update Config Server', 'Description', 'System', '', 'sse', 'http://update.com', '{}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Update Config Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Original-Header": "original",
		},
		Env: map[string]any{
			"ORIGINAL_ENV": "original_value",
		},
	}

	createdConfig, err := configStore.Create(ctx, config)
	require.NoError(t, err)

	// Update the config
	createdConfig.Headers = map[string]any{
		"Updated-Header": "updated",
	}
	createdConfig.Env = map[string]any{
		"UPDATED_ENV": "updated_value",
	}

	err = configStore.Update(ctx, createdConfig)
	require.NoError(t, err)

	// Verify update
	updatedConfig, err := configStore.FindByUserUUIDAndResourceID(ctx, userUUID, resourceID)
	require.NoError(t, err)
	require.NotNil(t, updatedConfig)
	require.Equal(t, "updated", updatedConfig.Headers["Updated-Header"])
	require.Equal(t, "updated_value", updatedConfig.Env["UPDATED_ENV"])
}

func TestAgentMCPServerStore_FindByID_InvalidID(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()

	// Test with empty ID
	_, err := store.FindByID(ctx, userUUID, "")
	require.Error(t, err)

	// Test with invalid format
	_, err = store.FindByID(ctx, userUUID, "invalid-id")
	require.Error(t, err)

	// Test with non-existent builtin ID
	_, err = store.FindByID(ctx, userUUID, "builtin:99999")
	require.Error(t, err)

	// Test with non-existent user ID
	_, err = store.FindByID(ctx, userUUID, "user:99999")
	require.Error(t, err)
}

func TestAgentMCPServerConfigStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server into mcp_resources
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Delete Config Server', 'Description', 'System', '', 'sse', 'http://delete.com', '{}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Delete Config Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Delete-Header": "delete_value",
		},
		Env: map[string]any{
			"DELETE_ENV": "delete_env_value",
		},
	}

	createdConfig, err := configStore.Create(ctx, config)
	require.NoError(t, err)

	// Verify config exists
	foundConfig, err := configStore.FindByUserUUIDAndResourceID(ctx, userUUID, resourceID)
	require.NoError(t, err)
	require.NotNil(t, foundConfig)
	require.Equal(t, createdConfig.ID, foundConfig.ID)

	// Delete the config
	err = configStore.Delete(ctx, createdConfig.ID)
	require.NoError(t, err)

	// Verify deletion - config should not be found
	deletedConfig, err := configStore.FindByUserUUIDAndResourceID(ctx, userUUID, resourceID)
	require.NoError(t, err)
	require.Nil(t, deletedConfig)
}

func TestAgentMCPServerStore_List_WithTransaction(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create a user server
	server := &database.AgentMCPServer{
		UserUUID: userUUID,
		Name:     "Transaction Test Server",
		Protocol: "sse",
		URL:      "http://transaction.com",
	}
	_, err := store.Create(ctx, server)
	require.NoError(t, err)

	// Test List - should work with transaction (SET LOCAL "app.current_user")
	filter := types.AgentMCPServerFilter{
		UserUUID: userUUID,
	}
	servers, total, err := store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 1)
	require.GreaterOrEqual(t, len(servers), 1)

	// Verify the server is in the results
	found := false
	for _, s := range servers {
		if s.Name == "Transaction Test Server" {
			found = true
			require.Equal(t, userUUID, s.UserUUID)
			require.False(t, s.BuiltIn)
			break
		}
	}
	require.True(t, found, "Transaction Test Server should be found")

	// Test List with need_install filter
	needInstallTrue := true
	needInstallFilter := types.AgentMCPServerFilter{
		UserUUID:    userUUID,
		NeedInstall: &needInstallTrue,
	}
	needInstallServers, _, err := store.List(ctx, needInstallFilter, 10, 1)
	require.NoError(t, err)
	for _, s := range needInstallServers {
		require.True(t, s.NeedInstall, "All servers should have need_install=true")
	}
}
