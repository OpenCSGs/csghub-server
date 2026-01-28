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

func TestWorkflow_InspectMCPServersWorkflow_SinglePage(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	// One page only: total=1 => done=true.
	targets := []types.AgentMCPServerInspectTarget{
		{ServerID: "user:1", UserUUID: "u1", Protocol: "sse", URL: "http://a"},
		{ServerID: "builtin:2", UserUUID: "u2", Protocol: "sse", URL: "http://b"},
	}
	tester.mocks.agentMCPServer.EXPECT().
		ListAllMCPServers(mock.Anything, 50, 1).
		Return(targets, 1, nil)

	// Best-effort: one target fails inspection, workflow should still complete.
	tester.mocks.agentMCPServer.EXPECT().
		InspectMCPServerAndPersist(mock.Anything, mock.MatchedBy(func(t types.AgentMCPServerInspectTarget) bool {
			return t.ServerID == "user:1"
		})).
		Return(nil)
	tester.mocks.agentMCPServer.EXPECT().
		InspectMCPServerAndPersist(mock.Anything, mock.MatchedBy(func(t types.AgentMCPServerInspectTarget) bool {
			return t.ServerID == "builtin:2"
		})).
		Return(errors.New("inspect failed"))

	tester.cronEnv.ExecuteWorkflow(workflow.InspectMCPServersWorkflow)
	require.True(t, tester.cronEnv.IsWorkflowCompleted())
	require.NoError(t, tester.cronEnv.GetWorkflowError())
}

func TestWorkflow_InspectMCPServersWorkflow_MultiPage(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	// Two pages: total=60 => page1 done=false, page2 done=true.
	targetsPage1 := []types.AgentMCPServerInspectTarget{
		{ServerID: "user:1", UserUUID: "u1", Protocol: "sse", URL: "http://a"},
	}
	targetsPage2 := []types.AgentMCPServerInspectTarget{} // should skip inspect on page2

	tester.mocks.agentMCPServer.EXPECT().
		ListAllMCPServers(mock.Anything, 50, 1).
		Return(targetsPage1, 60, nil).
		Once()
	tester.mocks.agentMCPServer.EXPECT().
		ListAllMCPServers(mock.Anything, 50, 2).
		Return(targetsPage2, 60, nil).
		Once()

	tester.mocks.agentMCPServer.EXPECT().
		InspectMCPServerAndPersist(mock.Anything, mock.MatchedBy(func(t types.AgentMCPServerInspectTarget) bool {
			return t.ServerID == "user:1"
		})).
		Return(nil).
		Once()

	tester.cronEnv.ExecuteWorkflow(workflow.InspectMCPServersWorkflow)
	require.True(t, tester.cronEnv.IsWorkflowCompleted())
	require.NoError(t, tester.cronEnv.GetWorkflowError())
}

func TestWorkflow_InspectMCPServersWorkflow_ListError(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	listErr := errors.New("list failed")
	// RetryPolicy.MaximumAttempts=3 in workflow; activity will be retried.
	tester.mocks.agentMCPServer.EXPECT().
		ListAllMCPServers(mock.Anything, 50, 1).
		Return(nil, 0, listErr).
		Times(3)

	tester.cronEnv.ExecuteWorkflow(workflow.InspectMCPServersWorkflow)
	require.True(t, tester.cronEnv.IsWorkflowCompleted())
	require.Error(t, tester.cronEnv.GetWorkflowError())
}
