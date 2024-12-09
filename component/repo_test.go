package component

import (
	"context"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
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

// func TestRepoComponent_Tree(t *testing.T) {
// 	{
// 		t.Run("can read self-owned", func(t *testing.T) {
// 			ctx := context.TODO()
// 			repoComp := initializeTestRepoComponent(ctx, t)

// 			user := database.User{}
// 			user.Username = "user_name"
// 			repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

// 			ns := database.Namespace{}
// 			ns.NamespaceType = "user"
// 			ns.Path = "user_name"
// 			repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

// 			repo := &database.Repository{
// 				Private: true,
// 				User:    user,
// 				Path:    fmt.Sprintf("%s/%s", ns.Path, "repo_name"),
// 				Source:  types.LocalSource,
// 			}
// 			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, types.ModelRepo, ns.Path, repo.Name).Return(repo, nil)

// 			tree := []*types.File{}
// 			repoComp.mocks.gitServer.EXPECT().GetRepoFileTree(mock.Anything, mock.Anything).Return(tree, nil)

// 			actualTree, err := repoComp.Tree(context.Background(), &types.GetFileReq{
// 				Namespace:   ns.Path,
// 				Name:        repo.Name,
// 				Path:        "",
// 				RepoType:    types.ModelRepo,
// 				CurrentUser: user.Username,
// 			})
// 			require.Nil(t, err)
// 			require.Equal(t, tree, actualTree)

// 		})

// 		t.Run("forbidden anoymous user to read private repo", func(t *testing.T) {
// 			ctx := context.TODO()
// 			repoComp := initializeTestRepoComponent(ctx, t)

// 			repoComp.mocks.stores.RepoMock().EXPECT().FindByPath(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&database.Repository{
// 				// private repo don't allow read from other user
// 				Private: true,
// 			}, nil)

// 			actualTree, err := repoComp.Tree(context.Background(), &types.GetFileReq{})
// 			require.Nil(t, actualTree)
// 			require.Equal(t, err, ErrForbidden)

// 		})
// 	}

// }
func TestRepoComponent_checkCurrentUserPermission(t *testing.T) {

	t.Run("can read self-owned", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})

	t.Run("can not read other's", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name_other"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("can not read org's if not org member", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		//user not belongs to org
		repoComp.mocks.userSvcClient.EXPECT().GetMemberRole(mock.Anything, ns.Path, user.Username).Return(membership.RoleUnknown, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("can read org's as org member", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		//user is read-only member of the org
		repoComp.mocks.userSvcClient.EXPECT().GetMemberRole(mock.Anything, ns.Path, user.Username).Return(membership.RoleRead, nil)

		//can read
		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)
		//can't write
		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.False(t, yes)
		require.NoError(t, err)
		//can't admin
		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.False(t, yes)
		require.NoError(t, err)
	})

	t.Run("admin read org's", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "organization"
		ns.Path = "org_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name_admin"
		user.RoleMask = "admin"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})

	t.Run("admin read other's", func(t *testing.T) {
		repoComp := initializeTestRepoComponent(context.TODO(), t)
		ns := database.Namespace{}
		ns.NamespaceType = "user"
		ns.Path = "user_name"
		repoComp.mocks.stores.NamespaceMock().EXPECT().FindByPath(mock.Anything, ns.Path).Return(ns, nil)

		user := database.User{}
		user.Username = "user_name_admin"
		user.RoleMask = "admin"
		repoComp.mocks.stores.UserMock().EXPECT().FindByUsername(mock.Anything, user.Username).Return(user, nil)

		yes, err := repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleRead)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleWrite)
		require.True(t, yes)
		require.NoError(t, err)

		yes, err = repoComp.CheckCurrentUserPermission(context.Background(), user.Username, ns.Path, membership.RoleAdmin)
		require.True(t, yes)
		require.NoError(t, err)
	})
}
