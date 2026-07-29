package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// TestMirrorComponentDeleteClearsRepoSyncCache verifies mirror deletion removes LFS sync cache for the repository.
func TestMirrorComponentDeleteClearsRepoSyncCache(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	jobClient := useFakeMirrorJobClient(mc)
	syncCache := mockcache.NewMockCache(t)
	mc.syncCache = syncCache
	mc.config.Mirror.PartSize = 64

	mirror := &database.Mirror{ID: 321, RepositoryID: 123}
	mc.mocks.stores.MirrorMock().EXPECT().FindByID(ctx, mirror.ID).Return(mirror, nil)
	mc.mocks.stores.MirrorMock().EXPECT().DeleteWithTaskCancelTx(ctx, mirror.ID, jobClient).Return(nil)
	syncCache.EXPECT().DeleteRepoSyncCache(ctx, mirror.RepositoryID, "64").Return(nil)

	err := mc.Delete(ctx, mirror.ID)
	require.NoError(t, err)
}

// TestMirrorComponentBatchCreatePersistsCredentialsAndDefaultsPriority verifies batch mirror data reaches the store.
func TestMirrorComponentBatchCreatePersistsCredentialsAndDefaultsPriority(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	updatedAt := time.Now()
	existing := database.Mirror{
		ID: 1, SourceUrl: "https://example.com/existing.git", Username: "old-user", AccessToken: "old-token",
	}
	preservedExisting := database.Mirror{
		ID: 2, SourceUrl: "https://example.com/clear.git", Username: "clear-user", AccessToken: "clear-token",
	}

	mc.mocks.stores.MirrorMock().EXPECT().FindBySourceURLs(ctx, []string{
		existing.SourceUrl,
		preservedExisting.SourceUrl,
		"https://example.com/new.git",
	}).Return([]database.Mirror{existing, preservedExisting}, nil)

	updated := existing
	updated.Priority = types.LowMirrorPriority
	updated.RemoteUpdatedAt = updatedAt
	updated.Username = "updated-user"
	updated.AccessToken = "updated-token"
	preserved := preservedExisting
	preserved.Priority = types.LowMirrorPriority
	preserved.RemoteUpdatedAt = updatedAt
	mc.mocks.stores.MirrorMock().EXPECT().BatchUpdate(ctx, []database.Mirror{updated, preserved}).Return(nil)
	mc.mocks.stores.MirrorMock().EXPECT().BatchCreate(ctx, []database.Mirror{{
		SourceUrl:       "https://example.com/new.git",
		MirrorSourceID:  2,
		Username:        "new-user",
		AccessToken:     "new-token",
		Status:          types.MirrorQueued,
		Priority:        types.LowMirrorPriority,
		RemoteUpdatedAt: updatedAt,
	}}).Return(nil)

	err := mc.BatchCreate(ctx, types.BatchCreateMirrorReq{Mirrors: []types.MirrorReq{
		{SourceURL: existing.SourceUrl, SourceID: 1, Username: "updated-user", AccessToken: "updated-token", UpdatedAt: updatedAt},
		{SourceURL: preservedExisting.SourceUrl, SourceID: 1, UpdatedAt: updatedAt},
		{SourceURL: "https://example.com/new.git", SourceID: 2, Username: "new-user", AccessToken: "new-token", UpdatedAt: updatedAt},
	}})
	require.NoError(t, err)
}

// TestMirrorComponentBatchCreateNormalizesEmbeddedCredentials verifies lookup and persistence use sanitized source URLs.
func TestMirrorComponentBatchCreateNormalizesEmbeddedCredentials(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	updatedAt := time.Now()
	cleanURL := "https://example.com/new.git"

	mc.mocks.stores.MirrorMock().EXPECT().FindBySourceURLs(ctx, []string{cleanURL}).Return(nil, nil)
	mc.mocks.stores.MirrorMock().EXPECT().BatchCreate(ctx, []database.Mirror{{
		SourceUrl:       cleanURL,
		MirrorSourceID:  2,
		Username:        "source-user",
		AccessToken:     "source-token",
		Status:          types.MirrorQueued,
		Priority:        types.LowMirrorPriority,
		RemoteUpdatedAt: updatedAt,
	}}).Return(nil)

	err := mc.BatchCreate(ctx, types.BatchCreateMirrorReq{Mirrors: []types.MirrorReq{{
		SourceURL: "https://source-user:source-token@example.com/new", SourceID: 2, UpdatedAt: updatedAt,
	}}})
	require.NoError(t, err)
}

func TestMirrorComponent_Repos(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: ""}, true).Return([]database.Mirror{
		{
			CurrentTask: &database.MirrorTask{ID: 123, Progress: 100, Status: types.MirrorLfsSyncFinished},
			Progress:    100,
			Repository:  &database.Repository{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepositoryType: types.ModelRepo},
		},
	}, 100, nil)

	data, total, err := mc.Repos(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MirrorRepo{
		{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepoType: types.ModelRepo, Progress: 100, TaskID: 123},
	}, data)
}

func TestMirrorComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, types.MirrorFilter{Search: "foo"}, false).Return(
		[]database.Mirror{{CurrentTask: &database.MirrorTask{Status: types.MirrorLfsSyncFinished}, Username: "user", LastMessage: "msg", Repository: &database.Repository{}}}, 100, nil,
	)

	data, total, err := mc.Index(ctx, 10, 1, types.MirrorFilter{Search: "foo"})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Mirror{
		{Username: "user", LastMessage: "msg", LocalRepoPath: "s/", Status: types.MirrorLfsSyncFinished},
	}, data)
}

func TestMirrorComponent_Statistic(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().StatusCount(ctx).Return([]database.MirrorStatusCount{
		{Status: types.MirrorRepoSyncFinished, Count: 100},
	}, nil)

	s, err := mc.Statistics(ctx)
	require.Nil(t, err)
	require.Equal(t, []types.MirrorStatusCount{
		{Status: types.MirrorRepoSyncFinished, Count: 100},
	}, s)

}
