package component

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	multisync_mock "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestMultiSyncComponent_More(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestMultiSyncComponent(ctx, t)

	mc.mocks.stores.MultiSyncMock().EXPECT().GetAfter(ctx, int64(1), int64(10)).Return(
		[]database.SyncVersion{{Version: 2}}, nil,
	)

	data, err := mc.More(ctx, 1, 10)
	require.Nil(t, err)
	require.Equal(t, []types.SyncVersion{
		{Version: 2},
	}, data)
}

func TestMultiSyncComponent_SyncAsClient(t *testing.T) {
	ctx := mock.Anything
	mc := initializeTestMultiSyncComponent(context.TODO(), t)

	mc.mocks.stores.MultiSyncMock().EXPECT().GetLatest(ctx).Return(database.SyncVersion{
		Version: 1,
	}, nil)
	mockedClient := multisync_mock.NewMockClient(t)
	mockedClient.EXPECT().Latest(ctx, int64(1)).Return(types.SyncVersionResponse{
		Data: struct {
			Versions []types.SyncVersion "json:\"versions\""
			HasMore  bool                "json:\"has_more\""
		}{
			Versions: []types.SyncVersion{
				{Version: 2},
			},
			HasMore: true,
		},
	}, nil)
	mockedClient.EXPECT().Latest(ctx, int64(2)).Return(types.SyncVersionResponse{
		Data: struct {
			Versions []types.SyncVersion "json:\"versions\""
			HasMore  bool                "json:\"has_more\""
		}{
			Versions: []types.SyncVersion{
				{Version: 3},
			},
			HasMore: false,
		},
	}, nil)
	mc.mocks.stores.SyncVersionMock().EXPECT().Create(ctx, &database.SyncVersion{
		Version: 2,
	}).Return(nil)
	mc.mocks.stores.SyncVersionMock().EXPECT().Create(ctx, &database.SyncVersion{
		Version: 3,
	}).Return(nil)
	dsvs := []database.SyncVersion{
		{RepoType: types.ModelRepo},
		{RepoType: types.DatasetRepo},
	}
	mc.mocks.stores.MultiSyncMock().EXPECT().GetAfterDistinct(ctx, int64(1)).Return(
		dsvs, nil,
	)
	svs := []types.SyncVersion{
		{RepoType: types.ModelRepo},
		{RepoType: types.DatasetRepo},
	}
	// new model mock
	mockedClient.EXPECT().ModelInfo(ctx, svs[0]).Return(&types.Model{
		User: &types.User{Nickname: "nn"},
		Path: "ns/user",
		Tags: []types.RepoTag{{Name: "t1"}},
	}, nil)
	mockedClient.EXPECT().ReadMeData(ctx, svs[0]).Return("readme", nil)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "CSG_ns").Return(database.User{}, sql.ErrNoRows)
	mc.mocks.gitServer.EXPECT().CreateUser(gitserver.CreateUserRequest{
		Nickname: "nn",
		Username: "CSG_ns",
		Email:    "ba63d40b48ed06ce1fba4f23c65c058c",
	}).Return(&gitserver.CreateUserResponse{GitID: 123}, nil)
	mc.mocks.stores.UserMock().EXPECT().Create(ctx, mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, u *database.User, n *database.Namespace) error {
			require.Equal(t, u.NickName, "nn")
			require.Equal(t, u.Username, "CSG_ns")
			require.Equal(t, u.Email, "ba63d40b48ed06ce1fba4f23c65c058c")
			require.Equal(t, u.GitID, int64(123))
			require.Equal(t, n.Path, "CSG_ns")
			require.Equal(t, n.Mirrored, true)
			return nil
		},
	)
	dbrepo := &database.Repository{
		Path:           "CSG_ns/user",
		GitPath:        "models_CSG_ns/user",
		Name:           "user",
		Readme:         "readme",
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		RepositoryType: types.ModelRepo,
		CSGPath:        "ns/user",
	}
	mc.mocks.stores.RepoMock().EXPECT().UpdateOrCreateRepo(ctx, *dbrepo).Return(dbrepo, nil)
	dbrepo.ID = 1
	mc.mocks.stores.TagMock().EXPECT().FindOrCreate(ctx, database.Tag{
		Name: "t1", Scope: types.ModelTagScope,
	}).Return(
		&database.Tag{Name: "t1", ID: 11}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().DeleteAllTags(ctx, int64(1)).Return(nil)
	mc.mocks.stores.RepoMock().EXPECT().BatchCreateRepoTags(ctx, []database.RepositoryTag{
		{RepositoryID: 1, TagID: 11},
	}).Return(nil)
	mc.mocks.stores.RepoMock().EXPECT().DeleteAllFiles(ctx, int64(1)).Return(nil)
	mockedClient.EXPECT().FileList(ctx, svs[0]).Return([]types.File{
		{Name: "foo.go"},
	}, nil)
	mc.mocks.stores.FileMock().EXPECT().BatchCreate(ctx, []database.File{
		{Name: "foo.go", ParentPath: "/", RepositoryID: 1},
	}).Return(nil)
	mc.mocks.stores.ModelMock().EXPECT().CreateIfNotExist(ctx, database.Model{
		RepositoryID: 1,
		Repository:   dbrepo,
	}).Return(nil, nil)

	// new dataset mock
	dbrepo = &database.Repository{
		Path:           "CSG_ns/user",
		GitPath:        "datasets_CSG_ns/user",
		Name:           "user",
		Readme:         "readme",
		Source:         types.OpenCSGSource,
		SyncStatus:     types.SyncStatusPending,
		RepositoryType: types.DatasetRepo,
		CSGPath:        "ns/user",
	}
	mockedClient.EXPECT().DatasetInfo(ctx, svs[1]).Return(&types.Dataset{
		User: types.User{Nickname: "nn"},
		Path: "ns/user",
		Tags: []types.RepoTag{{Name: "t2"}},
	}, nil)
	mockedClient.EXPECT().ReadMeData(ctx, svs[1]).Return("readme", nil)
	mc.mocks.stores.RepoMock().EXPECT().UpdateOrCreateRepo(ctx, *dbrepo).Return(dbrepo, nil)
	dbrepo.ID = 2
	mc.mocks.stores.TagMock().EXPECT().FindOrCreate(ctx, database.Tag{
		Name: "t2", Scope: types.DatasetTagScope,
	}).Return(
		&database.Tag{Name: "t2", ID: 12}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().DeleteAllTags(ctx, int64(2)).Return(nil)
	mc.mocks.stores.RepoMock().EXPECT().BatchCreateRepoTags(ctx, []database.RepositoryTag{
		{RepositoryID: 2, TagID: 12},
	}).Return(nil)
	mc.mocks.stores.RepoMock().EXPECT().DeleteAllFiles(ctx, int64(2)).Return(nil)
	mockedClient.EXPECT().FileList(ctx, svs[1]).Return([]types.File{
		{Name: "foo.go"},
	}, nil)
	mc.mocks.stores.FileMock().EXPECT().BatchCreate(ctx, []database.File{
		{Name: "foo.go", ParentPath: "/", RepositoryID: 2},
	}).Return(nil)
	mc.mocks.stores.DatasetMock().EXPECT().CreateIfNotExist(ctx, database.Dataset{
		RepositoryID: 2,
		Repository:   dbrepo,
	}).Return(nil, nil)

	err := mc.SyncAsClient(context.TODO(), mockedClient)
	require.Nil(t, err)

}
