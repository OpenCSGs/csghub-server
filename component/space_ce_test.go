//go:build !ee && !saas

package component

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
		ID:        1,
		Name:      "sp",
		Resources: `{"memory": "foo"}`,
	}, nil)

	sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, types.CheckResourceAndAccountReq{UserName: "user", ClusterID: "cluster", OrderDetailID: 0, CurrentUser: "user"}, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	sc.mocks.components.repo.EXPECT().CreateRepo(ctx, types.CreateRepoReq{
		DefaultBranch: "main",
		Readme:        generateReadmeData("MIT"),
		License:       "MIT",
		Namespace:     "ns",
		Name:          "n",
		Nickname:      "n",
		RepoType:      types.SpaceRepo,
		Username:      "user",
	}).Return(nil, &database.Repository{
		ID: 321,
		User: database.User{
			Username: "user",
			Email:    "foo@bar.com",
			UUID:     "user-uuid",
		},
		Path: "ns/n",
	}, &gitserver.CommitFilesReq{}, nil)
	sc.mocks.gitServer.EXPECT().CommitFiles(ctx, gitserver.CommitFilesReq{}).Return(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	sc.mocks.components.repo.EXPECT().SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
		return req.RepoType == types.SpaceRepo &&
			req.Operation == types.OperationCreate &&
			req.RepoPath == "ns/n" &&
			req.UserUUID == "user-uuid"
	})).RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
		wg.Done()
		return nil
	}).Once()

	sc.mocks.stores.SpaceMock().EXPECT().CreateAndUpdateRepoPath(ctx, database.Space{
		RepositoryID: 321,
		Sdk:          types.STREAMLIT.Name,
		SdkVersion:   "v1",
		Env:          "env",
		Hardware:     `{"memory": "foo"}`,
		Secrets:      "sss",
		SKU:          "1",
		ClusterID:    "cluster",
	}, "ns/n").Return(&database.Space{}, nil)

	commitReq := gitserver.CommitFilesReq{
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.SpaceRepo,
		Revision:  "main",
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Files: []gitserver.CommitFile{
			{
				Path:    types.ReadmeFileName,
				Content: base64.StdEncoding.EncodeToString([]byte(generateReadmeData("MIT"))),
				Action:  gitserver.CommitActionCreate,
			},
			{
				Path:    types.GitattributesFileName,
				Content: base64.StdEncoding.EncodeToString([]byte(spaceGitattributesContent)),
				Action:  gitserver.CommitActionCreate,
			},
			{
				Path:    streamlitConfig,
				Content: base64.StdEncoding.EncodeToString([]byte(streamlitConfigContent)),
				Action:  gitserver.CommitActionCreate,
			},
		},
	}

	sc.mocks.gitServer.EXPECT().CommitFiles(mock.Anything, commitReq).Return(nil)

	space, err := sc.Create(ctx, types.CreateSpaceReq{
		Sdk:        types.STREAMLIT.Name,
		SdkVersion: "v1",
		Env:        "env",
		Secrets:    "sss",
		ResourceID: 1,
		ClusterID:  "cluster",
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

	require.Equal(t, &types.Space{
		License:    "MIT",
		Name:       "n",
		Sdk:        "streamlit",
		SdkVersion: "v1",
		Env:        "env",
		Secrets:    "sss",
		Hardware:   `{"memory": "foo"}`,
		Creator:    "user",
		Path:       "ns/n",
	}, space)
	wg.Wait()
}

func TestSpaceComponent_Update(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(12)).Return(&database.SpaceResource{
		ID:        12,
		Name:      "sp",
		Resources: `{"memory": "foo"}`,
	}, nil)

	sc.mocks.components.repo.EXPECT().UpdateRepo(ctx, types.UpdateRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.SpaceRepo,
	}).Return(
		&database.Repository{
			ID:      123,
			Name:    "repo",
			Path:    "ns/n",
			Private: false,
			User: database.User{
				UUID:     "user-uuid",
				Username: "user",
			},
		}, nil,
	)
	sc.mocks.stores.SpaceMock().EXPECT().ByRepoID(ctx, int64(123)).Return(&database.Space{
		ID: 321,
	}, nil)
	sc.mocks.stores.SpaceMock().EXPECT().Update(ctx, database.Space{
		ID:       321,
		Hardware: `{"memory": "foo"}`,
		SKU:      "12",
	}).Return(nil)

	space, err := sc.Update(ctx, &types.UpdateSpaceReq{
		ResourceID: tea.Int64(12),
		UpdateRepoReq: types.UpdateRepoReq{
			Username:  "user",
			Namespace: "ns",
			Name:      "n",
		},
	})
	require.Nil(t, err)

	require.Equal(t, &types.Space{
		ID:       321,
		Name:     "repo",
		Path:     "ns/n",
		Hardware: `{"memory": "foo"}`,
		SKU:      "12",
	}, space)

}

func TestSpaceComponent_Deploy(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "ns1").Return("ns1-billing-uuid", nil)
	sc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindSpaceLatestVersion(ctx, mock.Anything, mock.Anything).Return(&database.RuntimeFramework{}, nil)

	t.Run("Deploy", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns1", "n1").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo1/bar1", User: database.User{Username: "user"}},
			SKU:        "1",
			HasAppFile: true,
		}, nil)
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID: 1,
		}, nil)
		sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, types.CheckResourceAndAccountReq{UserName: "ns1", ClusterID: "", OrderDetailID: 0, CurrentUser: "user"}, &database.SpaceResource{
			ID: 1,
		}).Return(&types.CheckExclusiveResp{}, nil)
		sc.mocks.deployer.EXPECT().Deploy(ctx, types.DeployRequest{
			SpaceID:        1,
			Path:           "foo1/bar1",
			Annotation:     "{\"hub-deploy-user\":\"user\",\"hub-res-name\":\"ns1/n1\",\"hub-res-type\":\"space\"}",
			ContainerPort:  8080,
			SKU:            "1",
			UserUUID:       "ns1-billing-uuid",
			OwnerNamespace: "ns1",
			SecureLevel:    types.EndpointPublic,
		}).Return(123, nil)

		id, err := sc.Deploy(ctx, "ns1", "n1", "user")
		require.Nil(t, err)
		require.Equal(t, int64(123), id)
	})
	t.Run("DeployWithoutAppFile", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns2", "n2").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo2/bar2", User: database.User{Username: "user"}},
			SKU:        "1",
			HasAppFile: false,
		}, nil)
		id, err := sc.Deploy(ctx, "ns2", "n2", "user")
		require.Equal(t, true, errors.Is(err, errorx.ErrNoEntryFile))
		require.Equal(t, int64(-1), id)
	})
}

func TestSpaceComponent_DeployUsesOwnerNamespaceBilling(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "org1").Return("org-billing-uuid", nil)
	sc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindSpaceLatestVersion(ctx, mock.Anything, mock.Anything).
		Return(&database.RuntimeFramework{}, nil)
	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "org1", "n1").Return(&database.Space{
		ID: 1,
		Repository: &database.Repository{
			Path: "foo1/bar1",
			User: database.User{Username: "creator"},
		},
		SKU:        "1",
		HasAppFile: true,
	}, nil)
	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
		ID: 1,
	}, nil)
	sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, types.CheckResourceAndAccountReq{
		UserName:    "org1",
		CurrentUser: "creator",
	}, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	sc.mocks.deployer.EXPECT().Deploy(ctx, mock.MatchedBy(func(dr types.DeployRequest) bool {
		return dr.UserUUID == "org-billing-uuid" &&
			dr.OwnerNamespace == "org1" &&
			strings.Contains(dr.Annotation, `"hub-deploy-user":"creator"`)
	})).Return(int64(123), nil)

	id, err := sc.Deploy(ctx, "org1", "n1", "creator")
	require.Nil(t, err)
	require.Equal(t, int64(123), id)
}

func TestSpaceComponent_DeployByOrgAdmin(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.components.repo.EXPECT().GetNameSpaceInfo(ctx, "org1").Return(&types.Namespace{
		Path: "org1",
		Type: types.OrganizationNamespaceType,
	}, nil)
	sc.mocks.components.repo.EXPECT().CheckCurrentUserPermission(ctx, "admin", "org1", rebac.NamespaceCanAdmin).
		Return(true, nil)
	sc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "org1").Return("org-billing-uuid", nil)
	sc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindSpaceLatestVersion(ctx, mock.Anything, mock.Anything).
		Return(&database.RuntimeFramework{}, nil)
	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "org1", "n1").Return(&database.Space{
		ID: 1,
		Repository: &database.Repository{
			Path: "foo1/bar1",
			User: database.User{Username: "creator"},
		},
		SKU:        "1",
		HasAppFile: true,
	}, nil)
	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
		ID: 1,
	}, nil)
	sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, types.CheckResourceAndAccountReq{
		UserName:    "org1",
		CurrentUser: "admin",
	}, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)
	sc.mocks.deployer.EXPECT().Deploy(ctx, mock.MatchedBy(func(dr types.DeployRequest) bool {
		return dr.UserUUID == "org-billing-uuid" &&
			dr.OwnerNamespace == "org1" &&
			strings.Contains(dr.Annotation, `"hub-deploy-user":"admin"`)
	})).Return(int64(123), nil)

	id, err := sc.Deploy(ctx, "org1", "n1", "admin")
	require.Nil(t, err)
	require.Equal(t, int64(123), id)
}

func TestSpaceComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.SpaceMock().EXPECT().FindByPath(mock.Anything, "ns", "n").Return(&database.Space{ID: 1}, nil)
	sc.mocks.components.repo.EXPECT().DeleteRepo(ctx, types.DeleteRepoReq{
		Username:  "user",
		Namespace: "ns",
		Name:      "n",
		RepoType:  types.SpaceRepo,
	}).Return(&database.Repository{
		User: database.User{
			UUID: "user-uuid",
		},
		Path: "ns/n",
	}, nil)
	sc.mocks.stores.SpaceMock().EXPECT().Delete(mock.Anything, database.Space{ID: 1}).Return(nil)

	sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(mock.Anything, int64(1)).Return(
		&database.Deploy{
			RepoID: 2,
			UserID: 3,
			ID:     4,
		}, nil,
	)
	var wgstop sync.WaitGroup
	wgstop.Add(1)
	sc.mocks.deployer.EXPECT().Stop(mock.Anything, mock.MatchedBy(func(req types.DeployRequest) bool {
		return req.SpaceID == 1 &&
			req.Namespace == "ns" &&
			req.Name == "n"
	})).
		RunAndReturn(func(ctx context.Context, req types.DeployRequest) error {
			wgstop.Done()
			return nil
		}).Once()
	sc.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(
		mock.Anything, types.SpaceRepo, int64(2), int64(4),
	).Return(nil)

	var wg sync.WaitGroup
	wg.Add(1)
	sc.mocks.components.repo.EXPECT().
		SendAssetManagementMsg(mock.Anything, mock.MatchedBy(func(req types.RepoNotificationReq) bool {
			return req.RepoType == types.SpaceRepo &&
				req.Operation == types.OperationDelete &&
				req.RepoPath == "ns/n" &&
				req.UserUUID == "user-uuid"
		})).
		RunAndReturn(func(ctx context.Context, req types.RepoNotificationReq) error {
			wg.Done()
			return nil
		}).Once()

	err := sc.Delete(ctx, "ns", "n", "user")
	require.Nil(t, err)
	wg.Wait()
	wgstop.Wait()
}

func TestSpaceComponent_Deploy_SecureLevel(t *testing.T) {
	cases := []struct {
		name      string
		private   bool
		wantLevel int
	}{
		{name: "public repo maps to endpoint public", private: false, wantLevel: types.EndpointPublic},
		{name: "private repo maps to endpoint private", private: true, wantLevel: types.EndpointPrivate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.TODO()
			sc := initializeTestSpaceComponent(ctx, t)

			sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns", "n").Return(&database.Space{
				ID:         1,
				HasAppFile: true,
				Sdk:        types.MCPSERVER.Name,
				SKU:        "1",
				Repository: &database.Repository{
					ID:            321,
					UserID:        11,
					Path:          "ns/n",
					GitPath:       "spaces_ns/n",
					DefaultBranch: "main",
					Private:       tc.private,
					User:          database.User{Username: "user"},
				},
			}, nil)

			sc.mocks.components.repo.EXPECT().GetNamespaceBillingUUID(ctx, "ns").Return("ns-billing-uuid", nil)

			sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
				ID:        1,
				ClusterID: "cluster",
			}, nil)

			sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, types.CheckResourceAndAccountReq{
				UserName:      "ns",
				ClusterID:     "cluster",
				OrderDetailID: 0,
				CurrentUser:   "user",
			}, mock.Anything).Return(&types.CheckExclusiveResp{}, nil)

			sc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindSpaceLatestVersion(ctx, "space", "").Return(
				&database.RuntimeFramework{FrameImage: "img"}, nil)

			sc.mocks.deployer.EXPECT().Deploy(ctx, mock.MatchedBy(func(dr types.DeployRequest) bool {
				return dr.SpaceID == 1 &&
					dr.Type == types.SpaceType &&
					dr.ContainerPort == types.MCPSERVER.Port &&
					dr.SecureLevel == tc.wantLevel
			})).Return(int64(1), nil)

			deployID, err := sc.Deploy(ctx, "ns", "n", "user")
			require.Nil(t, err)
			require.Equal(t, int64(1), deployID)
		})
	}
}

func TestSpaceComponent_Update_SecureLevelSync(t *testing.T) {
	newRepo := func(private bool) *database.Repository {
		return &database.Repository{
			ID:      321,
			Name:    "n",
			Path:    "ns/n",
			Private: private,
			User:    database.User{UUID: "uuid", Username: "user"},
		}
	}

	t.Run("private space syncs endpoint private to deploy", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceComponent(ctx, t)

		private := true
		sc.mocks.components.repo.EXPECT().UpdateRepo(ctx, mock.Anything).Return(newRepo(true), nil)
		sc.mocks.stores.SpaceMock().EXPECT().ByRepoID(ctx, int64(321)).Return(&database.Space{ID: 1, RepositoryID: 321}, nil)
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(&database.Deploy{ID: 9}, nil)
		sc.mocks.stores.SpaceMock().EXPECT().UpdateWithDeploySecureLevel(ctx, database.Space{ID: 1, RepositoryID: 321}, int64(9), types.EndpointPrivate).Return(nil)

		space, err := sc.Update(ctx, &types.UpdateSpaceReq{
			UpdateRepoReq: types.UpdateRepoReq{Namespace: "ns", Name: "n", Username: "user", Private: &private},
		})
		require.Nil(t, err)
		require.True(t, space.Private)
	})

	t.Run("public space syncs endpoint public to deploy", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceComponent(ctx, t)

		private := false
		sc.mocks.components.repo.EXPECT().UpdateRepo(ctx, mock.Anything).Return(newRepo(false), nil)
		sc.mocks.stores.SpaceMock().EXPECT().ByRepoID(ctx, int64(321)).Return(&database.Space{ID: 1, RepositoryID: 321}, nil)
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(&database.Deploy{ID: 9}, nil)
		sc.mocks.stores.SpaceMock().EXPECT().UpdateWithDeploySecureLevel(ctx, database.Space{ID: 1, RepositoryID: 321}, int64(9), types.EndpointPublic).Return(nil)

		_, err := sc.Update(ctx, &types.UpdateSpaceReq{
			UpdateRepoReq: types.UpdateRepoReq{Namespace: "ns", Name: "n", Username: "user", Private: &private},
		})
		require.Nil(t, err)
	})

	t.Run("space never deployed skips sync", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceComponent(ctx, t)

		private := true
		sc.mocks.components.repo.EXPECT().UpdateRepo(ctx, mock.Anything).Return(newRepo(true), nil)
		sc.mocks.stores.SpaceMock().EXPECT().ByRepoID(ctx, int64(321)).Return(&database.Space{ID: 1, RepositoryID: 321}, nil)
		sc.mocks.stores.SpaceMock().EXPECT().Update(ctx, mock.Anything).Return(nil)
		sc.mocks.stores.DeployTaskMock().EXPECT().GetLatestDeployBySpaceID(ctx, int64(1)).Return(nil, errorx.ErrDatabaseNoRows)

		_, err := sc.Update(ctx, &types.UpdateSpaceReq{
			UpdateRepoReq: types.UpdateRepoReq{Namespace: "ns", Name: "n", Username: "user", Private: &private},
		})
		require.Nil(t, err)
	})

	t.Run("no privacy change skips sync", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceComponent(ctx, t)

		sc.mocks.components.repo.EXPECT().UpdateRepo(ctx, mock.Anything).Return(newRepo(false), nil)
		sc.mocks.stores.SpaceMock().EXPECT().ByRepoID(ctx, int64(321)).Return(&database.Space{ID: 1, RepositoryID: 321}, nil)
		sc.mocks.stores.SpaceMock().EXPECT().Update(ctx, mock.Anything).Return(nil)

		_, err := sc.Update(ctx, &types.UpdateSpaceReq{
			UpdateRepoReq: types.UpdateRepoReq{Namespace: "ns", Name: "n", Username: "user"},
		})
		require.Nil(t, err)
	})
}
