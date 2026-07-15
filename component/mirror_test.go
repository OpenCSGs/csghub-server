package component

import (
	"context"
	"testing"

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

// TestMirrorComponent_Repos verifies mirror repositories are returned with their current task state.
func TestMirrorComponent_Repos(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, types.MirrorFilter{}, true).Return([]database.Mirror{
		{
			CurrentTask: &database.MirrorTask{Progress: 100, Status: types.MirrorLfsSyncFinished},
			Progress:    100,
			Repository:  &database.Repository{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepositoryType: types.ModelRepo},
		},
	}, 100, nil)

	data, total, err := mc.Repos(ctx, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.MirrorRepo{
		{Path: "foo", SyncStatus: types.SyncStatusCompleted, RepoType: types.ModelRepo, Progress: 100},
	}, data)
}

// TestMirrorComponent_Index verifies filters and effective mirror statuses are returned.
func TestMirrorComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMirrorComponent(ctx, t)
	filter := types.MirrorFilter{Search: "foo"}

	mc.mocks.stores.MirrorMock().EXPECT().IndexWithPagination(ctx, 10, 1, filter, false).Return(
		[]database.Mirror{
			{CurrentTask: &database.MirrorTask{Status: types.MirrorLfsSyncFinished}, Username: "user", LastMessage: "msg", Repository: &database.Repository{}},
			{Status: types.MirrorRepoSyncFailed, Username: "fallback"},
			{Username: "default"},
		}, 100, nil,
	)

	data, total, err := mc.Index(ctx, 10, 1, filter)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Mirror{
		{Username: "user", LastMessage: "msg", LocalRepoPath: "s/", Status: types.MirrorLfsSyncFinished},
		{Username: "fallback", Status: types.MirrorRepoSyncFailed},
		{Username: "default", Status: types.MirrorQueued},
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
