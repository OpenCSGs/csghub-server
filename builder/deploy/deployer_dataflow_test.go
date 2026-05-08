package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockrunner "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployer_CreateDataflowWorkflow(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		req := &types.DataflowCreateReq{
			ID:           1,
			ClusterID:    "cluster-1",
			ArgoTaskID:   "task-123",
			ResourceName: "gpu-resource",
			OpUserUUID:   "user-uuid-1",
			Username:     "testuser",
			NSUUID:       "ns-uuid-1",
			RepoIds:      []string{"repo1", "repo2"},
			ResourceId:   100,
			JobID:        "job-1",
			JobName:      "test-job",
			JobDesc:      "test description",
			StorageSize:  "10Gi",
			Entrypoint:   "main.py",
			Template: types.ArgoFlowTemplate{
				Name: "template-1",
			},
			DagTasks: []types.ArgoDagTask{
				{
					ID:   "task-1",
					Name: "dag-task-1",
				},
			},
			Nodes: []types.Node{
				{
					Name: "node-1",
				},
			},
			DeployExtend: types.DeployExtend{
				NodeAffinity: nil,
				Tolerations:  nil,
			},
		}

		expectedResp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "task-123",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Running",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().CreateDataflowWorkflow(ctx, mock.Anything).RunAndReturn(
			func(ctx context.Context, r *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error) {
				require.Equal(t, req.ID, r.ID)
				require.Equal(t, req.ClusterID, r.ClusterID)
				require.Equal(t, req.ArgoTaskID, r.ArgoTaskID)
				require.Equal(t, req.ResourceName, r.ResourceName)
				require.Equal(t, req.OpUserUUID, r.OpUserUUID)
				require.Equal(t, req.Username, r.Username)
				require.Equal(t, req.NSUUID, r.NSUUID)
				require.Equal(t, req.RepoIds, r.RepoIds)
				require.Equal(t, req.ResourceId, r.ResourceId)
				require.Equal(t, req.JobID, r.JobID)
				require.Equal(t, req.JobName, r.JobName)
				require.Equal(t, req.JobDesc, r.JobDesc)
				require.Equal(t, req.StorageSize, r.StorageSize)
				require.Equal(t, req.Entrypoint, r.Entrypoint)
				require.Equal(t, req.Template, r.Template)
				require.Equal(t, req.DagTasks, r.DagTasks)
				require.Equal(t, req.Nodes, r.Nodes)
				require.Equal(t, req.NodeAffinity, r.NodeAffinity)
				require.Equal(t, req.Tolerations, r.Tolerations)
				return expectedResp, nil
			},
		)

		d := &deployer{
			imageRunner: mockRunner,
		}

		resp, err := d.CreateDataflowJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, expectedResp, resp)
	})

	t.Run("runner returns error", func(t *testing.T) {
		req := &types.DataflowCreateReq{
			ID:          1,
			ClusterID:   "cluster-1",
			ArgoTaskID:  "task-123",
			ResourceId:  100,
			JobID:       "job-1",
			JobName:     "test-job",
			StorageSize: "10Gi",
			Entrypoint:  "main.py",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().CreateDataflowWorkflow(ctx, mock.Anything).Return(nil, errors.New("runner error"))

		d := &deployer{
			imageRunner: mockRunner,
		}

		resp, err := d.CreateDataflowJob(ctx, req)
		require.Error(t, err)
		require.Nil(t, resp)
		require.Contains(t, err.Error(), "failed to create dataflow job job-1 workflow error")
		require.Contains(t, err.Error(), "runner error")
	})

	t.Run("with scheduler", func(t *testing.T) {
		req := &types.DataflowCreateReq{
			ID:           1,
			ClusterID:    "cluster-1",
			ArgoTaskID:   "task-123",
			ResourceName: "gpu-resource",
			OpUserUUID:   "user-uuid-1",
			Username:     "testuser",
			NSUUID:       "ns-uuid-1",
			ResourceId:   100,
			JobID:        "job-1",
			JobName:      "test-job",
			StorageSize:  "10Gi",
			Entrypoint:   "main.py",
		}

		scheduler := &types.Scheduler{
			Volcano: &types.VolcanoConfig{
				SchedulerName: "custom-scheduler",
			},
		}

		expectedResp := &types.DataflowArgoJobResp{
			ID:         1,
			ArgoTaskID: "task-123",
			JobID:      "job-1",
			JobName:    "test-job",
			Status:     "Running",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().CreateDataflowWorkflow(ctx, mock.Anything).RunAndReturn(
			func(ctx context.Context, r *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error) {
				require.NotNil(t, r.Scheduler)
				require.Equal(t, scheduler, r.Scheduler)
				return expectedResp, nil
			},
		)

		d := &deployer{
			imageRunner:   mockRunner,
			kubeScheduler: scheduler,
		}

		resp, err := d.CreateDataflowJob(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})
}

func TestDeployer_DeleteDataflowWorkflow(t *testing.T) {
	ctx := context.TODO()

	t.Run("success", func(t *testing.T) {
		req := &types.DataflowArgoReq{
			ArgoTaskID: "task-123",
			ClusterID:  "cluster-1",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().DeleteDataflowWorkflow(ctx, req).Return(nil)

		d := &deployer{
			imageRunner: mockRunner,
		}

		err := d.DeleteDataflowJob(ctx, req)
		require.NoError(t, err)
	})

	t.Run("runner returns error", func(t *testing.T) {
		req := &types.DataflowArgoReq{
			ArgoTaskID: "task-456",
			ClusterID:  "cluster-2",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().DeleteDataflowWorkflow(ctx, req).Return(errors.New("delete failed"))

		d := &deployer{
			imageRunner: mockRunner,
		}

		err := d.DeleteDataflowJob(ctx, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete dataflow task-456 workflow error")
		require.Contains(t, err.Error(), "delete failed")
	})

	t.Run("empty cluster id", func(t *testing.T) {
		req := &types.DataflowArgoReq{
			ArgoTaskID: "task-789",
			ClusterID:  "",
		}

		mockRunner := mockrunner.NewMockRunner(t)
		mockRunner.EXPECT().DeleteDataflowWorkflow(ctx, req).Return(nil)

		d := &deployer{
			imageRunner: mockRunner,
		}

		err := d.DeleteDataflowJob(ctx, req)
		require.NoError(t, err)
	})
}
