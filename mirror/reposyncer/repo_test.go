package reposyncer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/hook"
)

type MockMessageSender struct {
	mock.Mock
}

func (m *MockMessageSender) Send(ctx context.Context, message types.MessageRequest) (hook.Response, error) {
	args := m.Called(ctx, message)
	return args.Get(0).(hook.Response), args.Error(1)
}

func TestRepoSyncWorker_checkSourceURL(t *testing.T) {
	t.Run("check success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/info/refs" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		worker := &RepoSyncWorker{
			httpClient: server.Client(),
		}

		err := worker.checkSourceURL(context.Background(), server.URL)
		assert.NoError(t, err)
	})

	t.Run("check success - not huggingface url", func(t *testing.T) {
		worker := &RepoSyncWorker{
			httpClient: &http.Client{
				Timeout: 1 * time.Millisecond,
			},
		}

		// Do not check non-huggingface URLs
		err := worker.checkSourceURL(context.Background(), "http://invalid-url.local")
		assert.Nil(t, err)
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
		mockMirrorStore := mockdb.NewMockMirrorStore(t)
		mockLfsMetaObjectStore := mockdb.NewMockLfsMetaObjectStore(t)

		// Expectations
		mockSender.On("Send", mock.Anything, mock.Anything).Return(hook.Response{}, nil)

		mockGit.EXPECT().RepositoryExists(mock.Anything, mock.Anything).Return(true, nil)

		// getRepoLastCommit will be called before checkSourceURL
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "old-commit"}, nil).Once()
		mockGit.EXPECT().MirrorSync(mock.Anything, mock.Anything).Return(nil)

		mockGit.EXPECT().GetRepo(mock.Anything, mock.Anything).Return(&gitserver.CreateRepoResp{
			DefaultBranch: "main",
		}, nil)

		mockGit.EXPECT().GetRepoBranches(mock.Anything, mock.Anything).Return([]types.Branch{
			{Name: "main"},
		}, nil)

		mockGit.EXPECT().GetRepoAllLfsPointers(mock.Anything, mock.Anything).Return([]*types.LFSPointer{
			{FileSize: 100, FileOid: "oid"},
		}, nil)

		mockLfsMetaObjectStore.EXPECT().BulkUpdateOrCreate(mock.Anything, mock.Anything, mock.Anything).Return(nil)

		mockMirrorStore.EXPECT().UpdateMirrorAndRepository(mock.Anything, mock.Anything, mock.Anything).Return(nil)

		// Second call to GetRepoLastCommit
		mockGit.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "new-commit"}, nil).Once()

		mockGit.EXPECT().UpdateRef(mock.Anything, mock.Anything).Return(nil)

		cfg := &config.Config{}
		cfg.Frontend.URL = "http://localhost"

		worker := &RepoSyncWorker{
			httpClient:         server.Client(),
			git:                mockGit,
			msgSender:          mockSender,
			mirrorTaskStore:    mockTaskStore,
			mirrorStore:        mockMirrorStore,
			lfsMetaObjectStore: mockLfsMetaObjectStore,
			config:             cfg,
		}

		mirror := database.Mirror{
			SourceUrl: server.URL,
			Repository: &database.Repository{
				Path: "namespace/name",
			},
		}
		mt := &database.MirrorTask{}

		_, err := worker.SyncRepo(context.Background(), &mirror, mt)
		assert.Nil(t, err)
	})
}
