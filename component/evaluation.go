package component

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	v1alpha1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/loki"
	"opencsg.com/csghub-server/builder/rebac"
	rebacfactory "opencsg.com/csghub-server/builder/rebac/factory"
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
	rebac                 rebac.Authorizer
	git                   gitserver.GitServer
}

type EvaluationComponent interface {
	CreateEvaluation(ctx context.Context, req types.EvaluationReq) (*types.ArgoWorkFlowRes, error)
	GetEvaluation(ctx context.Context, req types.EvaluationGetReq) (*types.EvaluationRes, error)
	DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error
	OrgEvaluations(ctx context.Context, req *types.OrgEvaluationsReq) ([]types.ArgoWorkFlowRes, int, error)
	ReadJobLogsNonStream(ctx context.Context, req types.EvaluationLogReq) (string, error)
	ReadJobLogsInStream(ctx context.Context, req types.EvaluationLogReq) (*deploy.MultiLogReader, error)
}

// evaluationTaskTypes limits list queries; log endpoints accept more types via isEvaluationWorkflowTaskType.
var evaluationTaskTypes = []types.TaskType{types.TaskTypeEvaluation, types.TaskTypeClawEval}

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
	c.rebac, err = rebacfactory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to create ReBAC authorizer, error: %w", err)
	}
	c.accountingComponent = ac
	c.userSvcClient = rpc.NewUserSvcHttpClient(
		fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken),
	)
	c.clusterStore = database.NewClusterInfoStore()
	c.git, err = git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}
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
		canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, operatorUsername, req.OwnerNamespace, rebac.NamespaceCanWrite)
		if err != nil {
			return nil, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canWrite {
			return nil, errorx.ErrForbiddenMsg("users do not have permission to create evaluation in this namespace")
		}
	}

	frame, err := c.runtimeFrameworkStore.FindEnabledByID(ctx, req.RuntimeFrameworkId)
	if err != nil {
		return nil, fmt.Errorf("cannot find available runtime framework, %w", err)
	}
	if strings.Contains(frame.FrameName, types.ClawEvalFrameName) {
		return c.createClawEvaluation(ctx, req, user, frame)
	}

	if err := c.resolveModelVersions(ctx, &req); err != nil {
		return nil, err
	}

	req.FrameworkConfig, err = normalizeFrameworkConfig(frame.FrameName, req.FrameworkConfig)
	if err != nil {
		return nil, err
	}

	token, err := c.tokenStore.FindByUID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("cant get git access token:%w", err)
	}
	var datasets datasetVersions
	if req.CustomDataSets != nil {
		datasets, err = c.generateDatasetsAndTasks(ctx, req.CustomDataSets)
		req.UseCustomDataset = true
	} else {
		datasets, err = c.GenerateMirrorRepoIds(ctx, req.Datasets)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to generate mirror repo ids, %w", err)
	}
	req.Datasets = datasets.Paths
	req.DatasetRevisions = datasets.Revisions
	req.DatasetCommits = datasets.Commits
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
		exclusiveResp, err := c.repoComponent.CheckAccountAndResource(ctx,
			types.CheckResourceAndAccountReq{
				UserName:      req.OwnerNamespace,
				ClusterID:     resource.ClusterID,
				OrderDetailID: 0,
				CurrentUser:   req.Username,
			},
			resource)
		if err != nil {
			return nil, err
		}
		req.ClusterID = resource.ClusterID
		req.ResourceName = resource.Name
		req.NodeAffinity = exclusiveResp.NodeAffinity
		req.Tolerations = exclusiveResp.Tolerations
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

// frameworkConfigSupported lists the frameworks whose images act on a framework
// configuration. A framework absent from this list rejects a non-empty configuration
// rather than accepting one it would silently ignore.
var frameworkConfigSupported = map[string]struct{}{
	"evalscope":     {},
	"amd-evalscope": {},
}

// The evalscope images carry their own defaults for these two knobs. The server fills
// them in and sends both on every task, so the configuration recorded in the snapshot
// is always the configuration that produced the scores, and the image defaults are
// never the ones in force. Keep these in step with docker/evaluation/evalscope/start.sh.
const (
	defaultEvaluationLimit            = 10
	defaultEvaluationGenerationConfig = `{"max_tokens":30000,"do_sample":false}`
)

// normalizeFrameworkConfig turns the caller's configuration into the one that will
// actually run. It rejects a configuration the chosen framework cannot act on and one
// carrying fields outside the declared contract, then fills in the effective defaults
// so that recording the result is the same as recording what ran. A framework that
// cannot act on a configuration yields an empty one.
func normalizeFrameworkConfig(frameName, raw string) (string, error) {
	_, supported := frameworkConfigSupported[frameName]
	if !supported {
		if strings.TrimSpace(raw) != "" {
			return "", fmt.Errorf("runtime framework %s does not support framework_config", frameName)
		}
		return "", nil
	}

	var cfg types.EvaluationFrameworkConfig
	if strings.TrimSpace(raw) != "" {
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return "", fmt.Errorf("invalid framework_config: %w", err)
		}
	}
	if cfg.Limit != nil && *cfg.Limit < 1 {
		return "", fmt.Errorf("invalid framework_config: limit must be at least 1")
	}

	if cfg.Limit == nil {
		limit := defaultEvaluationLimit
		cfg.Limit = &limit
	}
	if len(cfg.GenerationConfig) == 0 {
		if err := json.Unmarshal([]byte(defaultEvaluationGenerationConfig), &cfg.GenerationConfig); err != nil {
			return "", fmt.Errorf("invalid default generation config: %w", err)
		}
	}
	normalized, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("invalid framework_config: %w", err)
	}
	return string(normalized), nil
}

// maxEvaluationModelVersions caps how many model versions a single task may evaluate.
const maxEvaluationModelVersions = 10

// resolveModelVersions normalizes the model input of an evaluation request into
// index-aligned ModelIds and Revisions. Every entry is pinned to the commit it resolves
// to at submit time, so a task stays reproducible even after the branch moves on.
// Models takes precedence when present; ModelId and ModelIds keep their previous
// meaning and resolve to the head commit of the repository default branch. The same
// model may be listed more than once as long as the revisions differ.
func (c *evaluationComponentImpl) resolveModelVersions(ctx context.Context, req *types.EvaluationReq) error {
	refs := req.Models
	if len(refs) == 0 {
		modelIds := req.ModelIds
		if req.ModelId != "" {
			modelIds = append(modelIds, req.ModelId)
		}
		for _, modelId := range modelIds {
			refs = append(refs, types.EvaluationModelRef{RepoId: modelId})
		}
	}
	if len(refs) == 0 {
		return fmt.Errorf("at least one model is required")
	}
	if len(refs) > maxEvaluationModelVersions {
		return fmt.Errorf("at most %d model versions can be evaluated in one task", maxEvaluationModelVersions)
	}

	modelIds := make([]string, 0, len(refs))
	revisions := make([]string, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		parts := strings.Split(ref.RepoId, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid model id format: %s", ref.RepoId)
		}
		m, err := c.modelStore.FindByPath(ctx, parts[0], parts[1])
		if err != nil {
			return fmt.Errorf("cannot find model, %w", err)
		}
		wanted := ref.Revision
		if wanted == "" {
			if m.Repository == nil {
				return fmt.Errorf("model %s has no repository", ref.RepoId)
			}
			wanted = m.Repository.DefaultBranch
		}
		revision, err := c.resolveRepoRevision(ctx, types.ModelRepo, parts[0], parts[1], wanted)
		if err != nil {
			return err
		}
		key := ref.RepoId + "@" + revision
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		modelIds = append(modelIds, ref.RepoId)
		revisions = append(revisions, revision)
	}

	req.ModelIds = modelIds
	req.Revisions = revisions
	return nil
}

// resolveRepoRevision turns a branch, tag or commit SHA into the full commit SHA it
// points at. Gitaly reports an unknown revision as an empty commit rather than an
// error, so an empty ID is treated as not found.
func (c *evaluationComponentImpl) resolveRepoRevision(ctx context.Context, repoType types.RepositoryType, namespace, name, ref string) (string, error) {
	commit, err := c.git.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  repoType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve revision %s of %s %s/%s, error: %w", ref, repoType, namespace, name, err)
	}
	if commit == nil || commit.ID == "" {
		return "", fmt.Errorf("revision %s does not exist in %s/%s", ref, namespace, name)
	}
	return commit.ID, nil
}

// datasetVersions holds the dataset paths handed to the evaluation image, the revisions
// used to download them, and the commits pinned into the task snapshot.
type datasetVersions struct {
	Paths     []string
	Revisions []string
	Commits   []string
}

// generate mirror repo ids
func (c *evaluationComponentImpl) GenerateMirrorRepoIds(ctx context.Context, datasets []string) (datasetVersions, error) {
	return c.resolveDatasetVersions(ctx, datasets, func(repo *database.Repository) string {
		return repo.OriginPath()
	})
}

func (c *evaluationComponentImpl) generateDatasetsAndTasks(ctx context.Context, customDataSets []string) (datasetVersions, error) {
	return c.resolveDatasetVersions(ctx, customDataSets, func(repo *database.Repository) string {
		return repo.Path
	})
}

// resolveDatasetVersions looks up each dataset repo and records both the branch used for
// download and the commit that branch pointed at when the task was submitted. A commit
// that cannot be resolved is left empty instead of failing the submission, because the
// commit is snapshot metadata and the download still uses the branch.
func (c *evaluationComponentImpl) resolveDatasetVersions(ctx context.Context, datasets []string, pathOf func(*database.Repository) string) (datasetVersions, error) {
	var res datasetVersions
	for _, ds := range datasets {
		parts := strings.Split(ds, "/")
		if len(parts) != 2 {
			return datasetVersions{}, fmt.Errorf("invalid dataset path: %s", ds)
		}
		namespace := parts[0]
		name := parts[1]
		repo, err := c.repoStore.FindByPath(ctx, types.DatasetRepo, namespace, name)
		if err != nil {
			return datasetVersions{}, fmt.Errorf("failed to find dataset repo, %w", err)
		}
		commit, err := c.resolveRepoRevision(ctx, types.DatasetRepo, namespace, name, repo.DefaultBranch)
		if err != nil {
			slog.WarnContext(ctx, "failed to pin dataset commit for evaluation",
				slog.String("dataset", ds), slog.Any("error", err))
			commit = ""
		}
		res.Paths = append(res.Paths, pathOf(repo))
		res.Revisions = append(res.Revisions, repo.DefaultBranch)
		res.Commits = append(res.Commits, commit)
	}
	return res, nil
}

func (c *evaluationComponentImpl) DeleteEvaluation(ctx context.Context, req types.ArgoWorkFlowDeleteReq) error {
	wf, err := c.workflowStore.FindByID(ctx, req.ID)
	if err != nil {
		return fmt.Errorf("fail to get evaluation result, %w", err)
	}
	if wf.Username != req.Username {
		canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, wf.Username, rebac.NamespaceCanWrite)
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
	var (
		wf  database.ArgoWorkflow
		err error
	)
	if req.TaskID != "" {
		wfObj, findErr := c.workflowStore.FindByTaskID(ctx, req.TaskID)
		if findErr != nil {
			return nil, fmt.Errorf("fail to get evaluation result, %w", findErr)
		}
		wf = *wfObj
	} else {
		wf, err = c.workflowStore.FindByID(ctx, req.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("fail to get evaluation result, %w", err)
	}
	if !isEvaluationResultTaskType(wf.TaskType) {
		return nil, errorx.ErrForbiddenMsg("workflow is not an evaluation job")
	}
	if wf.Username != req.Username {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, wf.Username, rebac.NamespaceCanRead)
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

		RepoRevisions:    wf.RepoRevisions,
		DatasetRevisions: wf.DatasetRevisions,
		FrameworkConfig:  wf.FrameworkConfig,
		Hardware:         wf.Hardware,
	}
	attachClawEvalSummary(ctx, res)
	return res, nil
}

func isEvaluationResultTaskType(taskType types.TaskType) bool {
	switch taskType {
	case types.TaskTypeEvaluation, types.TaskTypeClawEval:
		return true
	default:
		return false
	}
}

func (c *evaluationComponentImpl) OrgEvaluations(ctx context.Context, req *types.OrgEvaluationsReq) ([]types.ArgoWorkFlowRes, int, error) {
	if req.CurrentUser != "" {
		canRead, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, rebac.NamespaceCanRead)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to check namespace permission, error: %w", err)
		}
		if !canRead {
			return nil, 0, errorx.ErrForbiddenMsg("users do not have permission to view evaluations in this namespace")
		}
	}

	workflows, total, err := c.workflowStore.FindByUsernameWithTaskTypes(ctx, req.Namespace, evaluationTaskTypes, req.PageSize, req.Page)
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

			RepoRevisions:    wf.RepoRevisions,
			DatasetRevisions: wf.DatasetRevisions,
			FrameworkConfig:  wf.FrameworkConfig,
			Hardware:         wf.Hardware,
		})
	}
	return res, total, nil
}

func (c *evaluationComponentImpl) createClawEvaluation(ctx context.Context, req types.EvaluationReq, user database.User, frame *database.RuntimeFramework) (*types.ArgoWorkFlowRes, error) {
	operatorUsername := req.Username
	if req.Model == "" {
		req.Model = req.ModelId
	}
	if req.TaskName == "" {
		return nil, fmt.Errorf("task_name is required")
	}
	if req.ResourceId == 0 {
		return nil, fmt.Errorf("resource_id is required")
	}
	if req.Model == "" {
		return nil, fmt.Errorf("model is required")
	}
	if req.BaseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}

	billingUUID := user.UUID
	if req.OwnerNamespace != operatorUsername {
		resolved, err := c.repoComponent.GetNamespaceBillingUUID(ctx, req.OwnerNamespace)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve billing UUID for namespace %s, error: %w", req.OwnerNamespace, err)
		}
		billingUUID = resolved
	}

	var hardware types.HardWare
	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceId)
	if err != nil {
		return nil, fmt.Errorf("cannot find resource, %w", err)
	}
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return nil, fmt.Errorf("invalid hardware setting, %w", err)
	}
	if frame.ComputeType != string(types.ResourceTypeCPU) && !common.ContainsGraphicResource(hardware) {
		return nil, fmt.Errorf("claw evaluation requires graphics card resources when runtime framework uses GPU")
	}

	exclusiveResp, err := c.repoComponent.CheckAccountAndResource(ctx,
		types.CheckResourceAndAccountReq{
			UserName:      req.OwnerNamespace,
			ClusterID:     resource.ClusterID,
			OrderDetailID: 0,
			CurrentUser:   req.Username,
		},
		resource)
	if err != nil {
		return nil, err
	}
	req.ClusterID = resource.ClusterID
	req.ResourceName = resource.Name
	req.NodeAffinity = exclusiveResp.NodeAffinity
	req.Tolerations = exclusiveResp.Tolerations
	req.Hardware = hardware

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

	req.UserUUID = billingUUID
	req.Username = req.OwnerNamespace
	req.Image = frame.FrameImage
	req.RepoType = string(types.ModelRepo)
	req.TaskType = types.TaskTypeClawEval

	clawReq := req.ToClawEvaluationReq()
	var defaultAPIKey string
	resolveDefaultAPIKey := func() (string, error) {
		if defaultAPIKey != "" {
			return defaultAPIKey, nil
		}
		apiKey, err := c.resolveClawEvalAIGatewayAPIKey(ctx, billingUUID, req.OwnerNamespace, operatorUsername)
		if err != nil {
			return "", err
		}
		defaultAPIKey = apiKey
		return defaultAPIKey, nil
	}
	if clawReq.ApiKey == "" {
		apiKey, err := resolveDefaultAPIKey()
		if err != nil {
			return nil, err
		}
		clawReq.ApiKey = apiKey
	}
	if !req.NoJudge {
		judgeAPIKey, err := resolveDefaultAPIKey()
		if err != nil {
			return nil, err
		}
		clawReq.JudgeBaseURL = c.config.PublicAIGatewayURL()
		clawReq.JudgeApiKey = judgeAPIKey
	}
	res, err := c.deployer.SubmitClawEvaluation(ctx, clawReq)
	if err != nil {
		return nil, err
	}
	if res == nil || res.TaskId == "" {
		return nil, fmt.Errorf("runner returned empty claw evaluation task id")
	}
	wf, err := c.workflowStore.CreateWorkFlow(ctx, database.ArgoWorkflow{
		Username:     req.Username,
		UserUUID:     req.UserUUID,
		TaskName:     req.TaskName,
		TaskId:       res.TaskId,
		TaskType:     req.TaskType,
		RepoIds:      []string{req.Model},
		TaskDesc:     req.TaskDesc,
		Image:        req.Image,
		ResourceId:   req.ResourceId,
		ResourceName: req.ResourceName,
		ClusterID:    req.ClusterID,
		RepoType:     req.RepoType,
		Namespace:    c.config.Cluster.SpaceNamespace,
		Status:       v1alpha1.WorkflowPending,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create claw evaluation placeholder workflow: %w", err)
	}
	res.ID = wf.ID
	res.ResourceId = wf.ResourceId
	res.ResourceName = wf.ResourceName
	res.Image = wf.Image
	res.Status = wf.Status
	return res, nil
}

func (c *evaluationComponentImpl) resolveClawEvalAIGatewayAPIKey(ctx context.Context, billingUUID, ownerNamespace, operatorUsername string) (string, error) {
	token, err := c.tokenStore.FindBuiltinByNsUUID(ctx, billingUUID, string(types.AccessTokenAppAIGateway))
	if err == nil && token != nil && token.Token != "" {
		return token.Token, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, errorx.ErrDatabaseNoRows) && !errors.Is(err, errorx.ErrNotFound) {
		return "", fmt.Errorf("failed to find builtin inference api key, %w", err)
	}

	apiKey, err := c.userSvcClient.GetOrCreateFirstAvaiTokens(
		ctx,
		ownerNamespace,
		operatorUsername,
		string(types.AccessTokenAppAIGateway),
		"claw-eval",
	)
	if err != nil {
		return "", fmt.Errorf("failed to resolve claw-eval api key, %w", err)
	}
	if apiKey == "" {
		return "", fmt.Errorf("failed to resolve claw-eval api key")
	}
	return apiKey, nil
}

func (c *evaluationComponentImpl) checkEvaluationLogPermission(ctx context.Context, req types.EvaluationLogReq) (bool, *database.ArgoWorkflow, error) {
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
		return false, nil, fmt.Errorf("failed to find evaluation workflow job by id %d or task id %s error: %w",
			req.ID, req.TaskID, err)
	}
	if !isEvaluationWorkflowTaskType(wf.TaskType) {
		return false, nil, errorx.ErrForbiddenMsg("workflow is not an evaluation job")
	}

	_, err = checkOwnerOrOrgMemberPermission(ctx, c.userSvcClient, c.rebac, req.CurrentUser, wf.UserUUID)
	if err != nil {
		return false, nil, errorx.ErrForbidden
	}
	return true, wf, nil
}

// isEvaluationWorkflowTaskType covers all workflow kinds that expose evaluation-style logs.
func isEvaluationWorkflowTaskType(taskType types.TaskType) bool {
	switch taskType {
	case types.TaskTypeEvaluation, types.TaskTypeClawEval, types.TaskTypeComparison, types.TaskTypeLeaderBoard:
		return true
	default:
		return false
	}
}

func (c *evaluationComponentImpl) ReadJobLogsNonStream(ctx context.Context, req types.EvaluationLogReq) (string, error) {
	allow, wf, err := c.checkEvaluationLogPermission(ctx, req)
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
		return "", fmt.Errorf("failed to read evaluation job logs, error:%w", err)
	}
	return c.formatEvaluationLogs(lokiResp), nil
}

func (c *evaluationComponentImpl) ReadJobLogsInStream(ctx context.Context, req types.EvaluationLogReq) (*deploy.MultiLogReader, error) {
	allow, wf, err := c.checkEvaluationLogPermission(ctx, req)
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

func (c *evaluationComponentImpl) formatEvaluationLogs(lokiLog *loki.LokiQueryResponse) string {
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
