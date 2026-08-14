package component

import (
	"context"
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mocks3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestRepositoryPackageCommitObjectKey(t *testing.T) {
	require.Equal(t,
		"codes/6b/86/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b/abc123.zip",
		repositoryPackageCommitObjectKey(types.CodeRepo, 1, "abc123"),
	)
}

func TestRepositoryPackageRepoPrefix(t *testing.T) {
	require.Equal(t,
		"skills/6b/86/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b/",
		repositoryPackageRepoPrefix(types.SkillRepo, 1),
	)
}

func TestRepositoryPackageSyncer_SyncRepoBranch(t *testing.T) {
	t.Run("syncs skill repo without agent template lookup", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)
		repo := &database.Repository{ID: 1, RepositoryType: types.SkillRepo, Path: "ns/skill", DefaultBranch: "main"}

		gitServer.EXPECT().GetRepoBranchByName(mock.Anything, gitserver.GetBranchReq{
			Namespace: "ns",
			Name:      "skill",
			Ref:       "main",
			RepoType:  types.SkillRepo,
		}).Return(&types.Branch{Commit: types.RepoBranchCommit{ID: "commit-main"}}, nil)
		gitServer.EXPECT().GetArchive(mock.Anything, gitserver.GetArchiveReq{
			Namespace: "ns",
			Name:      "skill",
			Revision:  "commit-main",
			RepoType:  types.SkillRepo,
		}).Return([]byte("zip-data"), nil)
		s3Client.EXPECT().PutObject(mock.Anything, "bucket", repositoryPackageCommitObjectKey(types.SkillRepo, 1, "commit-main"), mock.Anything, int64(len("zip-data")), mock.Anything).
			Return(minio.UploadInfo{}, nil)

		err := syncer.SyncRepoBranch(ctx, repo, "ns", "skill", "main")
		require.NoError(t, err)
	})

	t.Run("syncs code repo when repository-managed csgclaw template exists", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)
		repo := &database.Repository{ID: 2, RepositoryType: types.CodeRepo, Path: "ns/code", DefaultBranch: "main"}

		agentTemplateStore.EXPECT().FindByTypeAndName(mock.Anything, "csgclaw", "ns/code").Return([]database.AgentTemplate{{ID: 1}}, nil)
		gitServer.EXPECT().GetRepoBranchByName(mock.Anything, gitserver.GetBranchReq{
			Namespace: "ns",
			Name:      "code",
			Ref:       "main",
			RepoType:  types.CodeRepo,
		}).Return(&types.Branch{Commit: types.RepoBranchCommit{ID: "commit-main"}}, nil)
		gitServer.EXPECT().GetArchive(mock.Anything, gitserver.GetArchiveReq{
			Namespace: "ns",
			Name:      "code",
			Revision:  "commit-main",
			RepoType:  types.CodeRepo,
		}).Return([]byte("zip-data"), nil)
		s3Client.EXPECT().PutObject(mock.Anything, "bucket", repositoryPackageCommitObjectKey(types.CodeRepo, 2, "commit-main"), mock.Anything, int64(len("zip-data")), mock.Anything).
			Return(minio.UploadInfo{}, nil)

		err := syncer.SyncRepoBranch(ctx, repo, "ns", "code", "main")
		require.NoError(t, err)
	})

	t.Run("skips code repo when repository-managed csgclaw template is missing", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)
		repo := &database.Repository{ID: 2, RepositoryType: types.CodeRepo, Path: "ns/code", DefaultBranch: "main"}

		agentTemplateStore.EXPECT().FindByTypeAndName(mock.Anything, "csgclaw", "ns/code").Return(nil, nil)

		err := syncer.SyncRepoBranch(ctx, repo, "ns", "code", "main")
		require.NoError(t, err)
	})

	t.Run("returns template lookup error without archiving", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)
		repo := &database.Repository{ID: 2, RepositoryType: types.CodeRepo, Path: "ns/code", DefaultBranch: "main"}
		templateErr := errors.New("template lookup failed")

		agentTemplateStore.EXPECT().FindByTypeAndName(mock.Anything, "csgclaw", "ns/code").Return(nil, templateErr)

		err := syncer.SyncRepoBranch(ctx, repo, "ns", "code", "main")
		require.ErrorIs(t, err, templateErr)
	})
}

func TestRepositoryPackageSyncer_WriteArchive(t *testing.T) {
	t.Run("skips code repo upload when repository-managed template is missing", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)

		repoStore.EXPECT().FindById(ctx, int64(2)).Return(&database.Repository{ID: 2, Path: "ns/code", RepositoryType: types.CodeRepo}, nil)
		agentTemplateStore.EXPECT().FindByTypeAndName(ctx, "csgclaw", "ns/code").Return(nil, nil)

		err := syncer.WriteArchive(ctx, types.CodeRepo, 2, "commit-main", []byte("zip-data"))
		require.NoError(t, err)
	})

	t.Run("uploads code repo archive when repository-managed template exists", func(t *testing.T) {
		ctx := context.Background()
		repoStore := mockdatabase.NewMockRepoStore(t)
		gitServer := mockgitserver.NewMockGitServer(t)
		s3Client := mocks3.NewMockClient(t)
		agentTemplateStore := mockdatabase.NewMockAgentTemplateStore(t)
		syncer := newRepositoryPackageSyncer(testRepositoryPackageConfig(), repoStore, gitServer, s3Client, agentTemplateStore)

		repoStore.EXPECT().FindById(ctx, int64(2)).Return(&database.Repository{ID: 2, Path: "ns/code", RepositoryType: types.CodeRepo}, nil)
		agentTemplateStore.EXPECT().FindByTypeAndName(ctx, "csgclaw", "ns/code").Return([]database.AgentTemplate{{ID: 1}}, nil)
		s3Client.EXPECT().PutObject(ctx, "bucket", repositoryPackageCommitObjectKey(types.CodeRepo, 2, "commit-main"), mock.Anything, int64(len("zip-data")), mock.Anything).
			Return(minio.UploadInfo{}, nil)

		err := syncer.WriteArchive(ctx, types.CodeRepo, 2, "commit-main", []byte("zip-data"))
		require.NoError(t, err)
	})
}

func testRepositoryPackageConfig() *config.Config {
	cfg := &config.Config{}
	cfg.S3.Bucket = "bucket"
	return cfg
}
