package component

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/proxy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceComponent(config *config.Config) (*SpaceComponent, error) {
	c := &SpaceComponent{}
	c.user = database.NewUserStore()
	c.space = database.NewSpaceStore()
	c.org = database.NewOrgStore()
	c.namespace = database.NewNamespaceStore()
	c.repo = database.NewRepoStore()
	var err error
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.msc, err = NewMemberComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create membership component,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.sss = database.NewSpaceSdkStore()
	c.srs = database.NewSpaceResourceStore()
	return c, nil
}

type SpaceComponent struct {
	repoComponent
	space  *database.SpaceStore
	rproxy *proxy.ReverseProxy
	sss    *database.SpaceSdkStore
	srs    *database.SpaceResourceStore
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	spaceSdk, err := c.sss.FindByID(ctx, req.SdkID)
	if err != nil {
		slog.Error("fail to find space sdk in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to find space sdk in db, error: %w", err)
	}

	spaceResource, err := c.srs.FindByID(ctx, req.ResourceID)
	if err != nil {
		slog.Error("fail to find space resource in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to find space resource in db, error: %w", err)
	}
	req.RepoType = types.SpaceRepo
	req.Readme = "Please introduce your space!"
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbSpace := database.Space{
		RepositoryID:  dbRepo.ID,
		SdkID:         req.SdkID,
		ResourceID:    req.ResourceID,
		CoverImageUrl: req.CoverImageUrl,
	}

	resSpace, err := c.space.Create(ctx, dbSpace)
	if err != nil {
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
	}

	space := &types.Space{
		Creator:   req.Username,
		Namespace: req.Namespace,
		License:   req.License,
		Name:      req.Name,
		Sdk: types.SpaceSdk{
			ID:   spaceSdk.ID,
			Name: spaceSdk.Name,
		},
		Resource: types.SpaceResource{
			ID:   spaceResource.ID,
			Name: spaceResource.Name,
		},
		CoverImageUrl: resSpace.CoverImageUrl,
		// TODO: get running status and endpoint from inference service
		Endpoint:      "",
		RunningStatus: "",
		Private:       req.Private,
	}
	return space, nil
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
			Name: data.Repository.Name,
			Path: data.Repository.Path,
			Sdk: types.SpaceSdk{
				ID:   data.Sdk.ID,
				Name: data.Sdk.Name,
			},
			Resource: types.SpaceResource{
				ID:   data.Resource.ID,
				Name: data.Resource.Name,
			},
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
	repo, err := c.repo.FindByPath(ctx, types.SpaceRepo, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if !repo.Private {
		return true, nil
	}
	return c.checkCurrentUserPermission(ctx, username, namespace, membership.RoleRead)
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
