//go:build ee || saas

package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/common/types"
)

// TestWorkflow_BatchMigrateToXnetWorkflow_Success verifies the legacy activity keeps its name and no-argument command.
func TestWorkflow_BatchMigrateToXnetWorkflow_Success(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.env.OnActivity("BatchMigrateToXnet", mock.Anything).Return(nil)

	tester.env.ExecuteWorkflow(workflow.BatchMigrateToXnetWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())
	tester.env.AssertActivityCalled(t, "BatchMigrateToXnet", mock.Anything)
	tester.env.AssertActivityNotCalled(t, "BatchMigrateToXnetForType", mock.Anything, mock.Anything)
}

func TestWorkflow_BatchMigrateToXnetWorkflow_ActivityFailure(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	expectedErr := errors.New("migration failed")
	tester.env.OnActivity("BatchMigrateToXnet", mock.Anything).Return(expectedErr)

	tester.env.ExecuteWorkflow(workflow.BatchMigrateToXnetWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.Error(t, tester.env.GetWorkflowError())
}

func TestWorkflow_BatchMigrateToXnetWorkflow_RetryOnFailure(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	expectedErr := errors.New("temporary migration error")
	tester.env.OnActivity("BatchMigrateToXnet", mock.Anything).Return(expectedErr)

	tester.env.ExecuteWorkflow(workflow.BatchMigrateToXnetWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.Error(t, tester.env.GetWorkflowError())
}

// TestWorkflow_BatchMigrateModelsToXnetWorkflow verifies the fixed model scope and retry policy.
func TestWorkflow_BatchMigrateModelsToXnetWorkflow(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		failedCalls   int
		success       bool
		expectedCalls int
	}{
		{name: "success", success: true, expectedCalls: 1},
		{name: "activity failure", failedCalls: 3, expectedCalls: 3},
		{name: "retry succeeds", failedCalls: 1, success: true, expectedCalls: 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			tester, err := newWorkflowTester(t)
			require.NoError(t, err)
			if scenario.failedCalls > 0 {
				tester.env.OnActivity("BatchMigrateToXnetForType", mock.Anything, types.ModelRepo).
					Return(errors.New("model migration failed")).Times(scenario.failedCalls)
			}
			if scenario.success {
				tester.env.OnActivity("BatchMigrateToXnetForType", mock.Anything, types.ModelRepo).
					Return(nil).Once()
			}

			tester.env.ExecuteWorkflow(workflow.BatchMigrateModelsToXnetWorkflow)
			require.True(t, tester.env.IsWorkflowCompleted())
			if scenario.success {
				require.NoError(t, tester.env.GetWorkflowError())
			} else {
				require.ErrorContains(t, tester.env.GetWorkflowError(), "model migration failed")
			}
			tester.env.AssertExpectations(t)
			tester.env.AssertActivityNumberOfCalls(t, "BatchMigrateToXnetForType", scenario.expectedCalls)
			tester.env.AssertActivityNotCalled(t, "BatchMigrateToXnet", mock.Anything)
		})
	}
}
