package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRecomComponent_SetOpWeight(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRecomComponent(ctx, t)

	rc.mocks.stores.RepoMock().EXPECT().FindById(ctx, int64(1)).Return(&database.Repository{}, nil)
	rc.mocks.stores.RecomMock().EXPECT().UpsetOpWeights(ctx, int64(1), int64(100)).Return(nil)

	err := rc.SetOpWeight(ctx, 1, 100)
	require.Nil(t, err)
}

func TestRecomComponent_CalculateRecomScore(t *testing.T) {
	const batchSize = 2
	ctx := context.TODO()
	rc := initializeTestRecomComponent(ctx, t)

	rc.mocks.stores.RecomMock().EXPECT().LoadWeights(mock.Anything).Return(
		[]*database.RecomWeight{{Name: "freshness", WeightExp: "score = 12.34"}}, nil,
	)

	repo1 := database.Repository{ID: 1, Path: "foo/bar"}
	repo1.UpdatedAt = time.Now().Add(-24 * time.Hour)
	repo2 := database.Repository{ID: 2, Path: "foo/bar2"}
	repo2.UpdatedAt = time.Now().Add(24 * time.Hour)
	repo3 := database.Repository{ID: 3, Path: "foo/bar3"}
	repo3.UpdatedAt = time.Now().Add(24 * time.Hour)
	// loop 1
	rc.mocks.stores.RepoMock().EXPECT().BatchGet(mock.Anything, int64(0), batchSize, (*types.BatchGetFilter)(nil)).Return([]database.Repository{
		repo1, repo2,
	}, nil)
	// loop 2
	rc.mocks.stores.RepoMock().EXPECT().BatchGet(mock.Anything, int64(2), batchSize, (*types.BatchGetFilter)(nil)).Return([]database.Repository{
		repo3,
	}, nil)

	repo1FreshnessScore := database.RecomRepoScore{RepositoryID: 1, Score: 12.34, WeightName: "freshness"}
	repo1FreshnessScore.UpdatedAt = time.Now()
	repo1QualityScore := database.RecomRepoScore{RepositoryID: 1, Score: 300, WeightName: database.RecomWeightQuality}
	repo1QualityScore.UpdatedAt = time.Now()
	rc.mocks.stores.RecomMock().EXPECT().FindByRepoIDs(mock.Anything, []int64{1, 2}).Return(
		[]*database.RecomRepoScore{
			&repo1FreshnessScore,
			&repo1QualityScore,
		}, nil,
	)
	repo3Score := database.RecomRepoScore{RepositoryID: 3, Score: 12.34, WeightName: "freshness"}
	repo3Score.UpdatedAt = time.Now()
	rc.mocks.stores.RecomMock().EXPECT().FindByRepoIDs(mock.Anything, []int64{3}).Return(
		[]*database.RecomRepoScore{
			&repo3Score,
		}, nil,
	)

	rc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "foo", Name: "bar2", Limit: 500, Recursive: true},
	).Return(nil, nil)

	rc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "foo", Name: "bar3", Limit: 500, Recursive: true},
	).Return(nil, nil)

	// rc.mocks.stores.RecomMock().EXPECT().UpsertScore(ctx, int64(2), 12.34).Return(nil)
	rc.mocks.stores.RecomMock().EXPECT().UpsertScore(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, scores []*database.RecomRepoScore) error {
			// scores to map by repo id
			scoresMap := make(map[int64][]*database.RecomRepoScore)
			for _, score := range scores {
				if _, ok := scoresMap[score.RepositoryID]; !ok {
					scoresMap[score.RepositoryID] = make([]*database.RecomRepoScore, 0)
				}
				scoresMap[score.RepositoryID] = append(scoresMap[score.RepositoryID], score)
			}

			for _, s := range scoresMap {
				require.Len(t, s, 4)
				require.Equal(t, database.RecomWeightFreshness, s[0].WeightName)
				require.Equal(t, database.RecomWeightDownloads, s[1].WeightName)
				require.Equal(t, database.RecomWeightQuality, s[2].WeightName)
				require.Equal(t, database.RecomWeightTotal, s[3].WeightName)
				//check total score
				require.Equal(t, s[3].Score, s[0].Score+s[1].Score+s[2].Score)
			}
			return nil
		},
	)
	// rc.mocks.stores.RecomMock().EXPECT().UpsertScore(ctx, int64(3), 12.34).Return(nil)

	err := rc.CalculateRecomScore(ctx, batchSize)
	require.NoError(t, err)
}

func TestRecomComponent_calcFreshnessScore(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRecomComponent(ctx, t)

	// Test case 1: repository created 24 hours ago
	repo1 := &database.Repository{Path: "foo/bar"}
	repo1.CreatedAt = time.Now().Add(-24 * time.Hour)
	score1 := rc.calcFreshnessScore(repo1.CreatedAt, expFreshness)
	if score1 > 100 || score1 < 98 {
		t.Errorf("Expected score1 should in range [98,100], got: %f", score1)
	}

	// Test case 2: repository created 48 hours ago
	repo2 := &database.Repository{Path: "foo/bar"}
	repo2.CreatedAt = time.Now().Add(-50 * time.Hour)
	score2 := rc.calcFreshnessScore(repo2.CreatedAt, expFreshness)
	if score2 > 98.0 || score2 < 60.0 {
		t.Errorf("Expected score1 should in range [60,98), got: %f", score2)
	}

	// Test case 3: repository created 168 hours ago
	repo3 := &database.Repository{Path: "foo/bar"}
	repo3.CreatedAt = time.Now().Add(-168 * time.Hour)
	score3 := rc.calcFreshnessScore(repo3.CreatedAt, expFreshness)
	if score3 < 0 || score3 > 60 {
		t.Errorf("Expected score1 should in range [0,60), got: %f", score2)
	}
}

const expFreshness = `
if hours <= 48{
	score = 100 - 2.0/48.0 * hours
} else if hours <= 168{
	score = 98 - 38.0/120.0 * (hours - 48)
} else{
	score = 60 * (1 / (((hours - 168) / 48.0) + 1))
}
`

// const expDownloads = `
// if downloads <= 10{
//     score = downloads * (10 / 10)
// }else if downloads <= 100{
//     score = 10 + (downloads - 10) * (40.0 / 90.0)
// }else if downloads <= 1000{
//     score = 50 + (downloads - 100) * (40.0 / 900.0)
// }else{
//     score = 90 + 10 * (1 - (1 / (downloads / 1000.0)))
// }
// `
