package component

import (
	"context"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoComponent_CreateRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	gitrepo := &gitserver.CreateRepoResp{
		GitPath:       "gp",
		DefaultBranch: "main",
		HttpCloneURL:  "http",
		SshCloneURL:   "ssh",
	}
	repo.mocks.gitServer.EXPECT().CreateRepo(ctx, gitserver.CreateRepoReq{
		Username:      "user",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "n",
		License:       "MIT",
		DefaultBranch: "main",
		Readme:        "rr",
		Private:       true,
		RepoType:      types.ModelRepo,
	}).Return(gitrepo, nil)

	dbrepo := &database.Repository{
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "gp",
		Name:           "n",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
		HTTPCloneURL:   "http",
		SSHCloneURL:    "ssh",
	}
	repo.mocks.stores.RepoMock().EXPECT().CreateRepo(ctx, *dbrepo).Return(dbrepo, nil)

	r1, r2, err := repo.CreateRepo(ctx, types.CreateRepoReq{
		Username:      "user",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "nn",
		License:       "MIT",
		DefaultBranch: "main",
		Readme:        "rr",
		Private:       true,
		RepoType:      types.ModelRepo,
		Description:   "desc",
	})
	require.Nil(t, err)
	require.Equal(t, gitrepo, r1)
	dbrepo.User = dbuser
	require.Equal(t, dbrepo, r2)

}

func TestRepoComponent_UpdateRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbrepo := &database.Repository{
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "gp",
		Name:           "n",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
		HTTPCloneURL:   "http",
		SSHCloneURL:    "ssh",
	}
	repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "ns", string(types.ModelRepo), "n").Return(dbrepo, nil)
	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	gitrepo := &gitserver.CreateRepoResp{
		GitPath:       "gp",
		DefaultBranch: "main",
		HttpCloneURL:  "http",
		SshCloneURL:   "ssh",
	}
	repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, gitserver.UpdateRepoReq{
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "nn2",
		Description:   "desc2",
		DefaultBranch: "main",
		Private:       true,
		RepoType:      types.ModelRepo,
	}).Return(gitrepo, nil)

	dbrepo.Nickname = "nn2"
	dbrepo.Description = "desc2"
	repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, *dbrepo).Return(dbrepo, nil)

	r1, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
		Username:    "user",
		Namespace:   "ns",
		Name:        "n",
		RepoType:    types.ModelRepo,
		Nickname:    tea.String("nn2"),
		Description: tea.String("desc2"),
		Private:     tea.Bool(true),
	})
	require.Nil(t, err)
	require.Equal(t, dbrepo, r1)

}

func TestRepoComponent_DeleteRepo(t *testing.T) {
	ctx := context.TODO()
	repo := initializeTestRepoComponent(ctx, t)

	dbrepo := &database.Repository{
		ID:             1,
		UserID:         123,
		Path:           "ns/n",
		GitPath:        "gp",
		Name:           "n",
		Nickname:       "nn",
		Description:    "desc",
		Private:        true,
		License:        "MIT",
		DefaultBranch:  "main",
		RepositoryType: types.ModelRepo,
		HTTPCloneURL:   "http",
		SSHCloneURL:    "ssh",
	}
	repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "ns", string(types.ModelRepo), "n").Return(dbrepo, nil)
	repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil)
	dbuser := database.User{
		ID:       123,
		RoleMask: "admin",
		Email:    "foo@bar.com",
	}
	repo.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(dbuser, nil)
	repo.mocks.stores.RepoMock().EXPECT().CleanRelationsByRepoID(ctx, dbrepo.ID).Return(nil)

	repo.mocks.gitServer.EXPECT().DeleteRepo(ctx, gitserver.DeleteRepoReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
	}).Return(nil)

	repo.mocks.stores.RepoMock().EXPECT().DeleteRepo(ctx, *dbrepo).Return(nil)

	r1, err := repo.DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.ModelRepo,
	})
	require.Nil(t, err)
	require.Equal(t, dbrepo, r1)

}

// func TestRepoComponent_LastCommit(t *testing.T) {
// 	t.Run("can read self-owned", func(t *testing.T) {
// 		mockrs := mockdb.NewMockRepoStore(t)
// 		mockrs.EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{}, nil)

// 		mockns := mockdb.NewMockNamespaceStore(t)
// 		ns := database.Namespace{}
// 		ns.NamespaceType = "user"
// 		ns.Path = "user_name"
// 		mockns.EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

// 		mockus := mockdb.NewMockUserStore(t)
// 		user := database.User{}
// 		user.Username = "user_name"
// 		mockus.EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

// 		mockGit := mockgit.NewMockGitServer(t)
// 		commit := &types.Commit{}
// 		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(commit, nil)

// 		repoComp := &repoComponentImpl{
// 			user:      mockus,
// 			namespace: mockns,
// 			repo:      mockrs,
// 			git:       mockGit,
// 		}

// 		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
// 		require.True(t, yes)
// 		require.NoError(t, err)

// 		actualCommit, err := repoComp.LastCommit(context.Background(), &types.GetCommitsReq{})
// 		require.NoError(t, err)
// 		require.Equal(t, commit, actualCommit)

// 	})

// 	t.Run("forbidden to read other's", func(t *testing.T) {
// 		mockrs := mockdb.NewMockRepoStore(t)
// 		mockrs.EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
// 			// private repo don't allow read from other user
// 			Private: true,
// 		}, nil)

// 		mockns := mockdb.NewMockNamespaceStore(t)
// 		ns := database.Namespace{}
// 		ns.NamespaceType = "user"
// 		ns.Path = "user_name"
// 		mockns.EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

// 		mockus := mockdb.NewMockUserStore(t)
// 		user := database.User{}
// 		user.Username = "user_name_other"
// 		mockus.EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

// 		repoComp := &repoComponentImpl{
// 			user:      mockus,
// 			namespace: mockns,
// 			repo:      mockrs,
// 		}

// 		yes, err := repoComp.checkCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
// 		require.False(t, yes)
// 		require.NoError(t, err)

// 		actualCommit, err := repoComp.LastCommit(context.Background(), &types.GetCommitsReq{})
// 		require.Nil(t, actualCommit)
// 		require.Equal(t, err, ErrForbidden)

// 	})
// }
