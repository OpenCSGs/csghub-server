package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/bwmarrin/snowflake"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type PlatformDataflowComponent interface {
	CreateJob(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error)
	DeleteJob(ctx context.Context, req *types.DataflowDeleteReq) error
	GetJob(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error)
	ReadJobLogsInStream(ctx context.Context, req types.DataflowLogReq) (*deploy.MultiLogReader, error)
	ReadJobLogsNonStream(ctx context.Context, req types.DataflowLogReq) (string, error)
	CheckUserPermission(ctx context.Context, req types.DataflowLogReq) (bool, error)
}

type platformDataflowComponentImpl struct {
	deployer           deploy.Deployer
	workflowStore      database.ArgoWorkFlowStore
	userSvcClient      rpc.UserSvcClient
	clusterStore       database.ClusterInfoStore
	spaceResourceStore database.SpaceResourceStore
	repoComponent      RepoComponent
	snowflakeNode      *snowflake.Node
	config             *config.Config
}

func NewPlatformDataflowComponent(cfg *config.Config) (PlatformDataflowComponent, error) {
	var err error
	c := &platformDataflowComponentImpl{}
	c.config = cfg
	c.deployer = deploy.NewDeployer()
	c.workflowStore = database.NewArgoWorkFlowStore()
	c.userSvcClient = rpc.NewUserSvcHttpClient(
		fmt.Sprintf("%s:%d", cfg.User.Host, cfg.User.Port),
		rpc.AuthWithApiKey(cfg.APIToken),
	)
	c.clusterStore = database.NewClusterInfoStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.repoComponent, err = NewRepoComponent(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component, error: %w", err)
	}
	node, err := snowflake.NewNode(1)
	if err != nil || node == nil {
		return nil, fmt.Errorf("failed to create snowflake node, error: %w", err)
	}
	c.snowflakeNode = node
	return c, nil
}

func (c *platformDataflowComponentImpl) CreateJob(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error) {
	// Check user or org permission
	ns, err := checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, req.Username, req.NSUUID)
	if err != nil {
		return nil, err
	}

	// Get or create user's access token for dataflow job
	token, err := c.userSvcClient.GetOrCreateFirstAvaiTokens(ctx, req.Username, req.Username, string(types.AccessTokenAppGit), "dataflow")
	if err != nil {
		return nil, fmt.Errorf("failed to get user access token: %w", err)
	}
	if len(token) == 0 {
		return nil, fmt.Errorf("no available access token for user %s", req.Username)
	}
	req.AccessToken = token

	var hardware types.HardWare

	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceId)
	if err != nil {
		return nil, fmt.Errorf("cannot find resource %d error: %w", req.ResourceId, err)
	}

	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return nil, fmt.Errorf("invalid hardware setting error: %w", err)
	}

	// check resource available
	exclusiveResp, err := c.repoComponent.CheckAccountAndResource(ctx, ns.Path, resource.ClusterID, 0, resource)
	if err != nil {
		return nil, fmt.Errorf("failed to check account and resource, error: %w", err)
	}

	req.ClusterID = resource.ClusterID
	req.ResourceName = resource.Name
	req.NodeAffinity = exclusiveResp.NodeAffinity
	req.Tolerations = exclusiveResp.Tolerations

	clusterNodes, err := c.clusterStore.FindNodeByClusterID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by clusterID %s, error: %w", req.ClusterID, err)
	}

	uniqueJobID := c.snowflakeNode.Generate().Base36()

	now := time.Now()
	workflow := database.ArgoWorkflow{
		Username:     req.Username,
		UserUUID:     req.NSUUID,
		TaskName:     req.JobName,
		TaskId:       fmt.Sprintf("df%s", uniqueJobID),
		TaskType:     types.TaskTypeDataflow,
		ClusterID:    req.ClusterID,
		RepoIds:      req.RepoIds,
		RepoType:     string(types.DatasetRepo),
		TaskDesc:     req.JobDesc,
		Status:       v1alpha1.WorkflowPending,
		Image:        req.Template.Image,
		Datasets:     req.RepoIds,
		ResourceId:   req.ResourceId,
		ResourceName: req.ResourceName,
		SubmitTime:   now,
	}

	for _, node := range clusterNodes {
		req.Nodes = append(req.Nodes, types.Node{
			Name:       node.Name,
			EnableVXPU: node.EnableVXPU,
			HasXPU:     node.Hardware.HasXPU() || node.EnableVXPU,
		})
	}

	createdWorkflow, err := c.workflowStore.CreateWorkFlow(ctx, workflow)
	if err != nil {
		return nil, fmt.Errorf("failed to create ArgoWorkflow record, error: %w", err)
	}

	req.ID = createdWorkflow.ID
	req.ArgoTaskID = createdWorkflow.TaskId

	resp, err := c.deployer.CreateDataflowJob(ctx, req)
	if err != nil {
		// Delete ArgoWorkflow record
		delErr := c.workflowStore.DeleteWorkFlow(ctx, createdWorkflow.ID)
		if delErr != nil {
			slog.ErrorContext(ctx, "failed to delete ArgoWorkflow record due to create dataflow workflow failed",
				slog.Any("error", delErr))
		}
		return nil, fmt.Errorf("failed to create dataflow workflow, error: %w", err)
	}

	resp.ID = createdWorkflow.ID
	return resp, nil
}

func (c *platformDataflowComponentImpl) DeleteJob(ctx context.Context, req *types.DataflowDeleteReq) error {
	wf, err := c.workflowStore.FindByTaskID(ctx, req.ArgoTaskID)
	if err != nil {
		return fmt.Errorf("failed to find dataflow workflow by task_id %s error: %w", req.ArgoTaskID, err)
	}

	if wf.UserUUID != req.NSUUID {
		return fmt.Errorf("do not have permission to operate the target namespace's data: %w", err)
	}

	// Check owner or org permission
	_, err = checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, req.Username, req.NSUUID)
	if err != nil {
		return err
	}

	deleteReq := &types.DataflowArgoReq{
		ArgoTaskID: wf.TaskId,
		ClusterID:  wf.ClusterID,
	}
	err = c.deployer.DeleteDataflowJob(ctx, deleteReq)
	if err != nil {
		return fmt.Errorf("failed to delete dataflow workflow %s error: %w", req.ArgoTaskID, err)
	}

	err = c.workflowStore.DeleteWorkFlow(ctx, wf.ID)
	if err != nil {
		return fmt.Errorf("failed to delete dataflow workflow record %d error: %w", wf.ID, err)
	}

	return nil
}

func (c *platformDataflowComponentImpl) GetJob(ctx context.Context, req *types.DataflowArgoJobReq) (*types.DataflowArgoJobResp, error) {
	wf, err := c.workflowStore.FindByTaskID(ctx, req.ArgoTaskID)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataflow workflow by task_id %s: %w", req.ArgoTaskID, err)
	}

	_, err = checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, req.Username, req.NSUUID)
	if err != nil {
		return nil, err
	}

	resp := &types.DataflowArgoJobResp{
		ID:         wf.ID,
		ArgoTaskID: wf.TaskId,
		JobID:      wf.TaskId,
		JobName:    wf.TaskName,
		Status:     string(wf.Status),
		Message:    wf.Reason,
		CreatedAt:  wf.SubmitTime.Unix(),
		DagTasks:   wf.DagTasks,
		DeleteAt:   wf.DeletedAt.Unix(),
	}
	return resp, nil
}

func (c *platformDataflowComponentImpl) CheckUserPermission(ctx context.Context, req types.DataflowLogReq) (bool, error) {
	wf, err := c.workflowStore.FindByTaskID(ctx, req.TaskId)
	if err != nil {
		return false, fmt.Errorf("failed to find dataflow workflow by task_id %s error: %w", req.TaskId, err)
	}

	_, err = checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, req.CurrentUser, wf.UserUUID)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *platformDataflowComponentImpl) ReadJobLogsNonStream(ctx context.Context, req types.DataflowLogReq) (string, error) {
	wf, err := c.workflowStore.FindByTaskID(ctx, req.TaskId)
	if err != nil {
		return "", fmt.Errorf("failed to find dataflow workflow by task_id %s error: %w", req.TaskId, err)
	}

	logReq := types.WorkflowLogReq{
		Since:      req.Since,
		SubmitTime: wf.SubmitTime,
	}

	labels := map[string]string{
		types.DFArgoTaskIDKey: req.TaskId,
	}
	if len(req.DagTaskId) > 0 {
		labels[types.DFLabelDagTaskIDKey] = req.DagTaskId
	}

	lokiResp, err := c.deployer.GetWorkflowLogsNonStream(ctx, logReq, labels)
	if err != nil {
		return "", fmt.Errorf("failed to read dataflow job logs, error:%w", err)
	}

	return c.formatLogs(lokiResp), nil
}

func (c *platformDataflowComponentImpl) ReadJobLogsInStream(ctx context.Context, req types.DataflowLogReq) (*deploy.MultiLogReader, error) {
	wf, err := c.workflowStore.FindByTaskID(ctx, req.TaskId)
	if err != nil {
		return nil, fmt.Errorf("fail to find dataflow workflow by task_id %s error: %w", req.TaskId, err)
	}

	logReq := types.WorkflowLogReq{
		CurrentUser: req.CurrentUser,
		Since:       req.Since,
		PodName:     req.TaskId,
		SubmitTime:  wf.SubmitTime,
	}

	labels := map[string]string{
		types.DFArgoTaskIDKey: req.TaskId,
	}
	if len(req.DagTaskId) > 0 {
		labels[types.DFLabelDagTaskIDKey] = req.DagTaskId
	}

	return c.deployer.GetWorkflowLogsInStream(ctx, logReq, labels)
}

func (c *platformDataflowComponentImpl) formatLogs(lokiLog *loki.LokiQueryResponse) string {
	var bulkLog strings.Builder
	for _, item := range lokiLog.Data.Result {
		for _, valuePair := range item.Values {
			for _, log := range strings.Split(valuePair[1], "\n") {
				if log == "" {
					continue
				}
				bulkLog.WriteString(log)
				bulkLog.WriteString(c.config.LogCollector.LineSeparator)
			}
		}
	}
	return strings.TrimSuffix(bulkLog.String(), c.config.LogCollector.LineSeparator)
}
