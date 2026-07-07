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
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	deploy := &database.Deploy{ID: 1, Type: types.SpaceType, SvcName: "s1", Status: common.Deploying}
	ds.EXPECT().GetDeployByID(mock.Anything, int64(1)).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Once()
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.MatchedBy(func(d *database.Deploy) bool {
		return d.Status == common.Running && !d.StatusUpdateAt.IsZero()
	})).Return(nil).Once()
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
	a := newTestActivities(t)
	ds := a.stores.deployTask.(*mockdb.MockDeployTaskStore)
	md := a.deployer.(*mock_deploy.MockDeployer)

	md.EXPECT().CheckHeartbeatTimeout(mock.Anything, "c1").Return(false, nil).Once()
	md.EXPECT().BatchStatus(mock.Anything, mock.MatchedBy(func(req *runnerTypes.BatchStatusRequest) bool {
		return req.ClusterID == "c1" && len(req.Items) == 2
	})).Return(&runnerTypes.BatchStatusResponse{
		Items: []runnerTypes.BatchStatusItemResult{
			{Type: runnerTypes.ResourceTypeKsvc, Name: "s1", Code: common.Running},
			{Type: runnerTypes.ResourceTypeSandbox, Name: "s2", Status: common.Running},
		},
	}, nil).Once()

	ds.EXPECT().GetDeployByID(mock.Anything, mock.Anything).Return(&database.Deploy{ID: 1, Status: common.Deploying}, nil).Times(2)
	ds.EXPECT().UpdateDeploy(mock.Anything, mock.Anything).Return(nil).Times(2)

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
