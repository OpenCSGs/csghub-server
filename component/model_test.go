package component

import (
	"context"
	"testing"
	"time"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestModelComponent_Index(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	filter := &types.RepoFilter{Username: "user"}
	mc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.ModelRepo, "user", filter, 10, 1).Return(
		[]*database.Repository{
			{ID: 1, Name: "r1", Tags: []database.Tag{{Name: "t1"}}},
			{ID: 2, Name: "r2", Tags: []database.Tag{{Name: "t2"}}},
		}, 100, nil,
	)

	mc.mocks.stores.ModelMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2}).Return([]database.Model{
		{RepositoryID: 1, ID: 11, Repository: &database.Repository{}},
		{RepositoryID: 2, ID: 12, Repository: &database.Repository{}},
	}, nil)

	data, total, err := mc.Index(ctx, filter, 10, 1, false)
	require.Nil(t, err)
	require.Equal(t, 100, total)

	require.Equal(t, []*types.Model{
		{
			ID: 11, Name: "r1", Tags: []types.RepoTag{{Name: "t1"}}, RepositoryID: 1,
			Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			},
		},
		{
			ID: 12, Name: "r2", Tags: []types.RepoTag{{Name: "t2"}}, RepositoryID: 2,
			Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			},
		},
	}, data)

}

func TestModelComponent_Create(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	user := database.User{
		Username: "user",
		Email:    "foo@bar.com",
	}
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(user, nil)

	dbrepo := &database.Repository{
		ID:      321,
		User:    user,
		Tags:    []database.Tag{{Name: "t1"}},
		Name:    "n",
		License: "MIT",
	}
	mc.mocks.components.repo.EXPECT().CreateRepo(ctx, types.CreateRepoReq{
		DefaultBranch: "main",
		Readme:        generateReadmeData("MIT"),
		License:       "MIT",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "n",
		RepoType:      types.ModelRepo,
		Username:      "user",
	}).Return(nil, dbrepo, nil)

	mc.mocks.stores.ModelMock().EXPECT().Create(ctx, database.Model{
		Repository:   dbrepo,
		RepositoryID: dbrepo.ID,
		BaseModel:    "base",
	}).Return(&database.Model{
		Repository: dbrepo,
	}, nil)
	mc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   generateReadmeData("MIT"),
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  types.ReadmeFileName,
	}, types.ModelRepo)).Return(nil)
	mc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   spaceGitattributesContent,
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  gitattributesFileName,
	}, types.ModelRepo)).Return(nil)

	model, err := mc.Create(ctx, &types.CreateModelReq{
		BaseModel: "base",
		CreateRepoReq: types.CreateRepoReq{
			DefaultBranch: "main",
			Readme:        "readme",
			Namespace:     "ns",
			Name:          "n",
			License:       "MIT",
			Username:      "user",
		},
	})
	require.Nil(t, err)

	require.Equal(t, &types.Model{
		License: "MIT",
		Name:    "n",
		User: &types.User{
			Username: "user",
			Email:    "foo@bar.com",
		},
		Tags: []types.RepoTag{{Name: "t1"}},
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/.git",
			SSHCloneURL:  "test@127.0.0.1:s/.git",
		},
	}, model)

}

func TestModelComponent_Update(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	req := &types.UpdateModelReq{
		BaseModel:     tea.String("base2"),
		UpdateRepoReq: types.UpdateRepoReq{RepoType: types.ModelRepo},
	}
	mc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(&database.Repository{
		Name: "n",
		ID:   1,
	}, nil)
	mc.mocks.stores.ModelMock().EXPECT().ByRepoID(ctx, int64(1)).Return(&database.Model{
		ID:        2,
		BaseModel: "base",
	}, nil)

	mc.mocks.stores.ModelMock().EXPECT().Update(ctx, database.Model{
		ID:        2,
		BaseModel: "base2",
	}).Return(&database.Model{
		ID:        2,
		BaseModel: "base2",
	}, nil)

	m, err := mc.Update(ctx, req)
	require.Nil(t, err)
	require.Equal(t, &types.Model{
		ID:           2,
		BaseModel:    "base2",
		Name:         "n",
		RepositoryID: 1,
	}, m)
}

func TestModelComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{ID: 1}, nil)
	mc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
	}).Return(nil, nil)
	mc.mocks.stores.ModelMock().EXPECT().Delete(ctx, database.Model{ID: 1}).Return(nil)

	err := mc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)

}

func TestModelComponent_Show(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		ID:           1,
		RepositoryID: 123,
		Repository:   &database.Repository{ID: 123, Name: "n", Path: "foo/bar"},
	}, nil)
	mc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "user", &database.Repository{
		ID:   123,
		Name: "n",
		Path: "foo/bar",
	}).Return(
		&types.UserRepoPermission{CanRead: true, CanAdmin: true}, nil,
	)
	mc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{Path: "ns"}, nil)

	mc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "user", int64(123)).Return(true, nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().GetByRepoIDsAndType(
		ctx, int64(123), mock.Anything,
	).Return([]database.RepositoriesRuntimeFramework{{}}, nil)

	model, err := mc.Show(ctx, "ns", "n", "user", false)
	require.Nil(t, err)
	require.Equal(t, &types.Model{
		ID:                   1,
		Name:                 "n",
		Namespace:            &types.Namespace{Path: "ns"},
		UserLikes:            true,
		RepositoryID:         123,
		CanManage:            true,
		User:                 &types.User{},
		Path:                 "foo/bar",
		SensitiveCheckStatus: "Pending",
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/foo/bar.git",
			SSHCloneURL:  "test@127.0.0.1:s/foo/bar.git",
		},
		EnableInference:  true,
		EnableFinetune:   true,
		EnableEvaluation: true,
		WidgetType:       types.ModelWidgetTypeGeneration,
	}, model)
}

func TestModelComponent_GetServerless(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		ID:           1,
		RepositoryID: 123,
		Repository:   &database.Repository{ID: 123, Name: "n"},
	}, nil)

	mc.mocks.components.repo.EXPECT().AllowReadAccessRepo(
		ctx, &database.Repository{ID: 123, Name: "n"}, "user",
	).Return(true, nil)

	deploy := &database.Deploy{ID: 1}
	mc.mocks.stores.DeployTaskMock().EXPECT().GetServerlessDeployByRepID(ctx, int64(123)).Return(
		deploy, nil,
	)

	mc.mocks.components.repo.EXPECT().GenerateEndpoint(ctx, deploy).Return("ep", "")

	dr, err := mc.GetServerless(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.DeployRepo{
		DeployID:      1,
		ProxyEndpoint: "ep",
		Status:        "Stopped",
	}, dr)

}

func TestModelComponent_SDKModelInfo(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	repo := &database.Repository{
		ID: 123, Name: "n", Path: "p/p",
		User: database.User{Username: "user"},
		Tags: []database.Tag{{Name: "t1"}},
	}
	repo.CreatedAt = time.Date(2024, time.November, 6, 13, 19, 10, 1, time.UTC)
	repo.UpdatedAt = time.Date(2024, time.November, 6, 13, 19, 10, 1, time.UTC)

	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		ID:           1,
		RepositoryID: 123,
		Repository:   repo}, nil)
	mc.mocks.components.repo.EXPECT().AllowReadAccessRepo(
		ctx, repo, "user",
	).Return(true, nil)
	mc.mocks.gitServer.EXPECT().GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		RepoType:  types.ModelRepo,
	}).Return(&types.Commit{ID: "zzz"}, nil)
	file := types.File{Path: "file1.txt", Type: "file", Size: 100, SHA: "sha1"}
	repoFiles := []*types.File{&file}
	mc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, mock.Anything,
	).Return(&types.GetRepoFileTreeResp{Files: repoFiles, Cursor: ""}, nil)
	mc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(123), "user").Return(
		map[types.RepositoryType][]*database.Repository{
			types.SpaceRepo: {
				{Name: "sp"},
			},
		}, nil,
	)

	info, err := mc.SDKModelInfo(ctx, "ns", "n", "main", "user")
	require.Nil(t, err)
	require.Equal(t, &types.SDKModelInfo{
		ID:           "p/p",
		Spaces:       []string{"sp"},
		Author:       "user",
		Sha:          "zzz",
		Siblings:     []types.SDKFile{{Filename: "file1.txt"}},
		Tags:         []string{"t1"},
		CreatedAt:    time.Date(2024, time.November, 6, 13, 19, 10, 1, time.UTC),
		LastModified: time.Date(2024, time.November, 6, 13, 19, 10, 1, time.UTC),
	}, info)

}

func TestModelComponent_Relations(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	repo := &database.Repository{}
	mc.mocks.stores.ModelMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Model{
		RepositoryID: 123,
		Repository:   repo,
	}, nil)
	mc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "user").Return(true, nil)
	mc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(123), "user").Return(
		map[types.RepositoryType][]*database.Repository{
			types.DatasetRepo: {{Name: "d1"}},
			types.CodeRepo:    {{Name: "c1"}},
			types.PromptRepo:  {{Name: "p1"}},
			types.SpaceRepo:   {{Path: "sp"}},
		}, nil,
	)
	mc.mocks.components.space.EXPECT().ListByPath(ctx, []string{"sp"}).Return([]*types.Space{
		{Name: "s1"},
	}, nil)

	rels, err := mc.Relations(ctx, "ns", "n", "user")
	require.Nil(t, err)
	require.Equal(t, &types.Relations{
		Datasets: []*types.Dataset{{Name: "d1"}},
		Codes:    []*types.Code{{Name: "c1"}},
		Prompts:  []*types.PromptRes{{Name: "p1"}},
		Spaces:   []*types.Space{{Name: "s1"}},
	}, rels)
}

func TestModelComponent_SetRelationDatasets(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}).Return(&types.File{}, nil)
	mc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Username:  "user",
		Message:   "update dataset tags",
		Branch:    "main",
		Content:   "LS0tCmRhdGFzZXRzOgogICAgLSBkMQoKLS0tCg==",
		Namespace: "ns",
		Name:      "n",
		FilePath:  "README.md",
		RepoType:  types.ModelRepo,
	}).Return(nil)
	err := mc.SetRelationDatasets(ctx, types.RelationDatasets{
		Datasets:    []string{"d1"},
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
	})
	require.Nil(t, err)
}

func TestModelComponent_AddRelationDataset(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}).Return(&types.File{}, nil)
	mc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Username:  "user",
		Message:   "add relation dataset",
		Branch:    "main",
		Content:   "LS0tCmRhdGFzZXRzOgogICAgLSBkMQoKLS0tCg==",
		Namespace: "ns",
		Name:      "n",
		FilePath:  "README.md",
		RepoType:  types.ModelRepo,
	}).Return(nil)
	err := mc.AddRelationDataset(ctx, types.RelationDataset{
		Dataset:     "d1",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
	})
	require.Nil(t, err)
}

func TestModelComponent_DeleteRelationDataset(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		RoleMask: "admin",
	}, nil)
	mc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{}, nil,
	)
	mc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}).Return(&types.File{
		Content: "LS0tCiBkYXRhc2V0czoKICAgLSBkczE=",
	}, nil)
	mc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Username:  "user",
		Message:   "delete relation dataset",
		Branch:    "main",
		Content:   "LS0tCmRhdGFzZXRzOgogICAgLSBkczEKCi0tLQ==",
		Namespace: "ns",
		Name:      "n",
		FilePath:  "README.md",
		RepoType:  types.ModelRepo,
	}).Return(nil)
	err := mc.DelRelationDataset(ctx, types.RelationDataset{
		Dataset:     "d1",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "user",
	})
	require.Nil(t, err)
}

func TestModelComponent_ListModelsByRuntimeFrameworkID(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().ListByRuntimeFrameworkID(ctx, int64(123), 1).Return(
		[]database.RepositoriesRuntimeFramework{
			{RepoID: 1}, {RepoID: 2},
		}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().ListRepoPublicToUserByRepoIDs(ctx, types.ModelRepo, int64(1), "", "", 10, 1, []int64{1, 2}).Return([]*database.Repository{
		{ID: 1, Name: "r1"},
	}, 100, nil)

	models, total, err := mc.ListModelsByRuntimeFrameworkID(ctx, "user", 10, 1, 123, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Model{
		{Name: "r1", RepositoryID: 1},
	}, models)

}

func TestModelComponent_SetRuntimeFrameworkModes(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.RuntimeFramework{}, nil,
	)
	mc.mocks.stores.ModelMock().EXPECT().ListByPath(ctx, []string{"a", "b"}).Return(
		[]database.Model{
			{RepositoryID: 1, Repository: &database.Repository{ID: 1, Path: "m1/foo"}},
			{RepositoryID: 2, Repository: &database.Repository{ID: 2, Path: "m2/foo"}},
		}, nil,
	)
	rftags := []*database.Tag{{Name: "t1"}, {Name: "t2"}}
	filter := &types.TagFilter{
		Categories: []string{"runtime_framework", "resource"},
		Scopes:     []types.TagScope{types.ModelTagScope},
	}
	mc.mocks.stores.TagMock().EXPECT().AllTags(
		ctx, filter,
	).Return(rftags, nil)

	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().GetByIDsAndType(
		ctx, int64(1), int64(1), 1,
	).Return([]database.RepositoriesRuntimeFramework{}, nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().GetByIDsAndType(
		ctx, int64(1), int64(2), 1,
	).Return([]database.RepositoriesRuntimeFramework{{}}, nil)
	mc.mocks.components.repo.EXPECT().IsAdminRole(mock.Anything).Return(true)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Add(ctx, int64(1), int64(1), 1).Return(nil)
	mc.mocks.components.runtimeArchitecture.EXPECT().AddRuntimeFrameworkTag(
		ctx, rftags, int64(1), int64(1),
	).Return(nil)
	mc.mocks.components.runtimeArchitecture.EXPECT().AddResourceTag(ctx, rftags, "foo", int64(1)).Return(nil)
	f, err := mc.SetRuntimeFrameworkModes(ctx, "user", 1, 1, []string{"a", "b"})
	require.Nil(t, err)
	require.Empty(t, f)

}

func TestModelComponent_DeleteRuntimeFrameworkModes(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.ModelMock().EXPECT().ListByPath(ctx, []string{"a", "b"}).Return(
		[]database.Model{
			{RepositoryID: 1, Repository: &database.Repository{ID: 1, Path: "m1/foo"}},
			{RepositoryID: 2, Repository: &database.Repository{ID: 2, Path: "m2/foo"}},
		}, nil,
	)
	mc.mocks.components.repo.EXPECT().IsAdminRole(mock.Anything).Return(true)
	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)

	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Delete(ctx, int64(123), int64(1), 1).Return(nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().Delete(ctx, int64(123), int64(2), 1).Return(nil)

	f, err := mc.DeleteRuntimeFrameworkModes(ctx, "user", 1, 123, []string{"a", "b"})
	require.Nil(t, err)
	require.Empty(t, f)
}

func TestModelComponent_ListModelsOfRuntimeFrameworks(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	mc.mocks.stores.RepoRuntimeFrameworkMock().EXPECT().ListRepoIDsByType(ctx, 1).Return(
		[]database.RepositoriesRuntimeFramework{
			{RepoID: 123},
		}, nil,
	)
	mc.mocks.stores.RepoMock().EXPECT().ListRepoPublicToUserByRepoIDs(ctx, types.ModelRepo, int64(1), "s", "ss", 10, 1, []int64{123}).Return([]*database.Repository{
		{Name: "r1", Path: "foo"},
	}, 100, nil)
	data, total, err := mc.ListModelsOfRuntimeFrameworks(ctx, "user", "s", "ss", 10, 1, 1)
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Model{
		{
			Name:            "r1",
			Path:            "foo",
			EnableInference: true,
		},
	}, data)

}

func TestModelComponent_OrgModels(t *testing.T) {
	ctx := context.TODO()
	mc := initializeTestModelComponent(ctx, t)

	mc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "user").Return(membership.RoleAdmin, nil)
	mc.mocks.stores.ModelMock().EXPECT().ByOrgPath(ctx, "ns", 10, 1, false).Return([]database.Model{
		{RepositoryID: 1, Repository: &database.Repository{ID: 1, Path: "foo", Name: "r1"}},
	}, 100, nil)
	data, total, err := mc.OrgModels(ctx, &types.OrgModelsReq{
		Namespace:   "ns",
		CurrentUser: "user",
		PageOpts: types.PageOpts{
			Page:     1,
			PageSize: 10,
		},
	})
	require.Nil(t, err)
	require.Equal(t, 100, total)
	require.Equal(t, []types.Model{
		{
			Name:         "r1",
			Path:         "foo",
			RepositoryID: 1,
		},
	}, data)

}
