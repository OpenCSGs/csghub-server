package component

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
		repoType := types.DatasetRepo
		namespace := "test-namespace"
		name := "test-repo"
		repo := &database.Repository{
			ID:                   1,
			Name:                 name,
			Path:                 "test-namespace/test-repo",
			DefaultBranch:        "main",
			SensitiveCheckStatus: types.SensitiveCheckPass,
		}

		mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(repo, nil)
		file := types.File{Path: "file1.txt", Type: "file", Size: 100, SHA: "sha1"}
		repoFiles := []*types.File{&file}
		mockGitServer.EXPECT().GetRepoFileTree(mock.Anything, mock.Anything).Return(repoFiles, nil)
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

		err := componentImpl.GenRepoFileRecords(ctx, repoType, namespace, name)
		require.NoError(t, err)
	})

	t.Run("GenRepoFileRecords error finding repo", func(t *testing.T) { // Prepare mocks
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
		repoType := types.DatasetRepo
		namespace := "test-namespace"
		name := "test-repo"
		// Mock expectations for error case
		mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(nil, errors.New("repo not found"))

		// Call method
		err := componentImpl.GenRepoFileRecords(ctx, repoType, namespace, name)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find repo")
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
		repoType := types.DatasetRepo
		namespace := "test-namespace"
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(repo, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(true, nil)
		repoCopy := new(database.Repository)
		*repoCopy = *repo
		repoCopy.SensitiveCheckStatus = types.SensitiveCheckFail
		mockRepoStore.EXPECT().UpdateRepo(mock.Anything, *repoCopy).Return(repoCopy, nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repoType, namespace, name)

		// Assertions
		require.NoError(t, err)
		require.Equal(t, types.SensitiveCheckFail, repo.SensitiveCheckStatus)
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
		repoType := types.DatasetRepo
		namespace := "test-namespace"
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(repo, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(false, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckException).Return(true, nil)
		repoCopy := new(database.Repository)
		*repoCopy = *repo
		repoCopy.SensitiveCheckStatus = types.SensitiveCheckException
		mockRepoStore.EXPECT().UpdateRepo(mock.Anything, *repoCopy).Return(repoCopy, nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repoType, namespace, name)

		// Assertions
		require.NoError(t, err)
		require.Equal(t, types.SensitiveCheckException, repo.SensitiveCheckStatus)
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
		repoType := types.DatasetRepo
		namespace := "test-namespace"
		name := "test-repo"
		repo := &database.Repository{
			ID:            1,
			Name:          name,
			Path:          "test-namespace/test-repo",
			DefaultBranch: "main",
		}
		// Mock expectations for successful status check
		mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(repo, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckFail).Return(false, nil)
		mockRepoFileStore.EXPECT().ExistsSensitiveCheckRecord(mock.Anything, repo.ID, repo.DefaultBranch, types.SensitiveCheckException).Return(false, nil)

		repoCopy := new(database.Repository)
		*repoCopy = *repo
		repoCopy.SensitiveCheckStatus = types.SensitiveCheckPass
		mockRepoStore.EXPECT().UpdateRepo(mock.Anything, *repoCopy).Return(repoCopy, nil)

		// Call method
		err := componentImpl.DetectRepoSensitiveCheckStatus(ctx, repoType, namespace, name)

		// Assertions
		require.NoError(t, err)
		require.Equal(t, types.SensitiveCheckPass, repo.SensitiveCheckStatus)

	})
}
