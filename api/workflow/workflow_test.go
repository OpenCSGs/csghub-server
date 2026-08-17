package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/common/types"
)

func TestWorkflow_HandlePushWorkflow(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.callback.EXPECT().SetRepoVisibility(true).Return()
	tester.mocks.callback.EXPECT().WatchSpaceChange(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().WatchAgentChange(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SyncRepositoryPackage(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().WatchRepoRelation(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().GenSyncVersion(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SetRepoUpdateTime(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().UpdateRepoInfos(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SensitiveCheck(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().MCPScan(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().CalculateRepoSize(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)

	tester.env.ExecuteWorkflow(workflow.HandlePushWorkflow, &types.GiteaCallbackPushReq{})
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())

}

func TestWorkflow_HandlePushWorkflowContinuesWhenPackageSyncFails(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	req := &types.GiteaCallbackPushReq{}
	tester.mocks.callback.EXPECT().SetRepoVisibility(true).Return()
	tester.mocks.callback.EXPECT().WatchSpaceChange(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().WatchAgentChange(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().SyncRepositoryPackage(mock.Anything, req).Return(errors.New("package sync failed"))
	tester.mocks.callback.EXPECT().WatchRepoRelation(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().GenSyncVersion(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().SetRepoUpdateTime(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().UpdateRepoInfos(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().SensitiveCheck(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().MCPScan(mock.Anything, req).Return(nil)
	tester.mocks.callback.EXPECT().CalculateRepoSize(mock.Anything, req).Return(nil)

	tester.env.ExecuteWorkflow(workflow.HandlePushWorkflow, req)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())
}
