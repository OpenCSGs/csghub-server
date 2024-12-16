package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestHFDataset_GetPathsInfo(t *testing.T) {
	ctx := context.TODO()
	hc := initializeTestHFDatasetComponent(ctx, t)

	dataset := &database.Dataset{}
	hc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	hc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, dataset.Repository, "user").Return(true, nil)
	hc.mocks.gitServer.EXPECT().GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Path:      "a/b",
		Ref:       "main",
		RepoType:  types.DatasetRepo,
	}).Return(&types.File{
		Type: "go", LastCommitSHA: "sha", Size: 5, Path: "foo",
	}, nil)

	data, err := hc.GetPathsInfo(ctx, types.PathReq{
		Namespace:   "ns",
		Name:        "n",
		Ref:         "main",
		Path:        "a/b",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, []types.HFDSPathInfo{
		{Type: "file", Path: "foo", Size: 5, OID: "sha"},
	}, data)

}

func TestHFDataset_GetDatasetTree(t *testing.T) {
	ctx := context.TODO()
	hc := initializeTestHFDatasetComponent(ctx, t)

	dataset := &database.Dataset{}
	hc.mocks.stores.DatasetMock().EXPECT().FindByPath(ctx, "ns", "n").Return(dataset, nil)
	hc.mocks.components.repo.EXPECT().AllowReadAccessRepo(ctx, dataset.Repository, "user").Return(true, nil)
	hc.mocks.gitServer.EXPECT().GetRepoFileTree(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: "ns",
		Name:      "n",
		Path:      "a/b",
		RepoType:  types.DatasetRepo,
	}).Return([]*types.File{
		{Type: "go", LastCommitSHA: "sha", Size: 5, Path: "foo"},
	}, nil)

	data, err := hc.GetDatasetTree(ctx, types.PathReq{
		Namespace:   "ns",
		Name:        "n",
		Ref:         "main",
		Path:        "a/b",
		CurrentUser: "user",
	})
	require.Nil(t, err)
	require.Equal(t, []types.HFDSPathInfo{
		{Type: "go", Path: "foo", Size: 5, OID: "sha"},
	}, data)
}
