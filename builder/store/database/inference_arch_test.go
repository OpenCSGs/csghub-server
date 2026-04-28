package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestInferenceArchStore_GetInferenceArch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewInferenceArchStoreWithDB(db)

	// Test getting inference arch when no record exists
	arch, err := store.GetInferenceArch(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, arch)
	assert.Equal(t, "", arch.Patterns)
}

func TestInferenceArchStore_UpdateInferenceArch(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewInferenceArchStoreWithDB(db)

	// Test updating inference arch (should create a new record)
	req := &types.CreateInferenceArchReq{
		Patterns: "test-pattern\ntest-pattern-2",
	}

	updated, err := store.UpdateInferenceArch(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, updated)
	assert.Equal(t, "test-pattern\ntest-pattern-2", updated.Patterns)

	// Test updating inference arch again (should update existing record)
	req2 := &types.CreateInferenceArchReq{
		Patterns: "updated-pattern",
	}

	updated2, err := store.UpdateInferenceArch(context.Background(), req2)
	assert.NoError(t, err)
	assert.NotNil(t, updated2)
	assert.Equal(t, "updated-pattern", updated2.Patterns)
}
