package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestGatewayMCPServerCapabilityStore_CreateOrUpdate_FindByServerAndConfig(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServerCapabilityStoreWithDB(db)

	cap := &database.GatewayMCPServerCapability{
		MCPServerID:   1,
		MCPServerName: "test-server",
		ConfigHash:    "hash-abc",
		Capabilities:  map[string]any{"tools": []any{}},
		Status:        "connected",
		RefreshedAt:   time.Now(),
		ExpiresAt:     time.Now().Add(time.Hour),
	}

	err := store.CreateOrUpdate(ctx, cap)
	require.NoError(t, err)

	found, err := store.FindByServerAndConfig(ctx, 1, "hash-abc")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, int64(1), found.MCPServerID)
	require.Equal(t, "connected", found.Status)

	// Update same server+config (upsert)
	cap.Capabilities = map[string]any{"tools": []any{"tool1"}}
	cap.Status = "connected"
	err = store.CreateOrUpdate(ctx, cap)
	require.NoError(t, err)

	found2, err := store.FindByServerAndConfig(ctx, 1, "hash-abc")
	require.NoError(t, err)
	require.NotNil(t, found2)
	require.Equal(t, "connected", found2.Status)
}

func TestGatewayMCPServerCapabilityStore_UpsertErrorPreservesCatalog(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.Background()
	store := database.NewGatewayMCPServerCapabilityStoreWithDB(db)
	original := &database.GatewayMCPServerCapability{
		MCPServerID: 91, MCPServerName: "modern", ConfigHash: "config",
		Capabilities: map[string]any{"tools": []any{"read"}}, ProtocolVersion: "2026-07-28",
		Status: "connected", RefreshedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.CreateOrUpdate(ctx, original))
	failure := &database.GatewayMCPServerCapability{
		MCPServerID: 91, MCPServerName: "modern", ConfigHash: "config", Error: "unreachable",
		RefreshedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.UpsertError(ctx, failure))
	found, err := store.FindByServerAndConfig(ctx, 91, "config")
	require.NoError(t, err)
	require.Equal(t, original.Capabilities, found.Capabilities)
	require.Equal(t, "2026-07-28", found.ProtocolVersion)
	require.Equal(t, "error", found.Status)
	require.Equal(t, "unreachable", found.Error)
	// A different configuration must not inherit another configuration's catalog.
	failure.ID = 0
	failure.ConfigHash = "other"
	require.NoError(t, store.UpsertError(ctx, failure))
	other, err := store.FindByServerAndConfig(ctx, 91, "other")
	require.NoError(t, err)
	require.Empty(t, other.Capabilities)
	require.Empty(t, other.ProtocolVersion)
	original.ProtocolVersion = "2025-11-25"
	original.Capabilities = map[string]any{"tools": []any{"write"}}
	require.NoError(t, store.CreateOrUpdate(ctx, original))
	found, err = store.FindByServerAndConfig(ctx, 91, "config")
	require.NoError(t, err)
	require.Equal(t, original.Capabilities, found.Capabilities)
	require.Equal(t, original.ProtocolVersion, found.ProtocolVersion)
	require.Equal(t, "connected", found.Status)
}

func TestGatewayMCPServerCapabilityStore_DeleteByServer(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServerCapabilityStoreWithDB(db)

	cap := &database.GatewayMCPServerCapability{
		MCPServerID: 2, MCPServerName: "s2", ConfigHash: "h2", Status: "connected",
		RefreshedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.CreateOrUpdate(ctx, cap))

	found, _ := store.FindByServerAndConfig(ctx, 2, "h2")
	require.NotNil(t, found)

	err := store.DeleteByServer(ctx, 2)
	require.NoError(t, err)
	found, _ = store.FindByServerAndConfig(ctx, 2, "h2")
	require.Nil(t, found)
}

func TestGatewayMCPServerCapabilityStore_DeleteExpired_DeleteAllForUserBackend(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewGatewayMCPServerCapabilityStoreWithDB(db)

	expired := &database.GatewayMCPServerCapability{
		MCPServerID: 3, MCPServerName: "s3", ConfigHash: "exp", Status: "connected",
		RefreshedAt: time.Now().Add(-2 * time.Hour), ExpiresAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, store.CreateOrUpdate(ctx, expired))

	err := store.DeleteExpired(ctx)
	require.NoError(t, err)
	found, _ := store.FindByServerAndConfig(ctx, 3, "exp")
	require.Nil(t, found)

	// DeleteAllForUserBackend is alias for DeleteByServer
	cap := &database.GatewayMCPServerCapability{
		MCPServerID: 4, MCPServerName: "s4", ConfigHash: "h4", Status: "connected",
		RefreshedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, store.CreateOrUpdate(ctx, cap))
	err = store.DeleteAllForUserBackend(ctx, 4)
	require.NoError(t, err)
	found4, _ := store.FindByServerAndConfig(ctx, 4, "h4")
	require.Nil(t, found4)
}
