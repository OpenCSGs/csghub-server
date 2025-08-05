package checker

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestLFSExistsChecker_Check(t *testing.T) {
	ctx := context.TODO()
	c := initializeTestLFSExistsChecker(ctx, t)

	repo := &database.Repository{
		ID:       1,
		Migrated: false,
	}
	c.mocks.stores.RepoMock().
		EXPECT().
		FindByPath(ctx, types.ModelRepo, "foo", "bar").
		Return(repo, nil)

	c.mocks.gitServer.EXPECT().GetRepoLfsPointers(ctx, gitserver.GetRepoFilesReq{
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
	}).Return([]*types.LFSPointer{
		{
			FileOid:  "1234abcd",
			FileSize: 1233,
		},
	}, nil)

	c.mocks.s3Client.EXPECT().
		StatObject(ctx, c.config.S3.Bucket, "lfs/12/34/abcd", minio.StatObjectOptions{}).
		Return(minio.ObjectInfo{Size: 1234}, nil).Once()

	c.config.Git.LfsExistsCheck = true

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

	c.mocks.s3Client.EXPECT().
		StatObject(ctx, c.config.S3.Bucket, "lfs/12/34/abcd", minio.StatObjectOptions{}).
		Return(minio.ObjectInfo{Size: 1233}, nil).Once()

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
