package checker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestFileSizeChecker_Check(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestFileSizeChecker(ctx, t)

	repo := &database.Repository{
		ID: 1,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.ModelRepo, "foo", "bar").
		Return(repo, nil)

	c.mocks.gitServer.EXPECT().GetRepoFiles(ctx, gitserver.GetRepoFilesReq{
		Namespace: "foo",
		Name:      "bar",
		RepoType:  types.ModelRepo,
		Revisions: []string{
			"--not",
			"--all",
			"--not",
			"main",
		},
		GitObjectDirectoryRelative:            "relative",
		GitAlternateObjectDirectoriesRelative: []string{"objects"},
	}).Return([]*types.File{
		{
			Path: "foo/bar",
			Size: 10000,
		},
	}, nil)

	c.config.Git.CheckFileSizeEnabled = true
	c.config.Git.MaxUnLfsFileSize = 1000

	valid, err := c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "models/foo/bar",
		Changes:      "abc main",
		Env:          "{GIT_ALTERNATE_OBJECT_DIRECTORIES_RELATIVE: [\"objects\"], GIT_OBJECT_DIRECTORY_RELATIVE:\"relative\"}",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.NotNil(t, err)
	require.False(t, valid)

	c.config.Git.MaxUnLfsFileSize = 100000

	valid, err = c.Check(ctx, types.GitalyAllowedReq{
		GlRepository: "models/foo/bar",
		Changes:      "abc main",
		Env:          "{GIT_ALTERNATE_OBJECT_DIRECTORIES_RELATIVE: [\"objects\"], GIT_OBJECT_DIRECTORY_RELATIVE:\"relative\"}",
		GitEnv: types.GitEnv{
			GitAlternateObjectDirectoriesRelative: []string{"objects"},
			GitObjectDirectoryRelative:            "relative",
		},
	})

	require.Nil(t, err)
	require.True(t, valid)
}
