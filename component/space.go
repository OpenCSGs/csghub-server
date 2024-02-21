package component

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	"opencsg.com/csghub-server/builder/git"
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
	return c, nil
}

type SpaceComponent struct {
	repoComponent
	space *database.SpaceStore
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	req.CreateRepoReq.RepoType = types.SpaceRepo
	req.CreateRepoReq.Readme = "Please introduce your space!"
	gitRepo, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)

	dbSpace := database.Space{
		Name:          req.Name,
		UrlSlug:       gitRepo.Nickname,
		Path:          path.Join(req.Namespace, req.Name),
		GitPath:       gitRepo.GitPath,
		RepositoryID:  dbRepo.ID,
		LastUpdatedAt: time.Now(),
		Private:       req.Private,
		UserID:        dbRepo.UserID,
		Sdk:           req.Sdk,
	}

	_, err = c.space.Create(ctx, dbSpace)
	if err != nil {
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
	}

	space := &types.Space{
		Creator:   req.Username,
		Namespace: req.Namespace,
		License:   req.License,
		Name:      req.Name,
		Sdk:       req.Sdk,
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
			Creator:   data.User.Username,
			Namespace: data.User.Username,
			Name:      data.Name,
			Sdk:       data.Sdk,
			// TODO: get running status and endpoint from inference service
			Endpoint:      "",
			RunningStatus: "",
			Private:       data.Private,
			Likes:         data.Likes,
			CreatedAt:     data.CreatedAt,
			CoverImg:      "",
		})
	}
	return spaces, total, nil
}
