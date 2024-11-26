package component

import (
	"context"
	"errors"
	"io"
	"strings"
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
				Scenario: string(sensitive.ScenarioChatDetection),
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: string(sensitive.ScenarioCommentDetection),
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
				Scenario: string(sensitive.ScenarioChatDetection),
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: string(sensitive.ScenarioCommentDetection),
			},
		}
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)

		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), sensitive.Scenario(fields[0].Scenario), fields[0].Value()).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
		// not pass
		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), sensitive.Scenario(fields[1].Scenario), fields[1].Value()).
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
				Scenario: string(sensitive.ScenarioChatDetection),
			},
			{
				Name: "comment",
				Value: func() string {
					return "comment1"
				},
				Scenario: string(sensitive.ScenarioCommentDetection),
			},
		}
		mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)

		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), sensitive.Scenario(fields[0].Scenario), fields[0].Value()).
			Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
		// not pass
		mockSensitiveChecker.EXPECT().PassTextCheck(context.Background(), sensitive.Scenario(fields[1].Scenario), fields[1].Value()).
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
	repoType := types.DatasetRepo
	namespace := "test-namespace"
	name := "test-repo"
	repo := &database.Repository{
		ID:                   1,
		Name:                 name,
		Path:                 "test-namespace/test-repo",
		DefaultBranch:        "main",
		SensitiveCheckStatus: types.SensitiveCheckFail,
	}
	mockRepoStore.EXPECT().FindByPath(ctx, repoType, namespace, name).Return(repo, nil)
	mockRepoStore.EXPECT().UpdateRepo(ctx, *repo).Return(repo, nil)
	err := repoComp.UpdateRepoSensitiveCheckStatus(ctx, repoType, namespace, name, types.SensitiveCheckFail)
	require.Nil(t, err)
}

// unit test for func CheckRepoFiles
func TestRepoComponent_CheckRepoFiles(t *testing.T) {
	mockRepoStore := mockdb.NewMockRepoStore(t)
	mockRepoFileStore := mockdb.NewMockRepoFileStore(t)
	mockRepoFileCheckStore := mockdb.NewMockRepoFileCheckStore(t)
	mockGitServer := mockgit.NewMockGitServer(t)
	repoComp := &repoComponentImpl{
		rs:   mockRepoStore,
		rfs:  mockRepoFileStore,
		rfcs: mockRepoFileCheckStore,
		git:  mockGitServer,
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
		SensitiveCheckStatus: types.SensitiveCheckFail,
		RepositoryType:       repoType,
	}
	mockRepoStore.EXPECT().FindByPath(mock.Anything, repoType, namespace, name).Return(repo, nil).Once()
	// loop once
	mockRepoFileStore.EXPECT().BatchGet(mock.Anything, repo.ID, int64(0), int64(1)).Return([]*database.RepositoryFile{
		{
			ID:              1,
			RepositoryID:    1,
			Path:            "file1.txt",
			FileType:        "file",
			Size:            1,
			LastModify:      time.Now(),
			CommitSha:       "sha1",
			LfsRelativePath: "",
			Branch:          "main",
			Repository:      repo,
		},
	}, nil).Once()
	//loop twice
	mockRepoFileStore.EXPECT().BatchGet(mock.Anything, repo.ID, int64(1), int64(1)).Return([]*database.RepositoryFile{
		{
			ID:              2,
			RepositoryID:    1,
			Path:            "file2.txt",
			FileType:        "file",
			Size:            1,
			LastModify:      time.Now(),
			CommitSha:       "sha2",
			LfsRelativePath: "",
			Branch:          "main",
			Repository:      repo,
		},
	}, nil).Once()
	//loop third
	mockRepoFileStore.EXPECT().BatchGet(mock.Anything, repo.ID, int64(2), int64(1)).Return([]*database.RepositoryFile{}, nil).Once()

	mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      name,
		Path:      "file1.txt",
		RepoType:  repoType,
		Ref:       "main",
	}).
		Return(io.NopCloser(strings.NewReader("test string")), int64(len("test string")), nil).Once()
	mockGitServer.EXPECT().GetRepoFileReader(mock.Anything, gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      name,
		Path:      "file2.txt",
		RepoType:  repoType,
		Ref:       "main",
	}).
		Return(io.NopCloser(strings.NewReader("sensitive word")), int64(len("sensitive word")), nil).Once()

	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.Moderation.EncodedSensitiveWords = `5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`
	mockSensitiveChecker := mockSensit.NewMockSensitiveChecker(t)
	mockSensitiveChecker.EXPECT().PassTextCheck(mock.Anything, sensitive.ScenarioCommentDetection, "test string").
		Return(&sensitive.CheckResult{IsSensitive: false}, nil).Once()
	checker.InitWithContentChecker(cfg, mockSensitiveChecker)

	repoToUpdate := new(database.Repository)
	*repoToUpdate = *repo
	repoToUpdate.SensitiveCheckStatus = types.SensitiveCheckFail
	repoToUpdate.Private = true
	mockRepoStore.EXPECT().UpdateRepo(mock.Anything, *repoToUpdate).Return(repoToUpdate, nil).Once()

	mockRepoFileCheckStore.EXPECT().Upsert(mock.Anything, mock.Anything).
		// Return(nil).Twice()
		RunAndReturn(func(ctx context.Context, rfc database.RepositoryFileCheck) error {
			if rfc.RepoFileID == 1 {
				require.True(t, rfc.Status == types.SensitiveCheckPass)
			}

			if rfc.RepoFileID == 2 {
				require.True(t, rfc.Status == types.SensitiveCheckFail)
			}
			return nil
		}).Twice()
	err := repoComp.CheckRepoFiles(ctx, repoType, namespace, name, CheckOption{
		BatchSize:  1,
		ForceCheck: true,
	})
	require.Nil(t, err)
}
