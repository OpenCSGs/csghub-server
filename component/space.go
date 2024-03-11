package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/monitor"
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
	c.monitor = monitor.NewMonitor()
	c.deployer, err = deploy.NewDeployer()
	if err != nil {
		return nil, fmt.Errorf("create space deployer,%w", err)
	}
	return c, nil
}

type SpaceComponent struct {
	*RepoComponent
	space    *database.SpaceStore
	rproxy   *proxy.ReverseProxy
	sss      *database.SpaceSdkStore
	srs      *database.SpaceResourceStore
	monitor  monitor.Monitor
	deployer deploy.Deployer
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	req.RepoType = types.SpaceRepo
	req.Readme = "Please introduce your space!"
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
		// TODO: get running status and endpoint from inference service
		Endpoint:      "",
		RunningStatus: "",
		Private:       req.Private,
		CreatedAt:     resSpace.CreatedAt,
	}
	return space, nil
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
		spaces = append(spaces, types.Space{
			// Creator:   data.Repository.Username,
			// Namespace: data.Repository.,
			Name:          data.Repository.Name,
			Path:          data.Repository.Path,
			Sdk:           data.Sdk,
			SdkVersion:    data.SdkVersion,
			Template:      data.Template,
			Env:           data.Env,
			Hardware:      data.Hardware,
			Secrets:       data.Secrets,
			CoverImageUrl: data.CoverImageUrl,
			License:       data.Repository.License,
			// // TODO: get running status and endpoint from inference service
			// Endpoint:      "",
			// RunningStatus: "",
			Private: data.Repository.Private,
			// Likes:         data.Repository.Likes,
			CreatedAt: data.Repository.CreatedAt,
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

func (c *SpaceComponent) Status(ctx context.Context, namespace, name string) (string, error) {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't get space status", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return "", err
	}
	return c.deployer.Status(ctx, s.ID)
}

func (c *SpaceComponent) Logs(ctx context.Context, namespace, name string) (string, error) {
	s, err := c.space.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't get space logs", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return "", err
	}
	return c.monitor.Logs(ctx, s.ID)
}
