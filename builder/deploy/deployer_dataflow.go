package deploy

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/types"
)

func (d *deployer) CreateDataflowJob(ctx context.Context, req *types.DataflowCreateReq) (*types.DataflowArgoJobResp, error) {
	cluster, err := d.clusterStore.ByClusterID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster %s for dataflow job, error: %w", req.ClusterID, err)
	}

	runnerReq := &types.DataflowArgoJobReq{
		ID:           req.ID,
		ClusterID:    req.ClusterID,
		ArgoTaskID:   req.ArgoTaskID,
		ResourceName: req.ResourceName,
		OpUserUUID:   req.OpUserUUID,
		Username:     req.Username,
		NSUUID:       req.NSUUID,
		// dataflow specific
		RepoIds:     req.RepoIds,
		ResourceId:  req.ResourceId,
		JobID:       req.JobID,
		JobName:     req.JobName,
		JobDesc:     req.JobDesc,
		StorageSize: req.StorageSize,
		Entrypoint:  req.Entrypoint,
		Template:    req.Template,
		DagTasks:    req.DagTasks,
		AccessToken: req.AccessToken,
		// extra
		Nodes:     req.Nodes,
		Scheduler: common.GenerateScheduler(cluster.VXPUConfig),
		DeployExtend: types.DeployExtend{
			NodeAffinity: req.NodeAffinity,
			Tolerations:  req.Tolerations,
		},
	}

	resp, err := d.imageRunner.CreateDataflowWorkflow(ctx, runnerReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataflow job %s workflow error: %w", req.JobID, err)
	}
	return resp, nil
}

func (d *deployer) DeleteDataflowJob(ctx context.Context, req *types.DataflowArgoReq) error {
	err := d.imageRunner.DeleteDataflowWorkflow(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete dataflow %s workflow error: %w", req.ArgoTaskID, err)
	}
	return nil
}
