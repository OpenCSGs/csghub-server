package component

import (
	"context"
	"testing"

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
	dc.mocks.components.repo.EXPECT().CreateRepo(ctx, rq).Return(&gitserver.CreateRepoResp{}, &database.Repository{}, nil)
	dc.mocks.stores.DatasetMock().EXPECT().Create(ctx, database.Dataset{
		Repository: &database.Repository{},
	}).Return(&database.Dataset{
		Repository: &database.Repository{
			Tags: []database.Tag{{Name: "t1"}},
		},
	}, nil)
	dc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(
		&types.CreateFileParams{
			Username:  "user",
			Message:   types.InitCommitMessage,
			Branch:    "main",
			Content:   "\n---\nlicense: \n---\n\t",
			Namespace: "ns",
			Name:      "n",
			FilePath:  types.ReadmeFileName,
		}, types.DatasetRepo),
	).Return(nil)
	dc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(
		&types.CreateFileParams{
			Username:  "user",
			Message:   types.InitCommitMessage,
			Branch:    "main",
			Content:   types.DatasetGitattributesContent,
			Namespace: "ns",
			Name:      "n",
			FilePath:  gitattributesFileName,
		}, types.DatasetRepo),
	).Return(nil)

	resp, err := dc.Create(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Dataset{
		User: types.User{Username: "user"},
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		Tags: []types.RepoTag{{Name: "t1"}},
	}, resp)

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

	data, total, err := dc.Index(ctx, filter, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Dataset{
		{ID: 12, RepositoryID: 1, Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		}, User: types.User{Username: "user1"},
			Tags: []types.RepoTag{{Name: "t1"}},
		},
		{ID: 11, RepositoryID: 2, Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		}, User: types.User{Username: "user2"}},
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

	dc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Dataset{}, nil)
	dc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.DatasetRepo,
	}).Return(&database.Repository{}, nil)
	dc.mocks.stores.DatasetMock().EXPECT().Delete(ctx, database.Dataset{}).Return(nil)

	err := dc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)

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

	d, err := dc.Show(ctx, "ns", "n", "user")
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
		User:      types.User{Username: "user"},
		UserLikes: true,
		Namespace: &types.Namespace{},
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
