package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// TODO:remove after migration complete
// Sunday, April 7, 2024 3:41:09 AM
var migrate = time.Unix(1712461269, 0)

const spaceGitattributesContent = modelGitattributesContent

func NewSpaceComponent(config *config.Config) (*SpaceComponent, error) {
	c := &SpaceComponent{}
	c.ss = database.NewSpaceStore()
	var err error
	c.sss = database.NewSpaceSdkStore()
	c.srs = database.NewSpaceResourceStore()
	c.rs = database.NewRepoStore()
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
	ss               *database.SpaceStore
	sss              *database.SpaceSdkStore
	srs              *database.SpaceResourceStore
	rs               *database.RepoStore
	deployer         deploy.Deployer
	publicRootDomain string
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	var nickname string
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

	resSpace, err := c.ss.Create(ctx, dbSpace)
	if err != nil {
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
	}

	// Create README.md file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
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

	// Create .gitattributes file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   spaceGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  gitattributesFileName,
	}, types.SpaceRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
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

func (c *SpaceComponent) Show(ctx context.Context, namespace, name, currentUser string) (*types.Space, error) {
	var tags []types.RepoTag
	space, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, space.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	var endpoint string
	srvName, status, _ := c.status(ctx, space)
	if len(srvName) > 0 {
		endpoint = fmt.Sprintf("%s.%s", srvName, c.publicRootDomain)
	}

	likeExists, err := c.uls.IsExist(ctx, currentUser, space.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	resModel := &types.Space{
		ID:            space.ID,
		Name:          space.Repository.Name,
		Nickname:      space.Repository.Nickname,
		Description:   space.Repository.Description,
		Likes:         space.Repository.Likes,
		Path:          space.Repository.Path,
		License:       space.Repository.License,
		DefaultBranch: space.Repository.DefaultBranch,
		Repository: &types.Repository{
			HTTPCloneURL: space.Repository.HTTPCloneURL,
			SSHCloneURL:  space.Repository.SSHCloneURL,
		},
		Private: space.Repository.Private,
		Tags:    tags,
		User: &types.User{
			Username: space.Repository.User.Username,
			Nickname: space.Repository.User.Name,
			Email:    space.Repository.User.Email,
		},
		CreatedAt:     space.CreatedAt,
		UpdatedAt:     space.Repository.UpdatedAt,
		Status:        status,
		Endpoint:      endpoint,
		Hardware:      space.Hardware,
		RepositoryID:  space.Repository.ID,
		UserLikes:     likeExists,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		CoverImageUrl: space.CoverImageUrl,
	}

	return resModel, nil
}

func (c *SpaceComponent) Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error) {
	req.RepoType = types.SpaceRepo
	_, err := c.UpdateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	space, err := c.ss.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}

	space = mergeUpdateSpaceRequest(space, req)
	err = c.ss.Update(ctx, *space)
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

func (c *SpaceComponent) Index(ctx context.Context, username, search, sort string, tags []database.TagReq, per, page int) ([]types.Space, int, error) {
	var (
		resSpaces []types.Space
		user      database.User
		err       error
	)
	if username != "" {
		user, err = c.user.FindByUsername(ctx, username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			return nil, 0, newError
		}
	}
	repos, total, err := c.rs.PublicToUser(ctx, types.SpaceRepo, user.ID, search, sort, tags, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public space repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	spaces, err := c.ss.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get spaces by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var space *database.Space
		for _, s := range spaces {
			if s.RepositoryID == repo.ID {
				space = &s
				space.Repository = repo
				break
			}
		}
		if space == nil {
			continue
		}
		_, status, _ := c.status(ctx, space)
		var tags []types.RepoTag
		for _, tag := range space.Repository.Tags {
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
		resSpaces = append(resSpaces, types.Space{
			Name:          space.Repository.Name,
			Description:   space.Repository.Description,
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
			Likes:         space.Repository.Likes,
			CreatedAt:     space.Repository.CreatedAt,
			UpdatedAt:     space.Repository.UpdatedAt,
			Tags:          tags,
			Status:        status,
			RepositoryID:  space.Repository.ID,
		})
	}
	return resSpaces, total, nil
}

// UserSpaces get spaces of owner and visible to current user
func (c *SpaceComponent) UserSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	onlyPublic := req.Owner != req.CurrentUser
	ms, total, err := c.ss.ByUsername(ctx, req.Owner, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get spaces by username,%w", err)
		return nil, 0, newError
	}

	var resSpaces []types.Space
	for _, data := range ms {
		_, status, _ := c.status(ctx, &data)
		resSpaces = append(resSpaces, types.Space{
			ID:            data.ID,
			Name:          data.Repository.Name,
			Nickname:      data.Repository.Nickname,
			Description:   data.Repository.Description,
			Likes:         data.Repository.Likes,
			Path:          data.Repository.Path,
			RepositoryID:  data.RepositoryID,
			Private:       data.Repository.Private,
			CreatedAt:     data.CreatedAt,
			UpdatedAt:     data.Repository.UpdatedAt,
			Hardware:      data.Hardware,
			Status:        status,
			CoverImageUrl: data.CoverImageUrl,
		})
	}

	return resSpaces, total, nil
}

func (c *SpaceComponent) UserLikesSpaces(ctx context.Context, req *types.UserSpacesReq, userID int64) ([]types.Space, int, error) {
	ms, total, err := c.ss.ByUserLikes(ctx, userID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get spaces by username,%w", err)
		return nil, 0, newError
	}

	var resSpaces []types.Space
	for _, data := range ms {
		_, status, _ := c.status(ctx, &data)
		resSpaces = append(resSpaces, types.Space{
			ID:            data.ID,
			Name:          data.Repository.Name,
			Nickname:      data.Repository.Nickname,
			Description:   data.Repository.Description,
			Likes:         data.Repository.Likes,
			Path:          data.Repository.Path,
			Private:       data.Repository.Private,
			CreatedAt:     data.CreatedAt,
			UpdatedAt:     data.Repository.UpdatedAt,
			Hardware:      data.Hardware,
			Status:        status,
			CoverImageUrl: data.CoverImageUrl,
		})
	}

	return resSpaces, total, nil
}

func (c *SpaceComponent) ListByPath(ctx context.Context, paths []string) ([]*types.Space, error) {
	var spaces []*types.Space

	spacesData, err := c.ss.ListByPath(ctx, paths)
	if err != nil {
		return nil, fmt.Errorf("list space db failed, %w", err)
	}
	for _, data := range spacesData {
		_, status, _ := c.status(ctx, &data)
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
		spaces = append(spaces, &types.Space{
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
			Status:        status,
			RepositoryID:  data.Repository.ID,
		})
	}
	return spaces, nil
}

func (c *SpaceComponent) AllowCallApi(ctx context.Context, spaceID int64, username string) (bool, error) {
	if username == "" {
		return false, errors.New("user not found, please login first")
	}
	s, err := c.ss.ByID(ctx, spaceID)
	if err != nil {
		return false, fmt.Errorf("failed to get space by id:%d, %w", spaceID, err)
	}
	fields := strings.Split(s.Repository.Path, "/")
	return c.AllowReadAccess(ctx, fields[0], fields[1], username)
}

func (c *SpaceComponent) Delete(ctx context.Context, namespace, name, currentUser string) error {
	space, err := c.ss.FindByPath(ctx, namespace, name)
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

	err = c.ss.Delete(ctx, *space)
	if err != nil {
		return fmt.Errorf("failed to delete database space, error: %w", err)
	}
	return nil
}

func (c *SpaceComponent) Deploy(ctx context.Context, namespace, name string) (int64, error) {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't deploy space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return -1, err
	}

	return c.deployer.Deploy(ctx, types.Space{
		ID:         s.ID,
		Path:       s.Repository.GitPath,
		Sdk:        s.Sdk,
		SdkVersion: s.SdkVersion,
		Template:   s.Template,
		Env:        s.Env,
		Hardware:   s.Hardware,
		Secrets:    s.Secrets,
	})
}

func (c *SpaceComponent) Wakeup(ctx context.Context, namespace, name string) error {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't wakeup space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err

	}

	return c.deployer.Wakeup(ctx, types.Space{
		ID:        s.ID,
		Namespace: namespace,
		Name:      name,
	})
}

func (c *SpaceComponent) Stop(ctx context.Context, namespace, name string) error {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't stop space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}

	return c.deployer.Stop(ctx, types.Space{
		ID:        s.ID,
		Namespace: namespace,
		Name:      name,
	})
}

// FixHasAppFile checks whether git repo has app file and update space's HasAppFile property in db
func (c *SpaceComponent) FixHasAppFile(ctx context.Context, s *database.Space) *database.Space {
	namespace, repoName := s.Repository.NamespaceAndName()
	hasAppFile := c.HasAppFile(ctx, namespace, repoName)
	if s.HasAppFile != hasAppFile {
		s.HasAppFile = hasAppFile
		c.ss.Update(ctx, *s)
	}

	return s
}

func (c *SpaceComponent) status(ctx context.Context, s *database.Space) (string, string, error) {
	// TODO: should be removed later.
	// `HasAppFile` is a new field of type space, use folloing code to auto-fill its value
	if !s.HasAppFile && s.CreatedAt.Before(migrate) {
		s = c.FixHasAppFile(ctx, s)
	}
	if !s.HasAppFile {
		return "", SpaceStatusNoAppFile, nil
	}
	namespace, name := s.Repository.NamespaceAndName()
	srvName, code, err := c.deployer.Status(ctx, types.Space{
		ID:        s.ID,
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		slog.Error("error happen when get space status", slog.Any("error", err), slog.String("path", s.Repository.Path))
		return "", SpaceStatusStopped, err
	}
	return srvName, spaceStatusCodeToString(code), nil
}

func (c *SpaceComponent) Status(ctx context.Context, namespace, name string) (string, string, error) {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		return "", SpaceStatusStopped, fmt.Errorf("can't find space by path:%w", err)
	}
	return c.status(ctx, s)
}

func (c *SpaceComponent) Logs(ctx context.Context, namespace, name string) (*deploy.MultiLogReader, error) {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("can't find space by path:%w", err)
	}
	return c.deployer.Logs(ctx, types.Space{
		ID:        s.ID,
		Namespace: namespace,
		Name:      name,
	})
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

	return false
}

func spaceStatusCodeToString(code int) string {
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
		txt = SpaceStatusStopped
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

func mergeUpdateSpaceRequest(space *database.Space, req *types.UpdateSpaceReq) *database.Space {
	// Do not update column value if request body do not have it
	if req.Sdk != "" {
		space.Sdk = req.Sdk
	}
	if req.SdkVersion != "" {
		space.SdkVersion = req.SdkVersion
	}
	if req.Env != "" {
		space.Env = req.Env
	}
	if req.Hardware != "" {
		space.Hardware = req.Hardware
	}
	if req.Secrets != "" {
		space.Secrets = req.Secrets
	}
	if req.Template != "" {
		space.Template = req.Template
	}
	if req.CoverImageUrl != "" {
		space.CoverImageUrl = req.CoverImageUrl
	}
	return space
}

const (
	// SpaceStatusEmpty is the init status by default
	SpaceStatusEmpty        = ""
	SpaceStatusBuilding     = "Building"
	SpaceStatusBuildFailed  = "BuildingFailed"
	SpaceStatusDeploying    = "Deploying"
	SpaceStatusDeployFailed = "DeployFailed"
	SpaceStatusRunning      = "Running"
	SpaceStatusRuntimeError = "RuntimeError"
	SpaceStatusStopped      = "Stopped"
	SpaceStatusSleeping     = "Sleeping"

	SpaceStatusNoAppFile = "NoAppFile"
)
