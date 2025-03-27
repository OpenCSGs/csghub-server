package database_test

import (
	"context"
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

	err := store.UpsertScore(ctx, []*database.RecomRepoScore{
		{RepositoryID: 123, WeightName: database.RecomWeightFreshness, Score: 1},
	})
	require.Nil(t, err)
	repoScore, err := store.FindScoreByRepoIDs(ctx, []int64{123})
	require.Nil(t, err)
	require.Equal(t, float64(1), repoScore[0].Score)
	/*
		err = store.UpsertScore(ctx, []*database.RecomRepoScore{
			//change repo 1 weight1 score to 2
			{RepositoryID: 123, WeightName: database.RecomWeightFreshness, Score: 2},
			// new repo 1 weight2 score
			{RepositoryID: 123, WeightName: database.RecomWeightDownloads, Score: 2},
		})
		require.Nil(t, err)
		err = store.UpsertScore(ctx, []*database.RecomRepoScore{
			{RepositoryID: 456, WeightName: database.RecomWeightFreshness, Score: 1},
			{RepositoryID: 789, WeightName: database.RecomWeightOp, Score: 300},
		})
		require.Nil(t, err)

		scores, err := store.FindScoreByRepoIDs(ctx, []int64{123, 456, 789})
		require.Nil(t, err)
		require.Equal(t, 4, len(scores))
		var scoresToCompare []*database.RecomRepoScore
		for _, score := range scores {
			scoresToCompare = append(scoresToCompare, &database.RecomRepoScore{
				RepositoryID: score.RepositoryID,
				WeightName:   score.WeightName,
				Score:        score.Score,
			})
		}
		require.Contains(t, scoresToCompare, &database.RecomRepoScore{RepositoryID: 123, WeightName: database.RecomWeightFreshness, Score: 2})
		require.Contains(t, scoresToCompare, &database.RecomRepoScore{RepositoryID: 123, WeightName: database.RecomWeightDownloads, Score: 2})
		require.Contains(t, scoresToCompare, &database.RecomRepoScore{RepositoryID: 456, WeightName: database.RecomWeightFreshness, Score: 1})
		require.Contains(t, scoresToCompare, &database.RecomRepoScore{RepositoryID: 789, WeightName: database.RecomWeightOp, Score: 300})

		_, err = db.Core.NewInsert().Model(&database.RecomWeight{Name: "w1"}).Exec(ctx)
		require.Nil(t, err)
		ws, err := store.LoadWeights(ctx)
		require.Nil(t, err)
		require.Equal(t, 3, len(ws))
		names := []database.RecomWeightName{}
		for _, w := range ws {
			names = append(names, database.RecomWeightName(w.Name))
		}
		require.ElementsMatch(t, []database.RecomWeightName{"freshness", "downloads", "w1"}, names)
	*/
}
