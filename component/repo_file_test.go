package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoFileComponent_GenRepoFileRecords(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRepoFileComponent(ctx, t)

	rc.mocks.stores.RepoMock().EXPECT().FindByPath(ctx, types.ModelRepo, "ns", "n").Return(
		&database.Repository{ID: 1, Path: "foo/bar"}, nil,
	)

	rc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "foo", Name: "bar", Limit: 500, Recursive: true},
	).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
		{Path: "a/b", Type: "dir"},
		{Path: "foo.go", Type: "go"},
	}, Cursor: ""}, nil)
	rc.mocks.stores.RepoFileMock().EXPECT().Exists(ctx, database.RepositoryFile{
		RepositoryID: 1,
		Path:         "foo.go",
		FileType:     "go",
	}).Return(false, nil)
	rc.mocks.stores.RepoFileMock().EXPECT().Create(ctx, &database.RepositoryFile{
		RepositoryID: 1,
		Path:         "foo.go",
		FileType:     "go",
	}).Return(nil)

	err := rc.GenRepoFileRecords(ctx, types.ModelRepo, "ns", "n")
	require.Nil(t, err)

}

func TestRepoFileComponent_GenRepoFileRecordsBatch(t *testing.T) {
	ctx := context.TODO()
	rc := initializeTestRepoFileComponent(ctx, t)

	pendingStatus := types.SensitiveCheckPending
	filter := &types.BatchGetFilter{
		RepoType:             types.ModelRepo,
		SensitiveCheckStatus: &pendingStatus,
	}
	rc.mocks.stores.RepoMock().EXPECT().BatchGet(ctx, int64(1), 10, filter).Return(
		[]database.Repository{{ID: 1, Path: "foo/bar"}}, nil,
	)

	rc.mocks.gitServer.EXPECT().GetTree(
		mock.Anything, types.GetTreeRequest{Namespace: "foo", Name: "bar", Limit: 500, Recursive: true},
	).Return(&types.GetRepoFileTreeResp{Files: []*types.File{
		{Path: "a/b", Type: "dir"},
		{Path: "foo.go", Type: "go"},
	}, Cursor: ""}, nil)
	rc.mocks.stores.RepoFileMock().EXPECT().Exists(ctx, database.RepositoryFile{
		RepositoryID: 1,
		Path:         "foo.go",
		FileType:     "go",
	}).Return(false, nil)
	rc.mocks.stores.RepoFileMock().EXPECT().Create(ctx, &database.RepositoryFile{
		RepositoryID: 1,
		Path:         "foo.go",
		FileType:     "go",
	}).Return(nil)

	err := rc.GenRepoFileRecordsBatch(ctx, types.ModelRepo, 1, 10)
	require.Nil(t, err)
}
