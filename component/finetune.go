package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
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
	config                *config.Config
	accountingComponent   AccountingComponent
	repoComponent         RepoComponent
}

type FinetuneComponent interface {
	// Create argo workflow
	CreateFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error)
	GetFinetuneJob(ctx context.Context, req types.FinetineGetReq) (*types.FinetuneRes, error)
	DeleteFinetuneJob(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
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
	return c, nil
}

// Create finetune argo workflow
func (c *finetuneComponentImpl) CreateFinetuneJob(ctx context.Context, req types.FinetuneReq) (*types.ArgoWorkFlowRes, error) {
	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user %s, error: %w", req.Username, err)
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
		err = c.repoComponent.CheckAccountAndResource(ctx, req.Username, resource.ClusterID, 0, resource)
		if err != nil {
			return nil, err
		}

		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
	} else {
		// for share mode
		resource := ""
		if frame.ComputeType == string(types.ResourceTypeGPU) {
			hardware.Gpu.Num = c.config.Argo.QuotaGPUNumber
			hardware.Gpu.ResourceName = "nvidia.com/gpu"
			resource = fmt.Sprintf("%s GPU · ", c.config.Argo.QuotaGPUNumber)
		}
		hardware.Cpu.Num = "4"
		hardware.Memory = "32Gi"
		resource = fmt.Sprintf("%s%s vCPU · %s", resource, hardware.Cpu.Num, hardware.Memory)
		req.ResourceName = resource
	}

	req.Hardware = hardware
	// choose image
	containerImg := frame.FrameImage
	req.UserUUID = user.UUID
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
	}
	return res, nil
}
