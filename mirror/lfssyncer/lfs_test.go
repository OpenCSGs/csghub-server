package lfssyncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/cache"
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
	mirrorTaskStore    *mock_database.MockMirrorTaskStore
	lfsMetaObjectStore *mock_database.MockLfsMetaObjectStore
	ossClient          *mock_s3.MockClient
	ossCore            *mock_s3.MockCore
	syncCache          *mock_cache.MockCache
	msgSender          *mock_hook.MockMessageSender
	git                *mock_gitserver.MockGitServer
	workflowClient     *mock_workflow.MockClient
}

// recordingMirrorTaskProgressStore captures task progress without requiring database setup.
type recordingMirrorTaskProgressStore struct {
	database.MirrorTaskStore
	mu       sync.Mutex
	progress []int
}

// UpdateProgress records one persisted task percentage.
func (s *recordingMirrorTaskProgressStore) UpdateProgress(_ context.Context, task database.MirrorTask) (database.MirrorTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = append(s.progress, task.Progress)
	return task, nil
}

// values returns a stable copy of recorded progress percentages.
func (s *recordingMirrorTaskProgressStore) values() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.progress...)
}

// roundTripFunc adapts a function into an HTTP round tripper for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip executes the adapted function.
func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// trackingReadCloser records whether an HTTP response body was closed.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

// Close records that the response body was closed.
func (b *trackingReadCloser) Close() error {
	b.closed = true
	return nil
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

// TestMirrorTaskByteProgressWeightsObjectSizes verifies file count does not influence task progress.
func TestMirrorTaskByteProgressWeightsObjectSizes(t *testing.T) {
	store := &recordingMirrorTaskProgressStore{}
	task := &database.MirrorTask{ID: 1}
	progress := newMirrorTaskByteProgress(task, store, 1_000, 0)

	require.NoError(t, progress.completeObject(context.Background(), "small", 100))
	require.NoError(t, progress.addObjectBytes(context.Background(), "large", 900, 450))
	require.NoError(t, progress.completeObject(context.Background(), "large", 900))

	require.Equal(t, []int{10, 55, 99}, store.values())
	require.Equal(t, 99, task.Progress)
}

// TestMirrorTaskByteProgressConcurrentCredits verifies concurrent parts stay monotonic and object bytes are capped.
func TestMirrorTaskByteProgressConcurrentCredits(t *testing.T) {
	store := &recordingMirrorTaskProgressStore{}
	task := &database.MirrorTask{ID: 1}
	progress := newMirrorTaskByteProgress(task, store, 1_000, 0)

	var group sync.WaitGroup
	errors := make(chan error, 12)
	for range 12 {
		group.Add(1)
		go func() {
			defer group.Done()
			errors <- progress.addObjectBytes(context.Background(), "large", 600, 100)
		}()
	}
	group.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.NoError(t, progress.completeObject(context.Background(), "small", 400))

	values := store.values()
	require.NotEmpty(t, values)
	for index := 1; index < len(values); index++ {
		require.Greater(t, values[index], values[index-1])
	}
	require.Equal(t, 99, values[len(values)-1])
	require.Equal(t, int64(600), progress.objectBytes["large"])
	require.Equal(t, int64(1_000), progress.uploadedBytes)
}

// TestMirrorTaskByteProgressDoesNotRegress verifies reconstructed retry progress cannot overwrite a higher persisted value.
func TestMirrorTaskByteProgressDoesNotRegress(t *testing.T) {
	store := &recordingMirrorTaskProgressStore{}
	task := &database.MirrorTask{ID: 1, Progress: 60}
	progress := newMirrorTaskByteProgress(task, store, 1_000, 500)

	require.NoError(t, progress.persistInitial(context.Background()))
	require.NoError(t, progress.addObjectBytes(context.Background(), "remaining", 500, 100))
	require.Empty(t, store.values())
	require.NoError(t, progress.addObjectBytes(context.Background(), "remaining", 500, 100))

	require.Equal(t, []int{70}, store.values())
	require.Equal(t, 70, task.Progress)
}

func newTestLfsSyncWorker(t *testing.T, serverURL string) (*LfsSyncWorker, *testLfsSyncWorkerMocks) {
	cfg := createTestConfig()

	mocks := &testLfsSyncWorkerMocks{
		mirrorTaskStore:    new(mock_database.MockMirrorTaskStore),
		lfsMetaObjectStore: new(mock_database.MockLfsMetaObjectStore),
		ossClient:          new(mock_s3.MockClient),
		ossCore:            new(mock_s3.MockCore),
		syncCache:          new(mock_cache.MockCache),
		msgSender:          new(mock_hook.MockMessageSender),
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
		mirrorTaskStore:    mocks.mirrorTaskStore,
		lfsMetaObjectStore: mocks.lfsMetaObjectStore,
		ossClient:          mocks.ossClient,
		ossCore:            mocks.ossCore,
		config:             cfg,
		syncCache:          mocks.syncCache,
		msgSender:          mocks.msgSender,
		git:                mocks.git,
		httpClient:         httpClient,
		workflowClient:     mocks.workflowClient,
		mu:                 sync.Mutex{},
	}

	return worker, mocks
}

// TestNewLfsSyncWorkerRejectsNonPositivePartSize verifies invalid multipart sizing fails before dependency initialization.
func TestNewLfsSyncWorkerRejectsNonPositivePartSize(t *testing.T) {
	for _, partSize := range []int{0, -1} {
		cfg := createTestConfig()
		cfg.Mirror.PartSize = partSize

		worker, err := NewLfsSyncWorker(cfg)

		require.Nil(t, worker)
		require.EqualError(t, err, fmt.Sprintf("LFS multipart part size must be positive: %d", partSize))
	}
}

// TestLoggerFromLFSContextIncludesTaskFields verifies every task-aware log can be attributed to one mirror execution.
func TestLoggerFromLFSContextIncludesTaskFields(t *testing.T) {
	var output bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)
	worker := &LfsSyncWorker{}
	ctx := worker.withLFSContext(context.Background(), task)

	loggerFromLFSContext(ctx).InfoContext(ctx, "test lfs log")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &entry))
	require.Equal(t, float64(mirror.ID), entry["mirror_id"])
	require.Equal(t, float64(task.ID), entry["mirror_task_id"])
	require.Equal(t, float64(repo.ID), entry["repository_id"])
	require.Equal(t, fmt.Sprintf("%ss/%s", repo.RepositoryType, repo.Path), entry["repo_path"])
	require.NotContains(t, entry, "workerID")
	require.NotContains(t, entry, "mirrorTaskID")
	require.NotContains(t, entry, "repoID")
	require.NotContains(t, entry, "repoPath")
}

// SetupTest initializes test suite
func (suite *LfsSyncWorkerTestSuite) SetupTest() {
	suite.ctx = context.Background()
	suite.worker, suite.mocks = newTestLfsSyncWorker(suite.T(), "")
}

func (suite *LfsSyncWorkerTestSuite) TestRefreshLfsMetaObjectsUsesAfterCommit() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)

	suite.mocks.git.EXPECT().GetRepoAllLfsPointers(suite.ctx, mock.MatchedBy(func(req gitserver.GetRepoAllFilesReq) bool {
		return req.Namespace == "test" &&
			req.Name == "repo" &&
			req.Ref == task.AfterLastCommitID &&
			req.RepoType == repo.RepositoryType &&
			req.RelativePath == repo.GitalyPath()
	})).Return([]*types.LFSPointer{
		{FileOid: "oid1", FileSize: 100},
		{FileOid: "oid1", FileSize: 100},
		{FileOid: "oid2", FileSize: 200},
	}, nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().BulkUpdateOrCreate(suite.ctx, repo.ID, mock.MatchedBy(func(input []database.LfsMetaObject) bool {
		return len(input) == 2 &&
			input[0].RepositoryID == repo.ID &&
			input[0].Oid == "oid1" &&
			input[0].Size == 100 &&
			!input[0].Existing &&
			input[1].RepositoryID == repo.ID &&
			input[1].Oid == "oid2" &&
			input[1].Size == 200 &&
			!input[1].Existing
	})).Return(nil)

	err := suite.worker.refreshLfsMetaObjects(suite.ctx, task)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(300), repo.LFSObjectsSize)
}

// TestSyncLfsUsesRepositoryRelativePath verifies commit publication and callback Git calls avoid path lookup.
func (suite *LfsSyncWorkerTestSuite) TestSyncLfsUsesRepositoryRelativePath() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)
	expectedRelativePath := repo.GitalyPath()

	suite.mocks.msgSender.EXPECT().Send(suite.ctx, mock.Anything).Return(hook.Response{}, nil)
	suite.mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(suite.ctx, repo.ID).
		Return([]database.LfsMetaObject{}, nil)
	suite.mocks.git.EXPECT().GetRepoLastCommit(suite.ctx, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
		return req.RelativePath == expectedRelativePath
	})).Return(&types.Commit{ID: "old-commit"}, nil).Once()
	suite.mocks.git.EXPECT().UpdateRef(suite.ctx, mock.MatchedBy(func(req gitserver.UpdateRefReq) bool {
		return req.RelativePath == expectedRelativePath
	})).Return(nil)
	suite.mocks.git.EXPECT().GetRepoLastCommit(suite.ctx, mock.MatchedBy(func(req gitserver.GetRepoLastCommitReq) bool {
		return req.RelativePath == expectedRelativePath
	})).Return(&types.Commit{ID: task.AfterLastCommitID}, nil).Once()
	suite.mocks.git.EXPECT().GetDiffBetweenTwoCommits(suite.ctx, mock.MatchedBy(func(req gitserver.GetDiffBetweenTwoCommitsReq) bool {
		return req.RelativePath == expectedRelativePath
	})).Return(&types.GiteaCallbackPushReq{}, nil)
	suite.mocks.workflowClient.EXPECT().ExecuteWorkflow(
		suite.ctx, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)

	err := suite.worker.SyncLfs(suite.ctx, task)

	require.NoError(suite.T(), err)
}

// Test individual methods
func (suite *LfsSyncWorkerTestSuite) TestGetSyncPointers() {
	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")

	tests := []struct {
		name             string
		lfsObjects       []database.LfsMetaObject
		storageExisting  map[string]bool
		expectedCount    int
		findError        error
		statError        error
		expectedError    bool
		expectedErrorMsg string
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
				{Oid: "oid1", Size: 100, Existing: false},
				{Oid: "oid2", Size: 200, Existing: false},
			},
			storageExisting: map[string]bool{
				"oid1": true,
				"oid2": true,
			},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "mixed existing and non-existing objects",
			lfsObjects: []database.LfsMetaObject{
				{Oid: "oid1", Size: 100, Existing: false},
				{Oid: "oid2", Size: 200, Existing: true},
				{Oid: "oid3", Size: 300, Existing: true},
			},
			storageExisting: map[string]bool{
				"oid1": true,
				"oid2": false,
				"oid3": false,
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:             "database error",
			lfsObjects:       nil,
			expectedCount:    0,
			findError:        errors.New("database error"),
			expectedError:    true,
			expectedErrorMsg: "fail to get lfs meta objects",
		},
		{
			name: "object storage check error",
			lfsObjects: []database.LfsMetaObject{
				{Oid: "oid1", Size: 100, Existing: false},
			},
			storageExisting:  nil,
			expectedCount:    0,
			statError:        errors.New("network error"),
			expectedError:    true,
			expectedErrorMsg: "failed to check if lfs file exists",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Create a new worker and mocks for each test case
			worker, mocks := newTestLfsSyncWorker(t, "")

			if tt.findError != nil {
				mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(ctx, repo.ID).Return(nil, tt.findError)
			} else {
				mocks.lfsMetaObjectStore.EXPECT().FindByRepoID(ctx, repo.ID).Return(tt.lfsObjects, nil)
				var existingOIDs []string
				var missingOIDs []string
				for _, obj := range tt.lfsObjects {
					if tt.statError != nil {
						mocks.ossClient.EXPECT().StatObject(ctx, "test-bucket", "lfs/"+obj.Oid[:2]+"/"+obj.Oid[2:], mock.Anything).Return(minio.ObjectInfo{}, tt.statError)
						continue
					}
					if tt.storageExisting[obj.Oid] {
						mocks.ossClient.EXPECT().StatObject(ctx, "test-bucket", "lfs/"+obj.Oid[:2]+"/"+obj.Oid[2:], mock.Anything).Return(minio.ObjectInfo{Size: obj.Size}, nil)
						existingOIDs = append(existingOIDs, obj.Oid)
					} else {
						mocks.ossClient.EXPECT().StatObject(ctx, "test-bucket", "lfs/"+obj.Oid[:2]+"/"+obj.Oid[2:], mock.Anything).Return(minio.ObjectInfo{}, errors.New("NoSuchKey"))
						missingOIDs = append(missingOIDs, obj.Oid)
					}
				}
				if len(tt.lfsObjects) > 0 && !tt.expectedError {
					mocks.lfsMetaObjectStore.EXPECT().BulkUpdateExistingByOIDs(ctx, repo.ID, existingOIDs, missingOIDs).Return(nil)
				}
			}

			pointers, err := worker.getSyncPointers(ctx, task)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Len(t, pointers, tt.expectedCount)
			}
		})
	}
}

func (suite *LfsSyncWorkerTestSuite) TestCheckIfLFSFileExists() {
	oid := "test-object-oid"

	tests := []struct {
		name           string
		statError      error
		objectSize     int64
		expectedSize   int64
		expectDelete   bool
		deleteError    error
		expectedExists bool
		expectedError  bool
	}{
		{
			name:           "file exists and size matches",
			statError:      nil,
			objectSize:     1024,
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: true,
			expectedError:  false,
		},
		{
			name:           "file exists but size mismatch",
			statError:      nil,
			objectSize:     2048,
			expectedSize:   1024,
			expectDelete:   true,
			deleteError:    nil,
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "file exists but size mismatch with delete error",
			statError:      nil,
			objectSize:     2048,
			expectedSize:   1024,
			expectDelete:   true,
			deleteError:    errors.New("delete error"),
			expectedExists: false,
			expectedError:  true,
		},
		{
			name:           "file not found",
			statError:      errors.New("NoSuchKey"),
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "file not found from minio error code",
			statError:      minio.ErrorResponse{Code: "NoSuchKey"},
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "file not found from empty minio error code",
			statError:      minio.ErrorResponse{StatusCode: http.StatusNotFound},
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: false,
			expectedError:  false,
		},
		{
			name:           "bucket not found is an error",
			statError:      minio.ErrorResponse{Code: "NoSuchBucket", StatusCode: http.StatusNotFound},
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: false,
			expectedError:  true,
		},
		{
			name:           "other error",
			statError:      errors.New("network error"),
			expectedSize:   1024,
			expectDelete:   false,
			expectedExists: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			// Create a new worker and mocks for each test case
			worker, mocks := newTestLfsSyncWorker(t, "")

			if tt.statError == nil {
				mocks.ossClient.EXPECT().StatObject(suite.ctx, worker.config.S3.Bucket, oid, mock.Anything).Return(minio.ObjectInfo{Size: tt.objectSize}, nil)
				if tt.expectDelete {
					mocks.ossClient.EXPECT().RemoveObject(suite.ctx, worker.config.S3.Bucket, oid, mock.Anything).Return(tt.deleteError)
				}
			} else {
				mocks.ossClient.EXPECT().StatObject(suite.ctx, worker.config.S3.Bucket, oid, mock.Anything).Return(minio.ObjectInfo{}, tt.statError)
			}

			exists, err := worker.CheckIfLFSFileExists(suite.ctx, oid, tt.expectedSize)

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

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer, nil)

	assert.NoError(suite.T(), err)
}

func (suite *LfsSyncWorkerTestSuite) TestDownloadAndUploadLFSFile_CheckExistsError() {
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")
	repo := createTestRepository()
	pointer := &types.Pointer{
		Oid:  "test-oid-123",
		Size: 1024,
	}

	// The metadata must not be updated when object storage verification fails.
	suite.mocks.ossClient.EXPECT().StatObject(ctx, suite.worker.config.S3.Bucket, mock.Anything, mock.Anything).Return(minio.ObjectInfo{}, errors.New("network error"))

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer, nil)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to check if lfs file exists")
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

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer, nil)

	assert.Nil(suite.T(), err)
}

// TestDownloadAndUploadLFSFileRejectsNonPositivePartSize verifies invalid sizing returns an error before division.
func (suite *LfsSyncWorkerTestSuite) TestDownloadAndUploadLFSFileRejectsNonPositivePartSize() {
	ctx := context.WithValue(suite.ctx, rk, "models/test/repo")
	repo := createTestRepository()
	pointer := &types.Pointer{
		Oid:         "test-oid-123",
		Size:        1024,
		DownloadURL: "https://example.com/object",
	}
	suite.worker.config.Mirror.PartSize = 0

	suite.mocks.ossClient.EXPECT().StatObject(ctx, suite.worker.config.S3.Bucket, mock.Anything, mock.Anything).Return(minio.ObjectInfo{}, errors.New("NoSuchKey"))
	suite.mocks.lfsMetaObjectStore.EXPECT().UpdateOrCreate(ctx, mock.MatchedBy(func(lmo database.LfsMetaObject) bool {
		return lmo.Oid == pointer.Oid && !lmo.Existing
	})).Return(&database.LfsMetaObject{}, nil)

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer, nil)

	require.EqualError(suite.T(), err, "LFS multipart part size must be positive: 0")
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

	resp, err := suite.worker.downloadRange(suite.ctx, server.URL, nil, 0, 9)

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
			username, accessToken, ok := r.BasicAuth()
			if !ok || username != "source-user" || accessToken != "source-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			resp := types.BatchResponse{
				Objects: []*types.ObjectResponse{{
					Pointer: types.Pointer{Oid: "test-oid-123", Size: 100},
					Actions: map[string]*types.Link{
						"download": {
							Href: server.URL + "/download/test-oid-123",
							Header: map[string]any{
								"Authorization": "Bearer download-token",
							},
						},
					},
				}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Header.Get("Authorization") != "Bearer download-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	pointers := []*types.Pointer{
		{Oid: "test-oid-123", Size: 100},
	}

	result, err := suite.worker.GetLFSDownloadURLs(
		suite.ctx, server.URL+"/test/repo.git", "main", "source-user", "source-token", pointers,
	)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), result, 1)
	assert.Equal(suite.T(), server.URL+"/download/test-oid-123", result[0].DownloadURL)
	assert.Equal(suite.T(), "Bearer download-token", result[0].DownloadHeaders.Get("Authorization"))

	downloadResp, err := suite.worker.downloadRange(
		suite.ctx, result[0].DownloadURL, result[0].DownloadHeaders, 0, 3,
	)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), downloadResp)
	defer downloadResp.Body.Close()
	assert.Equal(suite.T(), http.StatusPartialContent, downloadResp.StatusCode)
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
				pointers := make([]*types.Pointer, 20) // Exceeds maxGroupCount (15)
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

// Test error scenarios
func TestLfsSyncWorker_DownloadRange_InvalidURL(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	resp, err := worker.downloadRange(context.Background(), "invalid-url", nil, 0, 10)
	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
}

// TestLfsSyncWorker_DownloadRange_ServerError verifies rejected responses are closed and not returned.
func TestLfsSyncWorker_DownloadRange_ServerError(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")
	body := &trackingReadCloser{Reader: strings.NewReader("server error")}
	worker.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       body,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})

	resp, err := worker.downloadRange(context.Background(), "https://example.com/object", nil, 0, 10)
	if resp != nil {
		defer resp.Body.Close()
	}

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.True(t, body.closed)
}

// TestLfsSyncWorker_DownloadPartRefreshesForbiddenURL verifies closed error responses still trigger URL refresh.
func TestLfsSyncWorker_DownloadPartRefreshesForbiddenURL(t *testing.T) {
	worker, mocks := newTestLfsSyncWorker(t, "")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/expired":
			w.WriteHeader(http.StatusForbidden)
		case strings.HasSuffix(r.URL.Path, "/objects/batch"):
			_ = json.NewEncoder(w).Encode(types.BatchResponse{Objects: []*types.ObjectResponse{{
				Pointer: types.Pointer{Oid: "oid", Size: 4},
				Actions: map[string]*types.Link{"download": {Href: server.URL + "/fresh"}},
			}}})
		case r.URL.Path == "/fresh":
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte("data"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx := context.WithValue(context.Background(), suk, server.URL+"/repo.git")
	ctx = context.WithValue(ctx, dbk, "main")
	mocks.ossCore.EXPECT().PutObjectPart(
		ctx, "test-bucket", "object", "upload", 1, mock.Anything, int64(4), mock.Anything,
	).Return(minio.ObjectPart{PartNumber: 1}, nil).Once()

	part, err := worker.downloadAndUploadPartWithRetry(ctx, "upload", "object", 1, 0, 3, &types.Pointer{
		Oid: "oid", Size: 4, DownloadURL: server.URL + "/expired",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, part.PartNumber)
}

func TestAcquireUploadSlotStopsWhenContextCanceled(t *testing.T) {
	slots := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := acquireUploadSlot(ctx, slots)

	require.ErrorIs(t, err, context.Canceled)
}

func TestLfsSyncWorker_MultipartUploadRejectsNonPositiveConcurrency(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	err := worker.multipartUploadWithRetry(
		context.Background(), 4, "upload", "object", 0, 1,
		&types.Pointer{Oid: "oid", Size: 4, DownloadURL: "https://example.com/object"},
		nil,
	)

	require.ErrorContains(t, err, "concurrency must be positive")
}

// TestLfsSyncWorker_MultipartResumeUsesPersistedPartBytes verifies resumed progress uses actual part sizes.
func TestLfsSyncWorker_MultipartResumeUsesPersistedPartBytes(t *testing.T) {
	worker, mocks := newTestLfsSyncWorker(t, "")
	store := &recordingMirrorTaskProgressStore{}
	task := &database.MirrorTask{ID: 1}
	progress := newMirrorTaskByteProgress(task, store, 10, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	uploadedParts := minio.ListObjectPartsResult{ObjectParts: []minio.ObjectPart{{
		PartNumber: 3,
		Size:       2,
	}}}
	mocks.ossCore.EXPECT().ListObjectParts(ctx, "test-bucket", "object", "upload", 0, 0).
		Return(uploadedParts, nil).Once()
	mocks.syncCache.EXPECT().CacheLfsSyncFileProgress(ctx, int64(1), "oid", "5", 33).
		Return(nil).Once()

	err := worker.multipartUploadWithRetry(
		ctx, 4, "upload", "object", 2, 1,
		&types.Pointer{Oid: "oid", Size: 10, DownloadURL: "https://example.com/object"},
		progress,
	)

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, []int{20}, store.values())
	require.Equal(t, int64(2), progress.uploadedBytes)
}

// TestLfsSyncWorker_MultipartRejectsExcessPartsWithoutProgress verifies invalid resumed parts are never credited.
func TestLfsSyncWorker_MultipartRejectsExcessPartsWithoutProgress(t *testing.T) {
	worker, mocks := newTestLfsSyncWorker(t, "")
	store := &recordingMirrorTaskProgressStore{}
	progress := newMirrorTaskByteProgress(&database.MirrorTask{ID: 1}, store, 10, 0)
	uploadedParts := minio.ListObjectPartsResult{ObjectParts: []minio.ObjectPart{
		{PartNumber: 1, Size: 4},
		{PartNumber: 2, Size: 4},
		{PartNumber: 3, Size: 2},
		{PartNumber: 4, Size: 1},
	}}
	mocks.ossCore.EXPECT().ListObjectParts(mock.Anything, "test-bucket", "object", "upload", 0, 0).
		Return(uploadedParts, nil).Once()
	mocks.ossCore.EXPECT().AbortMultipartUpload(mock.Anything, "test-bucket", "object", "upload").
		Return(nil).Once()

	err := worker.multipartUploadWithRetry(
		context.Background(), 4, "upload", "object", 2, 1,
		&types.Pointer{Oid: "oid", Size: 10, DownloadURL: "https://example.com/object"},
		progress,
	)

	require.ErrorContains(t, err, "uploaded part count exceeds expected total")
	require.Empty(t, store.values())
	require.Zero(t, progress.uploadedBytes)
}

func TestLfsSyncWorker_DownloadPartStopsRetryWhenContextCanceled(t *testing.T) {
	worker, mocks := newTestLfsSyncWorker(t, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	mocks.ossCore.EXPECT().PutObjectPart(
		mock.Anything, "test-bucket", "object", "upload", 1, mock.Anything, int64(4), mock.Anything,
	).Run(func(ctx context.Context, bucket, object, uploadID string, partID int, data io.Reader, size int64, opts minio.PutObjectPartOptions) {
		cancel()
	}).Return(minio.ObjectPart{}, context.Canceled).Once()

	start := time.Now()
	_, err := worker.downloadAndUploadPartWithRetry(ctx, "upload", "object", 1, 0, 3, &types.Pointer{
		Oid: "oid", Size: 4, DownloadURL: server.URL,
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), time.Second)
}

func TestLfsSyncWorker_DownloadRange_NotFoundError(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resp, err := worker.downloadRange(context.Background(), server.URL, nil, 0, 10)
	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestLfsSyncWorker_DownloadRange_ContextCanceled(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	resp, err := worker.downloadRange(ctx, server.URL, nil, 0, 10)
	elapsed := time.Since(start)

	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
	assert.True(t, elapsed < 100*time.Millisecond,
		"downloadRange should fail fast on cancelled context, took %v", elapsed)
}

func TestLfsSyncWorker_DownloadRange_Success(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("data"))
	}))
	defer server.Close()

	resp, err := worker.downloadRange(context.Background(), server.URL, nil, 0, 3)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "data", string(body))
}

func TestLfsSyncWorker_DownloadAndUploadLFSFiles_ContextCanceled(t *testing.T) {
	worker, mocks := newTestLfsSyncWorker(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	ctx = context.WithValue(ctx, rk, "models/test/repo")
	ctx = context.WithValue(ctx, suk, "http://example.com")
	ctx = context.WithValue(ctx, dbk, "main")
	cancel()

	repo := createTestRepository()
	mirror := createTestMirror(repo, "http://example.com/test/repo.git")
	task := createTestMirrorTask(mirror, types.MirrorLfsSyncStart)

	pointerGroups := [][]*types.Pointer{
		{{Oid: "oid1", Size: 1024, DownloadURL: "http://example.com/file"}},
	}

	err := worker.downloadAndUploadLFSFiles(ctx, task, mirror, pointerGroups, repo)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"expected context.Canceled, got %v", err)
	_ = mocks // unused mocks but needed for worker setup
}

func TestLfsSyncWorker_DownloadAndUploadSmallFile_ContextCanceled(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := createTestRepository()
	pointer := &types.Pointer{
		Oid:         "oid1",
		Size:        1024,
		DownloadURL: "http://example.com/file",
	}

	err := worker.downloadAndUploadSmallFile(ctx, repo, pointer, "test-key")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"expected context.Canceled, got %v", err)
}
