package component

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	tmocks "go.temporal.io/sdk/mocks"
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockSensit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mocktemporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mocktypes "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/checker"
	wfCommon "opencsg.com/csghub-server/moderation/workflow/common"
)

func TestRepoComponent_CheckRequestV2(t *testing.T) {
	t.Run("fail to check request sensitivity", func(t *testing.T) {
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), mock.Anything, mock.Anything).
			Return(nil, errors.New("fail to check request sensitivity")).Once()

		mockRequest := mocktypes.NewMockSensitiveRequestV2(t)
		mockRequest.EXPECT().GetSensitiveFields().Return([]types.SensitiveField{
			{
				Name: "chat",
				Value: func() string {
					return "chat1"
				},
				Scenario: types.ScenarioChatDetection,
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: types.ScenarioCommentDetection,
			},
		})

		repoComp := &repoComponentImpl{
			checker: mockSensitiveChecker,
		}

		_, err := repoComp.CheckRequestV2(context.Background(), mockRequest)
		require.ErrorContains(t, err, "fail to check request sensitivity")
	})

	t.Run("detect sensitive words", func(t *testing.T) {
		fields := []types.SensitiveField{
			{
				Name: "chat",
				Value: func() string {
					return "chat1"
				},
				Scenario: types.ScenarioChatDetection,
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: types.ScenarioCommentDetection,
			},
		}
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)

		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), fields[0].Scenario, fields[0].Value()).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
		// not pass
		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), fields[1].Scenario, fields[1].Value()).
			Return(&sensitive.CheckResult{IsSensitive: true}, nil).Once()

		mockRequest := mocktypes.NewMockSensitiveRequestV2(t)
		mockRequest.EXPECT().GetSensitiveFields().Return(fields)

		repoComp := &repoComponentImpl{
			checker: mockSensitiveChecker,
		}

		pass, err := repoComp.CheckRequestV2(context.Background(), mockRequest)
		require.ErrorContains(t, err, "found sensitive words in field: comment")
		require.False(t, pass)
	})

	t.Run("pass", func(t *testing.T) {
		fields := []types.SensitiveField{
			{
				Name: "chat",
				Value: func() string {
					return "chat1"
				},
				Scenario: types.ScenarioChatDetection,
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: types.ScenarioCommentDetection,
			},
		}
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)

		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), fields[0].Scenario, fields[0].Value()).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
		// not pass
		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), fields[1].Scenario, fields[1].Value()).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()

		mockRequest := mocktypes.NewMockSensitiveRequestV2(t)
		mockRequest.EXPECT().GetSensitiveFields().Return(fields)

		repoComp := &repoComponentImpl{
			checker: mockSensitiveChecker,
		}

		pass, err := repoComp.CheckRequestV2(context.Background(), mockRequest)
		require.Nil(t, err)
		require.True(t, pass)
	})
}

// unit test for func UpdateRepoSensitiveCheckStatus
func TestRepoComponent_UpdateRepoSensitiveCheckStatus(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	repoComp := &repoComponentImpl{
		rs: mockRepoStore,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(ctx, int64(1), types.SensitiveCheckFail).Return(nil)

	err := repoComp.UpdateRepoSensitiveCheckStatus(ctx, 1, types.SensitiveCheckFail)
	require.Nil(t, err)
}

func TestRepoComponent_SkipSensitiveCheckForWhiteList(t *testing.T) {
	ctx := context.Background()

	t.Run("namespace in whitelist should set skip status and return true", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRuleStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		repoComp := &repoComponentImpl{
			rs:            mockRepoStore,
			whitelistRule: mockRuleStore,
			config:        &config.Config{},
		}
		req := RepoFullCheckRequest{
			Namespace: "admin",
			Name:      "repo1",
			RepoType:  types.ModelRepo,
		}

		mockRuleStore.EXPECT().Exists(ctx, database.RuleTypeNamespace, req.Namespace).Return(true, nil).Once()
		mockRepoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{ID: 10}, nil).Once()
		mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(ctx, int64(10), types.SensitiveCheckSkip).Return(nil).Once()

		skipped, err := repoComp.SkipSensitiveCheckForWhiteList(ctx, req)
		require.NoError(t, err)
		require.True(t, skipped)
	})

	t.Run("namespace not in whitelist should return false", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRuleStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		repoComp := &repoComponentImpl{
			rs:            mockRepoStore,
			whitelistRule: mockRuleStore,
			config:        &config.Config{},
		}
		req := RepoFullCheckRequest{
			Namespace: "user1",
			Name:      "repo1",
			RepoType:  types.ModelRepo,
		}

		mockRuleStore.EXPECT().Exists(ctx, database.RuleTypeNamespace, req.Namespace).Return(false, nil).Once()

		skipped, err := repoComp.SkipSensitiveCheckForWhiteList(ctx, req)
		require.NoError(t, err)
		require.False(t, skipped)
	})
}

func TestRepoComponent_RepoFullCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("namespace in whitelist should return skipped result", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRuleStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		repoComp := &repoComponentImpl{
			rs:            mockRepoStore,
			whitelistRule: mockRuleStore,
			config:        &config.Config{},
		}
		req := RepoFullCheckRequest{
			Namespace: "admin",
			Name:      "repo1",
			RepoType:  types.ModelRepo,
		}

		mockRuleStore.EXPECT().Exists(ctx, database.RuleTypeNamespace, req.Namespace).Return(true, nil).Once()
		mockRepoStore.EXPECT().FindByPath(ctx, req.RepoType, req.Namespace, req.Name).Return(&database.Repository{ID: 10}, nil).Once()
		mockRepoStore.EXPECT().UpdateRepoSensitiveCheckStatus(ctx, int64(10), types.SensitiveCheckSkip).Return(nil).Once()

		result, err := repoComp.RepoFullCheck(ctx, req)
		require.NoError(t, err)
		require.True(t, result.Skipped)
		require.Empty(t, result.WorkflowID)
	})

	t.Run("namespace not in whitelist should start workflow", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRuleStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		cfg := &config.Config{}
		repoComp := &repoComponentImpl{
			rs:            mockRepoStore,
			whitelistRule: mockRuleStore,
			config:        cfg,
		}
		req := RepoFullCheckRequest{
			Namespace: "user1",
			Name:      "repo1",
			RepoType:  types.ModelRepo,
		}

		mockRuleStore.EXPECT().Exists(ctx, database.RuleTypeNamespace, req.Namespace).Return(false, nil).Once()
		mockWorkflowClient := mocktemporal.NewMockClient(t)
		temporal.Assign(mockWorkflowClient)
		workflowOptions := client.StartWorkflowOptions{
			TaskQueue: wfCommon.RepoFullCheckQueue,
		}
		workflowRun := tmocks.NewWorkflowRun(t)
		workflowRun.On("GetID").Return("wf-id").Once()
		mockWorkflowClient.EXPECT().ExecuteWorkflow(mock.Anything, workflowOptions, mock.Anything, wfCommon.Repo{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
		}, cfg).Return(workflowRun, nil).Once()

		result, err := repoComp.RepoFullCheck(ctx, req)
		require.NoError(t, err)
		require.False(t, result.Skipped)
		require.Equal(t, "wf-id", result.WorkflowID)
	})
}

func TestRepoComponent_CheckRepoFiles(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
	mockRepoFileCheckStore := mockdb.NewMockRepoFileCheckStore(t)
	mockGitServer := mockgit.NewMockGitServer(t)
	repoComp := &repoComponentImpl{
		rs:               mockRepoStore,
		rfs:              mockRepoFileStore,
		rfcs:             mockRepoFileCheckStore,
		git:              mockGitServer,
		concurrencyLimit: 10,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repoType := types.DatasetRepo
	name := "test-repo"
	repo := &database.Repository{
		ID:                   1,
		Name:                 name,
		Path:                 "test-namespace/test-repo",
		DefaultBranch:        "main",
		SensitiveCheckStatus: types.SensitiveCheckFail,
		RepositoryType:       repoType,
	}

	file1 := &database.RepositoryFile{
		ID:           1,
		RepositoryID: 1,
		Path:         "file1.txt",
		Repository:   repo,
	}

	file2 := &database.RepositoryFile{
		ID:           2,
		RepositoryID: 1,
		Path:         "file2.txt",
		Repository:   repo,
	}
	// The first batch returns two files
	mockRepoFileStore.EXPECT().BatchGet(mock.Anything, repo.ID, int64(0), int64(2)).Once().Return([]*database.RepositoryFile{file1, file2}, nil)
	mockRepoFileStore.EXPECT().BatchGet(mock.Anything, repo.ID, int64(2), int64(2)).Once().Return(nil, nil)
	mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoInfoByPathReq) bool {
		return req.Path == "file1.txt"
	})).Return(io.NopCloser(strings.NewReader("test string")), int64(len("test string")), nil).Once()

	mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoInfoByPathReq) bool {
		return req.Path == "file2.txt"
	})).Return(io.NopCloser(strings.NewReader("sensitive word")), int64(len("sensitive word")), nil).Once()

	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
	mockSensitiveChecker.EXPECT().PassTextCheck(mock.Anything, types.ScenarioCommentDetection, "test string").
		Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
	mockSensitiveChecker.EXPECT().PassTextCheck(mock.Anything, types.ScenarioCommentDetection, "sensitive word").
		Return(&sensitive.CheckResult{IsSensitive: true}, nil).Once()
	checker.InitWithContentChecker(cfg, mockSensitiveChecker)

	repoToUpdate := new(database.Repository)
	*repoToUpdate = *repo
	repoToUpdate.SensitiveCheckStatus = types.SensitiveCheckFail
	repoToUpdate.Private = true
	mockRepoStore.EXPECT().UpdateRepo(mock.Anything, *repoToUpdate).Return(repoToUpdate, nil).Once()

	// Use a channel to collect results concurrently without depending on call order
	results := make(chan database.RepositoryFileCheck, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	mockRepoFileCheckStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, rfc database.RepositoryFileCheck) error {
			defer wg.Done()
			results <- rfc
			return nil
		}).Twice()

	err := repoComp.CheckRepoFiles(ctx, repo.ID, CheckOption{
		BatchSize:  2,
		ForceCheck: true,
	})
	require.Nil(t, err)
	wg.Wait()
	close(results)
	// Assert results from the channel
	passFound := false
	failFound := false
	for rfc := range results {
		if rfc.RepoFileID == 1 {
			require.Equal(t, types.SensitiveCheckPass, rfc.Status)
			passFound = true
		}
		if rfc.RepoFileID == 2 {
			require.Equal(t, types.SensitiveCheckFail, rfc.Status)
			failFound = true
		}
	}

	require.True(t, passFound, "Check for passed file not found")
	require.True(t, failFound, "Check for failed file not found")
}

// TestRepoComponent_CheckRepoFiles_ImageByStream is a regression test for the
// 401 issue when the Aliyun moderation service fetches image files via the
// public download URL. Datasets require login to access file downloads even
// when public, so the remote checker gets a 401.
//
// processFile must NOT pass ImageURL to the file checker; image files must be
// checked by stream (upload to S3 + presigned URL) instead, regardless of
// whether the repo is public or private, or whether it is a dataset or not.
// Each subtest asserts that PassImageStreamCheck is called and
// PassImageURLCheck is never called.
func TestRepoComponent_CheckRepoFiles_ImageByStream(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		repo     *database.Repository
		imageExt string
	}{
		{
			name: "public dataset image checked by stream",
			repo: &database.Repository{
				ID:                   1,
				Name:                 "test-dataset",
				Path:                 "root/test-dataset",
				DefaultBranch:        "main",
				SensitiveCheckStatus: types.SensitiveCheckPass,
				RepositoryType:       types.DatasetRepo,
				Private:              false,
			},
			imageExt: "png",
		},
		{
			name: "private dataset image checked by stream",
			repo: &database.Repository{
				ID:                   2,
				Name:                 "private-dataset",
				Path:                 "root/private-dataset",
				DefaultBranch:        "main",
				SensitiveCheckStatus: types.SensitiveCheckPass,
				RepositoryType:       types.DatasetRepo,
				Private:              true,
			},
			imageExt: "jpg",
		},
		{
			name: "public model image checked by stream",
			repo: &database.Repository{
				ID:                   3,
				Name:                 "test-model",
				Path:                 "root/test-model",
				DefaultBranch:        "main",
				SensitiveCheckStatus: types.SensitiveCheckPass,
				RepositoryType:       types.ModelRepo,
				Private:              false,
			},
			imageExt: "jpeg",
		},
		{
			name: "private model image checked by stream",
			repo: &database.Repository{
				ID:                   4,
				Name:                 "private-model",
				Path:                 "root/private-model",
				DefaultBranch:        "main",
				SensitiveCheckStatus: types.SensitiveCheckPass,
				RepositoryType:       types.ModelRepo,
				Private:              true,
			},
			imageExt: "gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepoStore := mockdb.NewMockRepoStore(t)
			mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
			mockRepoFileCheckStore := mockdb.NewMockRepoFileCheckStore(t)
			mockGitServer := mockgit.NewMockGitServer(t)
			repoComp := &repoComponentImpl{
				rs:               mockRepoStore,
				rfs:              mockRepoFileStore,
				rfcs:             mockRepoFileCheckStore,
				git:              mockGitServer,
				concurrencyLimit: 10,
			}
			repoComp.config = &config.Config{}
			repoComp.config.APIServer.PublicDomain = "https://hub.opencsg.com"

			imageFile := &database.RepositoryFile{
				ID:           1,
				RepositoryID: tt.repo.ID,
				Path:         "screenshot." + tt.imageExt,
				FileType:     "file",
				Repository:   tt.repo,
			}

			// One batch returning the image file. Since count(1) < BatchSize(10),
			// the loop breaks after the first batch — no second batch call.
			mockRepoFileStore.EXPECT().BatchGetUnchcked(mock.Anything, tt.repo.ID, int64(0), int64(10)).Once().
				Return([]*database.RepositoryFile{imageFile}, nil)

			cfg := &config.Config{}
			cfg.SensitiveCheck.Enable = true
			cfg.SensitiveCheck.ImageCheckEnable = true
			mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
			// Stream check should be invoked (path that uploads to S3 +
			// presigned URL). The mock does not read from the reader, so the
			// lazy git reader (GetRepoFileReader) is never opened.
			mockSensitiveChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
				Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
			// URL check must never be called — this is the core of the
			// regression, for both public and private repos.
			mockSensitiveChecker.AssertNotCalled(t, "PassImageURLCheck", mock.Anything, mock.Anything, mock.Anything)
			checker.InitWithContentChecker(cfg, mockSensitiveChecker)

			mockRepoFileCheckStore.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()

			err := repoComp.CheckRepoFiles(ctx, tt.repo.ID, CheckOption{
				BatchSize: 10,
			})
			require.Nil(t, err)
		})
	}

	// "stream check reads actual content from git reader" verifies the
	// end-to-end path where the sensitive checker actually reads the image
	// stream from the git reader (GetRepoFileReader). This covers the review
	// feedback that the mock should read the stream instead of ignoring it.
	t.Run("stream check reads actual content from git reader", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoFileCheckStore := mockdb.NewMockRepoFileCheckStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)
		repoComp := &repoComponentImpl{
			rs:               mockRepoStore,
			rfs:              mockRepoFileStore,
			rfcs:             mockRepoFileCheckStore,
			git:              mockGitServer,
			concurrencyLimit: 10,
		}
		repoComp.config = &config.Config{}
		repoComp.config.APIServer.PublicDomain = "https://hub.opencsg.com"

		repo := &database.Repository{
			ID:                   1,
			Name:                 "test-dataset",
			Path:                 "root/test-dataset",
			DefaultBranch:        "main",
			SensitiveCheckStatus: types.SensitiveCheckPass,
			RepositoryType:       types.DatasetRepo,
			Private:              false,
		}
		imageFile := &database.RepositoryFile{
			ID:           1,
			RepositoryID: 1,
			Path:         "screenshot.png",
			FileType:     "file",
			Repository:   repo,
		}

		mockRepoFileStore.EXPECT().BatchGetUnchcked(mock.Anything, repo.ID, int64(0), int64(10)).Once().
			Return([]*database.RepositoryFile{imageFile}, nil)

		// Provide real image bytes via the git reader.
		imageContent := "png-image-bytes"
		mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoInfoByPathReq) bool {
			return req.Path == imageFile.Path
		})).Return(io.NopCloser(strings.NewReader(imageContent)), int64(len(imageContent)), nil).Once()

		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
		// The mock actually reads the stream and verifies the content matches
		// what GetRepoFileReader returned.
		mockSensitiveChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, r io.Reader) (*sensitive.CheckResult, error) {
				b, err := io.ReadAll(r)
				require.NoError(t, err)
				require.Equal(t, imageContent, string(b))
				return &sensitive.CheckResult{IsSensitive: false}, nil
			}).Once()
		mockSensitiveChecker.AssertNotCalled(t, "PassImageURLCheck", mock.Anything, mock.Anything, mock.Anything)
		checker.InitWithContentChecker(cfg, mockSensitiveChecker)

		mockRepoFileCheckStore.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()

		err := repoComp.CheckRepoFiles(ctx, repo.ID, CheckOption{
			BatchSize: 10,
		})
		require.Nil(t, err)
	})

	// "stream check retry reopens git reader with identical content" verifies
	// the full retry path: the first PassImageStreamCheck fails, checkByStream
	// retries by calling Seek(0) on RepoFileContentReader (which reopens the
	// git stream), and the second attempt succeeds. Both attempts must receive
	// the exact same image bytes. GetRepoFileReader is called twice (once per
	// open). This directly addresses the review feedback.
	t.Run("stream check retry reopens git reader with identical content", func(t *testing.T) {
		mockRepoStore := mockdb.NewMockRepoStore(t)
		mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
		mockRepoFileCheckStore := mockdb.NewMockRepoFileCheckStore(t)
		mockGitServer := mockgit.NewMockGitServer(t)
		repoComp := &repoComponentImpl{
			rs:               mockRepoStore,
			rfs:              mockRepoFileStore,
			rfcs:             mockRepoFileCheckStore,
			git:              mockGitServer,
			concurrencyLimit: 10,
		}
		repoComp.config = &config.Config{}
		repoComp.config.APIServer.PublicDomain = "https://hub.opencsg.com"

		repo := &database.Repository{
			ID:                   1,
			Name:                 "test-dataset",
			Path:                 "root/test-dataset",
			DefaultBranch:        "main",
			SensitiveCheckStatus: types.SensitiveCheckPass,
			RepositoryType:       types.DatasetRepo,
			Private:              false,
		}
		imageFile := &database.RepositoryFile{
			ID:           1,
			RepositoryID: 1,
			Path:         "screenshot.png",
			FileType:     "file",
			Repository:   repo,
		}

		mockRepoFileStore.EXPECT().BatchGetUnchcked(mock.Anything, repo.ID, int64(0), int64(10)).Once().
			Return([]*database.RepositoryFile{imageFile}, nil)

		imageContent := "png-image-bytes-for-retry"
		// GetRepoFileReader called twice: initial open + reopen after Seek.
		// Use RunAndReturn so each call returns a fresh reader (the previous
		// one is consumed/closed by the first attempt).
		mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, mock.MatchedBy(func(req gitserver.GetRepoInfoByPathReq) bool {
			return req.Path == imageFile.Path
		})).RunAndReturn(func(ctx context.Context, req gitserver.GetRepoInfoByPathReq) (io.ReadCloser, int64, error) {
			return io.NopCloser(strings.NewReader(imageContent)), int64(len(imageContent)), nil
		}).Twice()

		cfg := &config.Config{}
		cfg.SensitiveCheck.Enable = true
		cfg.SensitiveCheck.ImageCheckEnable = true
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)

		var contents []string
		var mu sync.Mutex
		mockSensitiveChecker.EXPECT().PassImageStreamCheck(mock.Anything, types.ScenarioImageBaseLineCheck, mock.Anything).
			RunAndReturn(func(ctx context.Context, scenario types.SensitiveScenario, r io.Reader) (*sensitive.CheckResult, error) {
				// Simulate chain layer: seek to start before reading. This
				// triggers RepoFileContentReader.Seek(0) which reopens the
				// git stream, so the retry gets full content.
				if seeker, ok := r.(io.Seeker); ok {
					_, _ = seeker.Seek(0, io.SeekStart)
				}
				b, _ := io.ReadAll(r)
				mu.Lock()
				contents = append(contents, string(b))
				mu.Unlock()
				// First attempt fails, forcing a retry.
				if len(contents) == 1 {
					return nil, errors.New("transient upload error")
				}
				return &sensitive.CheckResult{IsSensitive: false}, nil
			}).Twice()
		mockSensitiveChecker.AssertNotCalled(t, "PassImageURLCheck", mock.Anything, mock.Anything, mock.Anything)
		checker.InitWithContentChecker(cfg, mockSensitiveChecker)

		mockRepoFileCheckStore.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Once()

		err := repoComp.CheckRepoFiles(ctx, repo.ID, CheckOption{
			BatchSize: 10,
		})
		require.Nil(t, err)

		mu.Lock()
		defer mu.Unlock()
		require.Len(t, contents, 2, "expected 2 stream check attempts")
		for i, got := range contents {
			require.Equal(t, imageContent, got, "attempt %d: expected identical content", i)
		}
	})
}

func TestRepoComponent_GetNamespaceWhiteList(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		comp := &repoComponentImpl{
			whitelistRule: mockStore,
		}

		rules := []database.RepositoryFileCheckRule{
			{Pattern: "admin"},
			{Pattern: "test"},
		}

		mockStore.EXPECT().ListByRuleType(ctx, "namespace").Return(rules, nil).Once()

		patterns, err := comp.GetNamespaceWhiteList(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"admin", "test"}, patterns)
	})

	t.Run("error from store", func(t *testing.T) {
		mockStore := mockdb.NewMockRepositoryFileCheckRuleStore(t)
		comp := &repoComponentImpl{
			whitelistRule: mockStore,
		}

		expectedErr := errors.New("database error")
		mockStore.EXPECT().ListByRuleType(ctx, "namespace").Return(nil, expectedErr).Once()

		patterns, err := comp.GetNamespaceWhiteList(ctx)
		require.ErrorIs(t, err, expectedErr)
		require.Nil(t, patterns)
	})
}
