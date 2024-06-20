package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
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
	c.once = new(sync.Once)
	c.casConfig = &casdoorsdk.AuthConfig{
		Endpoint:         config.Casdoor.Endpoint,
		ClientId:         config.Casdoor.ClientID,
		ClientSecret:     config.Casdoor.ClientSecret,
		Certificate:      config.Casdoor.Certificate,
		OrganizationName: config.Casdoor.OrganizationName,
		ApplicationName:  config.Casdoor.ApplicationName,
	}
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
	casc           *casdoorsdk.Client
	casConfig      *casdoorsdk.AuthConfig
	once           *sync.Once
	deploy         *database.DeployTaskStore
}

func (c *UserComponent) lazyInit() {
	c.once.Do(func() {
		c.casc = casdoorsdk.NewClientWithConf(c.casConfig)
	})
}

func (c *UserComponent) Create(ctx context.Context, req *types.CreateUserRequest) (*database.User, error) {
	nsExists, err := c.ns.Exists(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the namespace,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	if nsExists {
		return nil, errors.New("namespace already exists")
	}

	userExists, err := c.us.IsExist(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	if userExists {
		return nil, errors.New("user already exists")
	}
	user, err := c.gs.CreateUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to create gitserver user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	namespace := &database.Namespace{
		Path: user.Username,
	}
	user.CasdoorUUID = req.CasdoorUID
	err = c.us.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	//skip casdoor update if it's not a casdoor user
	if req.CasdoorUID == "" {
		return user, nil
	}
	ureq := &types.UpdateUserRequest{
		Name:       req.Name,
		Username:   req.Username,
		Email:      req.Email,
		Phone:      req.Phone,
		CasdoorUID: req.CasdoorUID,
	}
	err = c.updateCasdoorUser(ureq)
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user,error:%w", err)
		return nil, newError
	}

	return user, nil
}

func (c *UserComponent) Update(ctx context.Context, req *types.UpdateUserRequest) (*database.User, error) {
	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return nil, newError
	}

	respUser, err := c.gs.UpdateUser(req, &user)
	if err != nil {
		newError := fmt.Errorf("failed to update git user,error:%w", err)
		return nil, newError
	}

	err = c.us.Update(ctx, respUser)
	if err != nil {
		newError := fmt.Errorf("failed to update database user,error:%w", err)
		return nil, newError
	}

	//skip casdoor update if it's not a casdoor user
	if req.CasdoorUID == "" {
		return respUser, nil
	}
	err = c.updateCasdoorUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user,error:%w", err)
		return nil, newError
	}

	return respUser, nil
}

func (c *UserComponent) updateCasdoorUser(req *types.UpdateUserRequest) error {
	c.lazyInit()

	casu, err := c.casc.GetUserByUserId(req.CasdoorUID)
	if err != nil {
		return fmt.Errorf("failed to get user from casdoor,error:%w", err)
	}
	if casu == nil {
		return fmt.Errorf("user not exists in casdoor")
	}
	var cols []string
	if req.Email != "" {
		casu.Email = req.Email
		cols = append(cols, "email")
	}
	if req.Phone != "" {
		casu.Phone = req.Phone
		cols = append(cols, "phone")
	}

	if len(cols) == 0 {
		return nil
	}

	//casdoor update user api don't allow empty display name, so we set it but not update it
	if casu.DisplayName == "" {
		casu.DisplayName = casu.Name
	}

	_, err = c.casc.UpdateUserForColumns(casu, cols)
	return err
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

func (c *UserComponent) FixUserData(ctx context.Context, userName string) error {
	err := c.gs.FixUserData(ctx, userName)
	if err != nil {
		return err
	}

	return nil
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

func (c *UserComponent) ListDeploys(ctx context.Context, repoType types.RepositoryType, req *types.UserRepoReq) ([]types.DeployRepo, int, error) {
	user, err := c.us.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user:%s, error:%w", req.CurrentUser, err)
		return nil, 0, newError
	}
	deploys, total, err := c.deploy.ListDeployByUserID(ctx, repoType, user.ID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get user deploys for %s with error:%w", repoType, err)
		return nil, 0, newError
	}

	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		repoPath := strings.TrimPrefix(deploy.GitPath, string(repoType)+"_")
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
			CostPerHour:      deploy.CostPerHour,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Type:             deploy.Type,
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
			CostPerHour:      deploy.CostPerHour,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Type:             deploy.Type,
		})
	}
	return resDeploys, total, nil
}
