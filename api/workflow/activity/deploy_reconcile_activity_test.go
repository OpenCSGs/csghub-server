package activity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mock_deploy "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	runnerTypes "opencsg.com/csghub-server/runner/types"
)

func newTestActivities(t *testing.T) *Activities {
	return &Activities{
		stores: stores{
			deployTask:   mockdb.NewMockDeployTaskStore(t),
			argoWorkFlow: mockdb.NewMockArgoWorkFlowStore(t),
		},
		deployer: mock_deploy.NewMockDeployer(t),
		deployConfig: common.DeployConfig{
			StuckTimeoutMin:      15,
			RunningReconcileHour: 2,
		},
	}
}

func TestStatusName(t *testing.T) {
	assert.Equal(t, "Pending", statusName(common.Pending))
	assert.Equal(t, "Running", statusName(common.Running))
	assert.Equal(t, "Unknown", statusName(999))
}

func TestMapSandboxStatusToDeployStatus(t *testing.T) {
	assert.Equal(t, common.Running, mapSandboxStatusToDeployStatus(common.Running, common.Deploying))
	assert.Equal(t, common.DeployFailed, mapSandboxStatusToDeployStatus(common.Stopped, common.Deploying))
	assert.Equal(t, common.Stopped, mapSandboxStatusToDeployStatus(common.Stopped, common.Running))
}

func TestGroupByCluster(t *testing.T) {
	deploys := []database.Deploy{
		{ID: 1, ClusterID: "a"}, {ID: 2, ClusterID: "a"}, {ID: 3, ClusterID: ""},
	}
	result := groupByCluster(deploys)
	assert.Len(t, result, 2)
}

func TestGroupWorkflowsByCluster(t *testing.T) {
	wfs := []database.ArgoWorkflow{
		{ID: 1, ClusterID: "a"}, {ID: 2, ClusterID: ""},
	}
	result := groupWorkflowsByCluster(wfs)
	assert.Len(t, result, 2)
}

func TestApplyStatusUpdate_Success(t *testing.T) {
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	deploy := &database.Deploy{ID: 1, Status: common.Deploying, SvcName: "s1"}
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.Running && !d.StatusUpdateAt.IsZero()
	})).Return(nil).Once()
	applyStatusUpdate(context.WithValue(context.Background(), "test", "test"), a, deploy, common.Deploying, common.Running, nil, "runner_status_sync")
}

func TestApplyStatusUpdate_ConcurrentModification(t *testing.T) {
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	deploy := &database.Deploy{ID: 1, Status: common.Deploying, SvcName: "s1"}
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Running}, nil).Once()
	applyStatusUpdate(context.WithValue(context.Background(), "test", "test"), a, deploy, common.Deploying, common.Running, nil, "test")
}

func TestProcessBatchResult_KsvcRunning(t *testing.T) {
	// When a Deploying deploy gets Running from runner, the informer handles
	// this transition. Reconcile should skip — not interfere.
	a := newTestActivities(t)
	deploy := &database.Deploy{ID: 1, Type: types.SpaceType, SvcName: "s1", Status: common.Deploying}
	// No DB calls expected — informer handles normal Deploying→Running flow
	processBatchResult(context.WithValue(context.Background(), "test", "test"), a, deploy, &runnerTypes.BatchStatusItemResult{Code: common.Running}, common.Deploying)
}

func TestProcessBatchResult_KsvcStopped(t *testing.T) {
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	deploy := &database.Deploy{ID: 1, Type: types.SpaceType, SvcName: "s1", Status: common.Deploying}
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.DeployFailed && !d.StatusUpdateAt.IsZero()
	})).Return(nil).Once()
	processBatchResult(context.WithValue(context.Background(), "test", "test"), a, deploy, &runnerTypes.BatchStatusItemResult{Code: common.Stopped}, common.Deploying)
}

func TestProcessBatchResult_SandboxRunning(t *testing.T) {
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	deploy := &database.Deploy{ID: 1, Type: types.SandboxType, SvcName: "s1", Status: common.Deploying}
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.Running && !d.StatusUpdateAt.IsZero()
	})).Return(nil).Once()
	processBatchResult(context.WithValue(context.Background(), "test", "test"), a, deploy, &runnerTypes.BatchStatusItemResult{Status: common.Running}, common.Deploying)
}

func TestReconcileDeployCluster_BatchSuccess(t *testing.T) {
	// KSVC Deploying/Startup deploys are skipped (informer handles normal flow).
	// Sandbox deploys are still processed. This test verifies both paths.
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.MatchedBy(func(req *runnerTypes.BatchStatusRequest) bool {
		return req.ClusterID == "c1" && len(req.Items) == 2
	})).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeKsvc, Name: "s1", Code: common.Running},    // Deploying KSVC → skipped
			{Type: runnerTypes.ResourceTypeSandbox, Name: "s2", Status: common.Running}, // Deploying Sandbox → processed
		},
	}, nil).Once()

	// Only sandbox deploy gets updated; KSVC deploy is skipped
	ds.EXPECT().GetDeployByID(mock.Anything, int64(2)).Return(&database.Deploy{ID: 2, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.Running && d.ID == int64(2)
	})).Return(nil).Once()

	deploys := []database.Deploy{
		{ID: 1, Type: types.SpaceType, SvcName: "s1", ClusterID: "c1", Status: common.Deploying},
		{ID: 2, Type: types.SandboxType, SvcName: "s2", ClusterID: "c1", Status: common.Deploying},
	}
	reconcileDeployCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", deploys, common.Deploying, 30*time.Minute)
}

func TestReconcileAllStatus(t *testing.T) {
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	wfs := a.stores.argoWorkFlow.(*mockdb.MockArgoWorkFlowStore)
	ds.EXPECT().ListDeploysNeedingReconcile(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Times(3)
	wfs.EXPECT().ListWorkflowsNeedingReconcile(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Times(2)
	require.NoError(t, a.ReconcileAllStatus(context.WithValue(context.Background(), "test", "test")))
}

func TestReconcileDeployCluster_BatchItemError_TimeoutFallback(t *testing.T) {
	// When individual batch items have errors and are past hardTimeout,
	// onBatchError should mark them as DeployFailed.
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	// BatchStatus returns individual item errors
	md.EXPECT().BatchStatus(mock.Anything, mock.MatchedBy(func(req *runnerTypes.BatchStatusRequest) bool {
		return req.ClusterID == "c1" && len(req.Items) == 1
	})).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeKsvc, Name: "s1", Error: "service not found in cluster"},
		},
	}, nil).Once()

	// onBatchError will re-read the deploy, then update to DeployFailed
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.DeployFailed && d.ID == int64(1)
	})).Return(nil).Once()

	// Set StatusUpdateAt far in the past so hardTimeout is exceeded
	pastTime := time.Now().Add(-2 * time.Hour)
	deploys := []database.Deploy{
		{ID: 1, Type: types.SpaceType, SvcName: "s1", ClusterID: "c1", Status: common.Deploying, StatusUpdateAt: pastTime},
	}
	reconcileDeployCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", deploys, common.Deploying, 30*time.Minute)
}

func TestReconcileDeployCluster_BatchItemError_NotTimedOut(t *testing.T) {
	// When individual batch items have errors but are NOT past hardTimeout,
	// onBatchError should be called but the internal check should skip the update.
	a := newTestActivities(t)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.Anything).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeKsvc, Name: "s1", Error: "service not found in cluster"},
		},
	}, nil).Once()

	// onBatchError checks hardTimeout internally; since StatusUpdateAt is recent,
	// it should NOT call GetDeployByID or UpdateDeploy.
	// No mock expectations set → test will fail if those are called.

	recentTime := time.Now() // not past hardTimeout
	deploys := []database.Deploy{
		{ID: 1, Type: types.SpaceType, SvcName: "s1", ClusterID: "c1", Status: common.Deploying, StatusUpdateAt: recentTime},
	}
	reconcileDeployCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", deploys, common.Deploying, 30*time.Minute)
}

func TestReconcileWorkflowCluster_BatchItemError_TimeoutFallback(t *testing.T) {
	// When workflow batch items have errors and are past hardTimeout,
	// onBatchError should mark them as WorkflowFailed.
	a := newTestActivities(t)
	wfs := a.stores.argoWorkFlow.(*mockdb.MockArgoWorkFlowStore)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.Anything).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeWorkflow, Name: "wf-task-1", Error: "workflow not found in cluster"},
		},
	}, nil).Once()

	// onBatchError will check hardTimeout, then update the workflow
	wfs.EXPECT().UpdateWorkFlow(mock.Anything, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
		return string(wf.Status) == "Failed" && wf.ID == int64(1)
	})).Return(&database.ArgoWorkflow{ID: 1, Status: "Failed"}, nil).Once()

	pastTime := time.Now().Add(-2 * time.Hour)
	wfsList := []database.ArgoWorkflow{
		{ID: 1, TaskId: "wf-task-1", ClusterID: "c1", Status: "Running", StatusUpdateAt: pastTime},
	}
	reconcileWorkflowCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", wfsList, 30*time.Minute)
}

func TestReconcileWorkflowCluster_BatchItemError_NotTimedOut(t *testing.T) {
	// When workflow batch items have errors but are NOT past hardTimeout,
	// onBatchError should be called but skip the update.
	a := newTestActivities(t)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.Anything).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeWorkflow, Name: "wf-task-1", Error: "workflow not found in cluster"},
		},
	}, nil).Once()

	// No DB calls expected — onBatchError checks hardTimeout internally and skips

	recentTime := time.Now()
	wfsList := []database.ArgoWorkflow{
		{ID: 1, TaskId: "wf-task-1", ClusterID: "c1", Status: "Running", StatusUpdateAt: recentTime},
	}
	reconcileWorkflowCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", wfsList, 30*time.Minute)
}

func TestReconcileWorkflowCluster_BatchSuccess(t *testing.T) {
	// Normal path: status synced from runner
	a := newTestActivities(t)
	wfs := a.stores.argoWorkFlow.(*mockdb.MockArgoWorkFlowStore)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.Anything).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeWorkflow, Name: "wf-task-1", Phase: "Succeeded"},
		},
	}, nil).Once()

	wfs.EXPECT().UpdateWorkFlow(mock.Anything, mock.MatchedBy(func(wf database.ArgoWorkflow) bool {
		return string(wf.Status) == "Succeeded" && wf.ID == int64(1)
	})).Return(&database.ArgoWorkflow{ID: 1, Status: "Succeeded"}, nil).Once()

	wfsList := []database.ArgoWorkflow{
		{ID: 1, TaskId: "wf-task-1", ClusterID: "c1", Status: "Running"},
	}
	reconcileWorkflowCluster(context.WithValue(context.Background(), "test", "test"), a, "c1", wfsList, 30*time.Minute)
}

func TestProcessBatchResult_RunningNotDowngraded(t *testing.T) {
	// When a KSVC deploy is Running and the runner returns Startup (e.g. scale-to-zero),
	// the status should NOT be downgraded.
	a := newTestActivities(t)
	deploy := &database.Deploy{ID: 1, Type: types.SpaceType, SvcName: "s1", Status: common.Running}
	// No DB calls expected — processBatchResult should return early
	processBatchResult(context.WithValue(context.Background(), "test", "test"), a, deploy,
		&runnerTypes.BatchStatusItemResult{Code: common.Startup}, common.Running)
}

func TestProcessBatchResult_DeployingStillStartup(t *testing.T) {
	// When a Deploying deploy gets Startup from runner, the informer handles
	// the normal transition. Reconcile should skip — not interfere.
	a := newTestActivities(t)
	deploy := &database.Deploy{ID: 1, Type: types.SpaceType, SvcName: "s1", Status: common.Deploying}
	// No DB calls expected — informer handles normal Deploying→Startup flow
	processBatchResult(context.WithValue(context.Background(), "test", "test"), a, deploy,
		&runnerTypes.BatchStatusItemResult{Code: common.Startup}, common.Deploying)
}
