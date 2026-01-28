package lfssyncer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	mock_gitserver "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_database "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mock_s3 "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	mock_workflow "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
	mock_filter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/filter"
	mock_hook "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/hook"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/mirror/hook"
)

// Test suite for LfsSyncWorker
type LfsSyncWorkerTestSuite struct {
	suite.Suite
	worker *LfsSyncWorker
	mocks  *testLfsSyncWorkerMocks
	ctx    context.Context
}

type testLfsSyncWorkerMocks struct {
	mirrorStore        *mock_database.MockMirrorStore
	mirrorTaskStore    *mock_database.MockMirrorTaskStore
	lfsMetaObjectStore *mock_database.MockLfsMetaObjectStore
	repoStore          *mock_database.MockRepoStore
	ossClient          *mock_s3.MockClient
	ossCore            *mock_s3.MockCore
	syncCache          *mock_cache.MockCache
	msgSender          *mock_hook.MockMessageSender
	recomComponent     *mock_component.MockRecomComponent
	filter             *mock_filter.MockFilter
	git                *mock_gitserver.MockGitServer
	workflowClient     *mock_workflow.MockClient
}

// Test data factories
func createTestConfig() *config.Config {
	// Initialize empty config
	cfg := &config.Config{}

	// Set API token
	cfg.APIToken = "test-token"

	// Configure Mirror settings
	cfg.Mirror.PartSize = 5 // 5MB
	cfg.Mirror.LfsConcurrency = 2

	// Configure S3 settings
	cfg.S3.Bucket = "test-bucket"

	// Configure Frontend settings
	cfg.Frontend.URL = "http://localhost:3000"

	// Configure Notification settings
	cfg.Notification.Host = "localhost"
	cfg.Notification.Port = 8080

	return cfg
}

func createTestRepository() *database.Repository {
	return &database.Repository{
		ID:             1,
		RepositoryType: types.ModelRepo,
		Path:           "test/repo",
		DefaultBranch:  "main",
	}
}

func createTestMirror(repo *database.Repository, sourceURL string) *database.Mirror {
	return &database.Mirror{
		ID:           1,
		RepositoryID: repo.ID,
		SourceUrl:    sourceURL,
		Repository:   repo,
		Priority:     types.HighMirrorPriority,
	}
}

func createTestMirrorTask(mirror *database.Mirror, status types.MirrorTaskStatus) *database.MirrorTask {
	return &database.MirrorTask{
		ID:                1,
		MirrorID:          mirror.ID,
		Status:            status,
		Mirror:            mirror,
		Priority:          types.HighMirrorPriority,
		AfterLastCommitID: "abc123",
		Progress:          0,
	}
}

func newTestLfsSyncWorker(t *testing.T, serverURL string) (*LfsSyncWorker, *testLfsSyncWorkerMocks) {
	cfg := createTestConfig()

	mocks := &testLfsSyncWorkerMocks{
		mirrorStore:        new(mock_database.MockMirrorStore),
		mirrorTaskStore:    new(mock_database.MockMirrorTaskStore),
		lfsMetaObjectStore: new(mock_database.MockLfsMetaObjectStore),
		repoStore:          new(mock_database.MockRepoStore),
		ossClient:          new(mock_s3.MockClient),
		ossCore:            new(mock_s3.MockCore),
		syncCache:          new(mock_cache.MockCache),
		msgSender:          new(mock_hook.MockMessageSender),
		recomComponent:     new(mock_component.MockRecomComponent),
		filter:             new(mock_filter.MockFilter),
		git:                new(mock_gitserver.MockGitServer),
		workflowClient:     new(mock_workflow.MockClient),
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	if serverURL != "" {
		u, err := url.Parse(serverURL)
		require.NoError(t, err)
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(u),
			},
		}
	}

	worker := &LfsSyncWorker{
		id:                 1,
		wg:                 &sync.WaitGroup{},
		mirrorStore:        mocks.mirrorStore,
		mirrorTaskStore:    mocks.mirrorTaskStore,
		lfsMetaObjectStore: mocks.lfsMetaObjectStore,
		repoStore:          mocks.repoStore,
		ossClient:          mocks.ossClient,
		ossCore:            mocks.ossCore,
		config:             cfg,
		syncCache:          mocks.syncCache,
		msgSender:          mocks.msgSender,
		recomComponent:     mocks.recomComponent,
		repoFilter:         mocks.filter,
		git:                mocks.git,
		httpClient:         httpClient,
		workflowClient:     mocks.workflowClient,
		mu:                 sync.Mutex{},
	}

	return worker, mocks
}

// SetupTest initializes test suite
func (suite *LfsSyncWorkerTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.worker, suite.mocks = newTestLfsSyncWorker(suite.T(), "")
	suite.worker.SetContext(suite.ctx)
}

// Test basic worker functionality
func (suite *LfsSyncWorkerTestSuite) TestID() {
	assert.Equal(suite.T(), 1, suite.worker.ID())
}

func (suite *LfsSyncWorkerTestSuite) TestSetContext() {
	newCtx := context.WithValue(context.Background(), "test", "value")
	suite.worker.SetContext(newCtx)
	assert.Equal(suite.T(), newCtx, suite.worker.ctx)
}

// Test Run method scenarios
func (suite *LfsSyncWorkerTestSuite) TestRun_MirrorNotFound() {
	task := createTestMirrorTask(&database.Mirror{ID: 999}, types.MirrorQueued)
	task.MirrorID = 999 // Non-existent mirror

	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).
		Return(nil, errors.New("mirror not found"))
	suite.mocks.mirrorTaskStore.EXPECT().Update(suite.ctx, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Status == types.MirrorLfsSyncFailed && mt.ErrorMessage == "mirror not found"
	})).Return(*task, nil)

	suite.worker.Run(task)

	assert.Equal(suite.T(), types.MirrorLfsSyncFailed, task.Status)
	assert.Equal(suite.T(), "mirror not found", task.ErrorMessage)
}

func (suite *LfsSyncWorkerTestSuite) TestRun_RepoNotFound() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorQueued)

	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(suite.ctx, mirror.RepositoryID).
		Return(nil, errors.New("repo not found"))

	suite.worker.Run(task)
	// Should return early without further processing
}

func (suite *LfsSyncWorkerTestSuite) TestRun_ShouldNotSync() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorQueued)

	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(suite.ctx, mirror.RepositoryID).Return(repo, nil)
	suite.mocks.filter.EXPECT().ShouldSync(suite.ctx, mirror.RepositoryID).
		Return(false, "repo too large", nil)

	// Expect priority to be lowered
	suite.mocks.mirrorTaskStore.EXPECT().Update(suite.ctx, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Priority == types.LowMirrorPriority
	})).Return(*task, nil)

	suite.mocks.mirrorStore.EXPECT().Update(suite.ctx, mock.MatchedBy(func(m *database.Mirror) bool {
		return m.Priority == types.LowMirrorPriority
	})).Return(nil)

	suite.mocks.msgSender.EXPECT().Send(suite.ctx, mock.MatchedBy(func(req types.MessageRequest) bool {
		return req.Scenario == types.MessageScenarioRepoSync
	})).Return(hook.Response{}, nil)

	suite.worker.Run(task)

	assert.Equal(suite.T(), types.LowMirrorPriority, task.Priority)
}

func (suite *LfsSyncWorkerTestSuite) TestRun_ContextCanceled() {
	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()
	suite.worker.SetContext(ctx)

	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)

	suite.mocks.mirrorStore.EXPECT().FindByID(ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(ctx, mirror.RepositoryID).Return(repo, nil)
	suite.mocks.filter.EXPECT().ShouldSync(ctx, mirror.RepositoryID).Return(true, "", nil)
	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.Anything).Return(hook.Response{}, nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(mock.Anything, repo.ID).Return(nil, context.Canceled)

	suite.mocks.mirrorTaskStore.EXPECT().UpdateStatusAndRepoSyncStatus(mock.Anything, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Status == types.MirrorCanceled
	}), types.SyncStatusCanceled).Return(*task, nil)

	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.Anything).Return(hook.Response{}, nil)
	suite.mocks.recomComponent.EXPECT().SetOpWeight(mock.Anything, mock.Anything, mock.Anything).Return(nil)

	suite.worker.Run(task)

	assert.Equal(suite.T(), types.MirrorCanceled, task.Status)
}

func (suite *LfsSyncWorkerTestSuite) TestRun_Success_NoLfsFiles() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorRepoSyncStart)

	commit := &types.Commit{
		ID: "abc123",
	}

	// Setup expectations
	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(suite.ctx, mirror.RepositoryID).Return(repo, nil)
	suite.mocks.filter.EXPECT().ShouldSync(suite.ctx, mirror.RepositoryID).Return(true, "", nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(mock.Anything, repo.ID).Return([]database.LfsMetaObject{}, nil)
	suite.mocks.git.EXPECT().GetDiffBetweenTwoCommits(mock.Anything, mock.Anything).Return(&types.GiteaCallbackPushReq{}, nil)

	// Message sending expectations
	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "start")
	})).Return(hook.Response{}, nil)

	// Git operations
	suite.mocks.git.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
		return req.Namespace == "test" && req.Name == "repo" && req.Ref == "main"
	})).Return(commit, nil).Times(2)

	suite.mocks.git.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
		return req.Namespace == "test" && req.Name == "repo" && req.NewObjectId == "abc123"
	})).Return(nil)

	// Workflow execution
	suite.mocks.workflowClient.EXPECT().ExecuteWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	// Final updates
	suite.mocks.mirrorTaskStore.EXPECT().UpdateStatusAndRepoSyncStatus(mock.Anything, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Status == types.MirrorRepoSyncFinished && mt.Progress == 100
	}), types.SyncStatusInProgress).Return(*task, nil)

	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "finished")
	})).Return(hook.Response{}, nil)

	suite.mocks.recomComponent.EXPECT().SetOpWeight(mock.Anything, mirror.RepositoryID, mock.Anything).Return(nil)

	// Execute
	suite.worker.Run(task)

	// Verify
	assert.Equal(suite.T(), types.MirrorRepoSyncFinished, task.Status)
	assert.Equal(suite.T(), 100, task.Progress)
}

func (suite *LfsSyncWorkerTestSuite) TestRun_SyncLfsError() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)

	expectedError := errors.New("database connection failed")

	// Setup expectations
	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(suite.ctx, mirror.RepositoryID).Return(repo, nil)
	suite.mocks.filter.EXPECT().ShouldSync(suite.ctx, mirror.RepositoryID).Return(true, "", nil)

	// Start message
	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "start")
	})).Return(hook.Response{}, nil)

	// Simulate error in getting LFS meta objects
	suite.mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(mock.Anything, repo.ID).Return(nil, expectedError)

	// Expect failure status update
	suite.mocks.mirrorTaskStore.EXPECT().UpdateStatusAndRepoSyncStatus(mock.Anything, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Status == types.MirrorLfsSyncFailed &&
			strings.Contains(mt.ErrorMessage, expectedError.Error())
	}), types.SyncStatusFailed).Return(*task, nil)

	// Failure message
	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "failed")
	})).Return(hook.Response{}, nil)

	suite.mocks.recomComponent.EXPECT().SetOpWeight(mock.Anything, mirror.RepositoryID, mock.Anything).Return(nil)

	// Execute
	suite.worker.Run(task)

	// Verify
	assert.Equal(suite.T(), types.MirrorLfsSyncFailed, task.Status)
	assert.Contains(suite.T(), task.ErrorMessage, expectedError.Error())
}

func (suite *LfsSyncWorkerTestSuite) TestRun_Success_HasLfsFiles() {
	repo := createTestRepository()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "objects/batch") {
			resp := types.LFSBatchResponse{
				Objects: []struct {
					Oid     string `json:"oid"`
					Size    int64  `json:"size"`
					Actions struct {
						Download *struct {
							Href string `json:"href"`
						} `json:"download"`
					} `json:"actions"`
				}{
					{
						Oid:  "oid1",
						Size: 1024,
						Actions: struct {
							Download *struct {
								Href string `json:"href"`
							} `json:"download"`
						}{
							Download: &struct {
								Href string `json:"href"`
							}{
								Href: server.URL + "/download/oid1",
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	mirror := createTestMirror(repo, server.URL+"/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorRepoSyncStart)

	commit := &types.Commit{
		ID: "abc123",
	}

	// Setup expectations
	suite.mocks.mirrorStore.EXPECT().FindByID(suite.ctx, task.MirrorID).Return(mirror, nil)
	suite.mocks.repoStore.EXPECT().FindById(suite.ctx, mirror.RepositoryID).Return(repo, nil)
	suite.mocks.filter.EXPECT().ShouldSync(suite.ctx, mirror.RepositoryID).Return(true, "", nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(mock.Anything, repo.ID).Return([]database.LfsMetaObject{
		{
			Oid:  "oid1",
			Size: 1024,
		},
	}, nil)
	suite.mocks.git.EXPECT().GetDiffBetweenTwoCommits(mock.Anything, mock.Anything).Return(&types.GiteaCallbackPushReq{}, nil)
	suite.mocks.ossClient.EXPECT().StatObject(mock.Anything, "test-bucket", "lfs/oi/d1", mock.Anything).Return(minio.ObjectInfo{Size: 1024}, nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().UpdateOrCreate(mock.Anything, mock.Anything).Return(nil, nil)

	suite.mocks.mirrorTaskStore.EXPECT().Update(mock.Anything, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.AfterLastCommitID == "abc123"
	})).Return(*task, nil)

	// Message sending expectations
	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "start")
	})).Return(hook.Response{}, nil)

	// Git operations
	suite.mocks.git.EXPECT().GetRepoLastCommit(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
		return req.Namespace == "test" && req.Name == "repo" && req.Ref == "main"
	})).Return(commit, nil).Times(2)

	suite.mocks.git.EXPECT().UpdateRef(mock.Anything, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
		return req.Namespace == "test" && req.Name == "repo" && req.NewObjectId == "abc123"
	})).Return(nil)

	// Workflow execution
	suite.mocks.workflowClient.EXPECT().ExecuteWorkflow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	// Final updates
	suite.mocks.mirrorTaskStore.EXPECT().UpdateStatusAndRepoSyncStatus(mock.Anything, mock.MatchedBy(func(mt database.MirrorTask) bool {
		return mt.Status == types.MirrorRepoSyncFinished && mt.Progress == 100
	}), types.SyncStatusInProgress).Return(*task, nil)

	suite.mocks.msgSender.EXPECT().Send(mock.Anything, mock.MatchedBy(func(req types.MessageRequest) bool {
		return strings.Contains(req.Parameters, "finished")
	})).Return(hook.Response{}, nil)

	suite.mocks.recomComponent.EXPECT().SetOpWeight(mock.Anything, mirror.RepositoryID, mock.Anything).Return(nil)

	// Execute
	suite.worker.Run(task)

	// Verify
	assert.Equal(suite.T(), types.MirrorRepoSyncFinished, task.Status)
	assert.Equal(suite.T(), 100, task.Progress)
}

// Test individual methods
func (suite *LfsSyncWorkerTestSuite) TestGetSyncPointers() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")

	tests := []struct {
		name          string
		lfsObjects    []database.LfsMetaObject
		expectedCount int
		expectedError bool
	}{
		{
			name:          "no lfs objects",
			lfsObjects:    []database.LfsMetaObject{},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "all existing objects",
			lfsObjects: []database.LfsMetaObject{
				{Oid: "oid1", Size: 100, Existing: true},
				{Oid: "oid2", Size: 200, Existing: true},
			},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "mixed existing and non-existing objects",
			lfsObjects: []database.LfsMetaObject{
				{Oid: "oid1", Size: 100, Existing: true},
				{Oid: "oid2", Size: 200, Existing: false},
				{Oid: "oid3", Size: 300, Existing: false},
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "database error",
			lfsObjects:    nil,
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Create a new worker and mocks for each test case
			worker, mocks := newTestLfsSyncWorker(t, "")

			if tt.expectedError {
				mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, errors.New("database error"))
			} else {
				mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(ctx, repo.ID).Return(tt.lfsObjects, nil)
			}

			pointers, err := worker.getSyncPointers(ctx, task)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "fail to get lfs meta objects")
			} else {
				assert.NoError(t, err)
				assert.Len(t, pointers, tt.expectedCount)
			}
		})
	}
}

func (suite *LfsSyncWorkerTestSuite) TestCheckIfLFSFileExists() {
	objectKey := "test-object-key"

	tests := []struct {
		name           string
		statError      error
		expectedExists bool
		expectedError  bool
	}{
		{
			name:           "file exists",
			statError:      nil,
			expectedExists: true,
			expectedError:  false,
		},
		{
			name:           "file not found",
			statError:      errors.New("NoSuchKey"),
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "other error",
			statError:      errors.New("network error"),
			expectedExists: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Create a new worker and mocks for each test case
			worker, mocks := newTestLfsSyncWorker(t, "")

			if tt.statError == nil {
				mocks.ossClient.EXPECT().StatObject(suite.ctx, worker.config.S3.Bucket, objectKey, mock.Anything).Return(minio.ObjectInfo{}, nil)
			} else {
				mocks.ossClient.EXPECT().StatObject(suite.ctx, worker.config.S3.Bucket, objectKey, mock.Anything).Return(minio.ObjectInfo{}, tt.statError)
			}

			exists, err := worker.CheckIfLFSFileExists(suite.ctx, objectKey)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedExists, exists)
		})
	}
}

func (suite *LfsSyncWorkerTestSuite) TestDownloadAndUploadLFSFile_FileExists() {
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")
	repo := createTestRepository()
	pointer := &types.Pointer{
		Oid:  "test-oid-123",
		Size: 1024,
	}

	// File already exists in S3
	suite.mocks.ossClient.EXPECT().StatObject(ctx, suite.worker.config.S3.Bucket, mock.Anything, mock.Anything).Return(minio.ObjectInfo{Size: 1024}, nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().UpdateOrCreate(ctx, mock.MatchedBy(func(lmo database.LfsMetaObject) bool {
		return lmo.Oid == pointer.Oid && lmo.Existing == true
	})).Return(&database.LfsMetaObject{}, nil)

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer)

	assert.NoError(suite.T(), err)
}

func (suite *LfsSyncWorkerTestSuite) TestDownloadAndUploadLFSFile_EmptyDownloadURL() {
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")
	repo := createTestRepository()
	pointer := &types.Pointer{
		Oid:         "test-oid-123",
		Size:        1024,
		DownloadURL: "", // Empty download URL
	}

	// File doesn't exist in S3
	suite.mocks.ossClient.EXPECT().StatObject(ctx, suite.worker.config.S3.Bucket, mock.Anything, mock.Anything).Return(minio.ObjectInfo{}, errors.New("NoSuchKey"))
	suite.mocks.lfsMetaObjectStore.EXPECT().UpdateOrCreate(ctx, mock.MatchedBy(func(lmo database.LfsMetaObject) bool {
		return lmo.Oid == pointer.Oid && lmo.Existing == false
	})).Return(&database.LfsMetaObject{}, nil)

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "pointer download url is empty")
}

func (suite *LfsSyncWorkerTestSuite) TestSendMessage() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")

	tests := []struct {
		name          string
		status        types.MirrorTaskStatus
		expectSend    bool
		sendError     error
		expectedError bool
	}{
		{
			name:          "successful message send",
			status:        types.MirrorLfsSyncStart,
			expectSend:    true,
			sendError:     nil,
			expectedError: false,
		},
		{
			name:          "message send error",
			status:        types.MirrorLfsSyncStart,
			expectSend:    true,
			sendError:     errors.New("network error"),
			expectedError: true,
		},
		{
			name:          "status not mapped",
			status:        types.MirrorTaskStatus("unknown_status"),
			expectSend:    false,
			sendError:     nil,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Create a new worker and mocks for each test case
			worker, mocks := newTestLfsSyncWorker(t, "")

			if tt.expectSend {
				mocks.msgSender.EXPECT().Send(suite.ctx, mock.MatchedBy(func(req types.MessageRequest) bool {
					return req.Scenario == types.MessageScenarioRepoSync
				})).Return(hook.Response{}, tt.sendError)
			}

			err := worker.sendMessage(suite.ctx, mirror, tt.status)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func (suite *LfsSyncWorkerTestSuite) TestDownloadRange() {
	// Create a test server that returns specific content for range requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return test data
		testData := []byte("test file content for range request")
		w.Header().Set("Content-Length", "10")
		w.Header().Set("Content-Range", "bytes 0-9/34")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(testData[:10])
	}))
	defer server.Close()

	resp, err := suite.worker.downloadRange(server.URL, 0, 9)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test file ", string(data))
}

func (suite *LfsSyncWorkerTestSuite) TestGetLFSDownloadURLs() {
	// Create a test server that returns LFS batch response
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "objects/batch") {
			resp := types.LFSBatchResponse{
				Objects: []struct {
					Oid     string `json:"oid"`
					Size    int64  `json:"size"`
					Actions struct {
						Download *struct {
							Href string `json:"href"`
						} `json:"download"`
					} `json:"actions"`
				}{
					{
						Oid:  "test-oid-123",
						Size: 100,
						Actions: struct {
							Download *struct {
								Href string `json:"href"`
							} `json:"download"`
						}{
							Download: &struct {
								Href string `json:"href"`
							}{
								Href: server.URL + "/download/test-oid-123",
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	pointers := []*types.Pointer{
		{Oid: "test-oid-123", Size: 100},
	}

	result, err := suite.worker.GetLFSDownloadURLs(suite.ctx, server.URL+"/test/repo.git", "main", pointers)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), server.URL+"/download/test-oid-123", result[0].DownloadURL)
}

// Test utility functions
func TestSplitPointersBySizeAndCount(t *testing.T) {
	tests := []struct {
		name     string
		pointers []*types.Pointer
		want     [][]*types.Pointer
	}{
		{
			name:     "empty pointers",
			pointers: []*types.Pointer{},
			want:     nil,
		},
		{
			name: "single small pointer",
			pointers: []*types.Pointer{
				{Oid: "1", Size: 100},
			},
			want: [][]*types.Pointer{
				{{Oid: "1", Size: 100}},
			},
		},
		{
			name: "pointers exceeding size limit",
			pointers: []*types.Pointer{
				{Oid: "1", Size: 5 * 1024 * 1024 * 1024}, // 5GB
				{Oid: "2", Size: 5 * 1024 * 1024 * 1024}, // 5GB
				{Oid: "3", Size: 5 * 1024 * 1024 * 1024}, // 5GB
			},
			want: [][]*types.Pointer{
				{
					{Oid: "1", Size: 5 * 1024 * 1024 * 1024},
					{Oid: "2", Size: 5 * 1024 * 1024 * 1024},
				},
				{
					{Oid: "3", Size: 5 * 1024 * 1024 * 1024},
				},
			},
		},
		{
			name: "pointers exceeding count limit",
			pointers: func() []*types.Pointer {
				pointers := make([]*types.Pointer, 20) // Exceeds MaxGroupCount (15)
				for i := 0; i < 20; i++ {
					pointers[i] = &types.Pointer{
						Oid:  string(rune('a' + i)),
						Size: 100,
					}
				}
				return pointers
			}(),
			want: func() [][]*types.Pointer {
				group1 := make([]*types.Pointer, 15)
				group2 := make([]*types.Pointer, 5)
				for i := 0; i < 15; i++ {
					group1[i] = &types.Pointer{
						Oid:  string(rune('a' + i)),
						Size: 100,
					}
				}
				for i := 0; i < 5; i++ {
					group2[i] = &types.Pointer{
						Oid:  string(rune('a' + 15 + i)),
						Size: 100,
					}
				}
				return [][]*types.Pointer{group1, group2}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitPointersBySizeAndCount(tt.pointers)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Benchmark tests
func BenchmarkSplitPointersBySizeAndCount(b *testing.B) {
	// Create test data
	pointers := make([]*types.Pointer, 100)
	for i := 0; i < 100; i++ {
		pointers[i] = &types.Pointer{
			Oid:  string(rune('a' + i%26)),
			Size: int64(i * 1024 * 1024), // Various sizes
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SplitPointersBySizeAndCount(pointers)
	}
}

// Integration tests using the test suite
func TestLfsSyncWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(LfsSyncWorkerTestSuite))
}

// Individual test functions for backward compatibility
func TestLfsSyncWorker_ID(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")
	assert.Equal(t, 1, worker.ID())
}

func TestLfsSyncWorker_SetContext(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")
	ctx := context.WithValue(context.Background(), "test", "value")

	worker.SetContext(ctx)

	assert.Equal(t, ctx, worker.ctx)
}

// Test error scenarios
func TestLfsSyncWorker_DownloadRange_InvalidURL(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	resp, err := worker.downloadRange("invalid-url", 0, 10)
	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestLfsSyncWorker_DownloadRange_ServerError(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	resp, err := worker.downloadRange(server.URL, 0, 10)
	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
}

// Test concurrent operations
func TestLfsSyncWorker_ConcurrentOperations(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	// Test that worker can handle concurrent context setting
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), "test", i)
			worker.SetContext(ctx)
		}(i)
	}
	wg.Wait()

	// Worker should still be functional
	assert.Equal(t, 1, worker.ID())
}
