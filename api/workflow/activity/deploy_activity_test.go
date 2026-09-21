package activity

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	corev1 "k8s.io/api/core/v1"

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
	mockMetadataStore     *mockdb.MockMetadataStore
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
	mockMetadataStore := mockdb.NewMockMetadataStore(t)
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
		mds: mockMetadataStore,
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
		mockMetadataStore:     mockMetadataStore,
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

	tester.mockClusterStore.EXPECT().ByClusterID(tester.ctx, task.Deploy.ClusterID).Return(database.ClusterInfo{}, nil)

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
	tester.mockDeployTaskStore.EXPECT().GetLastTaskByType(mock.Anything, mock.Anything, mock.Anything).Return(task, nil)
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

	tester.mockClusterStore.EXPECT().ByClusterID(mock.Anything, mock.Anything).Return(database.ClusterInfo{}, nil)

	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{}, nil)
	tester.mockDeployTaskStore.EXPECT().UpdateDeployTask(mock.Anything, mock.Anything).Return(nil).Maybe()
	tester.mockDeployTaskStore.EXPECT().GetLastTaskByType(mock.Anything, mock.Anything, mock.Anything).Return(buildTask, nil)
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
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "foo",
								Operator: corev1.NodeSelectorOpIn,
								Values:   []string{"bar"},
							},
						},
					},
				},
			},
		},
		Tolerations: []types.Toleration{
			{
				Key:      "foo",
				Operator: "Equal",
				Value:    "bar",
				Effect:   "NoSchedule",
			},
		},
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
	tester.mockDeployTaskStore.EXPECT().GetLastTaskByType(mock.Anything, mock.Anything, mock.Anything).Return(runTask, nil)
	tester.mockDeployTaskStore.EXPECT().UpdateInTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	tester.mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, mock.Anything).Return(deploy, nil)
	tester.mockSpaceStore.EXPECT().ByID(mock.Anything, mock.Anything).Return(&database.Space{
		Repository:   deploy.Repository,
		ID:           1,
		Sdk:          "gradio",
		RepositoryID: deploy.Repository.ID,
	}, nil)
	tester.mockImageRunner.EXPECT().Run(mock.Anything, mock.MatchedBy(func(req *types.RunRequest) bool {
		if req.DeployExtend.NodeAffinity == nil || len(req.DeployExtend.Tolerations) == 0 {
			return false
		}
		return req.DeployExtend.Tolerations[0].Key == "foo"
	})).Return(&types.RunResponse{
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
	tester.mockClusterStore.EXPECT().ByClusterID(mock.Anything, mock.Anything).Return(database.ClusterInfo{}, nil)

	err := tester.activities.Deploy(tester.ctx, runTask.ID)

	require.NoError(t, err)

}

func TestDeploy_RuntimeFrameworkNotFound(t *testing.T) {
	tester := setupTest(t)

	deploy := &database.Deploy{
		ID:               1,
		SpaceID:          1,
		User:             &database.User{},
		ImageID:          "test-image-id",
		RuntimeFramework: "vllm",
		Hardware:         `{}`,
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
		SvcName: "aaa",
	}

	runTask := &database.DeployTask{
		ID:       1,
		TaskType: 0,
		Status:   0,
		DeployID: 1,
		Deploy:   deploy,
	}

	tester.mockDeployTaskStore.EXPECT().GetDeployTask(mock.Anything, mock.Anything).Return(runTask, nil)
	tester.mockLogReporter.EXPECT().Report(mock.Anything).Return().Maybe()
	tester.mockSpaceStore.EXPECT().ByID(mock.Anything, mock.Anything).Return(&database.Space{
		Repository:   deploy.Repository,
		ID:           1,
		Sdk:          "gradio",
		RepositoryID: deploy.Repository.ID,
	}, nil)
	tester.mockClusterStore.EXPECT().ByClusterID(mock.Anything, mock.Anything).Return(database.ClusterInfo{}, nil)
	tester.mockTokenStore.EXPECT().FindByUID(mock.Anything, mock.Anything).Return(&database.AccessToken{
		Token: "test-token",
		User:  &database.User{},
	}, nil)
	tester.mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, mock.Anything).Return(deploy, nil)
	tester.mockRuntimeFrameworks.EXPECT().FindByImageID(mock.Anything, mock.Anything).Return(nil, nil)

	err := tester.activities.Deploy(tester.ctx, runTask.ID)

	require.Error(t, err)
	require.Contains(t, err.Error(), "runtime framework not found")
}

func TestApplyToolCallParser(t *testing.T) {
	parsers := map[string]string{
		"Qwen3ForCausalLM":      "qwen",
		"LlamaForCausalLM":      "llama3",
		"DeepseekV3ForCausalLM": "deepseekv3",
	}

	tests := []struct {
		name       string
		engineArgs string
		modelArch  string
		want       string
	}{
		{
			name:       "vllm arch with mapped parser",
			engineArgs: "--max-model-len 8192 --enable-auto-tool-choice",
			modelArch:  "Qwen3ForCausalLM",
			want:       "--max-model-len 8192 --enable-auto-tool-choice --tool-call-parser qwen",
		},
		{
			name:       "vllm arch without mapping falls back to openai",
			engineArgs: "--enable-auto-tool-choice",
			modelArch:  "UnknownForCausalLM",
			want:       "--enable-auto-tool-choice --tool-call-parser openai",
		},
		{
			name:       "vllm unknown arch removes flag",
			engineArgs: "--foo --enable-auto-tool-choice --bar",
			modelArch:  "",
			want:       "--foo  --bar",
		},
		{
			name:       "sglang arch with mapped parser replaces auto",
			engineArgs: "--context-length 8192 --tool-call-parser auto",
			modelArch:  "DeepseekV3ForCausalLM",
			want:       "--context-length 8192 --tool-call-parser deepseekv3",
		},
		{
			name:       "sglang arch without mapping keeps auto",
			engineArgs: "--tool-call-parser auto",
			modelArch:  "UnknownForCausalLM",
			want:       "--tool-call-parser auto",
		},
		{
			name:       "sglang unknown arch keeps auto",
			engineArgs: "--tool-call-parser auto",
			modelArch:  "",
			want:       "--tool-call-parser auto",
		},
		{
			name:       "no tool call flags unchanged",
			engineArgs: "--max-model-len 8192",
			modelArch:  "Qwen3ForCausalLM",
			want:       "--max-model-len 8192",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyToolCallParser(slog.Default(), tt.engineArgs, tt.modelArch, parsers)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestVLLMEnforceEagerEnabled(t *testing.T) {
	tests := []struct {
		name       string
		engineArgs string
		want       bool
	}{
		{name: "empty engine args defaults to disabled", engineArgs: "", want: false},
		{name: "missing enforce-eager key defaults to disabled", engineArgs: `{"max-model-len":"8192"}`, want: false},
		{name: "disable turns off enforce-eager", engineArgs: `{"enforce-eager":"disable"}`, want: false},
		{name: "false turns off enforce-eager", engineArgs: `{"enforce-eager":"false"}`, want: false},
		{name: "zero turns off enforce-eager", engineArgs: `{"enforce-eager":"0"}`, want: false},
		{name: "empty value turns off enforce-eager", engineArgs: `{"enforce-eager":""}`, want: false},
		{name: "enable keeps enforce-eager on", engineArgs: `{"enforce-eager":"enable"}`, want: true},
		{name: "true keeps enforce-eager on", engineArgs: `{"enforce-eager":"true"}`, want: true},
		{name: "one keeps enforce-eager on", engineArgs: `{"enforce-eager":"1"}`, want: true},
		{name: "invalid json defaults to disabled", engineArgs: "not-json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, vllmEnforceEagerEnabled(tt.engineArgs))
		})
	}
}

func TestAddLongCatVideoRuntimeEnv(t *testing.T) {
	cfg := common.DeployConfig{
		S3AccessID:     "access-id",
		S3AccessSecret: "access-secret",
		S3Endpoint:     "oss.example.com",
		S3PublicBucket: "video-bucket",
		S3SSLEnabled:   true,
	}

	for _, runtimeFramework := range []string{"longcat-video", "amd-longcat-video"} {
		t.Run(runtimeFramework, func(t *testing.T) {
			envMap := map[string]string{}
			addLongCatVideoRuntimeEnv(envMap, runtimeFramework, "longcat-service", cfg)

			require.Equal(t, "longcat-service", envMap["LONGCAT_TASK_NAMESPACE"])
			require.Equal(t, "access-id", envMap["S3_ACCESS_ID"])
			require.Equal(t, "access-secret", envMap["S3_ACCESS_SECRET"])
			require.Equal(t, "video-bucket", envMap["S3_BUCKET"])
			require.Equal(t, "oss.example.com", envMap["S3_ENDPOINT"])
			require.Equal(t, "true", envMap["S3_SSL_ENABLED"])
		})
	}

	t.Run("empty bucket leaves storage disabled", func(t *testing.T) {
		envMap := map[string]string{}
		disabled := cfg
		disabled.S3PublicBucket = ""
		addLongCatVideoRuntimeEnv(envMap, "longcat-video", "longcat-service", disabled)
		require.Equal(t, map[string]string{"LONGCAT_TASK_NAMESPACE": "longcat-service"}, envMap)
	})

	t.Run("other runtime does not receive credentials", func(t *testing.T) {
		envMap := map[string]string{}
		addLongCatVideoRuntimeEnv(envMap, "vllm", "vllm-service", cfg)
		require.Empty(t, envMap)
	})
}

func TestMakeDeployEnv_Space(t *testing.T) {
	tests := []struct {
		name          string
		sdk           string
		wantSDK       string
		wantPort      string
		containerPort int
	}{
		{name: "gradio", sdk: types.GRADIO.Name, wantSDK: types.GRADIO.Name, wantPort: strconv.Itoa(types.GRADIO.Port)},
		{name: "streamlit", sdk: types.STREAMLIT.Name, wantSDK: types.STREAMLIT.Name, wantPort: strconv.Itoa(types.STREAMLIT.Port)},
		{name: "nginx", sdk: types.NGINX.Name, wantSDK: types.NGINX.Name, wantPort: strconv.Itoa(types.NGINX.Port)},
		{name: "docker", sdk: types.DOCKER.Name, wantSDK: types.DOCKER.Name, wantPort: "8080", containerPort: 8080},
		{name: "mcp_server", sdk: types.MCPSERVER.Name, wantSDK: types.MCPSERVER.Name, wantPort: strconv.Itoa(types.MCPSERVER.Port)},
		{name: "default", sdk: "unknown", wantSDK: "", wantPort: strconv.Itoa(types.DefaultContainerPort)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := setupTest(t)
			deployInfo := &database.Deploy{
				ID:            1,
				SpaceID:       1,
				Type:          types.SpaceType,
				SvcName:       "test-space",
				ContainerPort: tt.containerPort,
				Hardware:      `{}`,
				Env:           `{}`,
				Variables:     `{}`,
			}
			repoInfo := common.RepoInfo{
				Path:         "org/repo",
				Sdk:          tt.sdk,
				HTTPCloneURL: "https://git.example.com/org/repo.git",
				RepoType:     "space",
			}
			accessToken := &database.AccessToken{
				Token: "tok",
				User:  &database.User{Username: "user"},
			}

			tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "abc1234"}, nil)

			envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
				Hardware:    types.HardWare{},
				AccessToken: accessToken,
				DeployInfo:  deployInfo,
				RepoInfo:    repoInfo,
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantSDK, envMap["SDK"])
			require.Equal(t, tt.wantPort, envMap["port"])
			require.Equal(t, tester.mockDeployCfg.ModelDownloadEndpoint, envMap["HF_ENDPOINT"])
			require.Equal(t, "tok", envMap["ACCESS_TOKEN"])
			require.Equal(t, "org/repo", envMap["REPO_ID"])
			require.Equal(t, "abc1234", envMap["REVISION"])
		})
	}
}

func TestMakeDeployEnv_Inference(t *testing.T) {
	tester := setupTest(t)
	deployInfo := &database.Deploy{
		ID:            1,
		ModelID:       1,
		Type:          types.InferenceType,
		SvcName:       "test-infer",
		ContainerPort: 9000,
		Hardware:      `{"gpu":{"num":"1","type":"nvidia","resource_name":"nvidia.com/gpu"}}`,
		Env:           `{}`,
		Variables:     `{}`,
		EngineArgs:    `{"enforce-eager":"enable","async-scheduling":"disable"}`,
		Task:          "text-generation",
	}
	repoInfo := common.RepoInfo{
		Path:         "org/model",
		HTTPCloneURL: "https://git.example.com/org/model.git",
		RepoType:     "model",
	}
	accessToken := &database.AccessToken{
		Token: "tok",
		User:  &database.User{Username: "user"},
	}

	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "abc1234"}, nil)

	envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
		Hardware:    types.HardWare{Gpu: types.Processor{Num: "1", Type: "nvidia", ResourceName: "nvidia.com/gpu"}},
		AccessToken: accessToken,
		DeployInfo:  deployInfo,
		Runtime: runtimeConfig{
			EngineVersion: "0.10.0",
		},
		RepoInfo: repoInfo,
	})
	require.NoError(t, err)
	require.Equal(t, "9000", envMap["port"])
	require.Equal(t, "1", envMap["HF_HUB_OFFLINE"])
	require.Equal(t, "text-generation", envMap["HF_TASK"])
	require.Equal(t, "1", envMap["VLLM_ENFORCE_EAGER"])
	require.Equal(t, "true", envMap["ASYNC_SCHEDULING_DISABLED"])
	require.Equal(t, "0.10.0", envMap["ENGINE_VERSION"])
}

func TestMakeDeployEnv_Finetune(t *testing.T) {
	tester := setupTest(t)
	deployInfo := &database.Deploy{
		ID:            1,
		Type:          types.FinetuneType,
		SvcName:       "test-ft",
		ContainerPort: 8888,
		Hardware:      `{}`,
		Env:           `{}`,
		Variables:     `{}`,
	}
	repoInfo := common.RepoInfo{
		Path:         "org/dataset",
		HTTPCloneURL: "https://git.example.com/org/dataset.git",
		RepoType:     "dataset",
	}
	accessToken := &database.AccessToken{
		Token: "ft-token",
		User:  &database.User{Username: "user"},
	}

	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "abc1234"}, nil)

	envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
		Hardware:    types.HardWare{},
		AccessToken: accessToken,
		DeployInfo:  deployInfo,
		RepoInfo:    repoInfo,
	})
	require.NoError(t, err)
	require.Equal(t, "8888", envMap["port"])
	require.Contains(t, envMap["HF_ENDPOINT"], "csg")
	require.Equal(t, "ft-token", envMap["HF_TOKEN"])
	require.Equal(t, "1", envMap["USE_CSGHUB_MODEL"])
	require.Equal(t, "1", envMap["USE_CSGHUB_DATASET"])
	require.Equal(t, "yes", envMap["JUPYTER_ENABLE_LAB"])
	require.Equal(t, "xterm-256color", envMap["TERM"])
}

func TestMakeDeployEnv_Notebook(t *testing.T) {
	tester := setupTest(t)
	deployInfo := &database.Deploy{
		ID:            1,
		Type:          types.NotebookType,
		SvcName:       "test-nb",
		ContainerPort: 8888,
		Hardware:      `{}`,
		Env:           `{}`,
		Variables:     `{}`,
	}
	repoInfo := common.RepoInfo{
		Path: "org/notebook",
	}
	accessToken := &database.AccessToken{
		Token: "nb-token",
		User:  &database.User{Username: "user"},
	}

	// Notebook should NOT call GetRepoLastCommit
	envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
		Hardware:    types.HardWare{},
		AccessToken: accessToken,
		DeployInfo:  deployInfo,
		RepoInfo:    repoInfo,
	})
	require.NoError(t, err)
	require.Equal(t, "8888", envMap["port"])
	require.Empty(t, envMap["HTTPCloneURL"])
	require.Empty(t, envMap["REPO_ID"])
	require.Empty(t, envMap["REVISION"])
}

func TestMakeDeployEnv_EngineArgs(t *testing.T) {
	tests := []struct {
		name                string
		deployType          int
		deployEngineArgs    string
		engineArgsTemplates []types.EngineArg
		toolCallParsers     map[string]string
		repoID              int64
		wantContains        []string
		wantNotContains     []string
		wantEnvKey          string
		wantEnvValue        string
	}{
		{
			name:             "parameter priority and boolean skip",
			deployType:       types.SpaceType,
			deployEngineArgs: `{"max-model-len":"8192","enforce-eager":"true","enable-feature":"false"}`,
			engineArgsTemplates: []types.EngineArg{
				{Name: "max-model-len", Format: "--max-model-len %s"},
				{Name: "enforce-eager", Format: "--enforce-eager"},
				{Name: "enable-feature", Format: "--enable-feature"},
				{Name: "async-scheduling", Format: "--async-scheduling"},
			},
			wantContains:    []string{"--max-model-len 8192", "--enforce-eager"},
			wantNotContains: []string{"--async-scheduling", "--enable-feature"},
		},
		{
			name:             "tool-call parser injection",
			deployType:       types.InferenceType,
			deployEngineArgs: `{"enable-auto-tool-choice":"enable"}`,
			engineArgsTemplates: []types.EngineArg{
				{Name: "enable-auto-tool-choice", Format: "--enable-auto-tool-choice"},
			},
			toolCallParsers: map[string]string{"Qwen3ForCausalLM": "qwen"},
			repoID:          42,
			wantContains:    []string{"--enable-auto-tool-choice", "--tool-call-parser qwen"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tester := setupTest(t)
			deployInfo := &database.Deploy{
				ID:            1,
				SpaceID:       1,
				Type:          tt.deployType,
				RepoID:        tt.repoID,
				SvcName:       "test-svc",
				Hardware:      `{}`,
				Env:           `{}`,
				Variables:     `{}`,
				EngineArgs:    tt.deployEngineArgs,
				ContainerPort: 9000,
			}
			repoInfo := common.RepoInfo{
				Path:         "org/repo",
				HTTPCloneURL: "https://git.example.com/org/repo.git",
				RepoType:     "model",
			}
			accessToken := &database.AccessToken{
				Token: "tok",
				User:  &database.User{Username: "user"},
			}

			tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "abc1234"}, nil)
			if tt.repoID > 0 && len(tt.toolCallParsers) > 0 {
				tester.mockMetadataStore.EXPECT().FindByRepoID(mock.Anything, tt.repoID).Return(&database.Metadata{
					Architecture: "Qwen3ForCausalLM",
				}, nil)
			}

			envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
				Hardware:    types.HardWare{},
				AccessToken: accessToken,
				DeployInfo:  deployInfo,
				Runtime: runtimeConfig{
					EngineArgsTemplates: tt.engineArgsTemplates,
					ToolCallParsers:     tt.toolCallParsers,
				},
				RepoInfo: repoInfo,
			})
			require.NoError(t, err)
			for _, s := range tt.wantContains {
				require.Contains(t, envMap["ENGINE_ARGS"], s)
			}
			for _, s := range tt.wantNotContains {
				require.NotContains(t, envMap["ENGINE_ARGS"], s)
			}
		})
	}
}

func TestMakeDeployEnv_VariablesMerge(t *testing.T) {
	tester := setupTest(t)
	deployInfo := &database.Deploy{
		ID:        1,
		SpaceID:   1,
		Type:      types.SpaceType,
		SvcName:   "test-space",
		Hardware:  `{}`,
		Env:       `{"ENV_KEY":"env_value"}`,
		Variables: `{"VAR_KEY":"var_value","ENV_KEY":"overridden"}`,
	}
	repoInfo := common.RepoInfo{
		Path:         "org/repo",
		HTTPCloneURL: "https://git.example.com/org/repo.git",
		RepoType:     "space",
	}
	accessToken := &database.AccessToken{
		Token: "tok",
		User:  &database.User{Username: "user"},
	}

	tester.mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{ID: "abc1234"}, nil)

	envMap, err := tester.activities.makeDeployEnv(tester.ctx, makeDeployEnvRequest{
		Hardware:    types.HardWare{},
		AccessToken: accessToken,
		DeployInfo:  deployInfo,
		RepoInfo:    repoInfo,
	})
	require.NoError(t, err)
	// Variables should override env
	require.Equal(t, "overridden", envMap["ENV_KEY"])
	require.Equal(t, "var_value", envMap["VAR_KEY"])
}
