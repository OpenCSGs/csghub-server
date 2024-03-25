package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceComponent(config *config.Config) (*SpaceComponent, error) {
	c := &SpaceComponent{}
	c.space = database.NewSpaceStore()
	var err error
	c.sss = database.NewSpaceSdkStore()
	c.srs = database.NewSpaceResourceStore()
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	c.deployer = deploy.NewDeployer()
	c.publicRootDomain = config.Space.PublicRootDomain
	return c, nil
}

type SpaceComponent struct {
	*RepoComponent
	space    *database.SpaceStore
	rproxy   *proxy.ReverseProxy
	sss      *database.SpaceSdkStore
	srs      *database.SpaceResourceStore
	deployer deploy.Deployer

	publicRootDomain string
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	var nickname string
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
	req.RepoType = types.SpaceRepo
	req.Readme = generateReadmeData(req.License)
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbSpace := database.Space{
		RepositoryID:  dbRepo.ID,
		Sdk:           req.Sdk,
		SdkVersion:    req.SdkVersion,
		CoverImageUrl: req.CoverImageUrl,
		Env:           req.Env,
		Hardware:      req.Hardware,
		Secrets:       req.Secrets,
	}

	resSpace, err := c.space.Create(ctx, dbSpace)
	if err != nil {
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
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
	}, types.SpaceRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	space := &types.Space{
		Creator:       req.Username,
		Namespace:     req.Namespace,
		License:       req.License,
		Path:          dbRepo.Path,
		Name:          req.Name,
		Sdk:           req.Sdk,
		SdkVersion:    req.SdkVersion,
		Env:           req.Env,
		Hardware:      req.Hardware,
		Secrets:       req.Secrets,
		CoverImageUrl: resSpace.CoverImageUrl,
		Endpoint:      "",
		Status:        "",
		Private:       req.Private,
		CreatedAt:     resSpace.CreatedAt,
	}
	return space, nil
}

func (c *SpaceComponent) Show(ctx context.Context, namespace, name, current_user string) (*types.Space, error) {
	var tags []types.RepoTag
	space, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}

	if space.Repository.Private {
		if space.Repository.User.Username != current_user {
			return nil, fmt.Errorf("failed to find space, error: %w", errors.New("the private space is not accessible to the current user"))
		}
	}

	var endpoint string
	var srvName string
	var status string
	if c.HasAppFile(ctx, namespace, name) {
		// get model rue status
		ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		srvName, status, _ = c.status(ctxTimeout, space)
		endpoint = fmt.Sprintf("%s.%s", srvName, c.publicRootDomain)
	} else {
		status = "NoAppFile"
	}

	resModel := &types.Space{
		ID:            space.ID,
		Name:          space.Repository.Name,
		Nickname:      space.Repository.Nickname,
		Description:   space.Repository.Description,
		Likes:         space.Repository.Likes,
		Path:          space.Repository.Path,
		DefaultBranch: space.Repository.DefaultBranch,
		Repository: types.Repository{
			HTTPCloneURL: space.Repository.HTTPCloneURL,
			SSHCloneURL:  space.Repository.SSHCloneURL,
		},
		Private: space.Repository.Private,
		Tags:    tags,
		User: types.User{
			Username: space.Repository.User.Username,
			Nickname: space.Repository.User.Name,
			Email:    space.Repository.User.Email,
		},
		CreatedAt: space.CreatedAt,
		UpdatedAt: space.UpdatedAt,
		Status:    status,
		Endpoint:  endpoint,
		Hardware:  space.Hardware,
	}

	return resModel, nil
}

func (c *SpaceComponent) Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error) {
	req.RepoType = types.SpaceRepo
	_, err := c.UpdateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	space, err := c.space.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}

	err = c.space.Update(ctx, *space)
	if err != nil {
		return nil, fmt.Errorf("failed to update database space, error: %w", err)
	}

	resDataset := &types.Space{
		ID:            space.ID,
		Name:          space.Repository.Name,
		Path:          space.Repository.Path,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		Template:      space.Template,
		Env:           space.Env,
		Hardware:      space.Hardware,
		Secrets:       space.Secrets,
		CoverImageUrl: space.CoverImageUrl,
		License:       space.Repository.License,
		Private:       space.Repository.Private,
		Creator:       space.Repository.User.Username,
		CreatedAt:     space.Repository.CreatedAt,
	}

	return resDataset, nil
}

func (c *SpaceComponent) Index(ctx context.Context, username, search, sort string, per, page int) ([]types.Space, int, error) {
	var (
		spaces []types.Space
		user   database.User
		err    error
	)
	if username == "" {
		slog.Info("get spaces without current username", slog.String("search", search))
	} else {
		user, err = c.user.FindByUsername(ctx, username)
		if err != nil {
			slog.Error("fail to get public spaces", slog.String("user", username), slog.String("search", search),
				slog.String("error", err.Error()))
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			return nil, 0, newError
		}
	}
	spaceData, total, err := c.space.PublicToUser(ctx, user.ID, search, sort, per, page)
	if err != nil {
		slog.Error("fail to get public spaces", slog.String("user", username), slog.String("search", search),
			slog.String("error", err.Error()))
		newError := fmt.Errorf("failed to get public spaces,error:%w", err)
		return nil, 0, newError
	}

	for _, data := range spaceData {
		var tags []types.RepoTag
		for _, tag := range data.Repository.Tags {
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
		spaces = append(spaces, types.Space{
			Name:          data.Repository.Name,
			Description:   data.Repository.Description,
			Path:          data.Repository.Path,
			Sdk:           data.Sdk,
			SdkVersion:    data.SdkVersion,
			Template:      data.Template,
			Env:           data.Env,
			Hardware:      data.Hardware,
			Secrets:       data.Secrets,
			CoverImageUrl: data.CoverImageUrl,
			License:       data.Repository.License,
			Private:       data.Repository.Private,
			Likes:         data.Repository.Likes,
			CreatedAt:     data.Repository.CreatedAt,
			UpdatedAt:     data.Repository.UpdatedAt,
			Tags:          tags,
		})
	}
	return spaces, total, nil
}

func (c *SpaceComponent) AllowCallApi(ctx context.Context, namespace, name, username string) (bool, error) {
	if username == "" {
		return false, errors.New("user not found, please login first")
	}
	return c.AllowReadAccess(ctx, namespace, name, username)
}

func (c *SpaceComponent) Delete(ctx context.Context, namespace, name, currentUser string) error {
	space, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find space, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.SpaceRepo,
	}
	_, err = c.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of space, error: %w", err)
	}

	err = c.space.Delete(ctx, *space)
	if err != nil {
		return fmt.Errorf("failed to delete database space, error: %w", err)
	}
	return nil
}

func (c *SpaceComponent) Deploy(ctx context.Context, namespace, name string) (int64, error) {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't deploy space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return -1, err
	}

	return c.deployer.Deploy(ctx, types.Space{
		ID:            s.ID,
		Creator:       s.Repository.User.Name,
		Namespace:     s.Repository.Name,
		Name:          s.Repository.Name,
		Path:          s.Repository.GitPath,
		Sdk:           s.Sdk,
		SdkVersion:    s.SdkVersion,
		CoverImageUrl: s.CoverImageUrl,
		Template:      s.Template,
		Env:           s.Env,
		Hardware:      s.Hardware,
		Secrets:       s.Secrets,
	})
}

func (c *SpaceComponent) Stop(ctx context.Context, namespace, name string) error {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't stop space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}

	return c.deployer.Stop(ctx, s.ID)
}

func (c *SpaceComponent) status(ctx context.Context, s *database.Space) (string, string, error) {
	srvName, code, err := c.deployer.Status(ctx, s.ID)
	if err != nil {
		slog.Error("error happen when get space status", slog.Any("error", err), slog.String("path", s.Repository.Path))
		return "", SpaceStatusStopped, err
	}
	return srvName, c.statusCodeToString(code), nil
}

func (c *SpaceComponent) Status(ctx context.Context, namespace, name string) (string, string, error) {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't get space status", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return "", SpaceStatusStopped, err
	}
	return c.status(ctx, s)
}

func (c *SpaceComponent) Logs(ctx context.Context, namespace, name string) (*deploy.MultiLogReader, error) {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't get space logs", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return nil, err
	}
	return c.deployer.Logs(ctx, s.ID)
}

func (c *SpaceComponent) HasAppFile(ctx context.Context, namespace, name string) bool {
	var req gitserver.GetRepoInfoByPathReq
	req.Namespace = namespace
	req.Name = name
	// root dir
	req.Path = ""
	req.RepoType = types.SpaceRepo
	files, err := c.git.GetRepoFileTree(ctx, req)
	if err != nil {
		slog.Error("check repo app file existence failed", slog.Any("eror", err))
		return false
	}

	for _, f := range files {
		if f.Type == "file" && f.Path == "app.py" {
			return true
		}
	}

	slog.Info("space has not app file", slog.String("namespace", namespace), slog.String("name", name))
	return false
}

func (c *SpaceComponent) statusCodeToString(code int) string {
	// DeployBuildPending    = 10
	// DeployBuildInProgress = 11
	// DeployBuildFailed     = 12
	// DeployBuildSucceed    = 13
	//
	// DeployPrepareToRun = 20
	// DeployStartUp      = 21
	// DeployRunning      = 22
	// DeployRunTimeError = 23

	// simplified status for frontend show
	var txt string
	switch code {
	case 10:
		txt = SpaceStatusEmpty
	case 11:
		txt = SpaceStatusBuilding
	case 12:
		txt = SpaceStatusBuildFailed
	case 13:
		txt = SpaceStatusDeploying
	case 20:
		txt = SpaceStatusDeploying
	case 21:
		txt = SpaceStatusDeployFailed
	case 22:
		txt = SpaceStatusDeploying
	case 23:
		txt = SpaceStatusRunning
	case 24:
		txt = SpaceStatusRuntimeError
	case 25:
		txt = SpaceStatusSleeping
	default:
		txt = SpaceStatusStopped
	}
	return txt
}

const (
	// SpaceStatusEmpty is the init status by default
	SpaceStatusEmpty        = ""
	SpaceStatusBuilding     = "Building"
	SpaceStatusBuildFailed  = "BuildFailed"
	SpaceStatusDeploying    = "Deploying"
	SpaceStatusDeployFailed = "DeployFailed"
	SpaceStatusRunning      = "Running"
	SpaceStatusRuntimeError = "RuntimeError"
	SpaceStatusStopped      = "Stopped"
	SpaceStatusSleeping     = "Sleeping"
)
