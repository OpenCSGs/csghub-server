package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoFileComponent_GenRepoFileRecords(t *testing.T) {

	t.Run("GenRepoFileRecords successful", func(t *testing.T) {
		// Prepare mocks
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)

		// Initialize component
		componentImpl := &repoFileComponentImpl{
			rfs: mockRepoFileStore,
			rs:  mockRepoStore,
			gs:  mockGitServer,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		name := "test-repo"
		repo := &database.Repository{
			ID:                   1,
			Name:                 name,
			Path:                 "test-namespace/test-repo",
			DefaultBranch:        "main",
			SensitiveCheckStatus: types.SensitiveCheckPass,
		}

		file := types.File{Path: "file1.txt", Type: "file", Size: 100, SHA: "sha1"}
		repoFiles := []*types.File{&file}
		mockGitServer.EXPECT().GetTree(
			mock.Anything, mock.Anything,
		).Return(&types.GetRepoFileTreeResp{Files: repoFiles, Cursor: ""}, nil)
		rf := database.RepositoryFile{
			RepositoryID:    repo.ID,
			Path:            file.Path,
			FileType:        file.Type,
			Size:            file.Size,
			CommitSha:       file.SHA,
			LfsRelativePath: file.LfsRelativePath,
			Branch:          repo.DefaultBranch,
		}
		mockRepoFileStore.EXPECT().Exists(mock.Anything, rf).Return(false, nil)
		mockRepoFileStore.EXPECT().Create(mock.Anything, &rf).Return(nil)

		err := componentImpl.GenRepoFileRecords(ctx, repo)
		require.NoError(t, err)
	})
}

func TestRepoFileComponent_DetectRepoSensitiveCheckStatus(t *testing.T) {
	t.Run("check failed", func(t *testing.T) {
		// Prepare mocks
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)

		// Initialize component
		componentImpl := &repoFileComponentImpl{
			rfs: mockRepoFileStore,
			rs:  mockRepoStore,
			gs:  mockGitServer,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(true, nil)
		mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(mock.Anything, repo.ID, types.SensitiveCheckFail).Return(nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repo.ID, repo.DefaultBranch)

		// Assertions
		require.NoError(t, err)
	})
	t.Run("check exception", func(t *testing.T) {
		// Prepare mocks
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)

		// Initialize component
		componentImpl := &repoFileComponentImpl{
			rfs: mockRepoFileStore,
			rs:  mockRepoStore,
			gs:  mockGitServer,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(false, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckException).Return(true, nil)
		repoCopy := new(database.Repository)
		*repoCopy = *repo
		repoCopy.SensitiveCheckStatus = types.SensitiveCheckException
		mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(mock.Anything, repo.ID, types.SensitiveCheckException).Return(nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repo.ID, repo.DefaultBranch)

		// Assertions
		require.NoError(t, err)
	})
	t.Run("check success", func(t *testing.T) {
		// Prepare mocks
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)

		// Initialize component
		componentImpl := &repoFileComponentImpl{
			rfs: mockRepoFileStore,
			rs:  mockRepoStore,
			gs:  mockGitServer,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(false, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckException).Return(false, nil)

		mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(mock.Anything, repo.ID, types.SensitiveCheckPass).Return(nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repo.ID, repo.DefaultBranch)

		// Assertions
		require.NoError(t, err)

	})
}
