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

func NewUserComponent(config *config.Config) (*UserComponent, error) {
	c := &UserComponent{}
	c.ms = database.NewModelStore()
	c.us = database.NewUserStore()
	c.ds = database.NewDatasetStore()
	c.cs = database.NewCodeStore()
	c.ss = database.NewSpaceStore()
	c.ns = database.NewNamespaceStore()
	c.cos = database.NewCollectionStore()
	var err error
	c.spaceComponent, err = NewSpaceComponent(config)
	if err != nil {
		newError := fmt.Errorf("failed to create space component,error:%w", err)
		return nil, newError
	}
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		return nil, newError
	}
	c.repoComponent, err = NewRepoComponent(config)
	if err != nil {
		newError := fmt.Errorf("failed to create repo component,error:%w", err)
		return nil, newError
	}
	c.deployer = deploy.NewDeployer()
	c.uls = database.NewUserLikesStore()
	c.repo = database.NewRepoStore()
	c.deploy = database.NewDeployTaskStore()
	return c, nil
}

type UserComponent struct {
	us             *database.UserStore
	ms             *database.ModelStore
	ds             *database.DatasetStore
	cs             *database.CodeStore
	ss             *database.SpaceStore
	ns             *database.NamespaceStore
	gs             gitserver.GitServer
	spaceComponent *SpaceComponent
	repoComponent  *RepoComponent
	deployer       deploy.Deployer
	uls            *database.UserLikesStore
	repo           *database.RepoStore
	deploy         *database.DeployTaskStore
	cos            *database.CollectionStore
}

func (c *UserComponent) Datasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
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
	ds, total, err := c.ds.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
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

func (c *UserComponent) Models(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
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
	ms, total, err := c.ms.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
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

func (c *UserComponent) Codes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error) {
	var resCodes []types.Code
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
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
	ms, total, err := c.cs.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
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

func (c *UserComponent) Spaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
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

func (c *UserComponent) AddLikes(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return newError
	}
	var likesRepoIDs []int64
	likesRepoIDs = append(likesRepoIDs, req.Repo_id)

	var opts []database.SelectOption
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private"))

	likesRepos, err := c.repo.FindByIds(ctx, likesRepoIDs, opts...)
	if err != nil {
		return fmt.Errorf("failed to get likes repositories by ids, error: %w", err)
	}
	likesRepos, err = c.repoComponent.visiableToUser(ctx, likesRepos, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("failed to check likes repositories visiable to user:%s, %w", req.CurrentUser, err)
	}

	if len(likesRepos) < 1 {
		return fmt.Errorf("do not found likes repositories visiable to user:%s, %w", req.CurrentUser, err)
	}

	err = c.uls.Add(ctx, user.ID, req.Repo_id)
	return err
}

// user likes collection
func (c *UserComponent) LikesCollection(ctx context.Context, req *types.UserSpacesReq) ([]types.Collection, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}
	collections, total, err := c.cos.ByUserLikes(ctx, user.ID, req.PageSize, req.Page)
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
func (c *UserComponent) Collections(ctx context.Context, req *types.UserCollectionReq) ([]types.Collection, int, error) {
	userExists, err := c.us.IsExist(ctx, req.Owner)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	if !userExists {
		return nil, 0, errors.New("user not exists")
	}

	if req.CurrentUser != "" {
		cuserExists, err := c.us.IsExist(ctx, req.CurrentUser)
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
	collections, total, err := c.cos.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
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

func (c *UserComponent) LikeCollection(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return newError
	}

	collection, err := c.cos.FindById(ctx, req.Collection_id)
	if err != nil {
		return fmt.Errorf("failed to get likes collection by id, error: %w", err)
	}

	if collection.Private && collection.UserID != user.ID {
		return fmt.Errorf("no permission to like this collection for user:%s", req.CurrentUser)
	}

	err = c.uls.LikeCollection(ctx, user.ID, req.Collection_id)
	return err
}

func (c *UserComponent) UnLikeCollection(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		return newError
	}
	err = c.uls.UnLikeCollection(ctx, user.ID, req.Collection_id)
	return err
}

func (c *UserComponent) DeleteLikes(ctx context.Context, req *types.UserLikesRequest) error {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		return newError
	}
	err = c.uls.Delete(ctx, user.ID, req.Repo_id)
	return err
}

func (c *UserComponent) LikesSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}
	return c.spaceComponent.UserLikesSpaces(ctx, req, user.ID)
}

func (c *UserComponent) LikesCodes(ctx context.Context, req *types.UserModelsReq) ([]types.Code, int, error) {
	var resCodes []types.Code
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ms, total, err := c.cs.UserLikesCodes(ctx, user.ID, req.PageSize, req.Page)
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

func (c *UserComponent) LikesModels(ctx context.Context, req *types.UserModelsReq) ([]types.Model, int, error) {
	var resModels []types.Model
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ms, total, err := c.ms.UserLikesModels(ctx, user.ID, req.PageSize, req.Page)
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

func (c *UserComponent) LikesDatasets(ctx context.Context, req *types.UserDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user, error:%w", err)
		return nil, 0, newError
	}

	ds, total, err := c.ds.UserLikesDatasets(ctx, user.ID, req.PageSize, req.Page)
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

func (c *UserComponent) ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.DeployReq) ([]types.DeployRepo, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
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
		endpoint, _ := c.repoComponent.generateEndpoint(ctx, d)
		repoPath := strings.TrimPrefix(deploy.GitPath, string(repoType)+"s_")
		var hardware types.HardWare
		json.Unmarshal([]byte(deploy.Hardware), &hardware)
		resourceType := ""
		if hardware.Gpu.Num != "" {
			resourceType = hardware.Gpu.Type
		} else {
			resourceType = hardware.Cpu.Type
		}
		tag := ""
		tags, _ := c.repo.TagsWithCategory(ctx, deploy.RepoID, "task")
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

func (c *UserComponent) ListInstances(ctx context.Context, req *types.UserRepoReq) ([]types.DeployRepo, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
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

func (c *UserComponent) ListServerless(ctx context.Context, req types.DeployReq) ([]types.DeployRepo, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	isAdmin := c.repoComponent.isAdminRole(user)
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
