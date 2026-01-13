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
	mockgit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockSensit "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mocktypes "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/checker"
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
	cfg.Moderation.EncodedSensitiveWords = `5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`
	mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
	mockSensitiveChecker.EXPECT().PassTextCheck(mock.Anything, types.ScenarioCommentDetection, "test string").
		Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
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
