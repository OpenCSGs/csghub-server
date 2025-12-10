package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const modelGitattributesContent = `*.7z filter=lfs diff=lfs merge=lfs -text
*.arrow filter=lfs diff=lfs merge=lfs -text
*.bin filter=lfs diff=lfs merge=lfs -text
*.bz2 filter=lfs diff=lfs merge=lfs -text
*.ckpt filter=lfs diff=lfs merge=lfs -text
*.ftz filter=lfs diff=lfs merge=lfs -text
*.gz filter=lfs diff=lfs merge=lfs -text
*.h5 filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
*.lfs.* filter=lfs diff=lfs merge=lfs -text
*.mlmodel filter=lfs diff=lfs merge=lfs -text
*.model filter=lfs diff=lfs merge=lfs -text
*.msgpack filter=lfs diff=lfs merge=lfs -text
*.npy filter=lfs diff=lfs merge=lfs -text
*.npz filter=lfs diff=lfs merge=lfs -text
*.onnx filter=lfs diff=lfs merge=lfs -text
*.ot filter=lfs diff=lfs merge=lfs -text
*.parquet filter=lfs diff=lfs merge=lfs -text
*.pb filter=lfs diff=lfs merge=lfs -text
*.pickle filter=lfs diff=lfs merge=lfs -text
*.pkl filter=lfs diff=lfs merge=lfs -text
*.pt filter=lfs diff=lfs merge=lfs -text
*.pth filter=lfs diff=lfs merge=lfs -text
*.rar filter=lfs diff=lfs merge=lfs -text
*.safetensors filter=lfs diff=lfs merge=lfs -text
saved_model/**/* filter=lfs diff=lfs merge=lfs -text
*.tar.* filter=lfs diff=lfs merge=lfs -text
*.tar filter=lfs diff=lfs merge=lfs -text
*.tflite filter=lfs diff=lfs merge=lfs -text
*.tgz filter=lfs diff=lfs merge=lfs -text
*.wasm filter=lfs diff=lfs merge=lfs -text
*.xz filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
*.zst filter=lfs diff=lfs merge=lfs -text
*tfevents* filter=lfs diff=lfs merge=lfs -text
*.gguf filter=lfs diff=lfs merge=lfs -text
*.ggml filter=lfs diff=lfs merge=lfs -text
*.pdparams filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
`

type ModelComponent interface {
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Model, int, error)
	Create(ctx context.Context, req *types.CreateModelReq) (*types.Model, error)
	Update(ctx context.Context, req *types.UpdateModelReq) (*types.Model, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.Model, error)
	GetServerless(ctx context.Context, namespace, name, currentUser string) (*types.DeployRepo, error)
	SDKModelInfo(ctx context.Context, namespace, name, ref, currentUser string, blobs bool) (*types.SDKModelInfo, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	SetRelationDatasets(ctx context.Context, req types.RelationDatasets) error
	AddRelationDataset(ctx context.Context, req types.RelationDataset) error
	DelRelationDataset(ctx context.Context, req types.RelationDataset) error
	// create model deploy as inference/serverless
	Deploy(ctx context.Context, deployReq types.DeployActReq, req types.ModelRunReq) (int64, error)
	Wakeup(ctx context.Context, namespace, name string, id int64) error
	ListModelsByRuntimeFrameworkID(ctx context.Context, currentUser string, per, page int, id int64, deployType int) ([]types.Model, int, error)
	ListAllByRuntimeFramework(ctx context.Context, currentUser string, deployType int) ([]database.RuntimeFramework, error)
	SetRuntimeFrameworkModes(ctx context.Context, deployType int, id int64, paths []string) ([]string, error)
	DeleteRuntimeFrameworkModes(ctx context.Context, deployType int, id int64, paths []string) ([]string, error)
	ListModelsOfRuntimeFrameworks(ctx context.Context, currentUser, search, sort string, per, page int, deployType int) ([]types.Model, int, error)
	OrgModels(ctx context.Context, req *types.OrgModelsReq) ([]types.Model, int, error)
	ListQuantizations(ctx context.Context, namespace, name string) ([]*types.File, error)
}

func NewModelComponent(config *config.Config) (ModelComponent, error) {
	c := &modelComponentImpl{config: config}
	var err error
	c.repoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	c.spaceComponent, _ = NewSpaceComponent(config)
	c.modelStore = database.NewModelStore()
	c.repoStore = database.NewRepoStore()
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.userStore = database.NewUserStore()
	c.userLikesStore = database.NewUserLikesStore()
	c.deployer = deploy.NewDeployer()
	c.tagStore = database.NewTagStore()
	c.runtimeArchComponent, err = NewRuntimeArchitectureComponent(config)
	if err != nil {
		return nil, err
	}
	c.accountingComponent, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.datasetStore = database.NewDatasetStore()
	c.runtimeArchitecturesStore = database.NewRuntimeArchitecturesStore()
	c.repoRuntimeFrameworkStore = database.NewRepositoriesRuntimeFramework()
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.deployTaskStore = database.NewDeployTaskStore()
	c.gitServer, err = git.NewGitServer(config)
	if err != nil {
		return nil, err
	}
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.recomStore = database.NewRecomStore()
	return c, nil
}

type modelComponentImpl struct {
	config                    *config.Config
	repoComponent             RepoComponent
	spaceComponent            SpaceComponent
	modelStore                database.ModelStore
	repoStore                 database.RepoStore
	spaceResourceStore        database.SpaceResourceStore
	userStore                 database.UserStore
	deployer                  deploy.Deployer
	accountingComponent       AccountingComponent
	tagStore                  database.TagStore
	runtimeArchComponent      RuntimeArchitectureComponent
	datasetStore              database.DatasetStore
	recomStore                database.RecomStore
	gitServer                 gitserver.GitServer
	userLikesStore            database.UserLikesStore
	runtimeArchitecturesStore database.RuntimeArchitecturesStore
	repoRuntimeFrameworkStore database.RepositoriesRuntimeFrameworkStore
	deployTaskStore           database.DeployTaskStore
	runtimeFrameworksStore    database.RuntimeFrameworksStore
	userSvcClient             rpc.UserSvcClient
}

func (c *modelComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Model, int, error) {
	var (
		err       error
		resModels []*types.Model
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.ModelRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	models, err := c.modelStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get models by repo ids,error:%w", err)
		return nil, 0, newError
	}

	// loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var model *database.Model
		for _, m := range models {
			if m.RepositoryID == repo.ID {
				model = &m
				break
			}
		}
		if model == nil {
			continue
		}
		var (
			tags             []types.RepoTag
			mirrorTaskStatus types.MirrorTaskStatus
		)
		for _, tag := range repo.Tags {
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
		if model.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = model.Repository.Mirror.CurrentTask.Status
		}
		resModels = append(resModels, &types.Model{
			ID:           model.ID,
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Likes:        repo.Likes,
			Downloads:    repo.DownloadCount,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
			CreatedAt:    model.CreatedAt,
			Tags:         tags,
			UpdatedAt:    repo.UpdatedAt,
			Source:       repo.Source,
			SyncStatus:   repo.SyncStatus,
			License:      repo.License,
			Repository:   common.BuildCloneInfo(c.config, model.Repository),
			MultiSource: types.MultiSource{
				HFPath:  model.Repository.HFPath,
				MSPath:  model.Repository.MSPath,
				CSGPath: model.Repository.CSGPath,
			},
			ReportURL:        model.ReportURL,
			MediumRiskCount:  model.MediumRiskCount,
			HighRiskCount:    model.HighRiskCount,
			MirrorTaskStatus: mirrorTaskStatus,
		})
	}
	if needOpWeight {
		c.addOpWeightToModel(ctx, repoIDs, resModels)
	}
	return resModels, total, nil
}

func (c *modelComponentImpl) Create(ctx context.Context, req *types.CreateModelReq) (*types.Model, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)
	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}
	req.Nickname = nickname
	req.RepoType = types.ModelRepo
	req.Readme = generateReadmeData(req.License)
	req.CommitFiles = []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: modelGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}
	_, dbRepo, commitFilesReq, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbModel := database.Model{
		Repository:      dbRepo,
		RepositoryID:    dbRepo.ID,
		BaseModel:       req.BaseModel,
		ReportURL:       req.ReportURL,
		MediumRiskCount: req.MediumRiskCount,
		HighRiskCount:   req.HighRiskCount,
	}
	repoPath := path.Join(req.Namespace, req.Name)

	model, err := c.modelStore.CreateAndUpdateRepoPath(ctx, dbModel, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database model, cause: %w", err)
	}

	_ = c.gitServer.CommitFiles(ctx, *commitFilesReq)

	for _, tag := range model.Repository.Tags {
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

	resModel := &types.Model{
		ID:           model.ID,
		Name:         model.Repository.Name,
		Nickname:     model.Repository.Nickname,
		Description:  model.Repository.Description,
		Likes:        model.Repository.Likes,
		Downloads:    model.Repository.DownloadCount,
		Path:         model.Repository.Path,
		RepositoryID: model.RepositoryID,
		Repository:   common.BuildCloneInfo(c.config, model.Repository),
		Private:      model.Repository.Private,
		User: &types.User{
			Username: user.Username,
			Nickname: user.NickName,
			Email:    user.Email,
		},
		Tags:            tags,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
		BaseModel:       model.BaseModel,
		License:         model.Repository.License,
		ReportURL:       model.ReportURL,
		MediumRiskCount: model.MediumRiskCount,
		HighRiskCount:   model.HighRiskCount,
		URL:             model.Repository.Path,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.ModelRepo,
			RepoPath:  model.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resModel, nil
}

func (c *modelComponentImpl) Update(ctx context.Context, req *types.UpdateModelReq) (*types.Model, error) {
	req.RepoType = types.ModelRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	model, err := c.modelStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	if req.BaseModel != nil {
		model.BaseModel = *req.BaseModel
	}
	model.ReportURL = req.ReportURL
	model.MediumRiskCount = req.MediumRiskCount
	model.HighRiskCount = req.HighRiskCount

	model, err = c.modelStore.Update(ctx, *model)
	if err != nil {
		return nil, fmt.Errorf("failed to update database model, error: %w", err)
	}
	resModel := &types.Model{
		ID:              model.ID,
		Name:            dbRepo.Name,
		Nickname:        dbRepo.Nickname,
		Description:     dbRepo.Description,
		Likes:           dbRepo.Likes,
		Downloads:       dbRepo.DownloadCount,
		Path:            dbRepo.Path,
		RepositoryID:    dbRepo.ID,
		Private:         dbRepo.Private,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
		BaseModel:       model.BaseModel,
		ReportURL:       model.ReportURL,
		MediumRiskCount: model.MediumRiskCount,
		HighRiskCount:   model.HighRiskCount,
	}

	return resModel, nil
}

func (c *modelComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	model, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.ModelRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of model, error: %w", err)
	}

	err = c.modelStore.Delete(ctx, *model)
	if err != nil {
		return fmt.Errorf("failed to delete database model, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.ModelRepo,
			RepoPath:  model.Repository.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *modelComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.Model, error) {
	var (
		tags             []types.RepoTag
		syncStatus       types.RepositorySyncStatus
		mirrorTaskStatus types.MirrorTaskStatus
	)
	model, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, model.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for model, error: %w", err)
	}

	for _, tag := range model.Repository.Tags {
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

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, model.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus, syncStatus = c.repoComponent.GetMirrorTaskStatusAndSyncStatus(model.Repository)

	resModel := &types.Model{
		ID:            model.ID,
		Name:          model.Repository.Name,
		Nickname:      model.Repository.Nickname,
		Description:   model.Repository.Description,
		Likes:         model.Repository.Likes,
		Downloads:     model.Repository.DownloadCount,
		Path:          model.Repository.Path,
		RepositoryID:  model.Repository.ID,
		DefaultBranch: model.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(c.config, model.Repository),
		Private:       model.Repository.Private,
		Tags:          tags,
		User: &types.User{
			Username: model.Repository.User.Username,
			Nickname: model.Repository.User.NickName,
			Email:    model.Repository.User.Email,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.Repository.UpdatedAt,
		// TODO:default to ModelWidgetTypeGeneration, need to config later
		WidgetType:          types.ModelWidgetTypeGeneration,
		UserLikes:           likeExists,
		Source:              model.Repository.Source,
		SyncStatus:          syncStatus,
		BaseModel:           model.BaseModel,
		License:             model.Repository.License,
		MirrorLastUpdatedAt: model.Repository.Mirror.LastUpdatedAt,
		CanWrite:            permission.CanWrite,
		CanManage:           permission.CanAdmin,
		Namespace:           ns,
		Metadata: types.Metadata{
			ModelParams:       model.Repository.Metadata.ModelParams,
			TensorType:        model.Repository.Metadata.TensorType,
			MiniGPUMemoryGB:   model.Repository.Metadata.MiniGPUMemoryGB,
			Architecture:      model.Repository.Metadata.Architecture,
			ModelType:         model.Repository.Metadata.ModelType,
			ClassName:         model.Repository.Metadata.ClassName,
			Quantizations:     model.Repository.Metadata.Quantizations,
			MiniGPUFinetuneGB: model.Repository.Metadata.MiniGPUFinetuneGB,
		},

		MultiSource: types.MultiSource{
			HFPath:  model.Repository.HFPath,
			MSPath:  model.Repository.MSPath,
			CSGPath: model.Repository.CSGPath,
		},
		ReportURL:        model.ReportURL,
		MediumRiskCount:  model.MediumRiskCount,
		HighRiskCount:    model.HighRiskCount,
		MirrorTaskStatus: mirrorTaskStatus,
		XnetEnabled:      model.Repository.XnetEnabled,
	}
	// admin user or owner can see the sensitive check status
	if permission.CanAdmin {
		resModel.SensitiveCheckStatus = model.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		c.addOpWeightToModel(ctx, []int64{model.RepositoryID}, []*types.Model{resModel})
	}
	// add recom_scores to model
	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToModel(ctx, resModel.RepositoryID, resModel, weightNames)
	}

	modelFormat := model.Repository.Format()
	archs := model.Repository.Archs()
	oriName := model.Repository.OriginName()
	enableInference, _ := c.runtimeArchitecturesStore.CheckEngineByArchModelNameAndType(ctx, archs, oriName, modelFormat, types.InferenceType)
	resModel.EnableInference = enableInference
	enableFinetune, _ := c.runtimeArchitecturesStore.CheckEngineByArchModelNameAndType(ctx, archs, oriName, modelFormat, types.FinetuneType)
	resModel.EnableFinetune = enableFinetune
	enableEvaluation, _ := c.runtimeArchitecturesStore.CheckEngineByArchModelNameAndType(ctx, archs, oriName, modelFormat, types.EvaluationType)
	resModel.EnableEvaluation = enableEvaluation
	updateDisabledReason(resModel, archs)
	return resModel, nil
}

func updateDisabledReason(resModel *types.Model, archs []string) {
	if len(archs) == 0 {
		resModel.DisableEvaluationReason = "metadata_not_recognized"
		resModel.DisableFinetuneReason = "metadata_not_recognized"
		resModel.DisableInferenceReason = "metadata_not_recognized"
		return
	}
	if !resModel.EnableInference {
		resModel.DisableInferenceReason = "model_not_support_inference"
	}
	if !resModel.EnableFinetune {
		resModel.DisableFinetuneReason = "model_not_support_finetune"
	}
	if !resModel.EnableEvaluation {
		resModel.DisableEvaluationReason = "model_not_support_evaluation"
	}
}

func (c *modelComponentImpl) GetServerless(ctx context.Context, namespace, name, currentUser string) (*types.DeployRepo, error) {
	model, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrUnauthorized
	}
	deploy, err := c.deployTaskStore.GetServerlessDeployByRepID(ctx, model.Repository.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get serverless deployment, error: %w", err)
	}
	if deploy == nil {
		return nil, nil
	}
	endpoint, _ := c.repoComponent.GenerateEndpoint(ctx, deploy)

	varMap, err := common.JsonStrToMap(deploy.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert variables to map, error: %w", err)
	}
	var entrypoint string
	val, exist := varMap[types.GGUFEntryPoint]
	if exist {
		entrypoint = val
	}

	resDeploy := types.DeployRepo{
		DeployID:         deploy.ID,
		DeployName:       deploy.DeployName,
		RepoID:           deploy.RepoID,
		SvcName:          deploy.SvcName,
		Status:           deployStatusCodeToString(deploy.Status),
		RuntimeFramework: deploy.RuntimeFramework,
		Hardware:         deploy.Hardware,
		Env:              deploy.Env,
		MinReplica:       deploy.MinReplica,
		MaxReplica:       deploy.MaxReplica,
		GitBranch:        deploy.GitBranch,
		ClusterID:        deploy.ClusterID,
		SecureLevel:      deploy.SecureLevel,
		CreatedAt:        deploy.CreatedAt,
		UpdatedAt:        deploy.UpdatedAt,
		ProxyEndpoint:    endpoint,
		Task:             string(deploy.Task),
		Entrypoint:       entrypoint,
	}
	return &resDeploy, nil
}

func (c *modelComponentImpl) SDKModelInfo(ctx context.Context, namespace, name, ref, currentUser string, blobs bool) (*types.SDKModelInfo, error) {
	model, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	var pipelineTag, libraryTag, sha string
	var tags []string
	for _, tag := range model.Repository.Tags {
		tags = append(tags, tag.Name)
		if tag.Category == "task" {
			pipelineTag = tag.Name
		}
		if tag.Category == "framework" {
			libraryTag = tag.Name
		}
	}

	files, err := GetFilePathObjects(ctx, namespace, name, "", types.ModelRepo, ref, c.gitServer.GetTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all %s files, error: %w", types.ModelRepo, err)
	}

	var sdkFiles []types.SDKFile
	for _, file := range files {
		if blobs {
			sdkFile := types.SDKFile{
				Filename: file.Path,
				BlobID:   file.SHA,
				Size:     file.Size,
			}
			if file.Lfs {
				sdkFile.LFS = &types.SDKLFS{
					SHA256:      file.LfsSHA256,
					PointerSize: file.LfsPointerSize,
					Size:        file.Size,
				}
			}
			sdkFiles = append(sdkFiles, sdkFile)
		} else {
			sdkFiles = append(sdkFiles, types.SDKFile{Filename: file.Path})
		}

	}

	if ref == "" {
		ref = model.Repository.DefaultBranch
	}
	getLastCommitReq := gitserver.GetRepoLastCommitReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  types.ModelRepo,
	}
	lastCommit, err := c.gitServer.GetRepoLastCommit(ctx, getLastCommitReq)
	if err != nil {
		slog.Error("failed to get last commit", slog.String("namespace", namespace), slog.String("name", name), slog.String("ref", ref), slog.Any("error", err))
		return nil, fmt.Errorf("failed to get last commit, error: %w", err)
	}

	relatedRepos, _ := c.repoComponent.RelatedRepos(ctx, model.RepositoryID, currentUser)
	relatedSpaces := relatedRepos[types.SpaceRepo]
	spaceNames := make([]string, len(relatedSpaces))
	for idx, s := range relatedSpaces {
		spaceNames[idx] = s.Name
	}

	if lastCommit != nil {
		sha = lastCommit.ID
	}

	hfCreatedAt := reSetHFTime(model.Repository.CreatedAt)
	hfUpdatedAt := reSetHFTime(model.Repository.UpdatedAt)

	resModel := &types.SDKModelInfo{
		ID:               model.Repository.Path,
		Author:           model.Repository.User.Username,
		Sha:              sha,
		CreatedAt:        hfCreatedAt,
		LastModified:     hfUpdatedAt,
		Private:          model.Repository.Private,
		Disabled:         false,
		Gated:            nil,
		Downloads:        int(model.Repository.DownloadCount),
		Likes:            int(model.Repository.Likes),
		LibraryName:      libraryTag,
		Tags:             tags,
		PipelineTag:      pipelineTag,
		MaskToken:        "",
		WidgetData:       nil,
		ModelIndex:       nil,
		Config:           nil,
		TransformersInfo: nil,
		CardData:         nil,
		Siblings:         sdkFiles,
		Spaces:           spaceNames,
		SafeTensors:      nil,
	}

	return resModel, nil
}

func (c *modelComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	model, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrUnauthorized
	}

	return c.getRelations(ctx, model.RepositoryID, currentUser)
}

func (c *modelComponentImpl) SetRelationDatasets(ctx context.Context, req types.RelationDatasets) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	_, err = c.repoStore.FindByPath(ctx, types.ModelRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       types.MainBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}

	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	metaMap["datasets"] = req.Datasets
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = types.MainBranch
	readmeReq.Message = "update dataset tags"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.ModelRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to set dataset tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *modelComponentImpl) AddRelationDataset(ctx context.Context, req types.RelationDataset) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	_, err = c.repoStore.FindByPath(ctx, types.ModelRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	datasets, ok := metaMap["datasets"]
	if !ok {
		datasets = []string{req.Dataset}
	} else {
		datasets = append(datasets.([]interface{}), req.Dataset)
	}
	metaMap["datasets"] = datasets
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = "main"
	readmeReq.Message = "add relation dataset"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.ModelRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to add dataset tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *modelComponentImpl) DelRelationDataset(ctx context.Context, req types.RelationDataset) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("user does not exist, %w", err)
	}

	_, err = c.repoStore.FindByPath(ctx, types.ModelRepo, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       "main",
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.ModelRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(c.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	datasets, ok := metaMap["datasets"]
	if !ok {
		return nil
	} else {
		var newDatasets []string
		for _, v := range datasets.([]interface{}) {
			if v.(string) != req.Dataset {
				newDatasets = append(newDatasets, v.(string))
			}
		}
		metaMap["datasets"] = newDatasets
	}
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = "main"
	readmeReq.Message = "delete relation dataset"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.ModelRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = req.CurrentUser
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = c.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to delete dataset tag to %s file, cause: %w", readmeReq.FilePath, err)
	}

	return nil
}

func (c *modelComponentImpl) getRelations(ctx context.Context, fromRepoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.repoComponent.RelatedRepos(ctx, fromRepoID, currentUser)
	if err != nil {
		return nil, err
	}
	rels := new(types.Relations)
	datasetRepos := res[types.DatasetRepo]
	for _, repo := range datasetRepos {
		rels.Datasets = append(rels.Datasets, &types.Dataset{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}
	codeRepos := res[types.CodeRepo]
	for _, repo := range codeRepos {
		rels.Codes = append(rels.Codes, &types.Code{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}
	spaceRepos := res[types.SpaceRepo]
	spacePaths := make([]string, 0)
	for _, repo := range spaceRepos {
		spacePaths = append(spacePaths, repo.Path)
	}
	spaces, err := c.spaceComponent.ListByPath(ctx, spacePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to get space info by paths, error: %w", err)
	}
	rels.Spaces = spaces

	promptRepos := res[types.PromptRepo]
	for _, repo := range promptRepos {
		rels.Prompts = append(rels.Prompts, &types.PromptRes{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}
	return rels, nil
}

func GetFilePathObjects(ctx context.Context, namespace, repoName, folder string, repoType types.RepositoryType, ref string, gsTree func(ctx context.Context, req types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)) ([]*types.File, error) {
	allFiles, err := getAllFiles(ctx, namespace, repoName, folder, repoType, ref, gsTree)
	if err != nil {
		return nil, err
	}
	return allFiles, nil
}

func GetFilePaths(ctx context.Context, namespace, repoName, folder string, repoType types.RepositoryType, ref string, gsTree func(ctx context.Context, req types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)) ([]string, error) {
	var filePaths []string
	allFiles, err := getAllFiles(ctx, namespace, repoName, folder, repoType, ref, gsTree)
	if err != nil {
		return nil, err
	}
	for _, f := range allFiles {
		filePaths = append(filePaths, f.Path)
	}

	return filePaths, nil
}

// create model deploy as inference/serverless
func (c *modelComponentImpl) Deploy(ctx context.Context, deployReq types.DeployActReq, req types.ModelRunReq) (int64, error) {
	m, err := c.modelStore.FindByPath(ctx, deployReq.Namespace, deployReq.Name)
	if err != nil {
		return -1, fmt.Errorf("cannot find model, %w", err)
	}
	if req.Revision == "" {
		req.Revision = m.Repository.DefaultBranch
	}
	task := GetBuiltInTaskFromTags(m.Repository.Tags)
	if deployReq.DeployType == types.ServerlessType {
		// only one service deploy was allowed
		d, err := c.deployTaskStore.GetServerlessDeployByRepID(ctx, m.Repository.ID)
		if err != nil {
			return -1, fmt.Errorf("fail to get deploy, %w", err)
		}
		if d != nil {
			return d.ID, nil
		}
	}
	// found user id
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return -1, fmt.Errorf("cannot find user for deploy model, %w", err)
	}

	frame, err := c.runtimeFrameworksStore.FindEnabledByID(ctx, req.RuntimeFrameworkID)
	if err != nil {
		return -1, fmt.Errorf("cannot find available runtime framework, %w", err)
	}

	varStr, err := c.buildVariables(req, frame)
	if err != nil {
		return -1, fmt.Errorf("fail to generate variables, %w", err)
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResTypeKey] = string(types.ModelRepo)
	annotations[types.ResNameKey] = fmt.Sprintf("%s/%s", deployReq.Namespace, deployReq.Name)
	annotations[types.ResDeployUser] = user.Username
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		return -1, errorx.InternalServerError(err, nil)
	}

	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return -1, fmt.Errorf("cannot find resource, %w", err)
	}

	req.ClusterID = resource.ClusterID

	// resource available only if err is nil, err message should contain
	// the reason why resource is unavailable
	err = c.repoComponent.CheckAccountAndResource(ctx, deployReq.CurrentUser, req.ClusterID, req.OrderDetailID, resource)
	if err != nil {
		return -1, err
	}

	// choose image
	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return -1, errorx.InternalServerError(err, nil)
	}

	// only vllm and sglang support multi-host inference
	if hardware.Replicas > 1 {
		if frame.FrameName != "vllm" && frame.FrameName != "sglang" {
			return -1, errorx.ErrMultiHostInferenceNotSupported
		}
		if req.MinReplica < 1 {
			return -1, errorx.ErrMultiHostInferenceReplicaCount
		}
	}

	// create deploy for model
	dp := types.DeployRepo{
		DeployName:       req.DeployName,
		SpaceID:          0,
		Path:             m.Repository.Path,
		GitPath:          m.Repository.GitPath,
		GitBranch:        req.Revision,
		Env:              req.Env,
		Hardware:         resource.Resources,
		UserID:           user.ID,
		ModelID:          m.ID,
		RepoID:           m.Repository.ID,
		RuntimeFramework: frame.FrameName,
		ContainerPort:    frame.ContainerPort, // default container port
		ImageID:          frame.FrameImage,    // do not need build pod image for model
		MinReplica:       req.MinReplica,
		MaxReplica:       req.MaxReplica,
		Annotation:       string(annoStr),
		ClusterID:        req.ClusterID,
		SecureLevel:      req.SecureLevel,
		Type:             deployReq.DeployType,
		UserUUID:         user.UUID,
		SKU:              strconv.FormatInt(resource.ID, 10),
		Task:             task,
		Variables:        varStr,
		EngineArgs:       req.EngineArgs,
	}
	dp = modelRunUpdateDeployRepo(dp, req)
	return c.deployer.Deploy(ctx, dp)
}

func (c *modelComponentImpl) Wakeup(ctx context.Context, namespace, name string, deployId int64) error {
	_, err := c.modelStore.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't wakeup inference", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}
	// get Deploy for inference
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployId)
	if err != nil {
		return fmt.Errorf("can't get inference delopyment,%w", err)
	}
	return c.deployer.Wakeup(ctx, types.DeployRepo{
		DeployID:  deployId,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
	})
}

func (c *modelComponentImpl) buildVariables(req types.ModelRunReq, frame *database.RuntimeFramework) (string, error) {
	engineName := strings.ToLower(frame.FrameName)
	if engineName == string(types.LlamaCpp) || engineName == string(types.Ktransformers) {
		//check entrypoint for llama.cpp
		if len(req.Entrypoint) < 1 {
			err := fmt.Errorf("entrypoint is required for llama.cpp or ktransformers")
			return "", errorx.ReqBodyFormat(err, errorx.Ctx().Set("body", "entrypoint"))
		}
		varMap := make(map[string]string)
		varMap[types.GGUFEntryPoint] = req.Entrypoint
		varBytes, err := json.Marshal(varMap)
		if err != nil {
			return "", errorx.InternalServerError(err, nil)
		}
		return string(varBytes), nil
	}
	return "", nil
}

func (c *modelComponentImpl) ListModelsByRuntimeFrameworkID(ctx context.Context, currentUser string, per, page int, id int64, deployType int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)
	if currentUser != "" {
		user, err = c.userStore.FindByUsername(ctx, currentUser)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get current user,error:%w", err)
		}
	}

	repos, total, err := c.repoStore.ListRepoByDeployType(ctx, types.ModelRepo, user.ID, "", "", types.InferenceType, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get public model repos,error:%w", err)
	}

	for _, repo := range repos {
		resModels = append(resModels, types.Model{
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
		})
	}
	return resModels, total, nil
}

func (c *modelComponentImpl) ListAllByRuntimeFramework(ctx context.Context, currentUser string, deployType int) ([]database.RuntimeFramework, error) {
	var (
		runtimes []database.RuntimeFramework
		err      error
	)
	if deployType == 0 {
		runtimes, err = c.runtimeFrameworksStore.ListAll(ctx)
	} else {
		runtimes, err = c.runtimeFrameworksStore.List(ctx, deployType)
		for i := range runtimes {
			frameVersion := strings.Split(runtimes[i].FrameImage, ":")[1]
			runtimes[i].FrameVersion = frameVersion
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	return runtimes, nil
}

func (c *modelComponentImpl) SetRuntimeFrameworkModes(ctx context.Context, deployType int, id int64, paths []string) ([]string, error) {
	models, err := c.modelStore.ListByPath(ctx, paths)
	if err != nil {
		return nil, err
	}

	//add resource tag, like ascend
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{types.ModelTagScope},
		Categories: []string{"runtime_framework", "resource"},
	}
	runtime_framework_tags, _ := c.tagStore.AllTags(ctx, filter)

	var failedModels []string
	for _, model := range models {
		relations, err := c.repoRuntimeFrameworkStore.GetByIDsAndType(ctx, id, model.Repository.ID, deployType)
		if err != nil {
			return nil, err
		}
		if len(relations) < 1 {
			err = c.repoRuntimeFrameworkStore.Add(ctx, id, model.Repository.ID, deployType)
			if err != nil {
				failedModels = append(failedModels, model.Repository.Path)
			}
			_, modelName := model.Repository.NamespaceAndName()
			err = c.runtimeArchComponent.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, model.Repository.ID, id)
			if err != nil {
				return nil, err
			}
			err = c.runtimeArchComponent.AddResourceTag(ctx, runtime_framework_tags, modelName, model.Repository.ID)
			if err != nil {
				return nil, err
			}
		}
	}

	return failedModels, nil
}

func (c *modelComponentImpl) DeleteRuntimeFrameworkModes(ctx context.Context, deployType int, id int64, paths []string) ([]string, error) {
	models, err := c.modelStore.ListByPath(ctx, paths)
	if err != nil {
		return nil, err
	}

	var failedModels []string
	for _, model := range models {
		err = c.repoRuntimeFrameworkStore.Delete(ctx, id, model.Repository.ID, deployType)
		if err != nil {
			failedModels = append(failedModels, model.Repository.Path)
		}
	}

	return failedModels, nil
}

func (c *modelComponentImpl) ListModelsOfRuntimeFrameworks(ctx context.Context, currentUser, search, sort string, per, page int, deployType int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)

	user, err = c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get current user %s, error:%w", currentUser, err)
	}

	repos, total, err := c.repoStore.ListRepoByDeployType(ctx, types.ModelRepo, user.ID, search, sort, deployType, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos, error:%w", err)
		return nil, 0, newError
	}
	// define EnableInference
	enableInference := deployType == types.InferenceType
	enableFinetune := deployType == types.FinetuneType
	enableEvaluation := deployType == types.EvaluationType

	for _, repo := range repos {
		resModels = append(resModels, types.Model{
			Name:             repo.Name,
			Nickname:         repo.Nickname,
			Description:      repo.Description,
			Path:             repo.Path,
			RepositoryID:     repo.ID,
			Private:          repo.Private,
			EnableInference:  enableInference,
			EnableFinetune:   enableFinetune,
			EnableEvaluation: enableEvaluation,
		})
	}
	return resModels, total, nil
}

func (c *modelComponentImpl) OrgModels(ctx context.Context, req *types.OrgModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.Error("faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	ms, total, err := c.modelStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range ms {
		resModels = append(resModels, types.Model{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.Repository.UpdatedAt,
		})
	}

	return resModels, total, nil
}

// only need it for huggingface_hub <= 0.26.2
func reSetHFTime(sysTime time.Time) time.Time {
	nanosecond := sysTime.Nanosecond()
	if nanosecond < 1 {
		nanosecond = 1
	}
	hfTime := time.Date(
		sysTime.Year(),
		sysTime.Month(),
		sysTime.Day(),
		sysTime.Hour(),
		sysTime.Minute(),
		sysTime.Second(),
		nanosecond,
		time.UTC)
	return hfTime
}

func (c *modelComponentImpl) ListQuantizations(ctx context.Context, namespace, name string) ([]*types.File, error) {
	repo, err := c.repoStore.FindByPath(ctx, types.ModelRepo, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if !isGGUFModel(repo) {
		//no need to get quantization files for non gguf models
		return nil, nil
	}
	files, err := getAllFiles(ctx, namespace, name, "", types.ModelRepo, repo.DefaultBranch, c.gitServer.GetTree)
	if err != nil {
		return nil, fmt.Errorf("get RepoFileTree for relation, %w", err)
	}
	var ggufFiles []*types.File
	for _, file := range files {
		if strings.HasSuffix(file.Name, ".gguf") {
			if strings.Contains(file.Name, "00001-of-") || !strings.Contains(file.Name, "-of-") {
				ggufFiles = append(ggufFiles, file)
			}
		}
	}
	return ggufFiles, nil
}

// get built-int task from tags
func GetBuiltInTaskFromTags(tags []database.Tag) string {
	for _, tag := range tags {
		if tag.Name == string(types.TextGeneration) {
			return tag.Name
		}
		if tag.Name == string(types.Text2Image) {
			return tag.Name
		}
		if tag.Name == string(types.FeatureExtraction) || tag.Name == string(types.SentenceSimilarity) {
			return string(types.FeatureExtraction)
		}
		if tag.Name == string(types.ImageText2Text) {
			return tag.Name
		}
	}
	return string(types.TextGeneration)
}

func (c *modelComponentImpl) addWeightsToModel(ctx context.Context, repoID int64, resModel *types.Model, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resModel.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resModel.Scores = append(resModel.Scores, score)
			}
		}
	}
}
