package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestMCPResource_CURD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewMCPResourceStoreWithDB(db)

	res := &database.MCPResource{
		Name:        "test-name",
		Description: "test-desc",
		Owner:       "OpenCSG",
		Avatar:      "test-avatar",
		Url:         "https://test.com",
		Protocol:    "sse",
		Headers: map[string]any{
			"key1": "value1",
		},
	}

	res, err := store.Create(ctx, res)
	require.Nil(t, err)
	require.Equal(t, res.Name, "test-name")

	filter := &types.MCPFilter{
		Per:  10,
		Page: 1,
	}

	resList, total, err := store.List(ctx, filter)
	require.Nil(t, err)
	require.Equal(t, total, 1)
	require.Equal(t, resList[0].Name, "test-name")

	res.Name = "updated-name"
	newRes, err := store.Update(ctx, res)
	require.Nil(t, err)
	require.Equal(t, newRes.Name, "updated-name")

	err = store.Delete(ctx, res)
	require.Nil(t, err)

}
