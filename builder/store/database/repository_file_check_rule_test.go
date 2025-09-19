package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRepositoryFileCheckRuleStore_Create(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepositoryFileCheckRuleStoreWithDB(db)
	ctx := context.Background()

	rule, err := store.Create(ctx, "namespace", "admin")
	require.NoError(t, err)
	require.NotNil(t, rule)

	exist, err := store.Exists(ctx, "namespace", "admin")
	require.NoError(t, err)
	require.True(t, exist)
}

func TestRepositoryFileCheckRuleStore_List(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepositoryFileCheckRuleStoreWithDB(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "namespace", "user1")
	require.NoError(t, err)
	_, err = store.Create(ctx, "file_ext", ".zip")
	require.NoError(t, err)

	exist, err := store.Exists(ctx, "namespace", "user1")
	require.NoError(t, err)
	require.True(t, exist)

	exist, err = store.Exists(ctx, "file_ext", ".zip")
	require.NoError(t, err)
	require.True(t, exist)
}

func TestRepositoryFileCheckRuleStore_ListByRuleType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepositoryFileCheckRuleStoreWithDB(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "namespace", "user1")
	require.NoError(t, err)
	_, err = store.Create(ctx, "namespace", "user2")
	require.NoError(t, err)
	_, err = store.Create(ctx, "file_ext", ".zip")
	require.NoError(t, err)

	rules, err := store.ListByRuleType(ctx, "file_ext")
	require.NoError(t, err)
	require.Len(t, rules, 1)

	var has bool
	for _, rule := range rules {
		require.Equal(t, "file_ext", rule.RuleType)
		if rule.Pattern == ".zip" {
			has = true
		}
	}
	require.True(t, has)
}

func TestRepositoryFileCheckRuleStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepositoryFileCheckRuleStoreWithDB(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "namespace", "admin")
	require.NoError(t, err)

	err = store.Delete(ctx, "namespace", "admin")
	require.NoError(t, err)

	exist, err := store.Exists(ctx, "namespace", "admin")
	require.NoError(t, err)
	require.False(t, exist)
}

func TestRepositoryFileCheckRuleStore_Exists(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	store := database.NewRepositoryFileCheckRuleStoreWithDB(db)
	ctx := context.Background()

	_, err := store.Create(ctx, "namespace", "admin")
	require.NoError(t, err)

	// test for existing rule
	exists, err := store.Exists(ctx, "namespace", "admin")
	require.NoError(t, err)
	require.True(t, exists)

	// test for non-existing rule
	exists, err = store.Exists(ctx, "namespace", "non-existent")
	require.NoError(t, err)
	require.False(t, exists)
}
