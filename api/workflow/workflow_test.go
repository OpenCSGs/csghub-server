package workflow_test

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	mocktemporal "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/temporal"
	"opencsg.com/csghub-server/api/workflow"
	"opencsg.com/csghub-server/common/config"
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

// publicationScheduleRecorder captures schedule registration in every edition.
type publicationScheduleRecorder struct{ options []client.ScheduleOptions }

// Create records schedule options without contacting a Temporal server.
func (s *publicationScheduleRecorder) Create(_ context.Context, options client.ScheduleOptions) (client.ScheduleHandle, error) {
	s.options = append(s.options, options)
	return nil, nil
}

// testPendingModelPublicationSchedule verifies the edition's original publication scope.
func testPendingModelPublicationSchedule(t *testing.T, wantRegistered bool) {
	t.Helper()
	recorder := &publicationScheduleRecorder{}
	temporalClient := mocktemporal.NewMockClient(t)
	temporalClient.EXPECT().GetScheduleClient().Return(recorder)
	cfg := &config.Config{}
	cfg.CronJob.PublicModelRepoCronExpression = "* * * * *"
	require.NoError(t, workflow.RegisterCronJobs(cfg, temporalClient))
	for _, options := range recorder.options {
		if options.ID != "public-model-repo-schedule" {
			continue
		}
		require.True(t, wantRegistered, "non-SaaS editions must not register automatic model publication")
		require.Equal(t, []string{"* * * * *"}, options.Spec.CronExpressions)
		action, ok := options.Action.(*client.ScheduleWorkflowAction)
		require.True(t, ok)
		require.Equal(t, "public-model-repo-workflow", action.ID)
		require.Equal(t, workflow.CronJobQueueName, action.TaskQueue)
		return
	}
	require.False(t, wantRegistered, "SaaS model publication schedule is missing")
}

// publicationWorkflowRecorder records publication registration while retaining Temporal's test registry.
type publicationWorkflowRecorder struct {
	*testsuite.TestWorkflowEnvironment
	publicationRegistered         bool
	publicationActivityRegistered bool
}

// RegisterActivity verifies that publication activities remain restricted to SaaS.
func (r *publicationWorkflowRecorder) RegisterActivity(a interface{}) {
	if _, ok := reflect.TypeOf(a).MethodByName("PublicModelRepo"); ok {
		r.publicationActivityRegistered = true
	}
	r.TestWorkflowEnvironment.RegisterActivity(a)
}

// RegisterWorkflow observes the real worker registration calls without running workflows.
func (r *publicationWorkflowRecorder) RegisterWorkflow(w interface{}) {
	if strings.HasSuffix(runtime.FuncForPC(reflect.ValueOf(w).Pointer()).Name(), ".PublicModelRepoWorkflow") {
		r.publicationRegistered = true
	}
	r.TestWorkflowEnvironment.RegisterWorkflow(w)
}
