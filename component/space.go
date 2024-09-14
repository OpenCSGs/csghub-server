package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/scheduler"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const spaceGitattributesContent = modelGitattributesContent

var (
	streamlitConfigContent = `[server]
enableCORS = false
enableXsrfProtection = false
`
	streamlitConfig = ".streamlit/config.toml"
)

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
	c.us = database.NewUserStore()
	c.ac, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}

	return c, nil
}

type SpaceComponent struct {
	*RepoComponent
	ss               *database.SpaceStore
	sss              *database.SpaceSdkStore
	srs              *database.SpaceResourceStore
	rs               *database.RepoStore
	us               *database.UserStore
	deployer         deploy.Deployer
	publicRootDomain string
	ac               *AccountingComponent
}

func (c *SpaceComponent) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	var nickname string
	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	req.Nickname = nickname
	req.RepoType = types.SpaceRepo
	req.Readme = generateReadmeData(req.License)
	resource, err := c.srs.FindByID(ctx, req.ResourceID)
	if err != nil {
		return nil, err
	}
	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return nil, fmt.Errorf("invalid hardware setting, %w", err)
	}
	_, err = c.deployer.CheckResourceAvailable(ctx, req.ClusterID, &hardware)
	if err != nil {
		return nil, fmt.Errorf("fail to check resource, %w", err)
	}

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
		Hardware:      resource.Resources,
		Secrets:       req.Secrets,
		SKU:           strconv.FormatInt(resource.ID, 10),
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

	if req.Sdk == scheduler.STREAMLIT.Name {
		// create .streamlit/config.toml for support cors
		fileReq := types.CreateFileParams{
			Username:  dbRepo.User.Username,
			Email:     dbRepo.User.Email,
			Message:   initCommitMessage,
			Branch:    req.DefaultBranch,
			Content:   streamlitConfigContent,
			NewBranch: req.DefaultBranch,
			Namespace: req.Namespace,
			Name:      req.Name,
			FilePath:  streamlitConfig,
		}
		err = c.git.CreateRepoFile(buildCreateFileReq(&fileReq, types.SpaceRepo))
		if err != nil {
			return nil, fmt.Errorf("failed to create .streamlit/config.toml file, cause: %w", err)
		}
	}

	space := &types.Space{
		Creator:       req.Username,
		License:       req.License,
		Path:          dbRepo.Path,
		Name:          req.Name,
		Sdk:           req.Sdk,
		SdkVersion:    req.SdkVersion,
		Env:           req.Env,
		Hardware:      resource.Resources,
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

	permission, err := c.getUserRepoPermission(ctx, currentUser, space.Repository)
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

	var endpoint string
	svcName, status, _ := c.status(ctx, space)
	if len(svcName) > 0 {
		if c.publicRootDomain == "" {
			if space.Sdk == scheduler.STREAMLIT.Name || space.Sdk == scheduler.GRADIO.Name {
				// if endpoint not ends with /, fastapi based app (gradio and streamlit) will redirect with http 307
				// see issue: https://stackoverflow.com/questions/70351360/keep-getting-307-temporary-redirect-before-returning-status-200-hosted-on-fast
				endpoint, _ = url.JoinPath(c.serverBaseUrl, "endpoint", svcName, "/")
			} else {
				endpoint, _ = url.JoinPath(c.serverBaseUrl, "endpoint", svcName)
			}
			endpoint = strings.Replace(endpoint, "http://", "", 1)
			endpoint = strings.Replace(endpoint, "https://", "", 1)
		} else {
			endpoint = fmt.Sprintf("%s.%s", svcName, c.publicRootDomain)
		}
	}

	likeExists, err := c.uls.IsExist(ctx, currentUser, space.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}
	repository := common.BuildCloneInfo(c.config, space.Repository)

	resModel := &types.Space{
		ID:            space.ID,
		Name:          space.Repository.Name,
		Nickname:      space.Repository.Nickname,
		Description:   space.Repository.Description,
		Likes:         space.Repository.Likes,
		Path:          space.Repository.Path,
		License:       space.Repository.License,
		DefaultBranch: space.Repository.DefaultBranch,
		Repository:    &repository,
		Private:       space.Repository.Private,
		Tags:          tags,
		User: &types.User{
			Username: space.Repository.User.Username,
			Nickname: space.Repository.User.NickName,
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
		Source:        space.Repository.Source,
		SyncStatus:    space.Repository.SyncStatus,
		SKU:           space.SKU,
		SvcName:       svcName,
		CanWrite:      permission.CanWrite,
		CanManage:     permission.CanAdmin,
		Namespace:     ns,
	}

	return resModel, nil
}

func (c *SpaceComponent) Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error) {
	req.RepoType = types.SpaceRepo
	dbRepo, err := c.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	space, err := c.ss.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}
	err = c.mergeUpdateSpaceRequest(ctx, space, req)
	if err != nil {
		return nil, fmt.Errorf("failed to merge update space request, error: %w", err)
	}

	err = c.ss.Update(ctx, *space)
	if err != nil {
		return nil, fmt.Errorf("failed to update database space, error: %w", err)
	}

	resDataset := &types.Space{
		ID:            space.ID,
		Name:          dbRepo.Name,
		Path:          dbRepo.Path,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		Template:      space.Template,
		Env:           space.Env,
		Hardware:      space.Hardware,
		Secrets:       space.Secrets,
		CoverImageUrl: space.CoverImageUrl,
		License:       dbRepo.License,
		Private:       dbRepo.Private,
		CreatedAt:     dbRepo.CreatedAt,
		SKU:           space.SKU,
	}

	return resDataset, nil
}

func (c *SpaceComponent) Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Space, int, error) {
	var (
		resSpaces []types.Space
		err       error
	)
	repos, total, err := c.PublicToUser(ctx, types.SpaceRepo, filter.Username, filter, per, page)
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

	// loop through repos to keep the repos in sort order
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
			Source:        repo.Source,
			SyncStatus:    repo.SyncStatus,
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
		return false, ErrUserNotFound
	}
	s, err := c.ss.ByID(ctx, spaceID)
	if err != nil {
		return false, fmt.Errorf("failed to get space by id:%d, %w", spaceID, err)
	}
	fields := strings.Split(s.Repository.Path, "/")
	return c.AllowReadAccess(ctx, s.Repository.RepositoryType, fields[0], fields[1], username)
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

	// stop any running space instance
	go c.Stop(ctx, namespace, name)

	return nil
}

func (c *SpaceComponent) Deploy(ctx context.Context, namespace, name, currentUser string) (int64, error) {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't deploy space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return -1, err
	}
	// found user id
	user, err := c.us.FindByUsername(ctx, currentUser)
	if err != nil {
		slog.Error("can't find user for create deploy space", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResTypeKey] = string(types.SpaceRepo)
	annotations[types.ResNameKey] = fmt.Sprintf("%s/%s", namespace, name)
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		slog.Error("fail to create annotations for deploy space", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	containerImg := ""
	slog.Info("get space for deploy", slog.Any("space", s), slog.Any("NGINX", scheduler.NGINX))
	slog.Info("compare space sdk", slog.Any("s.Sdk", s.Sdk), slog.Any("scheduler.NGINX.Name", scheduler.NGINX.Name))
	if s.Sdk == scheduler.NGINX.Name {
		slog.Warn("space use nginx pre-define image", slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("scheduler.NGINX.Image", scheduler.NGINX.Image))
		// Use default image for nginx space
		containerImg = scheduler.NGINX.Image
	}
	slog.Info("run space with container image", slog.Any("namespace", namespace), slog.Any("name", name), slog.Any("containerImg", containerImg))

	// create deploy for space
	return c.deployer.Deploy(ctx, types.DeployRepo{
		SpaceID:    s.ID,
		Path:       s.Repository.Path,
		GitPath:    s.Repository.GitPath,
		GitBranch:  s.Repository.DefaultBranch,
		Sdk:        s.Sdk,
		SdkVersion: s.SdkVersion,
		Template:   s.Template,
		Env:        s.Env,
		Hardware:   s.Hardware,
		Secret:     s.Secrets,
		RepoID:     s.Repository.ID,
		ModelID:    0,
		UserID:     user.ID,
		Annotation: string(annoStr),
		ImageID:    containerImg,
		Type:       types.SpaceType,
		UserUUID:   user.UUID,
		SKU:        s.SKU,
	})
}

func (c *SpaceComponent) Wakeup(ctx context.Context, namespace, name string) error {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't wakeup space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}
	// get latest Deploy for space
	deploy, err := c.deploy.GetLatestDeployBySpaceID(ctx, s.ID)
	if err != nil {
		return fmt.Errorf("can't get space delopyment,%w", err)
	}
	return c.deployer.Wakeup(ctx, types.DeployRepo{
		SpaceID:   s.ID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
	})
}

func (c *SpaceComponent) Stop(ctx context.Context, namespace, name string) error {
	s, err := c.ss.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't stop space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}
	// get latest Deploy of space
	deploy, err := c.deploy.GetLatestDeployBySpaceID(ctx, s.ID)
	if err != nil {
		slog.Error("can't get space deployment", slog.Any("error", err), slog.Any("space id", s.ID))
		return fmt.Errorf("can't get space deployment,%w", err)
	}
	if deploy == nil {
		return fmt.Errorf("can't get space deployment")
	}

	err = c.deployer.Stop(ctx, types.DeployRepo{
		SpaceID:   s.ID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
	})
	if err != nil {
		return fmt.Errorf("can't stop space service deploy for service '%s', %w", deploy.SvcName, err)
	}

	err = c.deploy.StopDeploy(ctx, types.SpaceRepo, deploy.RepoID, deploy.UserID, deploy.ID)
	if err != nil {
		return fmt.Errorf("fail to update space deploy status to stopped for deploy ID '%d', %w", deploy.ID, err)
	}
	return nil
}

// FixHasEntryFile checks whether git repo has entry point file and update space's HasAppFile property in db
func (c *SpaceComponent) FixHasEntryFile(ctx context.Context, s *database.Space) *database.Space {
	hasAppFile := c.HasEntryFile(ctx, s)
	if s.HasAppFile != hasAppFile {
		s.HasAppFile = hasAppFile
		c.ss.Update(ctx, *s)
	}

	return s
}

func (c *SpaceComponent) status(ctx context.Context, s *database.Space) (string, string, error) {
	if !s.HasAppFile {
		if s.Sdk == scheduler.NGINX.Name {
			return "", SpaceStatusNoNGINXConf, nil
		}
		return "", SpaceStatusNoAppFile, nil
	}
	// get latest Deploy for space by space id
	deploy, err := c.deploy.GetLatestDeployBySpaceID(ctx, s.ID)
	if err != nil || deploy == nil {
		slog.Error("fail to get latest space deploy by space id", slog.Any("SpaceID", s.ID))
		return "", SpaceStatusStopped, fmt.Errorf("can't get space deployment,%w", err)
	}

	namespace, name := s.Repository.NamespaceAndName()
	// request space deploy status by deploy id
	srvName, code, _, err := c.deployer.Status(ctx, types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
	}, true)
	if err != nil {
		slog.Error("error happen when get space status", slog.Any("error", err), slog.String("path", s.Repository.Path))
		return "", SpaceStatusStopped, err
	}
	return srvName, deployStatusCodeToString(code), nil
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
	return c.deployer.Logs(ctx, types.DeployRepo{
		SpaceID:   s.ID,
		Namespace: namespace,
		Name:      name,
	})
}

// HasEntryFile checks whether space repo has entry point file to run with
func (c *SpaceComponent) HasEntryFile(ctx context.Context, space *database.Space) bool {
	namespace, name := space.Repository.NamespaceAndName()
	entryFile := "app.py"
	if space.Sdk == scheduler.NGINX.Name {
		entryFile = "nginx.conf"
	}

	return c.hasEntryFile(ctx, namespace, name, entryFile)
}

func (c *SpaceComponent) hasEntryFile(ctx context.Context, namespace, name, entryFile string) bool {
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
		if f.Type == "file" && f.Path == entryFile {
			return true
		}
	}

	return false
}

func (c *SpaceComponent) mergeUpdateSpaceRequest(ctx context.Context, space *database.Space, req *types.UpdateSpaceReq) error {
	// Do not update column value if request body do not have it
	if req.Sdk != nil {
		space.Sdk = *req.Sdk
	}
	if req.SdkVersion != nil {
		space.SdkVersion = *req.SdkVersion
	}
	if req.Env != nil {
		space.Env = *req.Env
	}
	if req.Secrets != nil {
		space.Secrets = *req.Secrets
	}
	if req.Template != nil {
		space.Template = *req.Template
	}
	if req.CoverImageUrl != nil {
		space.CoverImageUrl = *req.CoverImageUrl
	}

	if req.ResourceID != nil {
		resource, err := c.srs.FindByID(ctx, *req.ResourceID)
		if err != nil {
			return fmt.Errorf("can't find space resource by id, resource id:%d, error:%w", *req.ResourceID, err)
		}
		space.Hardware = resource.Resources
		space.SKU = strconv.FormatInt(resource.ID, 10)
	}

	return nil
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

	SpaceStatusNoAppFile   = "NoAppFile"
	RepoStatusDeleted      = "Deleted"
	SpaceStatusNoNGINXConf = "NoNGINXConf"
)
