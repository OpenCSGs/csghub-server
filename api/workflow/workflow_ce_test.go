//go:build !ee && !saas

package workflow_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	mock_git "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	mock_temporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	mock_component "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	mock_callback "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component/callback"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
)

func newWorkflowTester(t *testing.T) (*workflowTester, error) {
	suite := testsuite.WorkflowTestSuite{}
	tester := &workflowTester{
		env:     suite.NewTestWorkflowEnvironment(),
		cronEnv: suite.NewTestWorkflowEnvironment(),
	}

	// Mock the dependencies
	tester.mocks.stores = tests.NewMockStores(t)

	mcb := mock_callback.NewMockGitCallbackComponent(t)
	tester.mocks.callback = mcb

	mr := mock_component.NewMockRecomComponent(t)
	tester.mocks.recom = mr

	mm := mock_component.NewMockMultiSyncComponent(t)
	tester.mocks.multisync = mm

	mg := mock_git.NewMockGitServer(t)
	tester.mocks.gitServer = mg

	cfg := &config.Config{}
	mtc := mock_temporal.NewMockClient(t)
	mtc.EXPECT().NewWorker(workflow.HandlePushQueueName, mock.Anything).Return(tester.env)
	mtc.EXPECT().NewWorker(workflow.CronJobQueueName, mock.Anything).Return(tester.cronEnv)
	mtc.EXPECT().Start().Return(nil)
	tester.mocks.temporal = mtc
	msc := mock_temporal.NewMockScheduleClient(t)
	mtc.EXPECT().GetScheduleClient().Return(msc)
	msc.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, so client.ScheduleOptions) (client.ScheduleHandle, error) {
		require.Equal(t, "sync-as-client-schedule", so.ID)
		return nil, nil
	}).Once()
	msc.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, so client.ScheduleOptions) (client.ScheduleHandle, error) {
		require.Equal(t, "calc-recom-score-schedule", so.ID)
		return nil, nil
	}).Once()

	err := workflow.StartWorkflowDI(
		cfg, mcb, mr, mg, mm, tester.mocks.stores.SyncClientSettingMock(), mtc,
	)

	if err != nil {
		return nil, err
	}
	return tester, nil
}
