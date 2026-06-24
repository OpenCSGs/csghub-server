package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type finetuneComponentImpl struct {
	deployer              deploy.Deployer
	userStore             database.UserStore
	modelStore            database.ModelStore
	repoStore             database.RepoStore
	datasetStore          database.DatasetStore
	mirrorStore           database.MirrorStore
	spaceResourceStore    database.SpaceResourceStore
	tokenStore            database.AccessTokenStore
	runtimeFrameworkStore database.RuntimeFrameworksStore
	workflowStore         database.ArgoWorkFlowStore
	deployTaskStore       database.DeployTaskStore
	config                *config.Config
	accountingComponent   AccountingComponent
	repoComponent         RepoComponent
	userSvcClient         rpc.UserSvcClient
	clusterStore          database.ClusterInfoStore
}

type FinetuneComponent interface {
	CreateFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error)
	GetFinetuneJob(ctx context.Context, req types.FinetineGetReq) (*types.FinetuneRes, error)
	DeleteFinetuneJob(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	OrgFinetuneInstances(ctx context.Context, req *types.OrgFinetunesReq) ([]types.DeployRequest, int, error)
	OrgFinetuneJobs(ctx context.Context, req *types.OrgFinetunesReq) ([]types.ArgoWorkFlowRes, int, error)
	CheckUserPermission(ctx context.Context, req types.FinetuneLogReq) (bool, *database.ArgoWorkflow, error)
	ReadJobLogsNonStream(ctx context.Context, req types.FinetuneLogReq) (string, error)
	ReadJobLogsInStream(ctx context.Context, req types.FinetuneLogReq) (*deploy.MultiLogReader, error)
}

func NewFinetuneComponent(config *config.Config) (FinetuneComponent, error) {
	c := &finetuneComponentImpl{}
	c.deployer = deploy.NewDeployer()
	c.userStore = database.NewUserStore()
	c.modelStore = database.NewModelStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.datasetStore = database.NewDatasetStore()
	c.mirrorStore = database.NewMirrorStore()
	c.tokenStore = database.NewAccessTokenStore()
	c.repoStore = database.NewRepoStore()
	c.runtimeFrameworkStore = database.NewRuntimeFrameworksStore()
	c.workflowStore = database.NewArgoWorkFlowStore()
	c.deployTaskStore = database.NewDeployTaskStore()
	c.config = config
	ac, err := NewAccountingComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounting component, %w", err)
	}
	c.repoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component, %w", err)
	}
	c.accountingComponent = ac
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	c.userSvcClient = rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	c.clusterStore = database.NewClusterInfoStore()
	return c, nil
}

// Create finetune argo workflow
func (c *finetuneComponentImpl) CreateFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error) {
	operatorUsername := req.Username
	if req.Namespace == "" {
		req.Namespace = operatorUsername
	}
	user, err := c.userStore.FindByUsername(ctx, operatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user %s, error: %w", req.Username, err)
	}

	if !user.CanAdmin() {
		canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, operatorUsername, req.Namespace, membership.RoleWrite)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canWrite {
			return nil, errorx.ErrForbiddenMsg("users do not have permission to create finetune in this namespace")
		}
	}

	// Query model's default branch as revision
	modelParts := strings.Split(req.ModelId, "/")
	if len(modelParts) != 2 {
		return nil, fmt.Errorf("invalid model id format: %s", req.ModelId)
	}
	model, err := c.modelStore.FindByPath(ctx, modelParts[0], modelParts[1])
	if err != nil {
		return nil, fmt.Errorf("cannot find model, %w", err)
	}
	req.Revision = model.Repository.DefaultBranch

	// Query dataset repo so an omitted dataset revision can fall back to its default branch.
	datasetParts := strings.Split(req.DatasetId, "/")
	if len(datasetParts) != 2 {
		return nil, fmt.Errorf("invalid dataset id format: %s", req.DatasetId)
	}
	datasetRepo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, datasetParts[0], datasetParts[1])
	if err != nil {
		return nil, fmt.Errorf("cannot find dataset repo, %w", err)
	}
	req.DatasetRevision = strings.TrimSpace(req.DatasetRevision)
	if req.DatasetRevision == "" {
		req.DatasetRevision = datasetRepo.DefaultBranch
	}

	token, err := c.tokenStore.FindByUID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("can not get git access token for finetune error: %w", err)
	}

	frame, err := c.runtimeFrameworkStore.FindEnabledByID(ctx, req.RuntimeFrameworkId)
	if err != nil {
		return nil, fmt.Errorf("cannot find available runtime framework, %w", err)
	}

	req.Token = token.Token
	var hardware types.HardWare
	if req.ResourceId != 0 {
		resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceId)
		if err != nil {
			return nil, fmt.Errorf("cannot find resource, %w", err)
		}
		err = json.Unmarshal([]byte(resource.Resources), &hardware)
		if err != nil {
			return nil, fmt.Errorf("invalid hardware setting, %w", err)
		}

		// check resource available
		exclusiveResp, err := c.repoComponent.CheckAccountAndResource(ctx, req.Namespace, resource.ClusterID, 0, resource)
		if err != nil {
			return nil, err
		}
		req.NodeAffinity = exclusiveResp.NodeAffinity
		req.Tolerations = exclusiveResp.Tolerations
		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
	} else {
		// Deprecated for share mode
		return nil, fmt.Errorf("share mode is deprecated.")
		// resource := ""
		// if frame.ComputeType == string(types.ResourceTypeGPU) {
		// 	hardware.Gpu.Num = c.config.Argo.QuotaGPUNumber
		// 	hardware.Gpu.ResourceName = c.deployer.GetSharedModeResourceName(c.config)
		// 	resource = fmt.Sprintf("%s GPU · ", c.config.Argo.QuotaGPUNumber)
		// }
		// hardware.Cpu.Num = "4"
		// hardware.Memory = "32Gi"
		// resource = fmt.Sprintf("%s%s vCPU · %s", resource, hardware.Cpu.Num, hardware.Memory)
		// req.ResourceName = resource
	}

	req.Hardware = hardware
	// choose image
	containerImg := frame.FrameImage
	req.UserUUID = user.UUID
	// Persist workflow under owner namespace (user or organization).
	req.Username = req.Namespace
	req.Image = containerImg
	req.RepoType = string(types.ModelRepo)
	req.TaskType = types.TaskTypeFinetune
	req.DownloadEndpoint = c.config.Model.DownloadEndpoint
	slog.Debug("submit finetune request to deployer", slog.Any("req", req))
	return c.deployer.SubmitFinetuneJob(ctx, req)
}

func (c *finetuneComponentImpl) DeleteFinetuneJob(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	wf, err := c.workflowStore.FindByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("failed to get finetune workflow by id %d error: %w", req.ID, err)
	}

	req.TaskID = wf.TaskId
	req.Namespace = wf.Namespace
	req.ClusterID = wf.ClusterID
	err = c.workflowStore.DeleteWorkFlow(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("failed to delete finetune record %d result, %w", req.ID, err)
	}

	return c.deployer.DeleteFinetuneJob(ctx, req)
}

// get finetune result
func (c *finetuneComponentImpl) GetFinetuneJob(ctx context.Context, req types.FinetineGetReq) (*types.FinetuneRes, error) {
	wf, err := c.workflowStore.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to find finetune workflow by id %d error: %w", req.ID, err)
	}

	var res = &types.FinetuneRes{
		ID:           wf.ID,
		RepoIds:      wf.RepoIds,
		RepoType:     wf.RepoType,
		Username:     wf.Username,
		TaskName:     wf.TaskName,
		TaskId:       wf.TaskId,
		TaskType:     wf.TaskType,
		TaskDesc:     wf.TaskDesc,
		ResourceId:   wf.ResourceId,
		ResourceName: wf.ResourceName,
		Status:       string(wf.Status),
		Reason:       wf.Reason,
		Image:        wf.Image,
		SubmitTime:   wf.SubmitTime,
		StartTime:    wf.StartTime,
		EndTime:      wf.EndTime,
		ResultURL:    wf.ResultURL,
		Datasets:     wf.Datasets,
	}
	return res, nil
}

func (c *finetuneComponentImpl) OrgFinetuneInstances(ctx context.Context, req *types.OrgFinetunesReq) ([]types.DeployRequest, int, error) {
	if req.CurrentUser != "" {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canRead {
			return nil, 0, errorx.ErrForbiddenMsg("users do not have permission to view finetunes in this namespace")
		}
	}

	priceData, err := c.accountingComponent.QueryPricesBySKUType("", types.AcctPriceListReq{
		SkuType: types.SKUCSGHub,
		Per:     int(math.MaxInt16),
		Page:    1,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("get price data with error: %w", err)
	}

	// List org finetunes from deploys (owner_namespace); model-instance finetunes are stored here.
	deployReq := &types.DeployReq{
		PageOpts:   types.PageOpts{Page: req.Page, PageSize: req.PageSize},
		DeployType: types.FinetuneType,
		RepoType:   types.ModelRepo,
	}
	deploys, total, err := c.deployTaskStore.ListDeployByOwnerNamespace(ctx, req.Namespace, deployReq)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get org finetunes, error: %w", err)
	}

	var resDeploys []types.DeployRequest
	for _, deploy := range deploys {
		resDeploys = append(resDeploys, types.DeployRequest{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             deploy.GitPath,
			SpaceID:          deploy.SpaceID,
			UserID:           deploy.UserID,
			RepoID:           deploy.RepoID,
			ModelID:          deploy.ModelID,
			SvcName:          deploy.SvcName,
			Status:           deployStatusCodeToString(deploy.Status),
			Hardware:         deploy.Hardware,
			Env:              deploy.Env,
			RuntimeFramework: deploy.RuntimeFramework,
			ImageID:          deploy.ImageID,
			MinReplica:       deploy.MinReplica,
			MaxReplica:       deploy.MaxReplica,
			GitPath:          deploy.GitPath,
			GitBranch:        deploy.GitBranch,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Type:             deploy.Type,
			UserUUID:         deploy.UserUUID,
			PayMode:          c.getPayMode(deploy.SKU, priceData),
		})
	}
	return resDeploys, total, nil
}

func (c *finetuneComponentImpl) OrgFinetuneJobs(ctx context.Context, req *types.OrgFinetunesReq) ([]types.ArgoWorkFlowRes, int, error) {
	if req.CurrentUser != "" {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canRead {
			return nil, 0, errorx.ErrForbiddenMsg("users do not have permission to view finetune jobs in this namespace")
		}
	}

	priceData, err := c.accountingComponent.QueryPricesBySKUType("", types.AcctPriceListReq{
		SkuType: types.SKUCSGHub,
		Per:     int(math.MaxInt16),
		Page:    1,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("get price data with error: %w", err)
	}

	workflows, total, err := c.workflowStore.FindByUsername(ctx, req.Namespace, types.TaskTypeFinetune, req.PageSize, req.Page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get org finetune jobs, error: %w", err)
	}

	var res []types.ArgoWorkFlowRes
	for _, wf := range workflows {
		res = append(res, types.ArgoWorkFlowRes{
			ID:           wf.ID,
			RepoIds:      wf.RepoIds,
			RepoType:     wf.RepoType,
			TaskName:     wf.TaskName,
			Username:     wf.Username,
			TaskId:       wf.TaskId,
			Status:       wf.Status,
			TaskType:     wf.TaskType,
			TaskDesc:     wf.TaskDesc,
			Datasets:     wf.Datasets,
			ResourceId:   wf.ResourceId,
			ResourceName: wf.ResourceName,
			Reason:       wf.Reason,
			SubmitTime:   wf.SubmitTime,
			StartTime:    wf.StartTime,
			EndTime:      wf.EndTime,
			DownloadURL:  wf.DownloadURL,
			ResultURL:    wf.ResultURL,
			Image:        wf.Image,
			PayMode:      c.getPayMode(fmt.Sprintf("%d", wf.ResourceId), priceData),
		})
	}
	return res, total, nil
}

func (c *finetuneComponentImpl) getPayMode(resourceID string, priceData *database.PriceResp) types.PayMode {
	payMode := types.PayModeFree
	if priceData != nil {
		price := getAccountPrice(resourceID, types.SKUPayAsYouGo, priceData.Prices)
		if price != nil {
			payMode = types.PayMode(price.SkuUnitType)
		}
	}
	return payMode
}

func (c *finetuneComponentImpl) CheckUserPermission(ctx context.Context, req types.FinetuneLogReq) (bool, *database.ArgoWorkflow, error) {
	var (
		err   error
		wf    *database.ArgoWorkflow
		wfObj database.ArgoWorkflow
	)

	if len(req.TaskID) > 0 {
		wf, err = c.workflowStore.FindByTaskID(ctx, req.TaskID)
	} else if req.ID > 0 {
		wfObj, err = c.workflowStore.FindByID(ctx, req.ID)
		wf = &wfObj
	} else {
		return false, nil, nil
	}

	if err != nil {
		return false, nil, fmt.Errorf("failed to find finetune workflow job by id %d or task id %s error: %w",
			req.ID, req.TaskID, err)
	}

	_, err = checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, req.CurrentUser, wf.UserUUID)
	if err != nil {
		return false, nil, errorx.ErrForbidden
	}

	return true, wf, nil
}

func (c *finetuneComponentImpl) ReadJobLogsNonStream(ctx context.Context, req types.FinetuneLogReq) (string, error) {
	allow, wf, err := c.CheckUserPermission(ctx, req)
	if err != nil {
		return "", err
	}

	if !allow {
		return "", errorx.ErrForbidden
	}

	req.PodName = wf.TaskId
	req.SubmitTime = wf.SubmitTime

	labels := map[string]string{
		types.StreamKeyInstanceName: req.PodName,
	}

	lokiResp, err := c.deployer.GetWorkflowLogsNonStream(ctx, req, labels)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow job logs, error:%w", err)
	}

	return c.formatLogs(lokiResp), nil
}

func (c *finetuneComponentImpl) ReadJobLogsInStream(ctx context.Context, req types.FinetuneLogReq) (*deploy.MultiLogReader, error) {
	allow, wf, err := c.CheckUserPermission(ctx, req)
	if err != nil {
		return nil, err
	}
	if !allow {
		return nil, errorx.ErrForbidden
	}

	req.PodName = wf.TaskId
	req.SubmitTime = wf.SubmitTime

	labels := map[string]string{
		types.StreamKeyInstanceName: req.PodName,
	}

	return c.deployer.GetWorkflowLogsInStream(ctx, req, labels)
}

func (c *finetuneComponentImpl) formatLogs(lokiLog *loki.LokiQueryResponse) string {
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
