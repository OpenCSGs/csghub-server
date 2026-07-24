package reposyncer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/hook"
)

// roundTripFunc adapts a function into an HTTP round tripper for request assertions.
type roundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip executes the wrapped request handler.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type MockMessageSender struct {
	mock.Mock
}

func (m *MockMessageSender) Send(ctx context.Context, message types.MessageRequest) (hook.Response, error) {
	args := m.Called(ctx, message)
	return args.Get(0).(hook.Response), args.Error(1)
}

func TestRepoSyncWorker_checkSourceURL(t *testing.T) {
	t.Run("check success", func(t *testing.T) {
		worker := &RepoSyncWorker{
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				username, accessToken, ok := req.BasicAuth()
				assert.Equal(t, "https", req.URL.Scheme)
				assert.Equal(t, types.HUGGINGFACE_HOST, req.URL.Hostname())
				assert.Equal(t, "/namespace/repo.git/info/refs", req.URL.Path)
				assert.True(t, ok)
				assert.Equal(t, "source-user", username)
				assert.Equal(t, "source-token", accessToken)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			})},
		}

		err := worker.checkSourceURL(context.Background(), "https://huggingface.co/namespace/repo.git", "source-user", "source-token")
		assert.NoError(t, err)
	})

	t.Run("skip non-huggingface url", func(t *testing.T) {
		requestSent := false
		worker := &RepoSyncWorker{
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestSent = true
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			})},
		}

		err := worker.checkSourceURL(context.Background(), "https://evil.example/huggingface.co/repo.git", "source-user", "source-token")
		assert.NoError(t, err)
		assert.False(t, requestSent)
	})

	t.Run("allow HTTP huggingface url", func(t *testing.T) {
		requestSent := false
		worker := &RepoSyncWorker{
			httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestSent = true
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			})},
		}

		err := worker.checkSourceURL(context.Background(), "http://huggingface.co/namespace/repo.git", "source-user", "source-token")
		assert.NoError(t, err)
		assert.True(t, requestSent)
	})
}

func TestRepoSyncWorker_SyncRepo(t *testing.T) {
	t.Run("check source url failed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockGit := mockgit.NewMockGitServer(t)
		mockSender := new(MockMessageSender)
		mockTaskStore := mockdb.NewMockMirrorTaskStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockPromptPrefixStore := mockdb.NewMockPromptPrefixStore(t)
		mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
		expectedRelativePath := "models_namespace/name.git"

		// Expectations
		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)

		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.MatchedBy(func(req gitserver.CheckRepoReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(true, nil)

		// getRepoLastCommit will be called before checkSourceURL
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&types.Commit{ID: "old-commit"}, nil).Once()
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.MatchedBy(func(req gitserver.MirrorSyncReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(nil)

		mockGit.EXPECT().GetRepo(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&gitserver.CreateRepoResp{
			DefaultBranch: "main",
		}, nil)

		// Second call to GetRepoLastCommit
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&types.Commit{ID: "new-commit"}, nil).Once()

		expectDescriptionGeneration(
			t,
			mockGit,
			mockRepoStore,
			mockPromptPrefixStore,
			mockLLMConfigStore,
			types.ModelRepo,
			"namespace",
			"name",
			"new-commit",
			nil,
		)

		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         mockRepoStore,
			promptPrefixStore: mockPromptPrefixStore,
			llmConfigStore:    mockLLMConfigStore,
			config:            cfg,
		}

		mirror := database.Mirror{
			SourceUrl: server.URL,
			Repository: &database.Repository{
				Path:           "namespace/name",
				RepositoryType: types.ModelRepo,
			},
		}
		mt := &database.MirrorTask{}

		result, err := worker.SyncRepo(context.Background(), &mirror, mt)
		assert.Nil(t, err)
		assert.Equal(t, "main", result.Mirror.Repository.DefaultBranch)
		assert.Equal(t, "old-commit", result.BeforeLastCommitID)
		assert.Equal(t, "new-commit", result.AfterLastCommitID)
	})

	t.Run("description generation error does not fail sync", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockGit := mockgit.NewMockGitServer(t)
		mockSender := new(MockMessageSender)
		mockTaskStore := mockdb.NewMockMirrorTaskStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockPromptPrefixStore := mockdb.NewMockPromptPrefixStore(t)
		mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)

		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)
		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.Anything).Return(true, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "same-commit"}, nil).Once()
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.Anything).Return(nil)
		mockGit.EXPECT().GetRepo(mock.Anything, mock.Anything).Return(&gitserver.CreateRepoResp{DefaultBranch: "main"}, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "same-commit"}, nil).Once()

		expectDescriptionGeneration(
			t,
			mockGit,
			mockRepoStore,
			mockPromptPrefixStore,
			mockLLMConfigStore,
			types.ModelRepo,
			"namespace",
			"name",
			"same-commit",
			assert.AnError,
		)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         mockRepoStore,
			promptPrefixStore: mockPromptPrefixStore,
			llmConfigStore:    mockLLMConfigStore,
			config:            cfg,
		}

		mirror := database.Mirror{
			SourceUrl: server.URL,
			Repository: &database.Repository{
				Path:           "namespace/name",
				RepositoryType: types.ModelRepo,
			},
		}
		mt := &database.MirrorTask{}

		result, err := worker.SyncRepo(context.Background(), &mirror, mt)
		assert.NoError(t, err)
		assert.Equal(t, "same-commit", result.BeforeLastCommitID)
		assert.Equal(t, "same-commit", result.AfterLastCommitID)
		assert.Equal(t, 0, mt.Progress)
	})

	t.Run("does not recalculate before commit after after checkpoint exists", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		mockGit := mockgit.NewMockGitServer(t)
		mockSender := new(MockMessageSender)
		mockTaskStore := mockdb.NewMockMirrorTaskStore(t)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockPromptPrefixStore := mockdb.NewMockPromptPrefixStore(t)
		mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)

		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)
		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.Anything).Return(true, nil)
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.Anything).Return(nil)
		mockGit.EXPECT().GetRepo(mock.Anything, mock.Anything).Return(&gitserver.CreateRepoResp{DefaultBranch: "main"}, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "after"}, nil).Once()

		expectDescriptionGeneration(
			t,
			mockGit,
			mockRepoStore,
			mockPromptPrefixStore,
			mockLLMConfigStore,
			types.ModelRepo,
			"namespace",
			"name",
			"after",
			nil,
		)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         mockRepoStore,
			promptPrefixStore: mockPromptPrefixStore,
			llmConfigStore:    mockLLMConfigStore,
			config:            cfg,
		}

		mirror := database.Mirror{
			SourceUrl: server.URL,
			Repository: &database.Repository{
				Path:           "namespace/name",
				RepositoryType: types.ModelRepo,
			},
		}
		mt := &database.MirrorTask{
			AfterLastCommitID: "after",
		}

		result, err := worker.SyncRepo(context.Background(), &mirror, mt)
		assert.NoError(t, err)
		assert.Empty(t, result.BeforeLastCommitID)
		assert.Equal(t, "after", result.AfterLastCommitID)
	})
}

// TestRepoSyncWorkerEnsureRepoExistsUsesRelativePath verifies repository creation reuses the loaded storage path.
func TestRepoSyncWorkerEnsureRepoExistsUsesRelativePath(t *testing.T) {
	mockGit := mockgit.NewMockGitServer(t)
	worker := &RepoSyncWorker{git: mockGit}
	const relativePath = "@hashed/ab/cd/repository.git"

	mockGit.EXPECT().RepositoryExists(mock.Anything, mock.MatchedBy(func(req gitserver.CheckRepoReq) bool {
		return req.RelativePath == relativePath
	})).Return(false, nil)
	mockGit.EXPECT().CreateRepo(mock.Anything, mock.MatchedBy(func(req gitserver.CreateRepoReq) bool {
		return req.RelativePath == relativePath
	})).Return(&gitserver.CreateRepoResp{}, nil)

	err := worker.ensureRepoExists(
		context.Background(), "namespace", "name", "main", types.ModelRepo, relativePath,
	)

	require.NoError(t, err)
}
