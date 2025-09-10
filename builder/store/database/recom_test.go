package database_test

import (
	"context"
	"slices"
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
	repoScore, err := store.FindByRepoIDs(ctx, []int64{123})
	require.Nil(t, err)
	require.Equal(t, float64(1), repoScore[0].Score)

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

	m, err := store.LoadRepoOpWeights(ctx, []int64{789})
	require.Nil(t, err)
	require.Equal(t, m, map[int64]int{789: 300})

	scores, err := store.FindByRepoIDs(ctx, []int64{123, 456, 789})
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
		names = append(names, w.Name)
	}
	require.ElementsMatch(t, []database.RecomWeightName{"freshness", "downloads", "w1"}, names)
}

func TestRecomStore_UpsetOpWeights(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewRecomStoreWithDB(db)

	t.Run("op not exists", func(t *testing.T) {

		err := store.UpsertScore(ctx, []*database.RecomRepoScore{
			{RepositoryID: 123, WeightName: database.RecomWeightFreshness, Score: 1},
			// {RepositoryID: 123, WeightName: database.RecomWeightOp, Score: 300},
		})
		require.Nil(t, err)

		err = store.UpsetOpWeights(ctx, 123, 300)
		require.Nil(t, err)

		repoScores, err := store.FindByRepoIDs(ctx, []int64{123})
		require.Nil(t, err)
		require.Len(t, repoScores, 3)
		ok := slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightOp && rs.Score == 300
		})
		require.True(t, ok)
		ok = slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightTotal && rs.Score == 301
		})
		require.True(t, ok)
	})

	t.Run("op exists", func(t *testing.T) {
		err := store.UpsertScore(ctx, []*database.RecomRepoScore{
			{RepositoryID: 456, WeightName: database.RecomWeightFreshness, Score: 1},
			{RepositoryID: 456, WeightName: database.RecomWeightOp, Score: 300},
		})
		require.Nil(t, err)

		err = store.UpsetOpWeights(ctx, 456, 400)
		require.Nil(t, err)

		repoScores, err := store.FindByRepoIDs(ctx, []int64{456})
		require.Nil(t, err)
		require.Len(t, repoScores, 3)
		ok := slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightOp && rs.Score == 400
		})
		require.True(t, ok)
		ok = slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightTotal && rs.Score == 401
		})
		require.True(t, ok)
	})

	t.Run("total exists", func(t *testing.T) {
		err := store.UpsertScore(ctx, []*database.RecomRepoScore{
			{RepositoryID: 789, WeightName: database.RecomWeightFreshness, Score: 1},
			{RepositoryID: 789, WeightName: database.RecomWeightOp, Score: 300},
			{RepositoryID: 789, WeightName: database.RecomWeightTotal, Score: 301},
		})
		require.Nil(t, err)

		err = store.UpsetOpWeights(ctx, 789, 400)
		require.Nil(t, err)

		repoScores, err := store.FindByRepoIDs(ctx, []int64{789})
		require.Nil(t, err)
		require.Len(t, repoScores, 3)
		ok := slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightOp && rs.Score == 400
		})
		require.True(t, ok)
		ok = slices.ContainsFunc(repoScores, func(rs *database.RecomRepoScore) bool {
			return rs.WeightName == database.RecomWeightTotal && rs.Score == 401
		})
		require.True(t, ok)
	})
}
