//go:build !ee && !saas

package component

import (
	"context"
	"sync"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
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
	}, nil)

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

	sc.mocks.stores.SpaceMock().EXPECT().Create(ctx, database.Space{
		RepositoryID: 321,
		Sdk:          types.STREAMLIT.Name,
		SdkVersion:   "v1",
		Env:          "env",
		Hardware:     `{"memory": "foo"}`,
		Secrets:      "sss",
		SKU:          "1",
		ClusterID:    "cluster",
	}).Return(&database.Space{}, nil)
	sc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   generateReadmeData("MIT"),
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  types.ReadmeFileName,
	}, types.SpaceRepo)).Return(nil)
	sc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   spaceGitattributesContent,
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  types.GitattributesFileName,
	}, types.SpaceRepo)).Return(nil)
	sc.mocks.gitServer.EXPECT().CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  "user",
		Email:     "foo@bar.com",
		Message:   types.InitCommitMessage,
		Branch:    "main",
		Content:   streamlitConfigContent,
		NewBranch: "main",
		Namespace: "ns",
		Name:      "n",
		FilePath:  streamlitConfig,
	}, types.SpaceRepo)).Return(nil)

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
