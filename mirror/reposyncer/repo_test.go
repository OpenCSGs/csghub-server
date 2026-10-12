package reposyncer

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mock_workflow "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
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

type testRepoDescriptionStore struct {
	database.RepoStore
}

func (s testRepoDescriptionStore) UpdateDescriptionIfEmpty(ctx context.Context, repoID int64, description string) (bool, error) {
	_, err := s.UpdateRepo(ctx, database.Repository{ID: repoID, Description: description})
	return err == nil, err
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
	t.Run("creates default SKILL.md when imported skill does not contain one", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		mockSender := new(MockMessageSender)
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockPromptPrefixStore := mockdb.NewMockPromptPrefixStore(t)
		mockLLMConfigStore := mockdb.NewMockLLMConfigStore(t)
		var refUpdates []string
		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)
		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.Anything).Return(true, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "initial-commit"}, nil).Once()
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.Anything).Return(nil)
		mockGit.EXPECT().GetRepo(mock.Anything, mock.Anything).Return(&gitserver.CreateRepoResp{DefaultBranch: "main"}, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "imported-commit"}, nil).Once()
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "namespace",
			Name:      "name",
			RepoType:  types.SkillRepo,
			Path:      "SKILL.md",
			Ref:       "imported-commit",
		}).Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "skill.md not found"), errorx.Ctx()))
		mockGit.EXPECT().CommitFiles(mock.Anything, mock.MatchedBy(func(req gitserver.CommitFilesReq) bool {
			if len(req.Files) != 1 {
				return false
			}
			decodedContent, err := base64.StdEncoding.DecodeString(req.Files[0].Content)
			var metadata map[string]string
			yamlErr := yaml.Unmarshal(decodedContent, &metadata)
			return err == nil && yamlErr == nil && metadata["name"] == "name" && metadata["description"] == "imported skill" &&
				req.Namespace == "namespace" && req.Name == "name" && req.RepoType == types.SkillRepo &&
				req.Revision == "main" && req.Message == "Add default SKILL.md" &&
				req.Files[0].Path == "SKILL.md" && req.Files[0].Action == gitserver.CommitActionCreate
		})).Return(nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "generated-skill-commit"}, nil).Once()
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.Ref == "refs/heads/main"
		})).Run(func(_ context.Context, req gitserver.UpdateRefReq) {
			refUpdates = append(refUpdates, req.NewObjectId)
		}).Return(nil).Times(3)
		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.Anything).Return([]*types.LFSPointer{{Oid: "lfs-oid"}}, nil)
		expectDescriptionGeneration(
			t, mockGit, mockRepoStore, mockPromptPrefixStore, mockLLMConfigStore,
			types.SkillRepo, "namespace", "name", "generated-skill-commit", nil,
		)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"
		worker := &RepoSyncWorker{
			git:               mockGit,
			msgSender:         mockSender,
			repoStore:         testRepoDescriptionStore{RepoStore: mockRepoStore},
			promptPrefixStore: mockPromptPrefixStore,
			llmConfigStore:    mockLLMConfigStore,
			config:            cfg,
		}
		mirror := database.Mirror{
			SourceUrl: "https://example.com/source.git",
			Repository: &database.Repository{
				Path:           "namespace/name",
				Name:           "name",
				Description:    "imported skill",
				RepositoryType: types.SkillRepo,
			},
		}
		mt := &database.MirrorTask{}

		result, err := worker.SyncRepo(context.Background(), &mirror, mt)

		require.NoError(t, err)
		require.Same(t, mt, result.Task)
		require.Equal(t, "generated-skill-commit", result.Task.AfterLastCommitID)
		require.Equal(t, []string{"initial-commit", "imported-commit", "initial-commit"}, refUpdates)
	})

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

		// Repository has LFS objects, so the sync takes the normal LFS path.
		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoAllFilesReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return([]*types.LFSPointer{{Oid: "lfs-oid", Size: 100}}, nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         testRepoDescriptionStore{RepoStore: mockRepoStore},
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
		require.NotNil(t, result.Task)
		assert.Equal(t, "main", result.Task.Mirror.Repository.DefaultBranch)
		assert.Equal(t, "old-commit", result.Task.BeforeLastCommitID)
		assert.Equal(t, "new-commit", result.Task.AfterLastCommitID)
		assert.False(t, result.NoLFS)
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

		// Repository has LFS objects, so the sync takes the normal LFS path.
		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.Anything).Return([]*types.LFSPointer{{Oid: "lfs-oid", Size: 100}}, nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         testRepoDescriptionStore{RepoStore: mockRepoStore},
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
		require.NotNil(t, result.Task)
		assert.Equal(t, "same-commit", result.Task.BeforeLastCommitID)
		assert.Equal(t, "same-commit", result.Task.AfterLastCommitID)
		assert.False(t, result.NoLFS)
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

		// Repository has LFS objects, so the sync takes the normal LFS path.
		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.Anything).Return([]*types.LFSPointer{{Oid: "lfs-oid", Size: 100}}, nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         testRepoDescriptionStore{RepoStore: mockRepoStore},
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
		require.NotNil(t, result.Task)
		assert.Empty(t, result.Task.BeforeLastCommitID)
		assert.Equal(t, "after", result.Task.AfterLastCommitID)
		assert.False(t, result.NoLFS)
	})

	t.Run("publishes commit and skips lfs when repo has no lfs", func(t *testing.T) {
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
		mockWorkflowClient := mock_workflow.NewMockClient(t)
		expectedRelativePath := "codes_namespace/name.git"

		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)
		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.MatchedBy(func(req gitserver.CheckRepoReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(true, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&types.Commit{ID: "old-commit"}, nil).Once()
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.MatchedBy(func(req gitserver.MirrorSyncReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(nil)
		mockGit.EXPECT().GetRepo(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&gitserver.CreateRepoResp{DefaultBranch: "main"}, nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return(&types.Commit{ID: "new-commit"}, nil).Once()

		expectDescriptionGeneration(
			t,
			mockGit,
			mockRepoStore,
			mockPromptPrefixStore,
			mockLLMConfigStore,
			types.CodeRepo,
			"namespace",
			"name",
			"new-commit",
			nil,
		)

		// Point HEAD to old commit before the fast path publishes the new one.
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.RelativePath == expectedRelativePath && req.NewObjectId == "old-commit"
		})).Return(nil)

		// No LFS objects: the fast path publishes the new commit and triggers the callback.
		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoAllFilesReq) bool {
			return req.RelativePath == expectedRelativePath
		})).Return([]*types.LFSPointer{}, nil)

		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.RelativePath == expectedRelativePath && req.NewObjectId == "new-commit"
		})).Return(nil)
		mockGit.EXPECT().GetDiffBetweenTwoCommits(mock.Anything, mock.MatchedBy(func(req gitserver.GetDiffBetweenTwoCommitsReq) bool {
			return req.RelativePath == expectedRelativePath && req.RightCommitId == "new-commit"
		})).Return(&types.GiteaCallbackPushReq{}, nil)
		mockWorkflowClient.EXPECT().ExecuteWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:        server.Client(),
			git:               mockGit,
			msgSender:         mockSender,
			mirrorTaskStore:   mockTaskStore,
			repoStore:         testRepoDescriptionStore{RepoStore: mockRepoStore},
			promptPrefixStore: mockPromptPrefixStore,
			llmConfigStore:    mockLLMConfigStore,
			workflowClient:    mockWorkflowClient,
			config:            cfg,
		}

		mirror := database.Mirror{
			SourceUrl: server.URL,
			Repository: &database.Repository{
				Path:           "namespace/name",
				RepositoryType: types.CodeRepo,
			},
		}
		mt := &database.MirrorTask{}

		result, err := worker.SyncRepo(context.Background(), &mirror, mt)
		assert.NoError(t, err)
		require.NotNil(t, result.Task)
		assert.Equal(t, "new-commit", result.Task.AfterLastCommitID)
		assert.True(t, result.NoLFS)
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

func TestRepoSyncWorkerEnsureSkillMetadataFile(t *testing.T) {
	t.Run("accepts skill metadata file", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		repository := &database.Repository{RepositoryType: types.SkillRepo}
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, gitserver.GetRepoInfoByPathReq{
			Namespace: "namespace",
			Name:      "name",
			RepoType:  types.SkillRepo,
			Path:      "SKILL.md",
			Ref:       "commit-id",
		}).Return("---\nname: example\ndescription: example skill\n---", nil)

		commit := &types.Commit{ID: "commit-id"}
		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: repository, namespace: "namespace", name: "name", branch: "main", commit: commit,
		})

		require.NoError(t, err)
		require.Same(t, commit, got)
	})

	t.Run("skips non-skill repository", func(t *testing.T) {
		worker := &RepoSyncWorker{git: mockgit.NewMockGitServer(t)}
		repository := &database.Repository{RepositoryType: types.CodeRepo}

		commit := &types.Commit{ID: "commit-id"}
		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: repository, namespace: "namespace", name: "name", branch: "main", commit: commit,
		})

		require.NoError(t, err)
		require.Same(t, commit, got)
	})

	t.Run("propagates skill metadata read failure", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		readErr := errorx.ErrGitGetTreeEntryFailed(assert.AnError, errorx.Ctx())
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).Return("", readErr)
		commit := &types.Commit{ID: "commit-id"}

		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main", commit: commit,
		})

		require.ErrorIs(t, err, readErr)
		require.Same(t, commit, got)
	})

	t.Run("does not generate metadata when file read fails with invalid argument", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		// GetRepoFileRaw also maps gRPC InvalidArgument to ErrGitFileNotFound.
		// Such a failure must be treated as a real read error, not as an absent
		// SKILL.md, so no commit is created on top of a broken state.
		readErr := errorx.GitFileNotFound(status.Error(codes.InvalidArgument, "invalid revision"), errorx.Ctx())
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).Return("", readErr)
		commit := &types.Commit{ID: "commit-id"}

		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main", commit: commit,
		})

		require.ErrorIs(t, err, readErr)
		require.Same(t, commit, got)
	})

	t.Run("reports commit and ref restoration failures", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		commitErr := errors.New("commit failed")
		restoreErr := errors.New("restore failed")
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).
			Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "skill.md not found"), errorx.Ctx()))
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "imported-commit"
		})).Return(nil)
		mockGit.EXPECT().CommitFiles(mock.Anything, mock.Anything).Return(commitErr)
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "previous-commit"
		})).Return(restoreErr)
		commit := &types.Commit{ID: "imported-commit"}

		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main",
			restoreCommitID: "previous-commit", commit: commit,
		})

		require.ErrorIs(t, err, commitErr)
		require.ErrorIs(t, err, restoreErr)
		require.Same(t, commit, got)
	})

	t.Run("reports latest commit and ref restoration failures", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		latestCommitErr := errors.New("latest commit failed")
		restoreErr := errors.New("restore failed")
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).
			Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "skill.md not found"), errorx.Ctx()))
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "imported-commit"
		})).Return(nil)
		mockGit.EXPECT().CommitFiles(mock.Anything, mock.Anything).Return(nil)
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(nil, latestCommitErr)
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "previous-commit"
		})).Return(restoreErr)
		commit := &types.Commit{ID: "imported-commit"}

		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main",
			restoreCommitID: "previous-commit", commit: commit,
		})

		require.ErrorIs(t, err, latestCommitErr)
		require.ErrorIs(t, err, restoreErr)
		require.Same(t, commit, got)
	})

	t.Run("attempts ref restoration when switching to imported commit fails", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		switchErr := errors.New("switch connection lost")
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).
			Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "skill.md not found"), errorx.Ctx()))
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "imported-commit"
		})).Return(switchErr)
		// The switch reported failure, but the server may have applied it;
		// the worker must still attempt to restore the previous ref using an
		// independent context.
		mockGit.EXPECT().UpdateRef(
			mock.MatchedBy(func(c context.Context) bool { return c.Err() == nil }),
			mock.MatchedBy(func(req gitserver.UpdateRefReq) bool { return req.NewObjectId == "previous-commit" }),
		).Return(nil)
		commit := &types.Commit{ID: "imported-commit"}

		got, err := worker.ensureSkillMetadataFile(context.Background(), skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main",
			restoreCommitID: "previous-commit", commit: commit,
		})

		require.ErrorIs(t, err, switchErr)
		require.Same(t, commit, got)
	})

	t.Run("restores ref with independent context after task cancellation", func(t *testing.T) {
		mockGit := mockgit.NewMockGitServer(t)
		worker := &RepoSyncWorker{git: mockGit}
		// Simulate a mirror task whose context is already cancelled, the most
		// common cause of a mid-generation failure.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).
			Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "skill.md not found"), errorx.Ctx()))
		mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
			return req.NewObjectId == "imported-commit"
		})).Return(nil)
		mockGit.EXPECT().CommitFiles(mock.Anything, mock.Anything).Return(context.Canceled)
		// Restoration must run on a fresh, non-cancelled context; matching the
		// ctx proves the compensation does not inherit the task cancellation.
		mockGit.EXPECT().UpdateRef(
			mock.MatchedBy(func(c context.Context) bool { return c.Err() == nil }),
			mock.MatchedBy(func(req gitserver.UpdateRefReq) bool { return req.NewObjectId == "previous-commit" }),
		).Return(nil)
		commit := &types.Commit{ID: "imported-commit"}

		got, err := worker.ensureSkillMetadataFile(ctx, skillMetadataEnsureRequest{
			repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
			namespace:  "namespace", name: "name", branch: "main",
			restoreCommitID: "previous-commit", commit: commit,
		})

		require.ErrorIs(t, err, context.Canceled)
		require.Same(t, commit, got)
	})
}

func TestBuildDefaultSkillMetadata(t *testing.T) {
	content, err := buildDefaultSkillMetadata("skill-name", "first line:\nsecond \"line\"")
	require.NoError(t, err)

	var metadata map[string]string
	require.NoError(t, yaml.Unmarshal([]byte(content), &metadata))
	require.Equal(t, "skill-name", metadata["name"])
	require.Equal(t, "first line:\nsecond \"line\"", metadata["description"])
}

func TestRepoSyncWorkerSkillRefRestoration(t *testing.T) {
	commitErr := errors.New("commit response lost")
	queryErr := errors.New("commit query failed")
	restoreErr := errors.New("restore failed")
	for _, tc := range []struct {
		name             string
		previous         string
		cancelAfterQuery bool
		commitErr        error
		queryErr         error
		restoreErr       error
	}{
		{name: "successful generation restores after cancellation", previous: "previous", cancelAfterQuery: true},
		{name: "unchanged import restores after successful generation", previous: "imported"},
		{name: "unchanged import restores after uncertain commit failure", previous: "imported", commitErr: commitErr},
		{name: "unchanged import restores after query failure", previous: "imported", queryErr: queryErr},
		{name: "successful generation reports restoration failure", previous: "previous", restoreErr: restoreErr},
		{name: "unchanged import preserves both failures", previous: "imported", commitErr: commitErr, restoreErr: restoreErr},
		{name: "initial import has no ref to restore"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockGit := mockgit.NewMockGitServer(t)
			worker := &RepoSyncWorker{git: mockGit}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			currentRef := tc.previous
			if currentRef == "" {
				currentRef = "imported"
			}
			mockGit.EXPECT().GetRepoFileRaw(mock.Anything, mock.Anything).
				Return("", errorx.GitFileNotFound(status.Error(codes.NotFound, "missing"), errorx.Ctx())).Once()
			if tc.previous != "" && tc.previous != "imported" {
				mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
					return req.NewObjectId == "imported"
				})).Run(func(context.Context, gitserver.UpdateRefReq) { currentRef = "imported" }).Return(nil).Once()
			}
			mockGit.EXPECT().CommitFiles(mock.Anything, mock.Anything).
				Run(func(context.Context, gitserver.CommitFilesReq) {
					require.Equal(t, "imported", currentRef)
					currentRef = "generated"
				}).Return(tc.commitErr).Once()
			if tc.commitErr == nil {
				mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).
					Run(func(context.Context, gitserver.GetRepoLastCommitReq) {
						require.Equal(t, "generated", currentRef)
						if tc.cancelAfterQuery {
							cancel()
						}
					}).Return(&types.Commit{ID: "generated"}, tc.queryErr).Once()
			}
			if tc.previous != "" {
				mockGit.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
					return req.NewObjectId == tc.previous
				})).Run(func(c context.Context, req gitserver.UpdateRefReq) {
					require.NoError(t, c.Err())
					_, bounded := c.Deadline()
					require.True(t, bounded)
					require.Equal(t, "generated", currentRef)
					if tc.restoreErr == nil {
						currentRef = req.NewObjectId
					}
				}).Return(tc.restoreErr).Once()
			}
			got, err := worker.ensureSkillMetadataFile(ctx, skillMetadataEnsureRequest{
				repository: &database.Repository{Path: "namespace/name", RepositoryType: types.SkillRepo},
				namespace:  "namespace", name: "name", branch: "main",
				restoreCommitID: tc.previous, commit: &types.Commit{ID: "imported"},
			})
			for _, expected := range []error{tc.commitErr, tc.queryErr, tc.restoreErr} {
				if expected != nil {
					require.ErrorIs(t, err, expected)
				}
			}
			if tc.commitErr == nil && tc.queryErr == nil && tc.restoreErr == nil {
				require.NoError(t, err)
			}
			if tc.commitErr == nil && tc.queryErr == nil {
				require.Equal(t, "generated", got.ID)
			}
			expectedRef := "generated"
			if tc.previous != "" && tc.restoreErr == nil {
				expectedRef = tc.previous
			}
			require.Equal(t, expectedRef, currentRef)
		})
	}
}
