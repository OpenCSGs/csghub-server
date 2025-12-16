package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "go.temporal.io/api/common/v1"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/testsuite"
	"opencsg.com/csghub-server/api/workflow/activity"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"

	mockclient "opencsg.com/csghub-server/_mocks/go.temporal.io/sdk/client"
	mockbuilder "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagebuilder"
	mockrunner "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagerunner"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	mockReporter "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/reporter"
)

func TestIsWorkflowNotFoundError(t *testing.T) {
	// Test with nil error
	require.False(t, IsWorkflowNotFoundError(nil))

	// Test with non-not-found error
	require.False(t, IsWorkflowNotFoundError(errors.New("some other error")))

	// Test with various "not found" error messages
	testCases := []struct {
		name     string
		errorMsg string
		expected bool
	}{
		{"Simple not found", "not found", true},
		{"Workflow execution not found", "workflow execution not found", true},
		{"Workflow not found", "workflow not found", true},
		{"Error with prefix", "error: workflow not found", true},
		{"Error with suffix", "workflow execution not found: details", true},
		{"Mixed case", "WORKFLOW NOT FOUND", false}, // Case sensitive
		{"Partial match", "workflow not foun", false},
		{"Different error", "connection error", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsWorkflowNotFoundError(errors.New(tc.errorMsg))
			require.Equal(t, tc.expected, result, "Failed test case: %s", tc.name)
		})
	}
}

func TestDeployWorkflowSuccess(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	mockDeployTaskStore := mockdb.NewMockDeployTaskStore(t)
	mockSpaceStore := mockdb.NewMockSpaceStore(t)
	mockModelStore := mockdb.NewMockModelStore(t)
	mockTokenStore := mockdb.NewMockAccessTokenStore(t)
	mockUrsStore := mockdb.NewMockUserResourcesStore(t)
	mockRuntimeFrameworks := mockdb.NewMockRuntimeFrameworksStore(t)
	mockMetadataStore := mockdb.NewMockMetadataStore(t)
	mockImageBuilder := mockbuilder.NewMockBuilder(t)
	mockImageRunner := mockrunner.NewMockRunner(t)
	mockGitServer := mock_git.NewMockGitServer(t)
	mockLogReporter := mockReporter.NewMockLogCollector(t)
	mockConfig := &config.Config{}
	mockDeployCfg := common.BuildDeployConfig(mockConfig)
	act := activity.NewDeployActivity(
		mockDeployCfg,
		mockLogReporter,
		mockImageBuilder,
		mockImageRunner,
		mockGitServer,
		mockDeployTaskStore,
		mockTokenStore,
		mockSpaceStore,
		mockModelStore,
		mockRuntimeFrameworks,
		mockUrsStore,
		mockMetadataStore,
	)
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(DeployWorkflow)
	env.RegisterActivity(act)

	deploy := &database.Deploy{
		ID:          5,
		RepoID:      23,
		Status:      1, // Assuming 1 means active
		GitPath:     "leida/rb-saas-test",
		GitBranch:   "main",
		Hardware:    "{\"cpu\": {\"type\": \"Intel\", \"num\": \"2\"}, \"memory\": \"4Gi\"}",
		ImageID:     "7edc3aad62f8a9c085a2fa1bcd25f88e1aec7cf9",
		UserID:      0, // Assuming user ID from hub-deploy-user
		SvcName:     "u-leida-rb-saas-test-5",
		Endpoint:    "http://u-leida-rb-saas-test-5.spaces-stg.opencsg.com",
		ClusterID:   "bd48840c-88df-4c39-8cdc-fb19055446ad",
		SecureLevel: 0,
		Type:        0, // Type 2 as indicated in the data
		UserUUID:    "75985189-39f6-431c-9b6b-6c10e0d49ba9",
		Annotation:  "{\"hub-deploy-user\":\"leida\",\"hub-res-name\":\"leida/rb-saas-test\",\"hub-res-type\":\"space\"}",
		Repository: &database.Repository{
			Path: "leida/rb-saas-test",
			Name: "rb-saas-test",
			User: database.User{
				Username: "leida",
			},
		},
	}
	buildTask := &database.DeployTask{
		ID:       1,
		DeployID: deploy.ID,
		Deploy:   deploy,
		Status:   scheduler.BuildSkip,
	}

	runTask := &database.DeployTask{
		ID:       2,
		DeployID: deploy.ID,
		Deploy:   deploy,
	}

	mockTokenStore.EXPECT().FindByUID(mock.Anything, mock.Anything).Return(&database.AccessToken{
		ID:     0,
		UserID: 0,
		Token:  "accesstoken456",
		User:   &database.User{},
	}, nil)

	mockDeployTaskStore.EXPECT().GetDeployTask(mock.Anything, buildTask.ID).Return(buildTask, nil)
	buildTask.Status = scheduler.BuildSucceed

	// deploy
	mockDeployTaskStore.EXPECT().GetLastTaskByType(mock.Anything, mock.Anything, mock.Anything).Return(runTask, nil).Times(1)
	mockLogReporter.EXPECT().Report(mock.Anything).Return().Maybe()

	mockDeployTaskStore.EXPECT().GetDeployTask(mock.Anything, runTask.ID).Return(runTask, nil)
	mockDeployTaskStore.EXPECT().GetDeployByID(mock.Anything, mock.Anything).Return(deploy, nil).Maybe()

	runTask.Status = common.Pending
	mockImageRunner.EXPECT().Run(mock.Anything, mock.Anything).Return(&types.RunResponse{
		DeployID: 0,
		Code:     0,
		Message:  "test",
	}, nil).Times(1)
	mockGitServer.EXPECT().GetRepoLastCommit(mock.Anything, mock.Anything).Return(&types.Commit{
		ID: "1234567",
	}, nil).Maybe()
	mockDeployTaskStore.EXPECT().UpdateInTx(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Times(1)
	env.ExecuteWorkflow(DeployWorkflow, buildTask.ID, runTask.ID)

	var result []string
	err := env.GetWorkflowResult(&result)
	require.NoError(t, err, "GetWorkflowResult should not return error")
}

func TestCancelRunningWorkflow(t *testing.T) {
	ctx := context.Background()
	workflowID := "test-workflow-id"
	runID := "test-run-id"

	// Test case 1: Successfully cancel running workflow
	t.Run("SuccessfullyCancelRunningWorkflow", func(t *testing.T) {
		// Setup mock client
		mockTemporalClient := mockclient.NewMockClient(t)

		// Setup describe response
		mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
			&workflowservice.DescribeWorkflowExecutionResponse{
				ExecutionConfig: &workflow.WorkflowExecutionConfig{},
				WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
					Status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING,
					Execution: &v1.WorkflowExecution{
						RunId: runID,
					},
				},
				PendingActivities:      []*workflow.PendingActivityInfo{},
				PendingChildren:        []*workflow.PendingChildExecutionInfo{},
				PendingWorkflowTask:    &workflow.PendingWorkflowTaskInfo{},
				Callbacks:              []*workflow.CallbackInfo{},
				PendingNexusOperations: []*workflow.PendingNexusOperationInfo{},
				WorkflowExtendedInfo:   &workflow.WorkflowExecutionExtendedInfo{},
			}, nil,
		)

		// Setup mock to return running status when called
		mockTemporalClient.EXPECT().CancelWorkflow(ctx, workflowID, runID).Return(nil)

		// Setup cancel workflow expectation
		// mockTemporalClient.EXPECT().CancelWorkflow(ctx, workflowID, runID).Return(nil)

		// Call the function
		cancelled, err := CancelRunningWorkflow(ctx, mockTemporalClient, workflowID)

		// Verify results
		require.NoError(t, err)
		require.True(t, cancelled)
	})

	// Test case 2: Workflow not found
	t.Run("WorkflowNotFound", func(t *testing.T) {
		mockTemporalClient := mockclient.NewMockClient(t)

		// Return a not found error
		mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
			nil, errors.New("workflow execution not found"),
		)

		// Call the function
		cancelled, err := CancelRunningWorkflow(ctx, mockTemporalClient, workflowID)

		// Verify results
		require.NoError(t, err)
		require.False(t, cancelled)
	})

	// Test case 3: Describe workflow fails with non-not-found error
	t.Run("DescribeWorkflowFails", func(t *testing.T) {
		mockTemporalClient := mockclient.NewMockClient(t)
		errMsg := "connection error"

		// Return a generic error
		mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
			nil, errors.New(errMsg),
		)

		// Call the function
		cancelled, err := CancelRunningWorkflow(ctx, mockTemporalClient, workflowID)

		// Verify results
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to describe workflow")
		require.Contains(t, err.Error(), errMsg)
		require.False(t, cancelled)
	})

	// Test case 4: Workflow is not running
	t.Run("WorkflowNotRunning", func(t *testing.T) {
		mockTemporalClient := mockclient.NewMockClient(t)

		// Setup describe response with non-running status
		mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
			&workflowservice.DescribeWorkflowExecutionResponse{
				ExecutionConfig: &workflow.WorkflowExecutionConfig{},
				WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
					Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
					Execution: &v1.WorkflowExecution{
						RunId: runID,
					},
				},
				PendingActivities:      []*workflow.PendingActivityInfo{},
				PendingChildren:        []*workflow.PendingChildExecutionInfo{},
				PendingWorkflowTask:    &workflow.PendingWorkflowTaskInfo{},
				Callbacks:              []*workflow.CallbackInfo{},
				PendingNexusOperations: []*workflow.PendingNexusOperationInfo{},
				WorkflowExtendedInfo:   &workflow.WorkflowExecutionExtendedInfo{},
			}, nil,
		)

		// Call the function - should not call CancelWorkflow
		cancelled, err := CancelRunningWorkflow(ctx, mockTemporalClient, workflowID)

		// Verify results
		require.NoError(t, err)
		require.False(t, cancelled)
	})

	// Test case 5: Cancel workflow fails
	t.Run("CancelWorkflowFails", func(t *testing.T) {
		mockTemporalClient := mockclient.NewMockClient(t)
		errMsg := "cancel failed"

		// Setup describe response
		mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
			&workflowservice.DescribeWorkflowExecutionResponse{
				ExecutionConfig: &workflow.WorkflowExecutionConfig{},
				WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
					Status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING,
					Execution: &v1.WorkflowExecution{
						RunId: runID,
					},
				},
				PendingActivities:      []*workflow.PendingActivityInfo{},
				PendingChildren:        []*workflow.PendingChildExecutionInfo{},
				PendingWorkflowTask:    &workflow.PendingWorkflowTaskInfo{},
				Callbacks:              []*workflow.CallbackInfo{},
				PendingNexusOperations: []*workflow.PendingNexusOperationInfo{},
				WorkflowExtendedInfo:   &workflow.WorkflowExecutionExtendedInfo{},
			}, nil,
		)
		// Setup cancel workflow to fail
		mockTemporalClient.EXPECT().CancelWorkflow(ctx, workflowID, runID).Return(errors.New(errMsg))

		// Call the function
		cancelled, err := CancelRunningWorkflow(ctx, mockTemporalClient, workflowID)

		// Verify results
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to cancel existing workflow")
		require.Contains(t, err.Error(), errMsg)
		require.False(t, cancelled)
	})
}

func TestWorkflowAlreadyTerminated(t *testing.T) {
	ctx := context.Background()
	workflowID := "test-workflow-id"
	mockTemporalClient := mockclient.NewMockClient(t)

	// Setup mock to return a non-running workflow
	mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
		&workflowservice.DescribeWorkflowExecutionResponse{
			WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
				Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
			},
		}, nil,
	)

	// Call the function
	err := WaitForWorkflowTermination(ctx, mockTemporalClient, workflowID, 5*time.Second)
	require.NoError(t, err)
}

func TestWorkflowNotFound(t *testing.T) {
	ctx := context.Background()
	workflowID := "test-workflow-id"
	mockTemporalClient := mockclient.NewMockClient(t)

	// Setup mock to return workflow not found error
	mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
		nil, errors.New("workflow not found"),
	)

	// Call the function
	err := WaitForWorkflowTermination(ctx, mockTemporalClient, workflowID, 5*time.Second)
	require.NoError(t, err)
}

func TestDescribeWorkflowFails(t *testing.T) {
	ctx := context.Background()
	workflowID := "test-workflow-id"
	mockTemporalClient := mockclient.NewMockClient(t)

	// Setup mock to return a generic error
	errMsg := "describe workflow failed"
	mockTemporalClient.EXPECT().DescribeWorkflowExecution(ctx, workflowID, "").Return(
		nil, errors.New(errMsg),
	)

	// Call the function
	err := WaitForWorkflowTermination(ctx, mockTemporalClient, workflowID, 5*time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to describe workflow")
	require.Contains(t, err.Error(), errMsg)
}

func TestWorkflowTransitionsFromRunningToCompleted(t *testing.T) {
	ctx := context.Background()
	workflowID := "test-workflow-id"
	mockTemporalClient := mockclient.NewMockClient(t)

	// Setup mock to first return running, then completed
	mockTemporalClient.EXPECT().DescribeWorkflowExecution(mock.Anything, workflowID, "").Return(
		&workflowservice.DescribeWorkflowExecutionResponse{
			WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
				Status: enums.WORKFLOW_EXECUTION_STATUS_RUNNING,
			},
		}, nil,
	).Once()

	mockTemporalClient.EXPECT().DescribeWorkflowExecution(mock.Anything, workflowID, "").Return(
		&workflowservice.DescribeWorkflowExecutionResponse{
			WorkflowExecutionInfo: &workflow.WorkflowExecutionInfo{
				Status: enums.WORKFLOW_EXECUTION_STATUS_COMPLETED,
			},
		}, nil,
	).Once()

	// Call the function with sufficient timeout
	err := WaitForWorkflowTermination(ctx, mockTemporalClient, workflowID, 2*time.Second)
	require.NoError(t, err)
}
