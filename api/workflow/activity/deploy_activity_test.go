package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockbuilder "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagebuilder"
	mockrunner "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagerunner"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
)

type testEnv struct {
	ctx        context.Context
	activities *DeployActivity

	// Mock dependencies
	mockDeployTaskStore   *mockdb.MockDeployTaskStore
	mockSpaceStore        *mockdb.MockSpaceStore
	mockModelStore        *mockdb.MockModelStore
	mockTokenStore        *mockdb.MockAccessTokenStore
	mockUrsStore          *mockdb.MockUserResourcesStore
	mockRuntimeFrameworks *mockdb.MockRuntimeFrameworksStore
	mockImageBuilder      *mockbuilder.MockBuilder
	mockImageRunner       *mockrunner.MockRunner
	mockGitServer         *mock_git.MockGitServer
	mockLogReporter       *mockReporter.MockLogCollector
	mockConfig            *config.Config
	mockDeployCfg         common.DeployConfig
	mockClusterStore      *mockdb.MockClusterInfoStore
}

func setupTest(t *testing.T) *testEnv {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "test", "test")

	// Create mock dependencies
	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
	mockSpaceStore := mockdb.NewMockSpaceStore(t)
	mockModelStore := mockdb.NewMockModelStore(t)
	mockTokenStore := mockdb.NewMockAccessTokenStore(t)
	mockUrsStore := mockdb.NewMockUserResourcesStore(t)
	mockRuntimeFrameworks := mockdb.NewMockRuntimeFrameworksStore(t)
	mockImageBuilder := mockbuilder.NewMockBuilder(t)
	mockImageRunner := mockrunner.NewMockRunner(t)
	mockGitServer := mock_git.NewMockGitServer(t)
	mockLogReporter := mockReporter.NewMockLogCollector(t)
	mockConfig := &config.Config{}
	mockDeployCfg := common.BuildDeployConfig(mockConfig)
	mockClusterStore := mockdb.NewMockClusterInfoStore(t)

	// Create activities instance
	activities := &DeployActivity{
		cfg: mockDeployCfg,
		lr:  mockLogReporter,
		ib:  mockImageBuilder,
		ir:  mockImageRunner,
		gs:  mockGitServer,
		ds:  mockDeployTaskStore,
		ts:  mockTokenStore,
		ss:  mockSpaceStore,
		ms:  mockModelStore,
		rfs: mockRuntimeFrameworks,
		urs: mockUrsStore,
		cls: mockClusterStore,
	}

	return &testEnv{
		ctx:                   ctx,
		activities:            activities,
		mockDeployTaskStore:   mockDeployTaskStore,
		mockSpaceStore:        mockSpaceStore,
		mockModelStore:        mockModelStore,
		mockTokenStore:        mockTokenStore,
		mockUrsStore:          mockUrsStore,
		mockRuntimeFrameworks: mockRuntimeFrameworks,
		mockImageBuilder:      mockImageBuilder,
		mockImageRunner:       mockImageRunner,
		mockGitServer:         mockGitServer,
		mockLogReporter:       mockLogReporter,
		mockConfig:            mockConfig,
		mockDeployCfg:         mockDeployCfg,
		mockClusterStore:      mockClusterStore,
	}
}

// TestActivities_createBuildRequest tests the createBuildRequest method
func TestActivities_createBuildRequest(t *testing.T) {
	tester := setupTest(t)

	task := &database.DeployTask{
		Deploy: &database.Deploy{
			UserID: 1,
		},
	}

	repoInfo := common.RepoInfo{
		Path:       "org/space",
		SdkVersion: "6.2.0",
	}

	tester.mockTokenStore.EXPECT().FindByUID(tester.ctx, task.Deploy.UserID).Return(&database.AccessToken{
		Token: "test-token",
		User: &database.User{
			Username: "uname",
		},
	}, nil)

	tester.mockGitServer.EXPECT().GetRepoLastCommit(tester.ctx, gitserver.GetRepoLastCommitReq{
		RepoType:  types.RepositoryType(repoInfo.RepoType),
		Namespace: "org",
		Name:      "space",
		Ref:       task.Deploy.GitBranch,
	}).Return(&types.Commit{
		ID: "id",
	}, nil)

	r, err := tester.activities.createBuildRequest(tester.ctx, task, repoInfo)
	require.Nil(t, err)
	require.Equal(t, r.Sdk_version, repoInfo.SdkVersion)
	require.Equal(t, r.LastCommitID, "id")
}

// TestActivities_handleDeployError tests the handleDeployError method
func TestActivities_handleDeployError(t *testing.T) {
	tester := setupTest(t)

	// Create a mock task with deploy information
	testError := errors.New("test deploy error")
	deploy := &database.Deploy{}
	task := &database.DeployTask{
		Deploy: deploy,
	}

	// Setup expectations
	tester.mockDeployTaskStore.EXPECT().UpdateInTx(
		mock.Anything,
		[]string{"status"},
		[]string{"status", "message"},
		deploy,
		task,
	).Return(nil)

	// Call the method under test
	err := tester.activities.handleDeployError(task, testError)

	// Verify the results
	require.NoError(t, err)
}

// TestActivities_parseHardware tests the parseHardware method
func TestActivities_parseHardware(t *testing.T) {
	tester := setupTest(t)

	// Test cases
	testCases := []struct {
		hardwareType string
		hardware     string
	}{{
		hardwareType: "GPU:1:16Gi:1:RTX3090",
		hardware:     "gpu",
	}, {
		hardwareType: "CPU:4:32Gi",
		hardware:     "cpu",
	}, {
		hardwareType: "NVIDIA:4:32Gi",
		hardware:     "gpu",
	}}

	for _, tc := range testCases {
		t.Run(tc.hardwareType, func(t *testing.T) {
			// Call the method under test
			hardware := tester.activities.parseHardware(tc.hardwareType)

			// Verify the results
			require.Equal(t, tc.hardware, hardware)
		})
	}
}

// TestActivities_reportLog tests the reportLog method
func TestActivities_reportLog(t *testing.T) {
	tester := setupTest(t)

	// Test data
	logMsg := "Test log message"
	task := &database.DeployTask{}

	// Setup expectations for LogCollector
	tester.mockLogReporter.EXPECT().Report(mock.Anything).Return()

	// Call the method under test
	tester.activities.reportLog(logMsg, types.StepBuildFailed, task)

	// Verify that the mock was called with the correct parameters
	// (This is implicitly done by gomock when the test completes)
}

// TestActivities_createModelRepoInfo tests the createModelRepoInfo method
func TestActivities_createModelRepoInfo(t *testing.T) {
	tester := setupTest(t)

	// Test data
	deployID := int64(123)
	model := &database.Model{
		ID:           int64(789),
		RepositoryID: 0,
		Repository: &database.Repository{
			ID:     123,
			UserID: 0,
			User: database.User{
				Username: "testuser",
			},
			Path: "namespace/testrepo",
			Name: "testrepo",
		},
		LastUpdatedAt:   time.Time{},
		BaseModel:       "",
		ReportURL:       "",
		MediumRiskCount: 0,
		HighRiskCount:   0,
	}

	// Call the method under test
	repoInfo := tester.activities.createModelRepoInfo(model, deployID)

	// Verify the results
	require.Equal(t, model.Repository.ID, repoInfo.DeployID)
	require.Equal(t, model.Repository.Path, repoInfo.Path)
	require.Equal(t, model.Repository.Name, repoInfo.Name)
	require.Equal(t, model.Repository.User.Username, repoInfo.UserName)
}

// TestActivities_createSpaceRepoInfo tests the createSpaceRepoInfo method
func TestActivities_createSpaceRepoInfo(t *testing.T) {
	tester := setupTest(t)

	// Test data
	deployID := int64(123)
	space := &database.Space{
		ID: int64(456),
		Repository: &database.Repository{
			ID:   int64(789),
			Path: "test-org/test-space",
			Name: "test-space",
			User: database.User{
				Username: "testuser",
			},
		},
		Sdk:           "gradio",
		SdkVersion:    "3.40.0",
		DriverVersion: "450.80.02",
	}

	// Call the method under test
	repoInfo := tester.activities.createSpaceRepoInfo(space, deployID)

	// Verify the results
	require.Equal(t, space.Repository.Path, repoInfo.Path)
	require.Equal(t, space.Repository.Name, repoInfo.Name)
	require.Equal(t, space.Sdk, repoInfo.Sdk)
	require.Equal(t, space.SdkVersion, repoInfo.SdkVersion)
	require.Equal(t, space.DriverVersion, repoInfo.DriverVersion)
	require.Equal(t, space.ID, repoInfo.SpaceID)
	require.Equal(t, space.Repository.ID, repoInfo.RepoID)
	require.Equal(t, space.Repository.User.Username, repoInfo.UserName)
	require.Equal(t, deployID, repoInfo.DeployID)
	require.Equal(t, int64(0), repoInfo.ModelID)
	require.Equal(t, string(types.SpaceRepo), repoInfo.RepoType)

	// Verify HTTPCloneURL is set (we don't know the exact format, just check it's not empty)
	require.NotEmpty(t, repoInfo.HTTPCloneURL)
}

// TestGetHttpCloneURLWithToken tests the getHttpCloneURLWithToken function
func TestGetHttpCloneURLWithToken(t *testing.T) {
	tester := setupTest(t)

	// Test cases
	testCases := []struct {
		name         string
		httpCloneURL string
		username     string
		token        string
		expected     string
		description  string
	}{
		{
			name:         "HTTP URL with protocol",
			httpCloneURL: "http://github.com/user/repo.git",
			username:     "testuser",
			token:        "testtoken123",
			expected:     "http://testuser:testtoken123@github.com/user/repo.git",
			description:  "should add credentials to HTTP URL",
		},
		{
			name:         "HTTPS URL with protocol",
			httpCloneURL: "https://gitlab.com/group/project.git",
			username:     "gituser",
			token:        "accesstoken456",
			expected:     "https://gituser:accesstoken456@gitlab.com/group/project.git",
			description:  "should add credentials to HTTPS URL",
		},
		{
			name:         "URL without protocol",
			httpCloneURL: "github.com/user/repo.git",
			username:     "testuser",
			token:        "testtoken123",
			expected:     "github.com/user/repo.git",
			description:  "should return original URL when no protocol is present",
		},
		{
			name:         "Empty URL",
			httpCloneURL: "",
			username:     "testuser",
			token:        "testtoken123",
			expected:     "",
			description:  "should return empty string for empty URL",
		},
		{
			name:         "Only protocol URL",
			httpCloneURL: "http://",
			username:     "testuser",
			token:        "testtoken123",
			expected:     "http://testuser:testtoken123@",
			description:  "should handle URL with only protocol",
		},
		{
			name:         "URL with special characters in username and token",
			httpCloneURL: "https://bitbucket.org/repo.git",
			username:     "user.name",
			token:        "token-with-special-chars!@#",
			expected:     "https://user.name:token-with-special-chars!@#@bitbucket.org/repo.git",
			description:  "should handle special characters in username and token",
		},
		{
			name:         "URL with subdomain",
			httpCloneURL: "https://dev.github.com/user/repo.git",
			username:     "devuser",
			token:        "devtoken",
			expected:     "https://devuser:devtoken@dev.github.com/user/repo.git",
			description:  "should handle URLs with subdomains",
		},
		{
			name:         "Empty username and token",
			httpCloneURL: "https://github.com/repo.git",
			username:     "",
			token:        "",
			expected:     "https://:@github.com/repo.git",
			description:  "should handle empty username and token",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tester.activities.getHttpCloneURLWithToken(tc.httpCloneURL, tc.username, tc.token)
			require.Equal(t, tc.expected, result, tc.description)
		})
	}
}

func TestBuildFailed(t *testing.T) {
	tester := setupTest(t)

	deploy := &database.Deploy{
		ID:         1,
		SpaceID:    1,
		Status:     0,
		DeployName: "",
		UserID:     0,
		User:       &database.User{},
		ModelID:    0,
		RepoID:     0,
		Repository: &database.Repository{
			ID:                   0,
			UserID:               0,
			User:                 database.User{},
			Path:                 "test/test-repo",
			GitPath:              "test/test-repo",
			Name:                 "test-repo",
			Nickname:             "",
			Description:          "",
			Private:              false,
			Labels:               "",
			License:              "",
			Readme:               "",
			DefaultBranch:        "",
			LfsFiles:             []database.LfsFile{},
			Likes:                0,
			DownloadCount:        0,
			Downloads:            []database.RepositoryDownload{},
			Tags:                 []database.Tag{},
			Metadata:             database.Metadata{},
			Mirror:               database.Mirror{},
			RepositoryType:       "",
			HTTPCloneURL:         "",
			SSHCloneURL:          "",
			Source:               "",
			SyncStatus:           "",
			SensitiveCheckStatus: 0,
			MSPath:               "",
			CSGPath:              "",
			HFPath:               "",
			GithubPath:           "",
			LFSObjectsSize:       0,
			StarCount:            0,
			DeletedAt:            time.Time{},
			Migrated:             false,
			Hashed:               false,
		},
		RuntimeFramework: "",
		ContainerPort:    0,
		Annotation:       "",
		MinReplica:       0,
		MaxReplica:       0,
		SvcName:          "",
		Endpoint:         "",
		ClusterID:        "",
		SecureLevel:      0,
		Type:             0,
		Task:             "",
		UserUUID:         "",
		SKU:              "",
		OrderDetailID:    0,
		EngineArgs:       "",
		Variables:        "",
		Message:          "",
		Reason:           "",
	}

	buildTask := &database.DeployTask{
		ID:       1,
		TaskType: 0,
		Status:   0,
		Message:  "",
		DeployID: 0,
		Deploy:   deploy,
	}

	tester.mockDeployTaskStore.EXPECT().GetDeployTask(mock.Anything, mock.Anything).Return(buildTask, nil)
	tester.mockTokenStore.EXPECT().FindByUID(mock.Anything, mock.Anything).Return(&database.AccessToken{
		ID:     0,
		UserID: 0,
		Token:  "accesstoken456",
		User:   &database.User{},
	}, nil)

	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{}, nil)
	tester.mockDeployTaskStore.EXPECT().UpdateDeployTask(mock.Anything, mock.Anything).Return(nil).Maybe()
	tester.mockDeployTaskStore.EXPECT().UpdateInTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	tester.mockSpaceStore.EXPECT().ByID(mock.Anything, mock.Anything).Return(&database.Space{
		Repository:   deploy.Repository,
		ID:           1,
		Sdk:          "gradio",
		RepositoryID: deploy.Repository.ID,
	}, nil)
	tester.mockImageBuilder.EXPECT().Build(mock.Anything, mock.Anything).Return(errors.New("build failed"))
	tester.mockLogReporter.EXPECT().Report(mock.Anything).Return().Maybe()
	err := tester.activities.Build(tester.ctx, buildTask.ID)

	require.Contains(t, err.Error(), "build failed")
}

func TestDeploy(t *testing.T) {
	tester := setupTest(t)

	deploy := &database.Deploy{
		ID:      1,
		SpaceID: 1,
		User:    &database.User{},
		Repository: &database.Repository{
			User:      database.User{},
			Path:      "test/test-repo",
			GitPath:   "test/test-repo",
			Name:      "test-repo",
			LfsFiles:  []database.LfsFile{},
			Downloads: []database.RepositoryDownload{},
			Tags:      []database.Tag{},
			Metadata:  database.Metadata{},
			Mirror:    database.Mirror{},
		},
		SvcName:  "aaa",
		ImageID:  "aaa",
		Hardware: `{}`,
	}

	runTask := &database.DeployTask{
		ID:       1,
		TaskType: 0,
		Status:   0,
		Message:  "",
		DeployID: 0,
		Deploy:   deploy,
	}

	tester.mockDeployTaskStore.EXPECT().GetDeployTask(mock.Anything, mock.Anything).Return(runTask, nil)
	tester.mockTokenStore.EXPECT().FindByUID(mock.Anything, mock.Anything).Return(&database.AccessToken{
		ID:     0,
		UserID: 0,
		Token:  "accesstoken456",
		User:   &database.User{},
	}, nil)

	tester.mockDeployTaskStore.EXPECT().UpdateDeployTask(mock.Anything, mock.Anything).Return(nil).Maybe()
	tester.mockDeployTaskStore.EXPECT().UpdateInTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	tester.mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, mock.Anything).Return(deploy, nil)
	tester.mockSpaceStore.EXPECT().ByID(mock.Anything, mock.Anything).Return(&database.Space{
		Repository:   deploy.Repository,
		ID:           1,
		Sdk:          "gradio",
		RepositoryID: deploy.Repository.ID,
	}, nil)
	tester.mockImageRunner.EXPECT().Run(mock.Anything, mock.Anything).Return(&types.RunResponse{
		DeployID: 0,
		Code:     0,
		Message:  "",
	}, nil)
	tester.mockLogReporter.EXPECT().Report(mock.Anything).Return().Maybe()
	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{
		ID: "1234567",
	}, nil)
	tester.ctx = context.WithValue(tester.ctx, "test", "test")

	tester.mockClusterStore.EXPECT().FindNodeByClusterID(mock.Anything, runTask.Deploy.ClusterID).Return([]database.ClusterNode{
		{
			Name: "node1",
		},
	}, nil)

	err := tester.activities.Deploy(tester.ctx, runTask.ID)

	require.NoError(t, err)

}
