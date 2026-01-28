//go:build ee || saas

package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/workflow"
)

func TestWorkflow_BatchMigrateToXnetWorkflow_Success(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.env.OnActivity("BatchMigrateToXnet", mock.Anything).Return(nil)

	tester.env.ExecuteWorkflow(workflow.BatchMigrateToXnetWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())
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
