package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/workflow/activity"
	"opencsg.com/csghub-server/moderation/workflow/common"

	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
)

func TestRepoFullCheckWorkflowSuccess(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	dbRepo := &database.Repository{
		ID:             1,
		Path:           "test_user/test_repo",
		DefaultBranch:  "main",
		Name:           "test_repo",
		RepositoryType: types.ModelRepo,
	}

	// Set up mock expectations
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(dbRepo, nil)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(wf.Execute)

	// Register activities
	env.RegisterActivity(activity.RepoSensitiveCheckPending)
	env.RegisterActivity(activity.GenRepoFileList)
	env.RegisterActivity(activity.CheckRepoFiles)
	env.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Set up activity expectations
	env.OnActivity(activity.RepoSensitiveCheckPending, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.GenRepoFileList, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.CheckRepoFiles, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.DetectRepoSensitiveCheckStatus, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow execution success
	require.NoError(t, env.GetWorkflowError())
}

func TestRepoFullCheckWorkflowRepoSensitiveCheckPendingFailed(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	dbRepo := &database.Repository{
		ID:             1,
		Path:           "test_user/test_repo",
		DefaultBranch:  "main",
		Name:           "test_repo",
		RepositoryType: types.ModelRepo,
	}

	// Set up mock expectations
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(dbRepo, nil)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(RepoFullCheckWorkflow)

	// Register activities
	env.RegisterActivity(activity.RepoSensitiveCheckPending)
	env.RegisterActivity(activity.GenRepoFileList)
	env.RegisterActivity(activity.CheckRepoFiles)
	env.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Set up activity expectations with RepoSensitiveCheckPending failing
	expectedError := fmt.Errorf("failed to update repo sensitive check status")
	env.OnActivity(activity.RepoSensitiveCheckPending, mock.Anything, mock.Anything, mock.Anything).Return(expectedError)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow returns error
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "failed to update repo sensitive check status")
}

func TestRepoFullCheckWorkflowGenRepoFileListFailed(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	dbRepo := &database.Repository{
		ID:             1,
		Path:           "test_user/test_repo",
		DefaultBranch:  "main",
		Name:           "test_repo",
		RepositoryType: types.ModelRepo,
	}

	// Set up mock expectations
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(dbRepo, nil)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(wf.Execute)

	// Register activities
	env.RegisterActivity(activity.RepoSensitiveCheckPending)
	env.RegisterActivity(activity.GenRepoFileList)
	env.RegisterActivity(activity.CheckRepoFiles)
	env.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Set up activity expectations with GenRepoFileList failing
	env.OnActivity(activity.RepoSensitiveCheckPending, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	expectedError := fmt.Errorf("failed to generate repo file list")
	env.OnActivity(activity.GenRepoFileList, mock.Anything, mock.Anything, mock.Anything).Return(expectedError)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow returns error
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "failed to generate repo file list")
}

func TestRepoFullCheckWorkflowCheckRepoFilesFailed(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	dbRepo := &database.Repository{
		ID:             1,
		Path:           "test_user/test_repo",
		DefaultBranch:  "main",
		Name:           "test_repo",
		RepositoryType: types.ModelRepo,
	}

	// Set up mock expectations
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(dbRepo, nil)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(wf.Execute)

	// Register activities
	env.RegisterActivity(activity.RepoSensitiveCheckPending)
	env.RegisterActivity(activity.GenRepoFileList)
	env.RegisterActivity(activity.CheckRepoFiles)
	env.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Set up activity expectations with CheckRepoFiles failing
	env.OnActivity(activity.RepoSensitiveCheckPending, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.GenRepoFileList, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	expectedError := fmt.Errorf("failed to check repo files")
	env.OnActivity(activity.CheckRepoFiles, mock.Anything, mock.Anything, mock.Anything).Return(expectedError)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow returns error
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "failed to check repo files")
}

func TestRepoFullCheckWorkflowRepoStoreFindByPathFailed(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	// Set up mock expectations to return an error
	expectedError := fmt.Errorf("database connection error")
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(nil, expectedError)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(wf.Execute)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow returns error
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "failed to get repo, error: database connection error")
}

func TestRepoFullCheckWorkflowDetectRepoSensitiveCheckStatusFailed(t *testing.T) {
	// Create mock RepoStore
	mockRepoStore := mockdb.NewMockRepoStore(t)

	// Create test data
	testRepo := common.Repo{
		Namespace: "test_user",
		Name:      "test_repo",
		RepoType:  types.ModelRepo,
		Branch:    "main",
	}

	testConfig := &config.Config{}

	dbRepo := &database.Repository{
		ID:             1,
		Path:           "test_user/test_repo",
		DefaultBranch:  "main",
		Name:           "test_repo",
		RepositoryType: types.ModelRepo,
	}

	// Set up mock expectations
	mockRepoStore.EXPECT().FindByPath(mock.Anything, testRepo.RepoType, testRepo.Namespace, testRepo.Name).Return(dbRepo, nil)

	// Create test suite and environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	wf := newRepoFullCheckWithDB(mockRepoStore)

	// Register workflow
	env.RegisterWorkflow(wf.Execute)

	// Register activities
	env.RegisterActivity(activity.RepoSensitiveCheckPending)
	env.RegisterActivity(activity.GenRepoFileList)
	env.RegisterActivity(activity.CheckRepoFiles)
	env.RegisterActivity(activity.DetectRepoSensitiveCheckStatus)

	// Set up activity expectations with DetectRepoSensitiveCheckStatus failing
	env.OnActivity(activity.RepoSensitiveCheckPending, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.GenRepoFileList, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activity.CheckRepoFiles, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	expectedError := fmt.Errorf("failed to detect repo sensitive check status")
	env.OnActivity(activity.DetectRepoSensitiveCheckStatus, mock.Anything, mock.Anything, mock.Anything).Return(expectedError)

	// Execute workflow
	env.ExecuteWorkflow(wf.Execute, testRepo, testConfig)

	// Verify workflow returns error
	require.Error(t, env.GetWorkflowError())
	require.Contains(t, env.GetWorkflowError().Error(), "failed to detect repo sensitive check status")
}
