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
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type evaluationComponentImpl struct {
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
	userSvcClient         rpc.UserSvcClient
	clusterStore          database.ClusterInfoStore
}

type EvaluationComponent interface {
	CreateEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.EvaluationRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	OrgEvaluations(ctx context.Context, req *types.OrgEvaluationsReq) ([]types.ArgoWorkFlowRes, int, error)
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
	c.userSvcClient = rpc.NewUserSvcHttpClient(
		fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken),
	)
	c.clusterStore = database.NewClusterInfoStore()
	return c, nil
}

// Create argo workflow
func (c *evaluationComponentImpl) CreateEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error) {
	operatorUsername := req.Username
	if req.OwnerNamespace == "" {
		req.OwnerNamespace = operatorUsername
	}
	user, err := c.userStore.FindByUsername(ctx, operatorUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to get current user %s, error:%w", req.Username, err)
	}

	if !user.CanAdmin() {
		canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, operatorUsername, req.OwnerNamespace, membership.RoleWrite)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canWrite {
			return nil, errorx.ErrForbiddenMsg("users do not have permission to create evaluation in this namespace")
		}
	}

	if req.ModelIds == nil {
		req.ModelIds = []string{}
	}
	if req.ModelId != "" {
		req.ModelIds = append(req.ModelIds, req.ModelId)
	}
	for _, modelId := range req.ModelIds {
		result := strings.Split(modelId, "/")
		if len(result) != 2 {
			return nil, fmt.Errorf("invalid model id format: %s", modelId)
		}
		m, err := c.modelStore.FindByPath(ctx, result[0], result[1])
		if err != nil {
			return nil, fmt.Errorf("cannot find model, %w", err)
		}
		req.Revisions = append(req.Revisions, m.Repository.DefaultBranch)
	}

	token, err := c.tokenStore.FindByUID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	var mirrorRepos []string
	var datasetRevisions []string
	if req.CustomDataSets != nil {
		mirrorRepos, datasetRevisions, err = c.generateDatasetsAndTasks(ctx, req.CustomDataSets)
		req.UseCustomDataset = true
	} else {
		mirrorRepos, datasetRevisions, err = c.GenerateMirrorRepoIds(ctx, req.Datasets)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate mirror repo ids, %w", err)
	}
	frame, err := c.runtimeFrameworkStore.FindEnabledByID(ctx, req.RuntimeFrameworkId)
	if err != nil {
		return nil, fmt.Errorf("cannot find available runtime framework, %w", err)
	}
	req.Datasets = mirrorRepos
	req.DatasetRevisions = datasetRevisions
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
		if frame.ComputeType != string(types.ResourceTypeCPU) && !common.ContainsGraphicResource(hardware) {
			return nil, fmt.Errorf("evaluation requires graphics card resources")
		}
		// check resource available
		exclusiveResp, err := c.repoComponent.CheckAccountAndResource(ctx, req.OwnerNamespace, resource.ClusterID, 0, resource)
		if err != nil {
			return nil, err
		}
		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
		req.NodeAffinity = exclusiveResp.NodeAffinity
		req.Tolerations = exclusiveResp.Tolerations
	} else {
		// for share mode
		resource := ""
		if frame.ComputeType == string(types.ResourceTypeGPU) {
			hardware.Gpu.Num = c.config.Argo.QuotaGPUNumber
			hardware.Gpu.ResourceName = c.deployer.GetSharedModeResourceName(c.config)
			resource = fmt.Sprintf("%s GPU · ", c.config.Argo.QuotaGPUNumber)
		}
		hardware.Cpu.Num = "4"
		hardware.Memory = "32Gi"
		resource = fmt.Sprintf("%s%s vCPU · %s", resource, hardware.Cpu.Num, hardware.Memory)
		req.ResourceName = resource
	}

	clusterNodes, err := c.clusterStore.FindNodeByClusterID(ctx, req.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to find nodes by cluster id, clusterID: %s, error: %w", req.ClusterID, err)
	}

	for _, node := range clusterNodes {
		req.Nodes = append(req.Nodes, types.Node{
			Name:       node.Name,
			EnableVXPU: node.EnableVXPU,
			HasXPU:     node.Hardware.HasXPU() || node.EnableVXPU,
		})
	}

	req.Hardware = hardware
	// choose image
	containerImg := frame.FrameImage
	billingUUID := user.UUID
	if req.OwnerNamespace != operatorUsername {
		resolved, err := c.repoComponent.GetNamespaceBillingUUID(ctx, req.OwnerNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve billing UUID for namespace %s, error: %w", req.OwnerNamespace, err)
		}
		billingUUID = resolved
	}
	req.UserUUID = billingUUID
	// Persist workflow under owner namespace (user or organization).
	req.Username = req.OwnerNamespace
	req.Image = containerImg
	req.RepoType = string(types.ModelRepo)
	req.TaskType = types.TaskTypeEvaluation
	req.DownloadEndpoint = c.config.Model.DownloadEndpoint
	return c.deployer.SubmitEvaluation(ctx, req)
}

// generate mirror repo ids
func (c *evaluationComponentImpl) GenerateMirrorRepoIds(ctx context.Context, datasets []string) ([]string, []string, error) {
	var mirrorRepos []string
	var revisions []string
	for _, ds := range datasets {
		parts := strings.Split(ds, "/")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid dataset path: %s", ds)
		}
		namespace := parts[0]
		name := parts[1]
		repo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, namespace, name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find dataset repo, %w", err)
		}
		mirrorRepos = append(mirrorRepos, repo.OriginPath())
		revisions = append(revisions, repo.DefaultBranch)
	}
	return mirrorRepos, revisions, nil
}

func (c *evaluationComponentImpl) generateDatasetsAndTasks(ctx context.Context, customDataSets []string) ([]string, []string, error) {
	var mirrorRepos []string
	var revisions []string
	for _, cds := range customDataSets {
		parts := strings.Split(cds, "/")
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("invalid dataset path: %s", cds)
		}
		namespace := parts[0]
		name := parts[1]
		repo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, namespace, name)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to find dataset repo, %w", err)
		}
		mirrorRepos = append(mirrorRepos, repo.Path)
		revisions = append(revisions, repo.DefaultBranch)
	}
	return mirrorRepos, revisions, nil
}

func (c *evaluationComponentImpl) DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	wf, err := c.workflowStore.FindByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("fail to get evaluation result, %w", err)
	}
	if wf.Username != req.Username {
		canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, wf.Username, membership.RoleWrite)
		if err != nil {
			return fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canWrite {
			return errorx.ErrForbiddenMsg("users do not have permission to delete evaluation in this namespace")
		}
	}
	req.TaskID = wf.TaskId
	req.Namespace = wf.Namespace
	req.ClusterID = wf.ClusterID
	err = c.workflowStore.DeleteWorkFlow(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("failed to delete evaluation result, %w", err)
	}
	return c.deployer.DeleteEvaluation(ctx, req)
}

// get evaluation result
func (c *evaluationComponentImpl) GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.EvaluationRes, error) {
	wf, err := c.workflowStore.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to get evaluation result, %w", err)
	}
	if wf.Username != req.Username {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, wf.Username, membership.RoleRead)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canRead {
			return nil, errorx.ErrForbidden
		}
	}
	var repoTags []types.RepoTags
	for _, path := range wf.Datasets {
		ds, err := c.datasetStore.FindByOriginPath(ctx, path)
		if err != nil {
			//use default value if not found
			var dsRepoTags = types.RepoTags{
				RepoId: path,
			}
			if errors.Is(err, sql.ErrNoRows) {
				dsRepoTags.Deleted = true
			}
			repoTags = append(repoTags, dsRepoTags)
			continue
		}
		var tags []types.RepoTag
		for _, tag := range ds.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
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
		Datasets:     repoTags,
		Image:        wf.Image,
		SubmitTime:   wf.SubmitTime,
		StartTime:    wf.StartTime,
		EndTime:      wf.EndTime,
		ResultURL:    wf.ResultURL,
		DownloadURL:  wf.DownloadURL,
		FailuresURL:  wf.FailuresURL,
	}
	return res, nil
}

func (c *evaluationComponentImpl) OrgEvaluations(ctx context.Context, req *types.OrgEvaluationsReq) ([]types.ArgoWorkFlowRes, int, error) {
	if req.CurrentUser != "" {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canRead {
			return nil, 0, errorx.ErrForbiddenMsg("users do not have permission to view evaluations in this namespace")
		}
	}

	workflows, total, err := c.workflowStore.FindByUsername(ctx, req.Namespace, types.TaskTypeEvaluation, req.PageSize, req.Page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get org evaluations, error: %w", err)
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
		})
	}
	return res, total, nil
}
