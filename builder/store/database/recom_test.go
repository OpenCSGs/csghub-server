package database_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestRecomStore_All(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRecomStoreWithDB(db)

	err := store.UpsertScore(ctx, 123, 1)
	require.Nil(t, err)
	err = store.UpsertScore(ctx, 123, 2)
	require.Nil(t, err)
	err = store.UpsertScore(ctx, 456, 1)
	require.Nil(t, err)

	scores, err := store.Index(ctx, 0, 10)
	require.Nil(t, err)
	require.Equal(t, 2, len(scores))
	ids := []string{}
	for _, s := range scores {
		ids = append(ids, fmt.Sprintf("%d/%.0f", s.RepositoryID, s.Score))
	}
	require.Equal(t, []string{"123/2", "456/1"}, ids)

	_, err = db.Core.NewInsert().Model(&database.RecomWeight{Name: "w1"}).Exec(ctx)
	require.Nil(t, err)
	ws, err := store.LoadWeights(ctx)
	require.Nil(t, err)
	require.Equal(t, 3, len(ws))
	names := []string{}
	for _, w := range ws {
		names = append(names, w.Name)
	}
	require.ElementsMatch(t, []string{"freshness", "downloads", "w1"}, names)

	_, err = db.Core.NewInsert().Model(&database.RecomOpWeight{
		Weight:       3,
		RepositoryID: 123,
	}).Exec(ctx)
	require.Nil(t, err)
	wos, err := store.LoadOpWeights(ctx)
	require.Nil(t, err)
	require.Equal(t, 1, len(wos))
	require.Equal(t, 3, wos[0].Weight)

}
