package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
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
	c.infer = inference.NewInferClient(config.Inference.ServerAddr)
	c.us = database.NewUserStore()
	c.deployer = deploy.NewDeployer()
	c.rtfm = database.NewRuntimeFrameworksStore()
	return c, nil
}

type ModelComponent struct {
	*RepoComponent
	spaceComonent *SpaceComponent
	ms            *database.ModelStore
	rs            *database.RepoStore
	infer         inference.Client
	us            *database.UserStore
	deployer      deploy.Deployer
	rtfm          *database.RuntimeFrameworksStore
}

func (c *ModelComponent) Index(ctx context.Context, username, search, sort string, ragReqs []database.TagReq, per, page int) ([]types.Model, int, error) {
	var (
		user      database.User
		err       error
		resModels []types.Model
	)
	if username != "" {
		user, err = c.user.FindByUsername(ctx, username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			return nil, 0, newError
		}
	}
	repos, total, err := c.rs.PublicToUser(ctx, types.ModelRepo, user.ID, search, sort, ragReqs, per, page)
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

	//loop through repos to keep the repos in sort order
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
		})
	}
	return resModels, total, nil
}

func (c *ModelComponent) Create(ctx context.Context, req *types.CreateModelReq) (*types.Model, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
		if err != nil {
			return nil, err
		}
		if !canWrite {
			return nil, errors.New("users do not have permission to create models in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, errors.New("users do not have permission to create models in this namespace")
		}
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
			Nickname: user.Name,
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
	dbRepo, err := c.UpdateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
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

	allow, _ := c.AllowReadAccessRepo(ctx, model.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	// get model running status
	var mid inference.ModelID
	mid.Owner = namespace
	mid.Name = name
	mi, err := c.infer.GetModelInfo(mid)
	if err != nil {
		slog.Error("failed to get model info", slog.Any("id", mid), slog.Any("error", err))
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
			Nickname: model.Repository.User.Name,
			Email:    model.Repository.User.Email,
		},
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.Repository.UpdatedAt,
		// TODO:default to ModelWidgetTypeGeneration, need to config later
		WidgetType: types.ModelWidgetTypeGeneration,
		Status:     mi.Status,
		UserLikes:  likeExists,
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
	datasetRepos := res["dataset"]
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
	codeRepos := res["code"]
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
	spaceRepos := res["space"]
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

	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      repoName,
		Ref:       "",
		Path:      folder,
		RepoType:  repoType,
	}
	gitFiles, err := gsTree(context.Background(), getRepoFileTree)
	if err != nil {
		return filePaths, fmt.Errorf("failed to get repo file tree,%w", err)
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			subFileNames, err := getFilePaths(namespace, repoName, file.Path, repoType, gsTree)
			if err != nil {
				return filePaths, err
			}
			filePaths = append(filePaths, subFileNames...)
		} else {
			filePaths = append(filePaths, file.Path)
		}
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

// create model deploy as inference
func (c *ModelComponent) Deploy(ctx context.Context, namespace, name, currentUser string, req types.ModelRunReq) (int64, error) {
	m, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't find model", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return -1, err
	}
	// found user id
	user, err := c.us.FindByUsername(ctx, currentUser)
	if err != nil {
		slog.Error("can't find user for deploy model", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	frame, err := c.rtfm.FindByID(ctx, req.RuntimeFrameworkID)
	if err != nil {
		slog.Error("can't find available runtime framework", slog.Any("error", err), slog.Any("frameworkID", req.RuntimeFrameworkID))
		return -1, err
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResTypeKey] = string(types.ModelRepo)
	annotations[types.ResNameKey] = fmt.Sprintf("%s/%s", namespace, name)
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		slog.Error("fail to create annotations for deploy model", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	// create deploy for model
	return c.deployer.Deploy(ctx, types.DeployRepo{
		DeployName:       req.DeployName,
		SpaceID:          0,
		GitPath:          m.Repository.GitPath,
		GitBranch:        req.Revision,
		Env:              req.Env,
		Hardware:         req.Hardware,
		UserID:           user.ID,
		ModelID:          m.ID,
		RepoID:           m.Repository.ID,
		RuntimeFramework: frame.FrameName,
		ImageID:          frame.FrameImage, // do not need build pod image for model
		MinReplica:       req.MinReplica,
		MaxReplica:       req.MaxReplica,
		Annotation:       string(annoStr),
		CostPerHour:      req.CostPerHour,
		ClusterID:        req.ClusterID,
		SecureLevel:      req.SecureLevel,
	})
}
