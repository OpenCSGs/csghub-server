package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"opencsg.com/csghub-server/builder/deploy"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/inference"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

func NewModelComponent(config *config.Config) (*ModelComponent, error) {
	c := &ModelComponent{}
	var err error
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	c.spaceComonent, _ = NewSpaceComponent(config)
	c.ms = database.NewModelStore()
	c.rs = database.NewRepoStore()
	c.SS = database.NewSpaceResourceStore()
	c.infer = inference.NewInferClient(config.Inference.ServerAddr)
	c.us = database.NewUserStore()
	c.deployer = deploy.NewDeployer()
	c.ac, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

type ModelComponent struct {
	*RepoComponent
	spaceComonent *SpaceComponent
	ms            *database.ModelStore
	rs            *database.RepoStore
	SS            *database.SpaceResourceStore
	infer         inference.Client
	us            *database.UserStore
	deployer      deploy.Deployer
	ac            *AccountingComponent
}

func (c *ModelComponent) Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)
	if filter.Username != "" {
		user, err = c.user.FindByUsername(ctx, filter.Username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			return nil, 0, newError
		}
	}
	repos, total, err := c.rs.PublicToUser(ctx, types.ModelRepo, user.ID, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	models, err := c.ms.ByRepoIDs(ctx, repoIDs)
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
		var tags []types.RepoTag
		for _, tag := range repo.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.ShowName,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		resModels = append(resModels, types.Model{
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
			Repository: types.Repository{
				HTTPCloneURL: repo.HTTPCloneURL,
				SSHCloneURL:  repo.SSHCloneURL,
			},
		})
	}
	return resModels, total, nil
}

func (c *ModelComponent) Create(ctx context.Context, req *types.CreateModelReq) (*types.Model, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)
	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}
	req.Nickname = nickname
	req.RepoType = types.ModelRepo
	req.Readme = generateReadmeData(req.License)
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbModel := database.Model{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	model, err := c.ms.Create(ctx, dbModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create database model, cause: %w", err)
	}

	// Create README.md file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  readmeFileName,
	}, types.ModelRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   modelGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  gitattributesFileName,
	}, types.ModelRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	for _, tag := range model.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
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
		Repository: types.Repository{
			HTTPCloneURL: model.Repository.HTTPCloneURL,
			SSHCloneURL:  model.Repository.SSHCloneURL,
		},
		Private: model.Repository.Private,
		User: &types.User{
			Username: user.Username,
			Nickname: user.NickName,
			Email:    user.Email,
		},
		Tags:      tags,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	return resModel, nil
}

func buildCreateFileReq(p *types.CreateFileParams, repoType types.RepositoryType) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  p.Username,
		Email:     p.Email,
		Message:   p.Message,
		Branch:    p.Branch,
		Content:   base64.StdEncoding.EncodeToString([]byte(p.Content)),
		NewBranch: p.Branch,
		NameSpace: p.Namespace,
		Name:      p.Name,
		FilePath:  p.FilePath,
		RepoType:  repoType,
	}
}

func (c *ModelComponent) Update(ctx context.Context, req *types.UpdateModelReq) (*types.Model, error) {
	req.RepoType = types.ModelRepo
	dbRepo, err := c.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	model, err := c.ms.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	model, err = c.ms.Update(ctx, *model)
	if err != nil {
		return nil, fmt.Errorf("failed to update database model, error: %w", err)
	}
	resModel := &types.Model{
		ID:           model.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}

	return resModel, nil
}

func (c *ModelComponent) Delete(ctx context.Context, namespace, name, currentUser string) error {
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.ModelRepo,
	}
	_, err = c.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of model, error: %w", err)
	}

	err = c.ms.Delete(ctx, *model)
	if err != nil {
		return fmt.Errorf("failed to delete database model, error: %w", err)
	}
	return nil
}

func (c *ModelComponent) Show(ctx context.Context, namespace, name, currentUser string) (*types.Model, error) {
	var tags []types.RepoTag
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	permission, err := c.getUserRepoPermission(ctx, currentUser, model.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	ns, err := c.getNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for model, error: %w", err)
	}

	for _, tag := range model.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.uls.IsExist(ctx, currentUser, model.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

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
		Repository: types.Repository{
			HTTPCloneURL: model.Repository.HTTPCloneURL,
			SSHCloneURL:  model.Repository.SSHCloneURL,
		},
		Private: model.Repository.Private,
		Tags:    tags,
		User: &types.User{
			Username: model.Repository.User.Username,
			Nickname: model.Repository.User.NickName,
			Email:    model.Repository.User.Email,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.Repository.UpdatedAt,
		// TODO:default to ModelWidgetTypeGeneration, need to config later
		WidgetType: types.ModelWidgetTypeGeneration,
		UserLikes:  likeExists,
		Source:     model.Repository.Source,
		SyncStatus: model.Repository.SyncStatus,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
	}
	inferences, _ := c.rrtfms.GetByRepoIDsAndType(ctx, model.Repository.ID, types.InferenceType)
	if len(inferences) > 0 {
		resModel.EnableInference = true
	}
	finetunes, _ := c.rrtfms.GetByRepoIDsAndType(ctx, model.Repository.ID, types.FinetuneType)
	if len(finetunes) > 0 {
		resModel.EnableFinetune = true
	}
	return resModel, nil
}

func (c *ModelComponent) GetServerless(ctx context.Context, namespace, name, currentUser string) (*types.DeployRepo, error) {
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	allow, _ := c.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}
	deploy, err := c.deploy.GetServerlessDeployByRepID(ctx, model.Repository.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get serverless deployment, error: %w", err)
	}
	if deploy == nil {
		return nil, nil
	}
	var endpoint string
	if len(deploy.SvcName) > 0 && deploy.Status == deployStatus.Running {
		cls, err := c.cluster.ByClusterID(ctx, deploy.ClusterID)
		zone := ""
		provider := ""
		if err != nil {
			return nil, fmt.Errorf("get cluster with error: %w", err)
		} else {
			zone = cls.Zone
			provider = cls.Provider
		}
		regionDomain := ""
		if len(zone) > 0 && len(provider) > 0 {
			regionDomain = fmt.Sprintf(".%s.%s", zone, provider)
		}
		endpoint = fmt.Sprintf("%s%s.%s", deploy.SvcName, regionDomain, c.publicRootDomain)
	}

	resDeploy := types.DeployRepo{
		DeployID:         deploy.ID,
		DeployName:       deploy.DeployName,
		RepoID:           deploy.RepoID,
		SvcName:          deploy.SvcName,
		Status:           deployStatusCodeToString(deploy.Status),
		Hardware:         deploy.Hardware,
		Env:              deploy.Env,
		RuntimeFramework: deploy.RuntimeFramework,
		MinReplica:       deploy.MinReplica,
		MaxReplica:       deploy.MaxReplica,
		GitBranch:        deploy.GitBranch,
		ClusterID:        deploy.ClusterID,
		SecureLevel:      deploy.SecureLevel,
		CreatedAt:        deploy.CreatedAt,
		UpdatedAt:        deploy.UpdatedAt,
		ProxyEndpoint:    endpoint,
	}
	return &resDeploy, nil
}

func (c *ModelComponent) SDKModelInfo(ctx context.Context, namespace, name, ref, currentUser string) (*types.SDKModelInfo, error) {
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	var pipelineTag, libraryTag string
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

	filePaths, err := getFilePaths(namespace, name, "", types.ModelRepo, c.git.GetRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all %s files, error: %w", types.ModelRepo, err)
	}

	var sdkFiles []types.SDKFile
	for _, filePath := range filePaths {
		sdkFiles = append(sdkFiles, types.SDKFile{Filename: filePath})
	}
	lastCommit, _ := c.LastCommit(ctx, &types.GetCommitsReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  types.ModelRepo,
	})

	relatedRepos, _ := c.relatedRepos(ctx, model.RepositoryID, currentUser)
	relatedSpaces := relatedRepos[types.SpaceRepo]
	spaceNames := make([]string, len(relatedSpaces))
	for idx, s := range relatedSpaces {
		spaceNames[idx] = s.Name
	}

	resModel := &types.SDKModelInfo{
		ID:               model.Repository.Path,
		Author:           model.Repository.User.Username,
		Sha:              lastCommit.ID,
		CreatedAt:        model.Repository.CreatedAt,
		LastModified:     model.Repository.UpdatedAt,
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

func (c *ModelComponent) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	return c.getRelations(ctx, model.RepositoryID, currentUser)
}

func (c *ModelComponent) getRelations(ctx context.Context, fromRepoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.relatedRepos(ctx, fromRepoID, currentUser)
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
	spaces, err := c.spaceComonent.ListByPath(ctx, spacePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to get space info by paths, error: %w", err)
	}
	rels.Spaces = spaces

	return rels, nil
}

func getFilePaths(namespace, repoName, folder string, repoType types.RepositoryType, gsTree func(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error)) ([]string, error) {
	var filePaths []string
	allFiles, err := getAllFiles(namespace, repoName, folder, repoType, gsTree)
	if err != nil {
		return nil, err
	}
	for _, f := range allFiles {
		filePaths = append(filePaths, f.Path)
	}

	return filePaths, nil
}

func (c *ModelComponent) Predict(ctx context.Context, req *types.ModelPredictReq) (*types.ModelPredictResp, error) {
	mid := inference.ModelID{
		Owner: req.Namespace,
		Name:  req.Name,
	}
	inferReq := &inference.PredictRequest{
		Prompt: req.Input,
	}
	inferResp, err := c.infer.Predict(mid, inferReq)
	if err != nil {
		slog.Error("failed to predict", slog.Any("req", *inferReq), slog.Any("model", mid), slog.String("error", err.Error()))
		return nil, err
	}
	resp := &types.ModelPredictResp{
		Content: inferResp.GeneratedText,
	}
	return resp, nil
}

// create model deploy as inference/serverless
func (c *ModelComponent) Deploy(ctx context.Context, deployReq types.DeployActReq, req types.ModelRunReq) (int64, error) {
	m, err := c.ms.FindByPath(ctx, deployReq.Namespace, deployReq.Name)
	if err != nil {
		return -1, fmt.Errorf("cannot find model, %w", err)
	}
	if deployReq.DeployType == types.ServerlessType {
		// only one service deploy was allowed
		d, err := c.deploy.GetServerlessDeployByRepID(ctx, m.Repository.ID)
		if err != nil {
			return -1, fmt.Errorf("fail to get deploy, %w", err)
		}
		if d != nil {
			return d.ID, nil
		}
	}
	// found user id
	user, err := c.us.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return -1, fmt.Errorf("cannot find user for deploy model, %w", err)
	}

	if deployReq.DeployType == types.ServerlessType {
		// Check if the user is an admin
		isAdmin := c.isAdminRole(user)
		if !isAdmin {
			return -1, fmt.Errorf("need admin permission for Serverless deploy")
		}
	}

	frame, err := c.rtfm.FindEnabledByID(ctx, req.RuntimeFrameworkID)
	if err != nil {
		return -1, fmt.Errorf("cannot find available runtime framework, %w", err)
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResTypeKey] = string(types.ModelRepo)
	annotations[types.ResNameKey] = fmt.Sprintf("%s/%s", deployReq.Namespace, deployReq.Name)
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		return -1, fmt.Errorf("fail to create annotations for deploy model, %w", err)
	}

	resource, err := c.SS.FindByID(ctx, req.ResourceID)
	if err != nil {
		return -1, fmt.Errorf("cannot find resource, %w", err)
	}

	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return -1, fmt.Errorf("invalid hardware setting, %w", err)
	}

	_, err = c.deployer.CheckResourceAvailable(ctx, req.ClusterID, &hardware)
	if err != nil {
		return -1, fmt.Errorf("fail to check resource, %w", err)
	}

	// choose image
	containerImg := frame.FrameCpuImage
	if hardware.Gpu.Num != "" {
		// use gpu image
		containerImg = frame.FrameImage
	}

	// create deploy for model
	return c.deployer.Deploy(ctx, types.DeployRepo{
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
		ImageID:          containerImg,        // do not need build pod image for model
		MinReplica:       req.MinReplica,
		MaxReplica:       req.MaxReplica,
		Annotation:       string(annoStr),
		CostPerHour:      resource.CostPerHour,
		ClusterID:        req.ClusterID,
		SecureLevel:      req.SecureLevel,
		Type:             deployReq.DeployType,
		UserUUID:         user.UUID,
		SKU:              strconv.FormatInt(resource.ID, 10),
	})
}

func (c *ModelComponent) ListModelsByRuntimeFrameworkID(ctx context.Context, currentUser string, per, page int, id int64, deployType int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)
	if currentUser != "" {
		user, err = c.user.FindByUsername(ctx, currentUser)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get current user,error:%w", err)
		}
	}

	runtimeRepos, err := c.rrtfms.ListByRuntimeFrameworkID(ctx, id, deployType)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get repo by runtime,error:%w", err)
	}

	if runtimeRepos == nil {
		return nil, 0, nil
	}

	var repoIDs []int64
	for _, repo := range runtimeRepos {
		repoIDs = append(repoIDs, repo.RepoID)
	}

	repos, total, err := c.rs.ListRepoPublicToUserByRepoIDs(ctx, types.ModelRepo, user.ID, "", "", per, page, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos,error:%w", err)
		return nil, 0, newError
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

func (c *ModelComponent) ListAllByRuntimeFramework(ctx context.Context, currentUser string) ([]database.RuntimeFramework, error) {
	runtimes, err := c.runFrame.ListAll(ctx)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos,error:%w", err)
		return nil, newError
	}

	return runtimes, nil
}

func (c *ModelComponent) SetRuntimeFrameworkModes(ctx context.Context, currentUser string, deployType int, id int64, paths []string) ([]string, error) {
	runtimeRepos, err := c.rtfm.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if runtimeRepos == nil {
		return nil, fmt.Errorf("failed to get runtime framework")
	}

	models, err := c.ms.ListByPath(ctx, paths)
	if err != nil {
		return nil, err
	}

	var failedModels []string
	for _, model := range models {
		relations, err := c.rrtfms.GetByIDsAndType(ctx, id, model.Repository.ID, deployType)
		if err != nil {
			return nil, err
		}
		if relations == nil || len(relations) < 1 {
			err = c.rrtfms.Add(ctx, id, model.Repository.ID, deployType)
			if err != nil {
				failedModels = append(failedModels, model.Repository.Path)
			}
		}
	}

	return failedModels, nil
}

func (c *ModelComponent) DeleteRuntimeFrameworkModes(ctx context.Context, currentUser string, deployType int, id int64, paths []string) ([]string, error) {
	models, err := c.ms.ListByPath(ctx, paths)
	if err != nil {
		return nil, err
	}

	var failedModels []string
	for _, model := range models {
		err = c.rrtfms.Delete(ctx, id, model.Repository.ID, deployType)
		if err != nil {
			failedModels = append(failedModels, model.Repository.Path)
		}
	}

	return failedModels, nil
}

func (c *ModelComponent) ListModelsOfRuntimeFrameworks(ctx context.Context, currentUser, search, sort string, per, page int, deployType int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)

	user, err = c.user.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get current user %s, error:%w", currentUser, err)
	}

	runtimeRepos, err := c.rrtfms.ListRepoIDsByType(ctx, deployType)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get repo by deploy type, error:%w", err)
	}

	if runtimeRepos == nil || len(runtimeRepos) < 1 {
		return nil, 0, nil
	}

	var repoIDs []int64
	for _, repo := range runtimeRepos {
		repoIDs = append(repoIDs, repo.RepoID)
	}

	repos, total, err := c.rs.ListRepoPublicToUserByRepoIDs(ctx, types.ModelRepo, user.ID, search, sort, per, page, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get public model repos, error:%w", err)
		return nil, 0, newError
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
