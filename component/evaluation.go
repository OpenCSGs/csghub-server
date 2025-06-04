package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type evaluationComponentImpl struct {
	deployer              deploy.Deployer
	userStore             database.UserStore
	modelStore            database.ModelStore
	datasetStore          database.DatasetStore
	mirrorStore           database.MirrorStore
	spaceResourceStore    database.SpaceResourceStore
	tokenStore            database.AccessTokenStore
	runtimeFrameworkStore database.RuntimeFrameworksStore
	config                *config.Config
	accountingComponent   AccountingComponent
}

type EvaluationComponent interface {
	// Create argo workflow
	CreateEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.EvaluationRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
}

func NewEvaluationComponent(config *config.Config) (EvaluationComponent, error) {
	c := &evaluationComponentImpl{}
	c.deployer = deploy.NewDeployer()
	c.userStore = database.NewUserStore()
	c.modelStore = database.NewModelStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.datasetStore = database.NewDatasetStore()
	c.mirrorStore = database.NewMirrorStore()
	c.tokenStore = database.NewAccessTokenStore()
	c.runtimeFrameworkStore = database.NewRuntimeFrameworksStore()
	c.config = config
	ac, err := NewAccountingComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create accounting component, %w", err)
	}
	c.accountingComponent = ac
	return c, nil
}

// Create argo workflow
func (c *evaluationComponentImpl) CreateEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error) {
	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user %s, error:%w", req.Username, err)
	}
	result := strings.Split(req.ModelId, "/")
	m, err := c.modelStore.FindByPath(ctx, result[0], result[1])
	if err != nil {
		return nil, fmt.Errorf("cannot find model, %w", err)
	}
	if req.Revision == "" {
		req.Revision = m.Repository.DefaultBranch
	}

	token, err := c.tokenStore.FindByUID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	mirrorRepos, err := c.GenerateMirrorRepoIds(ctx, req.Datasets)
	if err != nil {
		return nil, fmt.Errorf("failed to generate mirror repo ids, %w", err)
	}
	req.Datasets = mirrorRepos
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
		if !common.ContainsGraphicResource(hardware) {
			return nil, fmt.Errorf("evaluation requires graphics card resources")
		}
		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
	} else {
		// for share mode
		hardware.Gpu.Num = c.config.Argo.QuotaGPUNumber
		hardware.Gpu.ResourceName = "nvidia.com/gpu"
		hardware.Cpu.Num = "8"
		hardware.Memory = "32Gi"
	}
	frame, err := c.runtimeFrameworkStore.FindEnabledByID(ctx, req.RuntimeFrameworkId)
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
	return c.deployer.SubmitEvaluation(ctx, req)
}

// generate mirror repo ids
func (c *evaluationComponentImpl) GenerateMirrorRepoIds(ctx context.Context, datasets []string) ([]string, error) {
	var mirrorRepos []string
	for _, ds := range datasets {
		namespace := strings.Split(ds, "/")[0]
		name := strings.Split(ds, "/")[1]
		mirrorRepo, err := c.mirrorStore.FindByRepoPath(ctx, types.DatasetRepo, namespace, name)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				//no mirror, will use csghub repo
				mirrorRepos = append(mirrorRepos, ds)
				continue
			}
			return nil, fmt.Errorf("fail to get mirror repo, %w", err)
		}
		mirrorRepos = append(mirrorRepos, mirrorRepo.SourceRepoPath)
	}
	return mirrorRepos, nil
}

func (c *evaluationComponentImpl) DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	return c.deployer.DeleteEvaluation(ctx, req)
}

// get evaluation result
func (c *evaluationComponentImpl) GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.EvaluationRes, error) {
	wf, err := c.deployer.GetEvaluation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fail to get evaluation result, %w", err)
	}
	datasets, err := c.datasetStore.ListByPath(ctx, wf.Datasets)
	if err != nil {
		return nil, fmt.Errorf("fail to get datasets for evaluation, %w", err)
	}
	var repoTags []types.RepoTags
	for _, ds := range datasets {
		var tags []types.RepoTag
		for _, tag := range ds.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		var dsRepoTags = types.RepoTags{
			RepoId: ds.Repository.Path,
			Tags:   tags,
		}
		repoTags = append(repoTags, dsRepoTags)
	}
	var res = &types.EvaluationRes{
		ID:          wf.ID,
		RepoIds:     wf.RepoIds,
		RepoType:    wf.RepoType,
		Username:    wf.Username,
		TaskName:    wf.TaskName,
		TaskId:      wf.TaskId,
		TaskType:    wf.TaskType,
		TaskDesc:    wf.TaskDesc,
		ResourceId:  wf.ResourceId,
		Status:      string(wf.Status),
		Reason:      wf.Reason,
		Datasets:    repoTags,
		Image:       wf.Image,
		SubmitTime:  wf.SubmitTime,
		StartTime:   wf.StartTime,
		EndTime:     wf.EndTime,
		ResultURL:   wf.ResultURL,
		DownloadURL: wf.DownloadURL,
		FailuresURL: wf.FailuresURL,
	}
	return res, nil
}
