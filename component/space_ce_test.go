//go:build !ee && !saas

package component

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
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

	sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "user", "cluster", int64(0), mock.Anything).Return(nil)
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
			ID:   123,
			Name: "repo",
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
		Hardware: `{"memory": "foo"}`,
		SKU:      "12",
	}, space)

}

func TestSpaceComponent_Deploy(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceComponent(ctx, t)

	sc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
		Username: "user1",
	}, nil)
	sc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindSpaceLatestVersion(ctx, mock.Anything, mock.Anything).Return(&database.RuntimeFramework{}, nil)
	t.Run("Deploy", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns1", "n1").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo1/bar1"},
			SKU:        "1",
			HasAppFile: true,
		}, nil)
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{
			ID: 1,
		}, nil)
		sc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, "user", "", int64(0), &database.SpaceResource{
			ID: 1,
		}).Return(nil)
		sc.mocks.deployer.EXPECT().Deploy(ctx, types.DeployRepo{
			SpaceID:       1,
			Path:          "foo1/bar1",
			Annotation:    "{\"hub-deploy-user\":\"user1\",\"hub-res-name\":\"ns1/n1\",\"hub-res-type\":\"space\"}",
			ContainerPort: 8080,
			SKU:           "1",
		}).Return(123, nil)

		id, err := sc.Deploy(ctx, "ns1", "n1", "user")
		require.Nil(t, err)
		require.Equal(t, int64(123), id)
	})
	t.Run("DeployWithoutAppFile", func(t *testing.T) {
		sc.mocks.stores.SpaceMock().EXPECT().FindByPath(ctx, "ns2", "n2").Return(&database.Space{
			ID:         1,
			Repository: &database.Repository{Path: "foo2/bar2"},
			SKU:        "1",
			HasAppFile: false,
		}, nil)
		id, err := sc.Deploy(ctx, "ns2", "n2", "user")
		require.Equal(t, true, errors.Is(err, errorx.ErrNoEntryFile))
		require.Equal(t, int64(-1), id)
	})
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
	sc.mocks.deployer.EXPECT().Stop(mock.Anything, mock.MatchedBy(func(req types.DeployRepo) bool {
		return req.SpaceID == 1 &&
			req.Namespace == "ns" &&
			req.Name == "n"
	})).
		RunAndReturn(func(ctx context.Context, req types.DeployRepo) error {
			wgstop.Done()
			return nil
		}).Once()
	sc.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(
		mock.Anything, types.SpaceRepo, int64(2), int64(3), int64(4),
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
