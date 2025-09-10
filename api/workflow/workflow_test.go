package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_cache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	mock_temporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_callback "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/callback"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

type workflowTester struct {
	env       *testsuite.TestWorkflowEnvironment
	cronEnv   *testsuite.TestWorkflowEnvironment
	scheduler *temporal.TestScheduler
	mocks     struct {
		callback         *mock_callback.MockGitCallbackComponent
		recom            *mock_component.MockRecomComponent
		multisync        *mock_component.MockMultiSyncComponent
		modeltree        *mock_component.MockModelTreeComponent
		gitServer        *mock_git.MockGitServer
		temporal         *mock_temporal.MockClient
		stores           *tests.MockStores
		mirrorComponent  *mock_component.MockMirrorComponent
		statComponent    *mock_component.MockStatComponent
		accountComponent *mock_component.MockAccountingComponent
		repoComponent    *mock_component.MockRepoComponent
		cache            *mock_cache.MockRedisClient
	}
}

func TestWorkflow_HandlePushWorkflow(t *testing.T) {
	tester, err := newWorkflowTester(t)
	require.NoError(t, err)

	tester.mocks.callback.EXPECT().SetRepoVisibility(true).Return()
	tester.mocks.callback.EXPECT().WatchSpaceChange(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().WatchRepoRelation(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().GenSyncVersion(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SetRepoUpdateTime(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().UpdateRepoInfos(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().SensitiveCheck(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)
	tester.mocks.callback.EXPECT().MCPScan(mock.Anything, &types.GiteaCallbackPushReq{}).Return(nil)

	tester.env.ExecuteWorkflow(workflow.HandlePushWorkflow, &types.GiteaCallbackPushReq{})
	require.True(t, tester.env.IsWorkflowCompleted())
	require.NoError(t, tester.env.GetWorkflowError())

}
