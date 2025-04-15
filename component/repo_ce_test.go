//go:build !saas

package component

import (
	"context"
	"errors"
	"testing"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRepoComponentImpl_Update(t *testing.T) {
	ctx := context.TODO()

	t.Run("Successfully update repository", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		updatedRepo := &database.Repository{
			ID:             1,
			Path:           "test-namespace/test-repo",
			Name:           "test-repo",
			Nickname:       "New Nickname",
			Description:    "New Description",
			DefaultBranch:  "main",
			Private:        false,
			RepositoryType: types.ModelRepo,
		}
		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(updatedRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		assert.Equal(t, updatedRepo, result)
	})

	// Test case 2: Repository does not exist
	t.Run("Repository does not exist", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		// Mock repository lookup failure
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "non-exist-repo").Return(nil, errors.New("repository not found"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace: "test-namespace",
			Name:      "non-exist-repo",
			Username:  "test-user",
			RepoType:  types.ModelRepo,
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "repository does not exist")
		assert.Nil(t, result)
	})

	// Test case 3: Namespace does not exist
	t.Run("Namespace does not exist", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:             1,
			Path:           "non-exist-namespace/test-repo",
			Name:           "test-repo",
			RepositoryType: types.ModelRepo,
		}

		// Mock successful repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "non-exist-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup failure
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "non-exist-namespace").Return(database.Namespace{}, errors.New("namespace not found"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace: "non-exist-namespace",
			Name:      "test-repo",
			Username:  "test-user",
			RepoType:  types.ModelRepo,
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "namespace does not exist")
		assert.Nil(t, result)
	})

	// Test case 4: Git server update failure
	t.Run("Git server update failure", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock Git server update failure
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(nil, errors.New("git server error"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fail to update repo in git")
		assert.Nil(t, result)
	})

	// Test case 5: Database update failure
	t.Run("Database update failure", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.ModelRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "model", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update failure
		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(nil, errors.New("database error"))

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.ModelRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fail to update repo in database")
		assert.Nil(t, result)
	})
	// Test case 6: allow public for dataset repo with sensitive check pending
	t.Run("allow public for dataset repo with sensitive check pending", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPending,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "dataset", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		newRepo := *mockRepo
		newRepo.Private = false
		newRepo.Nickname = "New Nickname"
		newRepo.Description = "New Description"

		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&newRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.DatasetRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		require.Equal(t, *mockRepo, *result)
	})
	// Test case 7: allow public for dataset repo with sensitive check pass
	t.Run("allow public for dataset repo with sensitive check pass", func(t *testing.T) {
		repo := initializeTestRepoComponent(ctx, t)
		mockRepo := &database.Repository{
			ID:                   1,
			Path:                 "test-namespace/test-repo",
			Name:                 "test-repo",
			Nickname:             "Old Nickname",
			Description:          "Old Description",
			DefaultBranch:        "main",
			Private:              true,
			RepositoryType:       types.DatasetRepo,
			SensitiveCheckStatus: types.SensitiveCheckPass,
		}

		mockNamespace := database.Namespace{
			Path:          "test-namespace",
			NamespaceType: database.UserNamespace,
		}

		// Mock repository lookup
		repo.mocks.stores.RepoMock().EXPECT().Find(ctx, "test-namespace", "dataset", "test-repo").Return(mockRepo, nil)

		// Mock namespace lookup
		repo.mocks.stores.NamespaceMock().EXPECT().FindByPath(ctx, "test-namespace").Return(mockNamespace, nil)

		// Mock user permission check
		mockUserRepoAdminPermission(ctx, repo.mocks.stores, "test-user")

		// Mock successful Git server update
		repo.mocks.gitServer.EXPECT().UpdateRepo(ctx, mock.Anything).Return(&gitserver.CreateRepoResp{}, nil)

		// Mock database update
		newRepo := *mockRepo
		newRepo.Private = false
		newRepo.Nickname = "New Nickname"
		newRepo.Description = "New Description"

		repo.mocks.stores.RepoMock().EXPECT().UpdateRepo(ctx, mock.Anything).Return(&newRepo, nil)

		// Execute test
		result, err := repo.UpdateRepo(ctx, types.UpdateRepoReq{
			Namespace:   "test-namespace",
			Name:        "test-repo",
			Username:    "test-user",
			RepoType:    types.DatasetRepo,
			Nickname:    tea.String("New Nickname"),
			Description: tea.String("New Description"),
			Private:     tea.Bool(false),
		})

		// Verify results
		require.NoError(t, err)
		require.Equal(t, *mockRepo, *result)
	})
}
