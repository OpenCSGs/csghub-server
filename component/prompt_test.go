package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestPromptComponent_CheckPermission(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	req := types.PromptReq{
		Path:        "p",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	}

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, errors.New("")).Once()
	_, err := pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "namespace does not exist")

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{}, errors.New("")).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user does not exist")

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "zzz",
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "zzz",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "uuu",
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "vvv",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user do not have permission")

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.UserNamespace,
		Path:          "uuu",
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo",
		Username: "uuu",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "super-admin",
	}, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)

	// no write role
	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "person",
	}, nil).Once()
	pc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "foo", "ns", membership.RoleWrite).Return(false, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Contains(t, err.Error(), "user do not have permission")

	// has write role
	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "person",
	}, nil).Once()
	pc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "foo", "ns", membership.RoleWrite).Return(true, nil).Once()
	_, err = pc.checkPromptRepoPermission(ctx, req)
	require.Nil(t, err)
}

func TestPromptComponent_CreatePrompt(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	for _, exist := range []bool{true, false} {
		t.Run(fmt.Sprintf("file exist: %v", exist), func(t *testing.T) {

			pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
				NamespaceType: database.OrgNamespace,
			}, nil).Once()
			pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
				RoleMask: "foo-admin",
				Email:    "foo@bar.com",
			}, nil).Once()
			if exist {
				pc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", nil).Once()

				_, err := pc.CreatePrompt(ctx, types.PromptReq{
					Namespace:   "ns",
					Name:        "n",
					CurrentUser: "foo",
					Path:        "p",
				}, &types.CreatePromptReq{
					Prompt: types.Prompt{Title: "TEST", Content: "test"},
				})
				require.NotNil(t, err)
				return
			} else {
				pc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", errors.New("")).Once()
			}

			pc.mocks.components.repo.EXPECT().CreateFile(ctx, &types.CreateFileReq{
				Namespace:   "ns",
				Name:        "n",
				Branch:      types.MainBranch,
				FilePath:    "TEST.jsonl",
				Content:     "eyJ0aXRsZSI6IlRFU1QiLCJjb250ZW50IjoidGVzdCIsImxhbmd1YWdlIjoiIiwidGFncyI6bnVsbCwidHlwZSI6IiIsInNvdXJjZSI6IiIsImF1dGhvciI6IiIsInRpbWUiOiIiLCJjb3B5cmlnaHQiOiIiLCJmZWVkYmFjayI6bnVsbH0=",
				RepoType:    types.PromptRepo,
				CurrentUser: "foo",
				Username:    "foo",
				Email:       "foo@bar.com",
				Message:     fmt.Sprintf("create prompt %s", "TEST.jsonl"),
			}).Return(&types.CreateFileResp{}, nil).Once()

			_, err := pc.CreatePrompt(ctx, types.PromptReq{
				Namespace:   "ns",
				Name:        "n",
				CurrentUser: "foo",
				Path:        "p",
			}, &types.CreatePromptReq{
				Prompt: types.Prompt{Title: "TEST", Content: "test"},
			})
			require.Nil(t, err)

		})
	}

}

func TestPromptComponent_UpdatePrompt(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	for _, exist := range []bool{true, false} {
		t.Run(fmt.Sprintf("file exist: %v", exist), func(t *testing.T) {

			pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
				NamespaceType: database.OrgNamespace,
			}, nil).Once()
			pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
				RoleMask: "foo-admin",
				Email:    "foo@bar.com",
			}, nil).Once()
			if !exist {
				pc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", errors.New("")).Once()

				_, err := pc.UpdatePrompt(ctx, types.PromptReq{
					Namespace:   "ns",
					Name:        "n",
					CurrentUser: "foo",
					Path:        "TEST.jsonl",
				}, &types.UpdatePromptReq{
					Prompt: types.Prompt{Title: "TEST.jsonl", Content: "test"},
				})
				require.NotNil(t, err)
				return
			} else {
				pc.mocks.gitServer.EXPECT().GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
					Namespace: "ns",
					Name:      "n",
					Ref:       types.MainBranch,
					Path:      "TEST.jsonl",
					RepoType:  types.PromptRepo,
				}).Return("", nil).Once()
			}

			pc.mocks.components.repo.EXPECT().UpdateFile(ctx, &types.UpdateFileReq{
				Namespace:   "ns",
				Name:        "n",
				Branch:      types.MainBranch,
				FilePath:    "TEST.jsonl",
				Content:     "eyJ0aXRsZSI6IlRFU1QiLCJjb250ZW50IjoidGVzdCIsImxhbmd1YWdlIjoiIiwidGFncyI6bnVsbCwidHlwZSI6IiIsInNvdXJjZSI6IiIsImF1dGhvciI6IiIsInRpbWUiOiIiLCJjb3B5cmlnaHQiOiIiLCJmZWVkYmFjayI6bnVsbH0=",
				RepoType:    types.PromptRepo,
				CurrentUser: "foo",
				Username:    "foo",
				Email:       "foo@bar.com",
				Message:     fmt.Sprintf("update prompt %s", "TEST.jsonl"),
			}).Return(&types.UpdateFileResp{}, nil).Once()

			_, err := pc.UpdatePrompt(ctx, types.PromptReq{
				Namespace:   "ns",
				Name:        "n",
				CurrentUser: "foo",
				Path:        "TEST.jsonl",
			}, &types.UpdatePromptReq{
				Prompt: types.Prompt{Title: "TEST", Content: "test"},
			})
			require.Nil(t, err)

		})
	}

}

func TestPromptComponent_DeletePrompt(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{
		NamespaceType: database.OrgNamespace,
	}, nil).Once()
	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		RoleMask: "foo-admin",
		Email:    "foo@bar.com",
	}, nil).Once()

	pc.mocks.components.repo.EXPECT().DeleteFile(ctx, &types.DeleteFileReq{
		Namespace:   "ns",
		Name:        "n",
		Branch:      types.MainBranch,
		FilePath:    "TEST.jsonl",
		Content:     "",
		RepoType:    types.PromptRepo,
		CurrentUser: "foo",
		Username:    "foo",
		Email:       "foo@bar.com",
		Message:     fmt.Sprintf("delete prompt %s", "TEST.jsonl"),
	}).Return(&types.DeleteFileResp{}, nil).Once()

	err := pc.DeletePrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "TEST.jsonl",
	})
	require.Nil(t, err)

}

func TestPromptComponent_ListPrompt(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	repo := &database.Repository{}
	pc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	pc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, repo, "foo").Return(true, nil).Once()

	pc.mocks.gitServer.EXPECT().GetRepoFileTree(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return([]*types.File{
		{Name: "foo.jsonl", Path: "foo.jsonl"},
		{Name: "bar.jsonl", Path: "bar.jsonl"},
		{Name: "main.go", Path: "main.go"},
		{Name: "large.jsonl", Path: "large.jsonl", Size: 12345678987654},
	}, nil)

	for _, name := range []string{"foo.jsonl", "bar.jsonl"} {
		pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
			Namespace: "ns",
			Name:      "n",
			Ref:       "main",
			Path:      name,
			RepoType:  types.PromptRepo,
		}).Return(&types.File{
			Content: "LS0tCiB0aXRsZTogImFpIg==",
		}, nil).Once()
	}

	outputs, err := pc.ListPrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "p",
	})
	require.Nil(t, err)
	require.Equal(t, 2, len(outputs))
	paths := []string{}
	for _, o := range outputs {
		require.Equal(t, "ai", o.Title)
		paths = append(paths, o.FilePath)
	}
	require.ElementsMatch(t, []string{"foo.jsonl", "bar.jsonl"}, paths)

}

func TestPromptComponent_GetPrompt(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	repo := &database.Repository{}
	pc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "foo", repo).Return(&types.UserRepoPermission{
		CanRead: true,
	}, nil).Once()

	pc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "foo.jsonl",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()

	output, err := pc.GetPrompt(ctx, types.PromptReq{
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
		Path:        "foo.jsonl",
	})
	require.Nil(t, err)
	require.Equal(t, "ai", output.Title)
	require.Equal(t, "foo.jsonl", output.FilePath)
}

func TestPromptComponent_SetRelationModels(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:    123,
		Email: "foo@bar.com",
	}, nil).Once()
	repo := &database.Repository{}
	pc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "foo", repo).Return(&types.UserRepoPermission{
		CanWrite: true,
	}, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()
	pc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "update model relation tags",
		FilePath:  types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczoKICAgIC0gbWEKICAgIC0gbWIKICAgIC0gbWMKdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.SetRelationModels(ctx, types.RelationModels{
		Models:      []string{"ma", "mb", "mc"},
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_AddRelationModel(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()
	repo := &database.Repository{}
	pc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCiB0aXRsZTogImFpIg==",
	}, nil).Once()
	pc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "add relation model",
		FilePath:  types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczoKICAgIC0gbWEKdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.AddRelationModel(ctx, types.RelationModel{
		Model:       "ma",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_DelRelationModel(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()
	repo := &database.Repository{}
	pc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.PromptRepo, "ns", "n").Return(repo, nil).Once()
	pc.mocks.gitServer.EXPECT().GetRepoFileContents(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Ref:       "main",
		Path:      "README.md",
		RepoType:  types.PromptRepo,
	}).Return(&types.File{
		Content: "LS0tCm1vZGVsczoKICAgIC0gbWEKdGl0bGU6IGFpCgotLS0=",
	}, nil).Once()
	pc.mocks.gitServer.EXPECT().UpdateRepoFile(&types.UpdateFileReq{
		Branch:    types.MainBranch,
		Message:   "delete relation model",
		FilePath:  types.REPOCARD_FILENAME,
		RepoType:  types.PromptRepo,
		Namespace: "ns",
		Name:      "n",
		Username:  "foo",
		Email:     "foo@bar.com",
		Content:   "LS0tCm1vZGVsczogW10KdGl0bGU6IGFpCgotLS0=",
	}).Return(nil).Once()

	err := pc.DelRelationModel(ctx, types.RelationModel{
		Model:       "ma",
		Namespace:   "ns",
		Name:        "n",
		CurrentUser: "foo",
	})
	require.Nil(t, err)

}

func TestPromptComponent_CreatePromptRepo(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	pc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "foo").Return(database.User{
		ID:       123,
		Email:    "foo@bar.com",
		RoleMask: "foo-admin",
	}, nil).Once()

	req := types.CreateRepoReq{
		Username:      "foo",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "nc",
		Description:   "good",
		License:       "MIT",
		Readme:        "\n---\nlicense: MIT\n---\n\t",
		DefaultBranch: "main",
		RepoType:      types.PromptRepo,
	}
	pc.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "ns").Return(database.Namespace{}, nil).Once()
	dbRepo := &database.Repository{}
	pc.mocks.components.repo.EXPECT().CreateRepo(ctx, req).Return(&gitserver.CreateRepoResp{}, dbRepo, nil)
	dbPrompt := database.Prompt{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}
	pc.mocks.stores.PromptMock().EXPECT().Create(ctx, dbPrompt).Return(&database.Prompt{
		Repository: &database.Repository{
			Name: "r1",
			Tags: []database.Tag{
				{Name: "t1"},
				{Name: "t2"},
			},
		},
	}, nil)
	// create readme
	pc.mocks.gitServer.EXPECT().CreateRepoFile(&types.CreateFileReq{
		Email:     "foo@bar.com",
		Message:   "initial commit",
		Branch:    "main",
		Content:   "Ci0tLQpsaWNlbnNlOiBNSVQKLS0tCgk=",
		NewBranch: "main",
		FilePath:  "README.md",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil).Once()
	// create .gitattributes
	pc.mocks.gitServer.EXPECT().CreateRepoFile(&types.CreateFileReq{
		Email:     "foo@bar.com",
		Message:   "initial commit",
		Branch:    "main",
		Content:   base64.StdEncoding.EncodeToString([]byte(types.DatasetGitattributesContent)),
		NewBranch: "main",
		FilePath:  ".gitattributes",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil).Once()

	res, err := pc.CreatePromptRepo(ctx, &types.CreatePromptRepoReq{
		CreateRepoReq: types.CreateRepoReq{
			Username:    "foo",
			Namespace:   "ns",
			Name:        "n",
			Nickname:    "nc",
			Description: "good",
			License:     "MIT",
		},
	})
	require.Nil(t, err)
	expected := types.PromptRes{
		Name: "r1",
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/.git",
			SSHCloneURL:  "test@127.0.0.1:s/.git",
		},
		User: types.User{Email: "foo@bar.com"},
		Tags: []types.RepoTag{
			{Name: "t1"},
			{Name: "t2"},
		},
	}
	require.Equal(t, expected, *res)

}

func TestPromptComponent_IndexPromptRepo(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	filter := &types.RepoFilter{Username: "foo"}
	pc.mocks.components.repo.EXPECT().PublicToUser(ctx, types.PromptRepo, "foo", filter, 1, 1).Return([]*database.Repository{
		{ID: 1, Name: "rp1"}, {ID: 2, Name: "rp2"},
		{ID: 3, Name: "rp3", Tags: []database.Tag{{Name: "t1"}, {Name: "t2"}}},
	}, 30, nil).Once()
	pc.mocks.stores.PromptMock().EXPECT().ByRepoIDs(ctx, []int64{1, 2, 3}).Return([]database.Prompt{
		{ID: 6, RepositoryID: 2, Repository: &database.Repository{}},
		{ID: 5, RepositoryID: 1, Repository: &database.Repository{}},
		{ID: 4, RepositoryID: 3, Repository: &database.Repository{}},
		{ID: 3, RepositoryID: 2, Repository: &database.Repository{}},
	}, nil).Once()

	results, total, err := pc.IndexPromptRepo(ctx, filter, 1, 1)
	require.Nil(t, err)
	require.Equal(t, 30, total)
	require.Equal(t, []types.PromptRes{
		{
			RepositoryID: 1,
			ID:           5, Name: "rp1", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}},
		{
			RepositoryID: 2,
			ID:           6, Name: "rp2", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}},
		{
			RepositoryID: 3,
			ID:           4, Name: "rp3", Repository: types.Repository{
				HTTPCloneURL: "https://foo.com/s/.git",
				SSHCloneURL:  "test@127.0.0.1:s/.git",
			}, Tags: []types.RepoTag{{Name: "t1"}, {Name: "t2"}}},
	}, results)

}

func TestPromptComponent_UpdatePromptRepo(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	req := &types.UpdatePromptRepoReq{
		UpdateRepoReq: types.UpdateRepoReq{RepoType: types.PromptRepo},
	}
	mockedRepo := &database.Repository{Name: "rp1", ID: 123}
	pc.mocks.components.repo.EXPECT().UpdateRepo(ctx, req.UpdateRepoReq).Return(mockedRepo, nil).Once()
	mockedPrompt := &database.Prompt{ID: 3}
	pc.mocks.stores.PromptMock().EXPECT().ByRepoID(ctx, int64(123)).Return(mockedPrompt, nil).Once()
	pc.mocks.stores.PromptMock().EXPECT().Update(ctx, *mockedPrompt).Return(nil).Once()

	res, err := pc.UpdatePromptRepo(ctx, req)
	require.Nil(t, err)
	require.Equal(t, types.PromptRes{
		RepositoryID: 123,
		ID:           3,
		Name:         "rp1",
	}, *res)

}

func TestPromptComponent_RemovetRepo(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	mockedPrompt := &database.Prompt{}
	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	pc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "foo",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.PromptRepo,
	}).Return(nil, nil).Once()
	pc.mocks.stores.PromptMock().EXPECT().Delete(ctx, *mockedPrompt).Return(nil).Once()

	err := pc.RemoveRepo(ctx, "ns", "n", "foo")
	require.Nil(t, err)

}

func TestPromptComponent_Show(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	mockedPrompt := &database.Prompt{
		Repository: &database.Repository{
			ID:   123,
			Tags: []database.Tag{{Name: "t1"}},
		},
	}
	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	pc.mocks.components.repo.EXPECT().GetUserRepoPermission(ctx, "foo", mockedPrompt.Repository).Return(&types.UserRepoPermission{
		CanRead: true,
	}, nil).Once()
	pc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "ns").Return(&types.Namespace{}, nil).Once()
	pc.mocks.stores.UserLikesMock().EXPECT().IsExist(ctx, "foo", int64(123)).Return(true, nil).Once()

	res, err := pc.Show(ctx, "ns", "n", "foo")
	require.Nil(t, err)
	require.Equal(t, types.PromptRes{
		RepositoryID: 123,
		Repository: types.Repository{
			HTTPCloneURL: "https://foo.com/s/.git",
			SSHCloneURL:  "test@127.0.0.1:s/.git",
		},
		Tags:      []types.RepoTag{{Name: "t1"}},
		UserLikes: true,
		Namespace: &types.Namespace{},
	}, *res)

}

func TestPromptComponent_Relations(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	mockedPrompt := &database.Prompt{
		RepositoryID: 123,
		Repository: &database.Repository{
			ID:   123,
			Tags: []database.Tag{{Name: "t1"}},
		},
	}
	pc.mocks.stores.PromptMock().EXPECT().FindByPath(ctx, "ns", "n").Return(mockedPrompt, nil).Once()
	pc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, mockedPrompt.Repository, "foo").Return(true, nil).Once()
	pc.mocks.components.repo.EXPECT().RelatedRepos(ctx, int64(123), "foo").Return(
		map[types.RepositoryType][]*database.Repository{
			types.ModelRepo: {
				{ID: 1, Name: "r1"},
				{ID: 2, Name: "r2"},
			},
			types.PromptRepo: {
				{ID: 3, Name: "r3"},
				{ID: 4, Name: "r4"},
			},
		},
		nil).Once()

	r, err := pc.Relations(ctx, "ns", "n", "foo")
	require.Nil(t, err)
	require.Equal(t, types.Relations{
		Models: []*types.Model{
			{Name: "r1"},
			{Name: "r2"},
		},
	}, *r)

}

func TestPromptComponent_OrgPrompts(t *testing.T) {
	ctx := context.TODO()
	pc := initializeTestPromptComponent(ctx, t)

	cases := []struct {
		role       membership.Role
		publicOnly bool
	}{
		{membership.RoleUnknown, true},
		{membership.RoleAdmin, false},
	}

	for _, c := range cases {
		t.Run(string(c.role), func(t *testing.T) {
			pc.mocks.userSvcClient.EXPECT().GetMemberRole(ctx, "ns", "foo").Return(c.role, nil).Once()
			pc.mocks.stores.PromptMock().EXPECT().ByOrgPath(ctx, "ns", 1, 1, c.publicOnly).Return([]database.Prompt{
				{ID: 1, Repository: &database.Repository{Name: "r1"}},
				{ID: 2, Repository: &database.Repository{Name: "r2"}},
				{ID: 3, Repository: &database.Repository{Name: "r3"}},
			}, 100, nil)
			res, count, err := pc.OrgPrompts(ctx, &types.OrgDatasetsReq{
				Namespace:   "ns",
				CurrentUser: "foo",
				PageOpts: types.PageOpts{
					Page:     1,
					PageSize: 1,
				},
			})
			require.Nil(t, err)
			require.Equal(t, 100, count)
			require.Equal(t, []types.PromptRes{
				{ID: 1, Name: "r1"},
				{ID: 2, Name: "r2"},
				{ID: 3, Name: "r3"},
			}, res)
		})

	}

}
