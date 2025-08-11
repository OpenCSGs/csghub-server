package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestCodeComponent_Create(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	req := &types.CreateCodeReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:  "user",
			Namespace: "ns",
			Name:      "n",
			License:   "l",
			Readme:    "r",
		},
	}
	dbrepo := &database.Repository{
		ID:   1,
		User: database.User{Username: "user"},
		Tags: []database.Tag{{Name: "t1"}},
	}
	crq := req.CreateRepoReq
	crq.Nickname = "n"
	crq.Readme = generateReadmeData(req.License)
	crq.RepoType = types.CodeRepo
	crq.DefaultBranch = "main"
	cc.mocks.components.repo.EXPECT().CreateRepo(ctx, crq).Return(
		nil, dbrepo, nil,
	)
	cc.mocks.stores.CodeMock().EXPECT().Create(ctx, database.Code{
		Repository:   dbrepo,
		RepositoryID: 1,
	}).Return(&database.Code{
		RepositoryID: 1,
		Repository:   dbrepo,
	}, nil)
	cc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   crq.Readme,
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  types.ReadmeFileName,
	}, types.CodeRepo)).Return(nil)
	cc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   codeGitattributesContent,
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  gitattributesFileName,
	}, types.CodeRepo)).Return(nil)

	resp, err := cc.Create(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Code{
		RepositoryID: 1,
		User: types.User{
			Username: "user",
		},
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		Tags: []types.RepoTag{{Name: "t1"}},
	}, resp)
}

func TestCodeComponent_Index(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	filter := &types.RepoFilter{Username: "user"}
	repos := []*database.Repository{
		{ID: 1, Name: "r1", Tags: []database.Tag{{Name: "t1"}}},
		{ID: 2, Name: "r2"},
	}
	cc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.CodeRepo, "user", filter, 10, 1).Return(
		repos, 100, nil,
	)
	cc.mocks.stores.CodeMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2}).Return([]database.Code{
		{ID: 11, RepositoryID: 2},
		{ID: 12, RepositoryID: 1},
	}, nil)

	data, total, err := cc.Index(ctx, filter, 10, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Code{
		{ID: 12, RepositoryID: 1, Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}},
		{ID: 11, RepositoryID: 2, Name: "r2"},
	}, data)
}

func TestCodeComponent_Update(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	req := &types.UpdateCodeReq{
		UpdateRepoReq: types.UpdateRepoReq{
			RepoType: types.CodeRepo,
		},
	}
	dbrepo := &database.Repository{Name: "name"}
	cc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(dbrepo, nil)
	cc.mocks.stores.CodeMock().EXPECT().ByRepoID(ctx, dbrepo.ID).Return(&database.Code{ID: 1}, nil)
	cc.mocks.stores.CodeMock().EXPECT().Update(ctx, database.Code{
		ID: 1,
	}).Return(nil)

	data, err := cc.Update(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Code{ID: 1, Name: "name"}, data)

}

func TestCodeComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	cc.mocks.stores.CodeMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Code{}, nil)
	cc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.CodeRepo,
	}).Return(nil, nil)
	cc.mocks.stores.CodeMock().EXPECT().Delete(ctx, database.Code{}).Return(nil)

	err := cc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)
}

func TestCodeComponent_Show(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	code := &database.Code{ID: 1, Repository: &database.Repository{
		ID: 11, Name: "name", User: database.User{Username: "user"},
	}}
	cc.mocks.stores.CodeMock().EXPECT().FindByPath(ctx, "ns", "n").Return(code, nil)
	cc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", code.Repository).Return(
		&types.UserRepoPermission{CanRead: true, CanAdmin: true}, nil,
	)
	cc.mocks.components.repo.EXPECT().GetMirrorTaskStatusAndSyncStatus(code.Repository).Return(
		types.MirrorRepoSyncStart, types.SyncStatusInProgress,
	)
	cc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(11)).Return(true, nil)
	cc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil)

	data, err := cc.Show(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.Code{
		ID: 1,
		Repository: types.Repository{
			HTTPCloneURL: "/s/.git",
			SSHCloneURL:  ":s/.git",
		},
		RepositoryID:         11,
		Namespace:            &types.Namespace{},
		Name:                 "name",
		User:                 types.User{Username: "user"},
		CanManage:            true,
		UserLikes:            true,
		SensitiveCheckStatus: "Pending",
		MirrorTaskStatus:     types.MirrorRepoSyncStart,
		SyncStatus:           types.SyncStatusInProgress,
	}, data)
}

func TestCodeComponent_Relations(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	cc.mocks.stores.CodeMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Code{
		Repository:   &database.Repository{},
		RepositoryID: 1,
	}, nil)
	cc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, &database.Repository{}, "user").Return(true, nil)
	cc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(1), "user").Return(
		map[types.RepositoryType][]*database.Repository{
			types.ModelRepo: {
				{Name: "r1"},
			},
		}, nil,
	)

	data, err := cc.Relations(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.Relations{
		Models: []*types.Model{{Name: "r1"}},
	}, data)

}

func TestCodeComponent_OrgCodes(t *testing.T) {
	ctx := context.TODO()
	cc := initializeTestCodeComponent(ctx, t)

	cc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "user").Return(membership.RoleAdmin, nil)
	cc.mocks.stores.CodeMock().EXPECT().ByOrgPath(ctx, "ns", 10, 1, false).Return(
		[]database.Code{{
			ID: 1, Repository: &database.Repository{Name: "repo"},
			RepositoryID: 11,
		}}, 100, nil,
	)

	data, total, err := cc.OrgCodes(ctx, &types.OrgDatasetsReq{
		Namespace: "ns", CurrentUser: "user",
		PageOpts: types.PageOpts{Page: 1, PageSize: 10},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Code{
		{ID: 1, Name: "repo", RepositoryID: 11},
	}, data)

}
