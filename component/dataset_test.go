package component

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDatasetCompnent_Create(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	req := &types.CreateDatasetReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:  "user",
			Namespace: "ns",
			Name:      "n",
		},
	}
	dc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(
		database.Namespace{}, nil,
	)

	dc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
		Username: "user",
	}, nil)

	rq := req.CreateRepoReq
	rq.RepoType = types.DatasetRepo
	rq.Readme = "\n---\nlicense: \n---\n\t"
	rq.DefaultBranch = "main"
	rq.Nickname = "n"
	rq.CommitFiles = []types.CommitFile{
		{
			Content: "\n---\nlicense: \n---\n\t",
			Path:    types.ReadmeFileName,
		},
		{
			Content: types.DatasetGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}

	dc.mocks.components.repo.EXPECT().CreateRepo(ctx, rq).Return(&gitserver.CreateRepoResp{}, &database.Repository{
		Tags: []database.Tag{{Name: "t1"}},
		User: database.User{UUID: "user-uuid"},
	}, nil)

	dc.mocks.stores.DatasetMock().EXPECT().CreateAndUpdateRepoPath(ctx, mock.Anything, "ns/n").RunAndReturn(
		func(ctx context.Context, ds database.Dataset, _ string) (*database.Dataset, error) {
			require.NotNil(t, ds.Repository)
			require.Equal(t, "user-uuid", ds.Repository.User.UUID)
			require.Len(t, ds.Repository.Tags, 1)
			require.Equal(t, "t1", ds.Repository.Tags[0].Name)
			return &database.Dataset{
				Repository: &database.Repository{
					Tags: []database.Tag{{Name: "t1"}},
					User: database.User{UUID: "user-uuid"},
					Path: "ns/n",
				},
			}, nil
		},
	)

	var wg sync.WaitGroup
	wg.Add(1)
	dc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.DatasetRepo &&
				req.Operation == types.OperationCreate &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()

	resp, err := dc.Create(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		User: types.User{Username: "user"},
		Repository: types.Repository{
			HTTPCloneURL: "/s/ns/n.git",
			SSHCloneURL:  ":s/ns/n.git",
		},
		Tags: []types.RepoTag{{Name: "t1"}},
		Path: "ns/n",
		URL:  "ns/n",
	}, resp)
	wg.Wait()
}

func TestDatasetCompnent_Index(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	filter := &types.RepoFilter{Username: "user"}
	dc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.DatasetRepo, "user", filter, 10, 1).Return(
		[]*database.Repository{
			{ID: 1, Tags: []database.Tag{{Name: "t1"}}},
			{ID: 2},
		}, 100, nil,
	)
	dc.mocks.stores.DatasetMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2}).Return([]database.Dataset{
		{
			ID: 11, RepositoryID: 2, Repository: &database.Repository{
				User: database.User{Username: "user2"},
			},
		},
		{
			ID: 12, RepositoryID: 1, Repository: &database.Repository{
				User: database.User{Username: "user1"},
			},
		},
	}, nil)

	data, total, err := dc.Index(ctx, filter, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []*types.Dataset{
		{ID: 12, RepositoryID: 1, Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		}, User: types.User{Username: "user1"},
			Tags: []types.RepoTag{{Name: "t1"}},
		},
		{ID: 11, RepositoryID: 2, Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		}, User: types.User{Username: "user2"},
		},
	}, data)

}

func TestDatasetCompnent_Update(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	req := &types.UpdateDatasetReq{UpdateRepoReq: types.UpdateRepoReq{
		RepoType: types.DatasetRepo,
	}}
	dc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(
		&database.Repository{ID: 1, Name: "repo"}, nil,
	)
	dc.mocks.stores.DatasetMock().EXPECT().ByRepoID(ctx, int64(1)).Return(
		&database.Dataset{ID: 2}, nil,
	)
	dc.mocks.stores.DatasetMock().EXPECT().Update(ctx, database.Dataset{ID: 2}).Return(nil)

	d, err := dc.Update(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		ID:           2,
		RepositoryID: 1,
		Name:         "repo",
	}, d)
}

func TestDatasetCompnent_Delete(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	mockDataset := &database.Dataset{
		ID: 123,
	}
	mockRepo := &database.Repository{
		User: database.User{
			UUID: "user-uuid",
		},
		Path: "ns/n",
	}

	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockDataset, nil)
	dc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.DatasetRepo,
	}).Return(mockRepo, nil)
	dc.mocks.stores.DatasetMock().EXPECT().Delete(ctx, *mockDataset).Return(nil)
	var wg sync.WaitGroup
	wg.Add(1)
	dc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.DatasetRepo &&
				req.Operation == types.OperationDelete &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()

	err := dc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)
	wg.Wait()
}

func TestDatasetCompnent_Show(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	dataset := &database.Dataset{
		ID: 1,
		Repository: &database.Repository{
			ID:   2,
			Name: "n",
			Tags: []database.Tag{{Name: "t1"}},
			User: database.User{
				Username: "user",
			},
		},
	}
	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	dc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dataset.Repository).Return(&types.UserRepoPermission{CanRead: true}, nil)
	dc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)
	dc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(2)).Return(true, nil)
	dc.mocks.components.repo.EXPECT().GetMirrorTaskStatusAndSyncStatus(dataset.Repository).Return(
		types.MirrorRepoSyncStart, types.SyncStatusInProgress,
	)
	/*dc.mocks.stores.RecomMock().EXPECT().FindByRepoIDs(ctx, []int64{2}).Return([]*database.RecomRepoScore{
		{ID: 1, RepositoryID: 2, WeightName: database.RecomWeightTotal, Score: 100},
		{ID: 2, RepositoryID: 2, WeightName: database.RecomWeightDownloads, Score: 40},
		{ID: 3, RepositoryID: 2, WeightName: database.RecomWeightFreshness, Score: 50},
		{ID: 4, RepositoryID: 2, WeightName: database.RecomWeightQuality, Score: 80},
	}, nil)*/
	d, err := dc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		ID:           1,
		Name:         "n",
		RepositoryID: 2,
		Tags:         []types.RepoTag{{Name: "t1"}},
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		User:             types.User{Username: "user"},
		UserLikes:        true,
		Namespace:        &types.Namespace{},
		MirrorTaskStatus: types.MirrorRepoSyncStart,
		SyncStatus:       types.SyncStatusInProgress,
		/*Scores: []types.WeightScore{{
			WeightName: string(database.RecomWeightTotal),
			Score:      100,
		}, {
			WeightName: string(database.RecomWeightDownloads),
			Score:      40,
		}, {
			WeightName: string(database.RecomWeightFreshness),
			Score:      50,
		}, {
			WeightName: string(database.RecomWeightQuality),
			Score:      80,
		},
		},*/
	}, d)

}

func TestDatasetCompnent_Show_Mirror(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	dataset := &database.Dataset{
		ID: 1,
		Repository: &database.Repository{
			ID:   2,
			Name: "n",
			Tags: []database.Tag{{Name: "t1"}},
			User: database.User{
				Username: "user",
			},
			Mirror: database.Mirror{
				ID:     1,
				Status: types.MirrorLfsSyncFinished,
			},
		},
	}
	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	dc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dataset.Repository).Return(&types.UserRepoPermission{CanRead: true}, nil)
	dc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)
	dc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(2)).Return(true, nil)

	dc.mocks.components.repo.EXPECT().GetMirrorTaskStatusAndSyncStatus(dataset.Repository).Return(
		"", types.SyncStatusCompleted,
	)
	d, err := dc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		ID:           1,
		Name:         "n",
		RepositoryID: 2,
		Tags:         []types.RepoTag{{Name: "t1"}},
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		User:       types.User{Username: "user"},
		UserLikes:  true,
		Namespace:  &types.Namespace{},
		SyncStatus: types.SyncStatusCompleted,
	}, d)
}

func TestDatasetCompnent_Show_Repository(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	dataset := &database.Dataset{
		ID: 1,
		Repository: &database.Repository{
			ID:   2,
			Name: "n",
			Tags: []database.Tag{{Name: "t1"}},
			User: database.User{
				Username: "user",
			},
			Mirror: database.Mirror{
				ID: 0,
			},
			SyncStatus: types.SyncStatusPending,
		},
	}
	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	dc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", dataset.Repository).Return(&types.UserRepoPermission{CanRead: true}, nil)
	dc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)
	dc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(2)).Return(true, nil)
	dc.mocks.components.repo.EXPECT().GetMirrorTaskStatusAndSyncStatus(dataset.Repository).Return(
		"", types.SyncStatusPending,
	)

	d, err := dc.Show(ctx, "ns", "n", "user", false, false)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		ID:           1,
		Name:         "n",
		RepositoryID: 2,
		Tags:         []types.RepoTag{{Name: "t1"}},
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		User:       types.User{Username: "user"},
		UserLikes:  true,
		Namespace:  &types.Namespace{},
		SyncStatus: types.SyncStatusPending,
	}, d)
}

func TestDatasetCompnent_Relations(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	dataset := &database.Dataset{
		Repository:   &database.Repository{},
		RepositoryID: 1,
	}
	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	dc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(1), "user").Return(
		map[types.RepositoryType][]*database.Repository{
			types.ModelRepo: {
				{Name: "n"},
			},
		}, nil,
	)
	dc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, dataset.Repository, "user").Return(true, nil)

	rs, err := dc.Relations(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.Relations{
		Models: []*types.Model{{Name: "n"}},
	}, rs)

}

func TestDatasetCompnent_OrgDatasets(t *testing.T) {
	ctx := context.TODO()
	dc := initializeTestDatasetComponent(ctx, t)

	dc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "user").Return(membership.RoleAdmin, nil)
	dc.mocks.stores.DatasetMock().EXPECT().ByOrgPath(ctx, "ns", 10, 1, false).Return(
		[]database.Dataset{
			{ID: 1, Repository: &database.Repository{Name: "repo"}},
		}, 100, nil,
	)

	data, total, err := dc.OrgDatasets(ctx, &types.OrgDatasetsReq{
		Namespace:   "ns",
		CurrentUser: "user",
		PageOpts:    types.PageOpts{Page: 1, PageSize: 10},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Dataset{
		{ID: 1, Name: "repo"},
	}, data)

}
