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
			req.RepoType == repo.RepositoryType
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

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer)

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

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer)

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

	err := suite.worker.downloadAndUploadLFSFile(ctx, repo, pointer)

	assert.Nil(suite.T(), err)
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

	resp, err := suite.worker.downloadRange(suite.ctx, server.URL, 0, 9)

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

	resp, err := worker.downloadRange(context.Background(), "invalid-url", 0, 10)
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

	resp, err := worker.downloadRange(context.Background(), server.URL, 0, 10)
	if resp != nil {
		resp.Body.Close()
	}

	assert.Error(t, err)
}

func TestLfsSyncWorker_DownloadRange_NotFoundError(t *testing.T) {
	worker, _ := newTestLfsSyncWorker(t, "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resp, err := worker.downloadRange(context.Background(), server.URL, 0, 10)
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
	resp, err := worker.downloadRange(ctx, server.URL, 0, 10)
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

	resp, err := worker.downloadRange(context.Background(), server.URL, 0, 3)
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
