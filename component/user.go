package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type UserComponent interface {
	Datasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error)
	Models(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error)
	Codes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error)
	Spaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error)
	AddLikes(ctx context.Context, req *types.UserLikesRequest) error
	// user likes collection
	LikesCollection(ctx context.Context, req *types.UserSpacesReq) ([]types.Collection, int, error)
	// UserCollections get collections of owner or visible to current user
	Collections(ctx context.Context, req *types.UserCollectionReq) ([]types.Collection, int, error)
	LikeCollection(ctx context.Context, req *types.UserLikesRequest) error
	UnLikeCollection(ctx context.Context, req *types.UserLikesRequest) error
	DeleteLikes(ctx context.Context, req *types.UserLikesRequest) error
	LikesSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error)
	LikesCodes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error)
	LikesModels(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error)
	LikesDatasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error)
	ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.DeployReq) ([]types.DeployRepo, int, error)
	ListInstances(ctx context.Context, req *types.UserRepoReq) ([]types.DeployRepo, int, error)
	ListServerless(ctx context.Context, req types.DeployReq) ([]types.DeployRepo, int, error)
	GetUserByName(ctx context.Context, userName string) (*database.User, error)
	Prompts(ctx context.Context, req *types.UserPromptsReq) ([]types.PromptRes, int, error)
	Evaluations(ctx context.Context, req *types.UserEvaluationReq) ([]types.ArgoWorkFlowRes, int, error)
}

func NewUserComponent(config *config.Config) (UserComponent, error) {
	c := &userComponentImpl{}
	c.modelStore = database.NewModelStore()
	c.userStore = database.NewUserStore()
	c.datasetStore = database.NewDatasetStore()
	c.codeStore = database.NewCodeStore()
	c.spaceStore = database.NewSpaceStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.collectionStore = database.NewCollectionStore()
	var err error
	c.spaceComponent, err = NewSpaceComponent(config)
	if err != nil {
		newError := fmt.Errorf("failed to create space component,error:%w", err)
		return nil, newError
	}
	c.gitServer, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		return nil, newError
	}
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		newError := fmt.Errorf("failed to create repo component,error:%w", err)
		return nil, newError
	}
	c.deployer = deploy.NewDeployer()
	c.userLikeStore = database.NewUserLikesStore()
	c.repoStore = database.NewRepoStore()
	c.deploy = database.NewDeployTaskStore()
	c.accountingComponent, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.promptStore = database.NewPromptStore()
	c.promptStore = database.NewPromptStore()
	c.wfs = database.NewArgoWorkFlowStore()
	return c, nil
}

type userComponentImpl struct {
	userStore           database.UserStore
	modelStore          database.ModelStore
	datasetStore        database.DatasetStore
	codeStore           database.CodeStore
	spaceStore          database.SpaceStore
	namespaceStore      database.NamespaceStore
	gitServer           gitserver.GitServer
	spaceComponent      SpaceComponent
	repoComponent       RepoComponent
	deployer            deploy.Deployer
	userLikeStore       database.UserLikesStore
	repoStore           database.RepoStore
	deploy              database.DeployTaskStore
	collectionStore     database.CollectionStore
	accountingComponent AccountingComponent
	// srs            database.SpaceResourceStore
	// urs            *database.UserResourcesStore
	promptStore database.PromptStore
	wfs         database.ArgoWorkFlowStore
}

func (c *userComponentImpl) Datasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	onlyPublic := req.Owner != req.CurrentUser
	ds, total, err := c.datasetStore.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range ds {
		resDatasets = append(resDatasets, types.Dataset{
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

	return resDatasets, total, nil
}

func (c *userComponentImpl) Models(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	onlyPublic := req.Owner != req.CurrentUser
	ms, total, err := c.modelStore.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user models,error:%w", err)
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

func (c *userComponentImpl) Codes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error) {
	var resCodes []types.Code
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	onlyPublic := req.Owner != req.CurrentUser
	ms, total, err := c.codeStore.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user codes,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range ms {
		resCodes = append(resCodes, types.Code{
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

	return resCodes, total, nil
}

func (c *userComponentImpl) Spaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	return c.spaceComponent.UserSpaces(ctx, req)
}

func (c *userComponentImpl) AddLikes(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return newError
	}
	var likesRepoIDs []int64
	likesRepoIDs = append(likesRepoIDs, req.Repo_id)

	var opts []database.SelectOption
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private"))

	likesRepos, err := c.repoStore.FindByIds(ctx, likesRepoIDs, opts...)
	if err != nil {
		return fmt.Errorf("failed to get likes repositories by ids, error: %w", err)
	}
	likesRepos, err = c.repoComponent.VisiableToUser(ctx, likesRepos, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("failed to check likes repositories visiable to user:%s, %w", req.CurrentUser, err)
	}

	if len(likesRepos) < 1 {
		return fmt.Errorf("do not found likes repositories visiable to user:%s, %w", req.CurrentUser, err)
	}

	err = c.userLikeStore.Add(ctx, user.ID, req.Repo_id)
	return err
}

// user likes collection
func (c *userComponentImpl) LikesCollection(ctx context.Context, req *types.UserSpacesReq) ([]types.Collection, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}
	collections, total, err := c.collectionStore.ByUserLikes(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get collections by username,%w", err)
		return nil, 0, newError
	}

	var newCollection []types.Collection
	temporaryVariable, _ := json.Marshal(collections)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, 0, err
	}

	return newCollection, total, nil
}

// UserCollections get collections of owner or visible to current user
func (c *userComponentImpl) Collections(ctx context.Context, req *types.UserCollectionReq) ([]types.Collection, int, error) {
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	onlyPublic := req.Owner != req.CurrentUser
	collections, total, err := c.collectionStore.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get collections by username,%w", err)
		return nil, 0, newError
	}

	var newCollection []types.Collection
	temporaryVariable, _ := json.Marshal(collections)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, 0, err
	}

	return newCollection, total, nil
}

func (c *userComponentImpl) LikeCollection(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return newError
	}

	collection, err := c.collectionStore.FindById(ctx, req.Collection_id)
	if err != nil {
		return fmt.Errorf("failed to get likes collection by id, error: %w", err)
	}

	if collection.Private && collection.UserID != user.ID {
		return fmt.Errorf("no permission to like this collection for user:%s", req.CurrentUser)
	}

	err = c.userLikeStore.LikeCollection(ctx, user.ID, req.Collection_id)
	return err
}

func (c *userComponentImpl) UnLikeCollection(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		return newError
	}
	err = c.userLikeStore.UnLikeCollection(ctx, user.ID, req.Collection_id)
	return err
}

func (c *userComponentImpl) DeleteLikes(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		return newError
	}
	err = c.userLikeStore.Delete(ctx, user.ID, req.Repo_id)
	return err
}

func (c *userComponentImpl) LikesSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}
	return c.spaceComponent.UserLikesSpaces(ctx, req, user.ID)
}

func (c *userComponentImpl) LikesCodes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error) {
	var resCodes []types.Code
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ms, total, err := c.codeStore.UserLikesCodes(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user codes,error:%w", err)
		return nil, 0, newError
	}

	for _, data := range ms {
		resCodes = append(resCodes, types.Code{
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

	return resCodes, total, nil
}

func (c *userComponentImpl) LikesModels(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ms, total, err := c.modelStore.UserLikesModels(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user models,error:%w", err)
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

func (c *userComponentImpl) LikesDatasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ds, total, err := c.datasetStore.UserLikesDatasets(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		return nil, 0, newError
	}

	for _, data := range ds {
		resDatasets = append(resDatasets, types.Dataset{
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

	return resDatasets, total, nil
}

func (c *userComponentImpl) ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.DeployReq) ([]types.DeployRepo, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	deploys, total, err := c.deploy.ListDeployByUserID(ctx, user.ID, req)
	if err != nil {
		newError := fmt.Errorf("failed to get user deploys for %s with error:%w", repoType, err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		d := &database.Deploy{
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
			Status:    deploy.Status,
		}
		endpoint, _ := c.repoComponent.GenerateEndpoint(ctx, d)
		repoPath := strings.TrimPrefix(deploy.GitPath, string(repoType)+"s_")
		var hardware types.HardWare
		_ = json.Unmarshal([]byte(deploy.Hardware), &hardware)
		resourceType := ""
		if hardware.Gpu.Num != "" {
			resourceType = hardware.Gpu.Type
		} else {
			resourceType = hardware.Cpu.Type
		}
		tag := ""
		tags, _ := c.repoStore.TagsWithCategory(ctx, deploy.RepoID, "task")
		if len(tags) > 0 {
			tag = tags[0].Name
		}
		resDeploys = append(resDeploys, types.DeployRepo{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             repoPath,
			RepoID:           deploy.RepoID,
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
			ResourceType:     resourceType,
			RepoTag:          tag,
			Endpoint:         endpoint,
		})
	}
	return resDeploys, total, nil
}

func (c *userComponentImpl) ListInstances(ctx context.Context, req *types.UserRepoReq) ([]types.DeployRepo, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	deploys, total, err := c.deploy.ListInstancesByUserID(ctx, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user instances error:%w", err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		repoPath := strings.TrimPrefix(deploy.GitPath, "models_")
		resDeploys = append(resDeploys, types.DeployRepo{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             repoPath,
			RepoID:           deploy.RepoID,
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
		})
	}
	return resDeploys, total, nil
}

func (c *userComponentImpl) ListServerless(ctx context.Context, req types.DeployReq) ([]types.DeployRepo, int, error) {
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	isAdmin := c.repoComponent.IsAdminRole(user)
	if !isAdmin {
		return nil, 0, fmt.Errorf("user %s does not have admin privileges", req.CurrentUser)
	}
	deploys, total, err := c.deploy.ListServerless(ctx, req)
	if err != nil {
		newError := fmt.Errorf("failed to get user serverless for %s with error:%w", req.RepoType, err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		repoPath := strings.TrimPrefix(deploy.GitPath, string(req.RepoType)+"s_")
		resDeploys = append(resDeploys, types.DeployRepo{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			Path:             repoPath,
			RepoID:           deploy.RepoID,
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
			SKU:              deploy.SKU,
		})
	}
	return resDeploys, total, nil
}

func (c *userComponentImpl) GetUserByName(ctx context.Context, userName string) (*database.User, error) {
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to check for the presence of the user %s,error:%w", userName, err)
	}
	return &user, nil
}

func (c *userComponentImpl) Prompts(ctx context.Context, req *types.UserPromptsReq) ([]types.PromptRes, int, error) {
	var resPrompts []types.PromptRes
	userExists, err := c.userStore.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.userStore.IsExist(ctx, req.CurrentUser)
		if err != nil {
			newError := fmt.Errorf("failed to check for the presence of current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}

		if !cuserExists {
			return nil, 0, errors.New("current user not exists")
		}
	}

	onlyPublic := req.Owner != req.CurrentUser
	ds, total, err := c.promptStore.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user prompts,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range ds {

		resPrompts = append(resPrompts, types.PromptRes{
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

	return resPrompts, total, nil
}

func (c *userComponentImpl) Evaluations(ctx context.Context, req *types.UserEvaluationReq) ([]types.ArgoWorkFlowRes, int, error) {

	_, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	res, err := c.deployer.ListEvaluations(ctx, req.CurrentUser, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user evaluations,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}
	return res.List, res.Total, nil
}
