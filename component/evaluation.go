package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type evaluationComponentImpl struct {
	deployer   deploy.Deployer
	us         database.UserStore
	ms         database.ModelStore
	ss         database.SpaceResourceStore
	tokenStore database.AccessTokenStore
	rtfm       database.RuntimeFrameworksStore
	config     *config.Config
	ac         AccountingComponent
}

type EvaluationComponent interface {
	// Create argo workflow
	CreateEvaluation(ctx context.Context, req *types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
}

func NewEvaluationComponent(config *config.Config) (EvaluationComponent, error) {
	c := &evaluationComponentImpl{}
	c.deployer = deploy.NewDeployer()
	c.us = database.NewUserStore()
	c.ms = database.NewModelStore()
	c.ss = database.NewSpaceResourceStore()
	c.tokenStore = database.NewAccessTokenStore()
	c.rtfm = database.NewRuntimeFrameworksStore()
	c.config = config
	ac, err := NewAccountingComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounting component, %w", err)
	}
	c.ac = ac
	return c, nil
}

// Create argo workflow
func (c *evaluationComponentImpl) CreateEvaluation(ctx context.Context, req *types.EvaluationReq) (*types.ArgoWorkFlowRes, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user %s, error:%w", req.Username, err)
	}
	result := strings.Split(req.ModelId, "/")
	_, err = c.ms.FindByPath(ctx, result[0], result[1])
	if err != nil {
		return nil, fmt.Errorf("cannot find model, %w", err)
	}

	token, err := c.tokenStore.FindByUID(context.Background(), user.ID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	req.Token = token.Token
	var hardware types.HardWare
	if req.ResourceId != 0 {
		resource, err := c.ss.FindByID(ctx, req.ResourceId)
		if err != nil {
			return nil, fmt.Errorf("cannot find resource, %w", err)
		}
		err = json.Unmarshal([]byte(resource.Resources), &hardware)
		if err != nil {
			return nil, fmt.Errorf("invalid hardware setting, %w", err)
		}
		if hardware.Gpu.Num == "" {
			return nil, fmt.Errorf("evaluation requires GPU or NPU resources")
		}
		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
	} else {
		// for share mode
		hardware.Gpu.Num = c.config.Argo.QuotaGPUNumber
		hardware.Gpu.ResourceName = "nvidia.com/gpu"
		hardware.Cpu.Num = "12"
		hardware.Memory = "32Gi"
	}
	frame, err := c.rtfm.FindEnabledByID(ctx, req.RuntimeFrameworkId)
	if err != nil {
		return nil, fmt.Errorf("cannot find available runtime framework, %w", err)
	}
	req.Hardware = hardware
	// choose image
	containerImg := frame.FrameImage
	req.UserUUID = user.UUID
	req.Image = containerImg
	req.RepoType = string(types.ModelRepo)
	req.TaskType = types.TaskTypeEvaluation
	req.DownloadEndpoint = c.config.Model.DownloadEndpoint
	return c.deployer.SubmitEvaluation(ctx, *req)
}

func (c *evaluationComponentImpl) DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	return c.deployer.DeleteEvaluation(ctx, req)
}
