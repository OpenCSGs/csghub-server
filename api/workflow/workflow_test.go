package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_temporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_callback "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/callback"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type workflowTester struct {
	env   *testsuite.TestWorkflowEnvironment
	mocks struct {
		callback  *mock_callback.MockGitCallbackComponent
		recom     *mock_component.MockRecomComponent
		multisync *mock_component.MockMultiSyncComponent
		gitServer *mock_git.MockGitServer
		temporal  *mock_temporal.MockClient
		stores    *tests.MockStores
	}
}

func TestWorkflow_CalcRecomScoreWorkflow(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.recom.EXPECT().CalculateRecomScore(mock.Anything).Return()
	tester.env.ExecuteWorkflow(workflow.CalcRecomScoreWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())
}

func TestWorkflow_SyncAsClient(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.stores.SyncClientSettingMock().EXPECT().First(mock.Anything).Return(
		&database.SyncClientSetting{Token: "tk"}, nil,
	)
	tester.mocks.multisync.EXPECT().SyncAsClient(
		mock.Anything, multisync.FromOpenCSG("", "tk"),
	).Return(nil)

	tester.env.ExecuteWorkflow(workflow.SyncAsClientWorkflow)
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())

}

func TestWorkflow_HandlePushWorkflow(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.callback.EXPECT().SetRepoVisibility(true).Return()
	tester.mocks.callback.EXPECT().WatchSpaceChange(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().WatchRepoRelation(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SetRepoUpdateTime(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().UpdateRepoInfos(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SensitiveCheck(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)

	tester.env.ExecuteWorkflow(workflow.HandlePushWorkflow, &types.GiteaCallbackPushReq{})
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())

}
