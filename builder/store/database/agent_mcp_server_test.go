package database_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
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

func TestAgentMCPServerCapabilitiesStore_CreateOrUpdateAndFind(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStatusStoreWithDB(db)

	now := time.Now().UTC().Truncate(time.Second)
	capabilities := &database.AgentMCPServerStatus{
		ServerID:        "user:42",
		UserUUID:        "u",
		Status:          "connected",
		Capabilities:    map[string]any{"tools": []any{"tool-a"}},
		LastInspectedAt: now,
	}

	err := store.CreateOrUpdate(ctx, capabilities)
	require.NoError(t, err)

	found, err := store.FindByServerID(ctx, "user:42", "u")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, "connected", found.Status)
	require.NotNil(t, found.Capabilities)

	updated := &database.AgentMCPServerStatus{
		ServerID:        "user:42",
		UserUUID:        "u",
		Status:          "error",
		Error:           "inspect failed",
		Capabilities:    map[string]any{"tools": []any{"tool-b"}},
		LastInspectedAt: now.Add(2 * time.Minute),
	}
	err = store.CreateOrUpdate(ctx, updated)
	require.NoError(t, err)

	found, err = store.FindByServerID(ctx, "user:42", "u")
	require.NoError(t, err)
	require.Equal(t, "error", found.Status)
	require.Equal(t, "inspect failed", found.Error)
	require.NotNil(t, found.Capabilities)
	require.False(t, found.LastInspectedAt.IsZero())
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

func TestAgentMCPServerStore_ListAll(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)
	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Create a built-in server that does NOT require installation (always included in all_views).
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, need_install, created_at, updated_at)
		VALUES ('ListAll Builtin No Install', 'desc', 'System', '', 'sse', 'http://builtin-no-install', '{}'::jsonb, FALSE, NOW() - INTERVAL '3 hours', NOW() - INTERVAL '2 hours')
	`)
	require.NoError(t, err)

	var resourceNoInstallID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "ListAll Builtin No Install").
		Scan(ctx, &resourceNoInstallID)
	require.NoError(t, err)

	// Create a built-in server that DOES require installation; it appears in all_views only when a user override config exists.
	_, err = db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, need_install, created_at, updated_at)
		VALUES ('ListAll Builtin Need Install', 'desc', 'System', '', 'sse', 'http://builtin-need-install', '{}'::jsonb, TRUE, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour')
	`)
	require.NoError(t, err)

	var resourceNeedInstallID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "ListAll Builtin Need Install").
		Scan(ctx, &resourceNeedInstallID)
	require.NoError(t, err)

	overrideUserUUID := uuid.New().String()
	_, err = configStore.Create(ctx, &database.AgentMCPServerConfig{
		UserUUID:   overrideUserUUID,
		ResourceID: resourceNeedInstallID,
		Headers: map[string]any{
			"Authorization": "Bearer token",
		},
		Env: map[string]any{},
	})
	require.NoError(t, err)

	// Create a user-added server and force its updated_at to be the newest.
	userUUID := uuid.New().String()
	created, err := store.Create(ctx, &database.AgentMCPServer{
		UserUUID:    userUUID,
		Name:        "ListAll User Server",
		Description: "desc",
		Protocol:    "sse",
		URL:         "http://user-server",
		Headers:     map[string]any{},
		Env:         map[string]any{},
	})
	require.NoError(t, err)

	_, err = db.Core.ExecContext(ctx, `UPDATE agent_mcp_servers SET updated_at = NOW() WHERE id = ?`, created.ID)
	require.NoError(t, err)

	// Expect ordering by updated_at DESC:
	// 1) user server (NOW)
	// 2) builtin need_install (NOW - 1 hour)
	// 3) builtin no_install (NOW - 2 hours)
	expectedIDs := []string{
		fmt.Sprintf("user:%d", created.ID),
		fmt.Sprintf("builtin:%d", resourceNeedInstallID),
		fmt.Sprintf("builtin:%d", resourceNoInstallID),
	}

	page1, total, err := store.ListAll(ctx, 2, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page1, 2)
	require.Equal(t, expectedIDs[0], page1[0].ID)
	require.Equal(t, expectedIDs[1], page1[1].ID)

	page2, total2, err := store.ListAll(ctx, 2, 2)
	require.NoError(t, err)
	require.Equal(t, 3, total2)
	require.Len(t, page2, 1)
	require.Equal(t, expectedIDs[2], page2[0].ID)
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

func TestAgentMCPServerStore_IsServerNameExists(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID1 := uuid.New().String()
	userUUID2 := uuid.New().String()

	t.Run("success - name does not exist", func(t *testing.T) {
		exists, err := store.IsServerNameExists(ctx, userUUID1, "NonExistent Server")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("success - name exists for user server", func(t *testing.T) {
		// Create a user server
		server := &database.AgentMCPServer{
			UserUUID: userUUID1,
			Name:     "User Server Name",
			Protocol: "sse",
			URL:      "http://user.com",
		}
		_, err := store.Create(ctx, server)
		require.NoError(t, err)

		// Check if name exists for the same user
		exists, err := store.IsServerNameExists(ctx, userUUID1, "User Server Name")
		require.NoError(t, err)
		require.True(t, exists)

		// Check if name exists for a different user (should not exist)
		exists, err = store.IsServerNameExists(ctx, userUUID2, "User Server Name")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("success - name exists for built-in server", func(t *testing.T) {
		// Insert a built-in server into mcp_resources
		_, err := db.Core.ExecContext(ctx, `
			INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
			VALUES ('Built-in Name Check', 'Description', 'System', '', 'sse', 'http://builtin.com', '{}'::jsonb, NOW(), NOW())
			RETURNING id
		`)
		require.NoError(t, err)

		// Check if name exists for any user (built-in servers are visible to all users)
		exists, err := store.IsServerNameExists(ctx, userUUID1, "Built-in Name Check")
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = store.IsServerNameExists(ctx, userUUID2, "Built-in Name Check")
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("success - user server name conflict with built-in server", func(t *testing.T) {
		// Insert a built-in server
		_, err := db.Core.ExecContext(ctx, `
			INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
			VALUES ('Conflict Server', 'Description', 'System', '', 'sse', 'http://conflict-builtin.com', '{}'::jsonb, NOW(), NOW())
			RETURNING id
		`)
		require.NoError(t, err)

		// Try to create a user server with the same name (should fail name check)
		exists, err := store.IsServerNameExists(ctx, userUUID1, "Conflict Server")
		require.NoError(t, err)
		require.True(t, exists, "Name should exist because of built-in server")
	})

	t.Run("success - built-in server name conflict with user server", func(t *testing.T) {
		// Create a user server first
		server := &database.AgentMCPServer{
			UserUUID: userUUID1,
			Name:     "User Conflict Server",
			Protocol: "sse",
			URL:      "http://user-conflict.com",
		}
		_, err := store.Create(ctx, server)
		require.NoError(t, err)

		// Check if name exists (should exist for the user)
		exists, err := store.IsServerNameExists(ctx, userUUID1, "User Conflict Server")
		require.NoError(t, err)
		require.True(t, exists)

		// Check if name exists for a different user (should not exist)
		exists, err = store.IsServerNameExists(ctx, userUUID2, "User Conflict Server")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("failed - database error", func(t *testing.T) {
		// Create a new database connection and close it to simulate database error
		closedDB := tests.InitTestDB()
		closedDB.Close()

		closedStore := database.NewAgentMCPServerStoreWithDB(closedDB)

		exists, err := closedStore.IsServerNameExists(ctx, userUUID1, "Test Server")
		require.Error(t, err, "should return error when database connection is closed")
		require.False(t, exists)
		// Check that error is from HandleDBError (CustomError type)
		_, ok := err.(errorx.CustomError)
		require.True(t, ok, "error should be of type errorx.CustomError from HandleDBError")
	})
}

func TestAgentMCPServerStore_List_WithPinned(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)
	preferenceStore := database.NewAgentUserPreferenceStoreWithDB(db)

	userUUID := uuid.New().String()

	// Create user servers
	server1 := &database.AgentMCPServer{
		UserUUID: userUUID,
		Name:     "First Server",
		Protocol: "sse",
		URL:      "http://first.com",
	}
	createdServer1, err := store.Create(ctx, server1)
	require.NoError(t, err)

	server2 := &database.AgentMCPServer{
		UserUUID: userUUID,
		Name:     "Second Server",
		Protocol: "streamable",
		URL:      "http://second.com",
	}
	createdServer2, err := store.Create(ctx, server2)
	require.NoError(t, err)

	// Pin server2
	server2ID := fmt.Sprintf("user:%d", createdServer2.ID)
	pinPreference := &database.AgentUserPreference{
		UserUUID:   userUUID,
		EntityType: types.AgentUserPreferenceEntityTypeAgentMCPServer,
		EntityID:   server2ID,
		Action:     types.AgentUserPreferenceActionPin,
	}
	err = preferenceStore.Create(ctx, pinPreference)
	require.NoError(t, err)

	// List servers - pinned should appear first
	filter := types.AgentMCPServerFilter{
		UserUUID: userUUID,
	}
	servers, total, err := store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 2)
	require.GreaterOrEqual(t, len(servers), 2)

	// Find the pinned server
	var pinnedServer *database.AgentMCPServerView
	for i := range servers {
		if servers[i].ID == server2ID {
			pinnedServer = &servers[i]
			break
		}
	}
	require.NotNil(t, pinnedServer, "Pinned server should be found")
	require.True(t, pinnedServer.IsPinned, "Server should be marked as pinned")
	require.NotNil(t, pinnedServer.PinnedAt, "PinnedAt should be set")

	// The pinned server should be first in the list (if no built-in servers are present)
	// Note: Built-in servers might be present, so we check that pinned server is before non-pinned user servers
	foundPinned := false
	foundNonPinned := false
	for _, s := range servers {
		if s.ID == server2ID {
			foundPinned = true
			require.True(t, s.IsPinned, "Pinned server should be marked as pinned")
		}
		if s.ID == fmt.Sprintf("user:%d", createdServer1.ID) {
			foundNonPinned = true
			require.False(t, s.IsPinned, "Non-pinned server should not be marked as pinned")
		}
		// If we found both, pinned should come before non-pinned
		if foundPinned && foundNonPinned {
			break
		}
	}
	require.True(t, foundPinned, "Pinned server should be in the list")
	require.True(t, foundNonPinned, "Non-pinned server should be in the list")
}

func TestAgentMCPServerStore_Find(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)

	userUUID := uuid.New().String()
	server := &database.AgentMCPServer{
		UserUUID:    userUUID,
		Name:        "Find Test Server",
		Description: "Test description",
		Protocol:    "sse",
		URL:         "http://find.com",
		Headers: map[string]any{
			"Test-Header": "test_value",
		},
		Env: map[string]any{
			"TEST_ENV": "test_env_value",
		},
	}

	createdServer, err := store.Create(ctx, server)
	require.NoError(t, err)

	// Test Find with numeric ID
	foundServer, err := store.Find(ctx, createdServer.ID)
	require.NoError(t, err)
	require.NotNil(t, foundServer)
	require.Equal(t, createdServer.ID, foundServer.ID)
	require.Equal(t, userUUID, foundServer.UserUUID)
	require.Equal(t, "Find Test Server", foundServer.Name)
	require.Equal(t, "Test description", foundServer.Description)
	require.Equal(t, "sse", foundServer.Protocol)
	require.Equal(t, "http://find.com", foundServer.URL)
	require.NotNil(t, foundServer.Headers)
	require.Equal(t, "test_value", foundServer.Headers["Test-Header"])
	require.NotNil(t, foundServer.Env)
	require.Equal(t, "test_env_value", foundServer.Env["TEST_ENV"])

	// Test Find with non-existent ID
	nonExistentServer, err := store.Find(ctx, 99999)
	require.NoError(t, err)
	require.Nil(t, nonExistentServer)
}

func TestAgentMCPServerStore_FindByID_BuiltInServer_WithOverride_EdgeCases(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)
	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server with no default headers
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('No Headers Server', 'Description', 'System', '', 'sse', 'http://noheaders.com', NULL::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "No Headers Server").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()

	// Create override config with headers only
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		Headers: map[string]any{
			"Override-Header": "override",
		},
		// No Env override
	}
	_, err = configStore.Create(ctx, config)
	require.NoError(t, err)

	id := fmt.Sprintf("builtin:%d", resourceID)
	detail, err := store.FindByID(ctx, userUUID, id)
	require.NoError(t, err)
	require.NotNil(t, detail)
	require.True(t, detail.BuiltIn)

	// Headers should be set from override (even though default was NULL)
	require.NotNil(t, detail.Headers)
	require.Equal(t, "override", detail.Headers["Override-Header"])

	// Env should be nil (no override, and default is NULL)
	require.Nil(t, detail.Env)
}

func TestAgentMCPServerStore_FindByID_BuiltInServer_WithOverride_EnvOnly(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewAgentMCPServerStoreWithDB(db)
	configStore := database.NewAgentMCPServerConfigStoreWithDB(db)

	// Insert a built-in server with default headers
	_, err := db.Core.ExecContext(ctx, `
		INSERT INTO mcp_resources (name, description, owner, avatar, protocol, url, headers, created_at, updated_at)
		VALUES ('Env Only Override', 'Description', 'System', '', 'sse', 'http://envonly.com', '{"Default-Header": "default"}'::jsonb, NOW(), NOW())
		RETURNING id
	`)
	require.NoError(t, err)

	var resourceID int64
	err = db.Core.NewSelect().
		TableExpr("mcp_resources").
		ColumnExpr("id").
		Where("name = ?", "Env Only Override").
		Scan(ctx, &resourceID)
	require.NoError(t, err)

	userUUID := uuid.New().String()

	// Create override config with env only (no headers override)
	config := &database.AgentMCPServerConfig{
		UserUUID:   userUUID,
		ResourceID: resourceID,
		// No Headers override
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
	require.True(t, detail.BuiltIn)

	// Headers should be from default (no override)
	require.NotNil(t, detail.Headers)
	require.Equal(t, "default", detail.Headers["Default-Header"])

	// Env should be from override
	require.NotNil(t, detail.Env)
	require.Equal(t, "override_value", detail.Env["OVERRIDE_ENV"])
	require.False(t, detail.NeedInstall, "NeedInstall should be false when config exists")
}
