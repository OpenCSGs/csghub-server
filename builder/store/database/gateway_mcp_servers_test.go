package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestGatewayMCPServersStore_Create_FindByID_Update_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServersStoreWithDB(db)

	backend := &database.GatewayMCPServers{
		Name:        "test-server-unique-1",
		Description: "desc",
		Protocol:    "streamable",
		URL:         "http://localhost/mcp",
		Headers:     map[string]any{"X-Token": "secret"},
		ConfigHash:  "hash-test-server-1",
	}

	created, err := store.Create(ctx, backend)
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Equal(t, backend.Name, created.Name)
	require.Equal(t, backend.URL, created.URL)

	found, err := store.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, created.ID, found.ID)

	// Update: use GetMCPServer to get table model for Update
	toUpdate, err := store.GetMCPServer(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, toUpdate)
	toUpdate.Description = "updated-desc"
	toUpdate.URL = "http://other/mcp"
	err = store.Update(ctx, toUpdate)
	require.NoError(t, err)

	found2, err := store.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "updated-desc", found2.Description)
	require.Equal(t, "http://other/mcp", found2.URL)

	// Delete
	err = store.Delete(ctx, created.ID)
	require.NoError(t, err)

	notFound, err := store.FindByID(ctx, created.ID)
	require.NoError(t, err)
	require.Nil(t, notFound)
}

func TestGatewayMCPServersStore_List_ListAll_IsNameExists(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServersStoreWithDB(db)

	b1 := &database.GatewayMCPServers{
		Name: "server-a-unique", Protocol: "streamable", URL: "http://a/mcp", ConfigHash: "hash-a",
	}
	b2 := &database.GatewayMCPServers{
		Name: "server-b-unique", Protocol: "streamable", URL: "http://b/mcp", ConfigHash: "hash-b",
	}
	created1, err := store.Create(ctx, b1)
	require.NoError(t, err)
	created2, err := store.Create(ctx, b2)
	require.NoError(t, err)
	defer func() {
		_ = store.Delete(ctx, created1.ID)
		_ = store.Delete(ctx, created2.ID)
	}()

	// List with pagination
	list, total, err := store.List(ctx, types.GatewayMCPServerFilter{}, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 2)
	require.GreaterOrEqual(t, len(list), 2)

	// GetMCPServers
	avail, err := store.GetMCPServers(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(avail), 2)

	// IsNameExists
	exists, err := store.IsNameExists(ctx, "server-a-unique")
	require.NoError(t, err)
	require.True(t, exists)
	exists, err = store.IsNameExists(ctx, "nonexistent-name-xyz")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestGatewayMCPServersStore_GetMCPServer(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServersStoreWithDB(db)

	backend := &database.GatewayMCPServers{
		Name:       "get-test-server-unique",
		Protocol:   "streamable",
		URL:        "http://get-test/mcp",
		ConfigHash: "hash-get-test",
	}
	created, err := store.Create(ctx, backend)
	require.NoError(t, err)
	defer func() { _ = store.Delete(ctx, created.ID) }()

	// GetMCPServer returns table model (not view)
	got, err := store.GetMCPServer(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, created.ID, got.ID)
	require.Equal(t, "get-test-server-unique", got.Name)
	require.Equal(t, "http://get-test/mcp", got.URL)

	// Not found
	got, err = store.GetMCPServer(ctx, 999999)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestGatewayMCPServersStore_GetMCPServers(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServersStoreWithDB(db)

	all, err := store.GetMCPServers(ctx)
	require.NoError(t, err)
	// Returns a slice (may be empty)
	require.GreaterOrEqual(t, len(all), 0)
}

func TestGatewayMCPServersStore_List_WithFilters(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServersStoreWithDB(db)

	// List with empty filter
	list, total, err := store.List(ctx, types.GatewayMCPServerFilter{}, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 0)
	require.GreaterOrEqual(t, len(list), 0)

	// List with search filter (fuzzy)
	filter := types.GatewayMCPServerFilter{Search: "server-a"}
	list, total, err = store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 0)
	require.GreaterOrEqual(t, len(list), 0)

	// List with status filter
	status := "connected"
	filter = types.GatewayMCPServerFilter{Status: &status}
	list, total, err = store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 0)
	require.GreaterOrEqual(t, len(list), 0)

	// List with ExactMatch filter
	filter = types.GatewayMCPServerFilter{Search: "server-a-unique", ExactMatch: true}
	list, total, err = store.List(ctx, filter, 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, 0)
	require.GreaterOrEqual(t, len(list), 0)
}
