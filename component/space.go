package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
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

type SpaceComponent interface {
	Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error)
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool) (*types.Space, error)
	Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error)
	Index(ctx context.Context, repoFilter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Space, int, error)
	OrgSpaces(ctx context.Context, req *types.OrgSpacesReq) ([]types.Space, int, error)
	// UserSpaces get spaces of owner and visible to current user
	UserSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error)
	UserLikesSpaces(ctx context.Context, req *types.UserCollectionReq, userID int64) ([]types.Space, int, error)
	ListByPath(ctx context.Context, paths []string) ([]*types.Space, error)
	AllowCallApi(ctx context.Context, spaceID int64, username string) (bool, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Deploy(ctx context.Context, namespace, name, currentUser string) (int64, error)
	Wakeup(ctx context.Context, namespace, name string) error
	Stop(ctx context.Context, namespace, name string, deleteSpace bool) error
	// FixHasEntryFile checks whether git repo has entry point file and update space's HasAppFile property in db
	FixHasEntryFile(ctx context.Context, s *database.Space) *database.Space
	Status(ctx context.Context, namespace, name string) (string, string, error)
	Logs(ctx context.Context, namespace, name string) (*deploy.MultiLogReader, error)
	// HasEntryFile checks whether space repo has entry point file to run with
	HasEntryFile(ctx context.Context, space *database.Space) bool
	GetByID(ctx context.Context, spaceID int64) (*database.Space, error)
	MCPIndex(ctx context.Context, repoFilter *types.RepoFilter, per, page int) ([]*types.MCPService, int, error)
}

func (c *spaceComponentImpl) Create(ctx context.Context, req types.CreateSpaceReq) (*types.Space, error) {
	var nickname string
	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	req.Nickname = nickname
	req.RepoType = types.SpaceRepo
	req.Readme = generateReadmeData(req.License)
	resource, err := c.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("fail to find resource by id, %w", err)
	}
	err = c.checkResourcePurchasableForCreate(ctx, req, resource)
	if err != nil {
		return nil, err
	}
	var hardware types.HardWare
	err = json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return nil, fmt.Errorf("invalid hardware setting, %w", err)
	}
	_, err = c.checkResourceAvailable(ctx, req, hardware)
	if err != nil {
		return nil, fmt.Errorf("fail to check resource, %w", err)
	}

	var templatePath string
	if req.Sdk == types.DOCKER.Name {
		templatePath, err = c.getSpaceDockerTemplatePath(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("fail to get space docker template path, %w", err)
		}
	}

	_, dbRepo, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbSpace := database.Space{
		RepositoryID:  dbRepo.ID,
		Sdk:           req.Sdk,
		SdkVersion:    req.SdkVersion,
		DriverVersion: req.DriverVersion,
		CoverImageUrl: req.CoverImageUrl,
		Env:           req.Env,
		Hardware:      resource.Resources,
		Secrets:       req.Secrets,
		SKU:           strconv.FormatInt(resource.ID, 10),
		Variables:     req.Variables,
		Template:      req.Template,
		ClusterID:     req.ClusterID,
	}
	dbSpace = c.updateSpaceByReq(dbSpace, req)

	resSpace, err := c.spaceStore.Create(ctx, dbSpace)
	if err != nil {
		slog.Error("fail to create space in db", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to create space in db, error: %w", err)
	}

	err = c.createSpaceDefaultFiles(dbRepo, req, templatePath)
	if err != nil {
		slog.Error("fail to create space default files", slog.Any("req", req), slog.Any("error", err))
		return nil, fmt.Errorf("fail to create space default files, error: %w", err)
	}

	space := &types.Space{
		Creator:       req.Username,
		License:       req.License,
		Path:          dbRepo.Path,
		Name:          req.Name,
		Sdk:           req.Sdk,
		SdkVersion:    req.SdkVersion,
		DriverVersion: req.DriverVersion,
		Template:      resSpace.Template,
		Env:           req.Env,
		Hardware:      resource.Resources,
		Secrets:       req.Secrets,
		Variables:     resSpace.Variables,
		CoverImageUrl: resSpace.CoverImageUrl,
		Endpoint:      "",
		Status:        "",
		Private:       req.Private,
		CreatedAt:     resSpace.CreatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SpaceRepo,
			RepoPath:  dbRepo.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return space, nil
}

func (c *spaceComponentImpl) getSpaceDockerTemplatePath(ctx context.Context, req types.CreateSpaceReq) (string, error) {
	// check docker template
	if len(req.Template) < 1 {
		return "", fmt.Errorf("template must be specified when creating a docker space")
	}
	template, err := c.templateStore.FindByName(ctx, req.Sdk, req.Template)
	if err != nil {
		return "", fmt.Errorf("get %s template by name %s error: %w", req.Sdk, req.Template, err)
	}
	if len(template.Path) < 1 {
		return "", fmt.Errorf("invalid docker template path error: %w", err)
	}
	templatePath, err := getSpaceTemplatePath(template.Path)
	if err != nil {
		return "", fmt.Errorf("check docker template path %s error: %w", templatePath, err)
	}
	return templatePath, nil
}

func (c *spaceComponentImpl) createSpaceDefaultFiles(dbRepo *database.Repository, req types.CreateSpaceReq, templatePath string) error {
	err := c.createSpaceReadmeFile(dbRepo, req)
	if err != nil {
		return fmt.Errorf("failed to create space readme file, cause: %w", err)
	}

	err = c.createSpaceGitAttibuteFile(dbRepo, req)
	if err != nil {
		return fmt.Errorf("failed to create space gitattibute file, cause: %w", err)
	}

	if req.Sdk == types.STREAMLIT.Name {
		err = c.createSpaceStreamlitFile(dbRepo, req)
		if err != nil {
			return fmt.Errorf("failed to create space streamlit file, cause: %w", err)
		}
	}

	if req.Sdk == types.DOCKER.Name && len(templatePath) > 0 {
		err = c.createSpaceDockerTemplateFile(dbRepo, req, templatePath)
		if err != nil {
			return fmt.Errorf("failed to create space docker template file, cause: %w", err)
		}
	}

	if req.Sdk == types.MCPSERVER.Name {
		err = c.createSpaceMCPServerTemplateFile(dbRepo, req)
		if err != nil {
			return fmt.Errorf("failed to create space mcp server file, cause: %w", err)
		}
	}

	if req.Sdk == types.NGINX.Name {
		err = c.createSpaceNginxTemplateFile(dbRepo, req)
		if err != nil {
			return fmt.Errorf("failed to create space nginx template file, cause: %w", err)
		}
	}
	return nil
}

func (c *spaceComponentImpl) createSpaceReadmeFile(dbRepo *database.Repository, req types.CreateSpaceReq) error {
	// Create README.md file
	err := c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.ReadmeFileName,
	}, types.SpaceRepo))
	if err != nil {
		return fmt.Errorf("failed to create %s file, cause: %w", types.ReadmeFileName, err)
	}
	return nil
}

func (c *spaceComponentImpl) createSpaceGitAttibuteFile(dbRepo *database.Repository, req types.CreateSpaceReq) error {
	// Create .gitattributes file
	err := c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   spaceGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.GitattributesFileName,
	}, types.SpaceRepo))
	if err != nil {
		return fmt.Errorf("failed to create %s file, cause: %w", types.GitattributesFileName, err)
	}
	return nil
}

func (c *spaceComponentImpl) createSpaceStreamlitFile(dbRepo *database.Repository, req types.CreateSpaceReq) error {
	// create .streamlit/config.toml for support cors
	fileReq := types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   streamlitConfigContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  streamlitConfig,
	}
	err := c.git.CreateRepoFile(buildCreateFileReq(&fileReq, types.SpaceRepo))
	if err != nil {
		return fmt.Errorf("failed to create %s file for streamlit space, cause: %w", streamlitConfig, err)
	}
	return nil
}

func (c *spaceComponentImpl) createSpaceDockerTemplateFile(dbRepo *database.Repository, req types.CreateSpaceReq, templatePath string) error {
	// create docker template files
	entries, err := os.ReadDir(templatePath)
	if err != nil {
		return fmt.Errorf("failed to list dir %s error: %w", templatePath, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		nameI := entries[i].Name()
		nameJ := entries[j].Name()
		if nameI == types.EntryFileDockerfile {
			return false
		}
		if nameJ == types.EntryFileDockerfile {
			return true
		}
		return nameI < nameJ
	})

	err = c.uploadTemplateFiles(entries, req, dbRepo, templatePath)
	if err != nil {
		return fmt.Errorf("fail to upload space docker template files error: %w", err)
	}

	return nil
}

func (c *spaceComponentImpl) createSpaceMCPServerTemplateFile(dbRepo *database.Repository, req types.CreateSpaceReq) error {
	// create mcp server template files
	templatePath, err := getSpaceTemplatePath(req.Sdk)
	if err != nil {
		return fmt.Errorf("check mcp server template path %s error: %w", templatePath, err)
	}
	entries, err := os.ReadDir(templatePath)
	if err != nil {
		return fmt.Errorf("failed to list dir %s error: %w", templatePath, err)
	}

	sort.Slice(entries, func(i, j int) bool {
		nameI := entries[i].Name()
		nameJ := entries[j].Name()
		if nameI == types.EntryFileAppFile {
			return false
		}
		if nameJ == types.EntryFileAppFile {
			return true
		}
		return nameI < nameJ
	})

	err = c.uploadTemplateFiles(entries, req, dbRepo, templatePath)
	if err != nil {
		return fmt.Errorf("fail to upload space mcp server template files error: %w", err)
	}

	return nil
}

func (c *spaceComponentImpl) createSpaceNginxTemplateFile(dbRepo *database.Repository, req types.CreateSpaceReq) error {
	templatePath, err := getSpaceTemplatePath(req.Sdk)
	if err != nil {
		return fmt.Errorf("check space nginx template path %s error: %w", templatePath, err)
	}
	entries, err := os.ReadDir(templatePath)
	if err != nil {
		return fmt.Errorf("failed to list dir %s error: %w", templatePath, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		nameI := entries[i].Name()
		nameJ := entries[j].Name()
		if nameI == types.EntryFileNginx {
			return false
		}
		if nameJ == types.EntryFileNginx {
			return true
		}
		return nameI < nameJ
	})
	err = c.uploadTemplateFiles(entries, req, dbRepo, templatePath)
	if err != nil {
		return fmt.Errorf("failed to upload space nginx template files error: %w", err)
	}
	return nil
}

func getSpaceTemplatePath(subPath string) (string, error) {
	currentDir, err := filepath.Abs(filepath.Dir("."))
	if err != nil {
		return "", fmt.Errorf("getting current directory error: %w", err)
	}
	templatePath := filepath.Join(currentDir, "docker", "spaces", "templates", subPath)
	_, err = os.Stat(templatePath)
	if err != nil {
		return "", fmt.Errorf("get template path %s error: %w", templatePath, err)
	}
	return templatePath, nil
}

func (c *spaceComponentImpl) uploadTemplateFiles(entries []os.DirEntry, req types.CreateSpaceReq, dbRepo *database.Repository, templatePath string) error {
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		fileName := entry.Name()
		slog.Debug("get file", slog.Any("sdk", req.Sdk), slog.Any("template", req.Template), slog.Any("templatePath", templatePath), slog.Any("fileName", fileName))

		content, err := os.ReadFile(filepath.Join(templatePath, fileName))
		if err != nil {
			return fmt.Errorf("failed to read %s/%s file for %s space, cause: %w", templatePath, fileName, req.Sdk, err)
		}

		fileReq := types.CreateFileParams{
			Username:  dbRepo.User.Username,
			Email:     dbRepo.User.Email,
			Message:   types.InitCommitMessage,
			Branch:    req.DefaultBranch,
			Content:   string(content),
			NewBranch: req.DefaultBranch,
			Namespace: req.Namespace,
			Name:      req.Name,
			FilePath:  fileName,
		}
		err = c.git.CreateRepoFile(buildCreateFileReq(&fileReq, types.SpaceRepo))
		if err != nil {
			return fmt.Errorf("failed to create %s file for %s space, cause: %w", fileName, req.Sdk, err)
		}
	}
	return nil
}

func (c *spaceComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool) (*types.Space, error) {
	var tags []types.RepoTag
	space, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find space %s/%s, error: %w", namespace, name, err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, space.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s repo permission, error: %w", currentUser, err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrUnauthorized
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace %s info for space, error: %w", namespace, err)
	}

	spaceStatus, _ := c.status(ctx, space)
	endpoint := c.getEndpoint(spaceStatus.SvcName, space)

	req := types.DeployRepo{
		DeployID:  spaceStatus.DeployID,
		SpaceID:   space.ID,
		Namespace: namespace,
		Name:      name,
		SvcName:   spaceStatus.SvcName,
		ClusterID: spaceStatus.ClusterID,
	}
	_, _, instList, err := c.deployer.GetReplica(ctx, req)
	if err != nil {
		slog.Warn("no space deployment replica found", slog.Any("req", req), slog.Any("err", err))
	}

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, space.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}
	repository := common.BuildCloneInfo(c.config, space.Repository)

	resSpace := &types.Space{
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
		Status:        spaceStatus.Status,
		Endpoint:      endpoint,
		Hardware:      space.Hardware,
		RepositoryID:  space.Repository.ID,
		UserLikes:     likeExists,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		Variables:     space.Variables,
		CoverImageUrl: space.CoverImageUrl,
		Source:        space.Repository.Source,
		SyncStatus:    space.Repository.SyncStatus,
		SKU:           space.SKU,
		SvcName:       spaceStatus.SvcName,
		CanWrite:      permission.CanWrite,
		CanManage:     permission.CanAdmin,
		Namespace:     ns,
		DeployID:      spaceStatus.DeployID,
		Instances:     instList,
		ClusterID:     space.ClusterID,
	}
	if permission.CanAdmin {
		resSpace.SensitiveCheckStatus = space.Repository.SensitiveCheckStatus.String()
	}
	if permission.CanWrite {
		resSpace.Env = space.Env
		resSpace.Secrets = space.Secrets
	}
	if needOpWeight {
		c.addOpWeightToSpaces(ctx, []int64{resSpace.RepositoryID}, []*types.Space{resSpace})
	}

	return resSpace, nil
}

func (c *spaceComponentImpl) Update(ctx context.Context, req *types.UpdateSpaceReq) (*types.Space, error) {
	req.RepoType = types.SpaceRepo
	if req.ResourceID != nil {
		resource, err := c.spaceResourceStore.FindByID(ctx, *req.ResourceID)
		if err != nil {
			return nil, fmt.Errorf("fail to find resource by id, %w", err)
		}

		err = c.checkResourcePurchasableForUpdate(ctx, *req, resource)
		if err != nil {
			return nil, err
		}
	}
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	space, err := c.spaceStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find space, error: %w", err)
	}
	// don't support switch reserved resource
	if c.resourceReserved(space, req) {
		return nil, fmt.Errorf("don't support switch reserved resource so far")
	}
	err = c.mergeUpdateSpaceRequest(ctx, space, req)
	if err != nil {
		return nil, fmt.Errorf("failed to merge update space request, error: %w", err)
	}

	err = c.spaceStore.Update(ctx, *space)
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
		Variables:     space.Variables,
		CoverImageUrl: space.CoverImageUrl,
		License:       dbRepo.License,
		Private:       dbRepo.Private,
		CreatedAt:     dbRepo.CreatedAt,
		SKU:           space.SKU,
	}

	return resDataset, nil
}

func (c *spaceComponentImpl) Index(ctx context.Context, repoFilter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Space, int, error) {
	var (
		resSpaces []*types.Space
		err       error
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.SpaceRepo, repoFilter.Username, repoFilter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public space repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	spaces, err := c.spaceStore.ByRepoIDs(ctx, repoIDs)
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
		spaceStatus, _ := c.status(ctx, space)
		var tags []types.RepoTag
		for _, tag := range space.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		resSpaces = append(resSpaces, &types.Space{
			Name:          space.Repository.Name,
			Description:   space.Repository.Description,
			Path:          space.Repository.Path,
			Sdk:           space.Sdk,
			SdkVersion:    space.SdkVersion,
			Template:      space.Template,
			Hardware:      space.Hardware,
			Secrets:       space.Secrets,
			CoverImageUrl: space.CoverImageUrl,
			License:       space.Repository.License,
			Private:       space.Repository.Private,
			Likes:         space.Repository.Likes,
			CreatedAt:     space.Repository.CreatedAt,
			UpdatedAt:     space.Repository.UpdatedAt,
			Tags:          tags,
			Status:        spaceStatus.Status,
			RepositoryID:  space.Repository.ID,
			Source:        repo.Source,
			SyncStatus:    repo.SyncStatus,
			User: &types.User{
				Nickname: space.Repository.User.NickName,
				Avatar:   space.Repository.User.Avatar,
			},
		})
	}
	return resSpaces, total, nil
}

func (c *spaceComponentImpl) OrgSpaces(ctx context.Context, req *types.OrgSpacesReq) ([]types.Space, int, error) {
	var resSpaces []types.Space
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.Error("faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	spaces, total, err := c.spaceStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org spaces,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range spaces {
		spaceStatus, _ := c.status(ctx, &data)
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
			RepositoryID:  data.Repository.ID,
			CoverImageUrl: data.CoverImageUrl,
			Status:        spaceStatus.Status,
		})
	}

	return resSpaces, total, nil
}

// UserSpaces get spaces of owner and visible to current user
func (c *spaceComponentImpl) UserSpaces(ctx context.Context, req *types.UserSpacesReq) ([]types.Space, int, error) {
	onlyPublic := req.Owner != req.CurrentUser
	ms, total, err := c.spaceStore.ByUsername(ctx, req, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get spaces by username,%w", err)
		return nil, 0, newError
	}

	var resSpaces []types.Space
	for _, data := range ms {
		spaceStatus, _ := c.status(ctx, &data)
		endpoint := c.getEndpoint(spaceStatus.SvcName, &data)
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
			Status:        spaceStatus.Status,
			CoverImageUrl: data.CoverImageUrl,
			Sdk:           data.Sdk,
			Endpoint:      endpoint,
		})
	}

	return resSpaces, total, nil
}

func (c *spaceComponentImpl) UserLikesSpaces(ctx context.Context, req *types.UserCollectionReq, userID int64) ([]types.Space, int, error) {
	ms, total, err := c.spaceStore.ByUserLikes(ctx, userID, req.PageSize, req.Page)
	if err != nil {
		newError := fmt.Errorf("failed to get spaces by username,%w", err)
		return nil, 0, newError
	}

	var resSpaces []types.Space
	for _, data := range ms {
		spaceStatus, _ := c.status(ctx, &data)
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
			Status:        spaceStatus.Status,
			CoverImageUrl: data.CoverImageUrl,
		})
	}

	return resSpaces, total, nil
}

func (c *spaceComponentImpl) ListByPath(ctx context.Context, paths []string) ([]*types.Space, error) {
	var spaces []*types.Space

	spacesData, err := c.spaceStore.ListByPath(ctx, paths)
	if err != nil {
		return nil, fmt.Errorf("list space db failed, %w", err)
	}
	for _, data := range spacesData {
		spaceStatus, _ := c.status(ctx, &data)
		var tags []types.RepoTag
		for _, tag := range data.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
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
			Status:        spaceStatus.Status,
			RepositoryID:  data.Repository.ID,
		})
	}
	return spaces, nil
}

func (c *spaceComponentImpl) AllowCallApi(ctx context.Context, spaceID int64, username string) (bool, error) {
	if username == "" {
		return false, errorx.ErrUserNotFound
	}
	s, err := c.spaceStore.ByID(ctx, spaceID)
	if err != nil {
		return false, fmt.Errorf("failed to get space by id:%d, %w", spaceID, err)
	}
	fields := strings.Split(s.Repository.Path, "/")
	return c.repoComponent.AllowReadAccess(ctx, s.Repository.RepositoryType, fields[0], fields[1], username)
}

func (c *spaceComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	space, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find space, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.SpaceRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of space, error: %w", err)
	}

	err = c.spaceStore.Delete(ctx, *space)
	if err != nil {
		return fmt.Errorf("failed to delete database space, error: %w", err)
	}

	// stop any running space instance
	go func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := c.stopSpaceDeploy(cleanCtx, namespace, name, space)
		if err != nil {
			slog.Error("stop space failed", slog.Any("error", err))
		}
	}()

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SpaceRepo,
			RepoPath:  repo.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *spaceComponentImpl) Deploy(ctx context.Context, namespace, name, currentUser string) (int64, error) {
	space, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't find space to deploy", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return -1, err
	}
	if !space.HasAppFile {
		return -1, errorx.NoEntryFile(errors.New("no app file"),
			errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", namespace, name)),
		)
	}

	// found user id
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		slog.Error("can't find user for deploy space", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	resID, err := strconv.Atoi(space.SKU)
	if err != nil {
		return -1, fmt.Errorf("invalid space %s/%s resource id %s, error: %w", namespace, name, space.SKU, err)
	}

	_, err = c.spaceResourceStore.FindByID(ctx, int64(resID))
	if err != nil {
		return -1, fmt.Errorf("fail to find resource by id %d, error: %w", resID, err)
	}

	// put repo-type and namespace/name in annotation
	annotations := make(map[string]string)
	annotations[types.ResTypeKey] = string(types.SpaceRepo)
	annotations[types.ResNameKey] = fmt.Sprintf("%s/%s", namespace, name)
	annotations[types.ResDeployUser] = user.Username
	annoStr, err := json.Marshal(annotations)
	if err != nil {
		slog.Error("fail to create annotations for deploy space", slog.Any("error", err), slog.String("username", currentUser))
		return -1, err
	}

	containerPort := types.DefaultContainerPort
	if space.Sdk == types.GRADIO.Name {
		containerPort = types.GRADIO.Port
	} else if space.Sdk == types.STREAMLIT.Name {
		containerPort = types.STREAMLIT.Port
	} else if space.Sdk == types.NGINX.Name {
		containerPort = types.NGINX.Port
	} else if space.Sdk == types.DOCKER.Name {
		template, err := c.templateStore.FindByName(ctx, types.DOCKER.Name, space.Template)
		if err != nil {
			return -1, fmt.Errorf("fail to query %s template %s error: %w", types.DOCKER.Name, space.Template, err)
		}
		containerPort = template.Port
	} else if space.Sdk == types.MCPSERVER.Name {
		containerPort = types.MCPSERVER.Port
	}

	// create deploy for space
	dr := types.DeployRepo{
		SpaceID:       space.ID,
		Path:          space.Repository.Path,
		GitPath:       space.Repository.GitPath,
		GitBranch:     space.Repository.DefaultBranch,
		Sdk:           space.Sdk,
		SdkVersion:    space.SdkVersion,
		Template:      space.Template,
		Env:           space.Env,
		Hardware:      space.Hardware,
		Secret:        space.Secrets,
		RepoID:        space.Repository.ID,
		ModelID:       0,
		UserID:        user.ID,
		Annotation:    string(annoStr),
		ImageID:       "",
		Type:          types.SpaceType,
		UserUUID:      user.UUID,
		SKU:           space.SKU,
		ContainerPort: containerPort,
		Variables:     space.Variables,
		ClusterID:     space.ClusterID,
	}
	dr = c.updateDeployRepoBySpace(dr, space)
	return c.deployer.Deploy(ctx, dr)
}

func (c *spaceComponentImpl) Wakeup(ctx context.Context, namespace, name string) error {
	s, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't wakeup space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}
	if !s.HasAppFile {
		return errorx.NoEntryFile(errors.New("no app file"),
			errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", namespace, name)),
		)
	}
	// get latest Deploy for space
	deploy, err := c.deployTaskStore.GetLatestDeployBySpaceID(ctx, s.ID)
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

func (c *spaceComponentImpl) Stop(ctx context.Context, namespace, name string, deleteSpace bool) error {
	s, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		slog.Error("can't stop space", slog.Any("error", err), slog.String("namespace", namespace), slog.String("name", name))
		return err
	}
	if !s.HasAppFile {
		return errorx.NoEntryFile(errors.New("no app file"),
			errorx.Ctx().
				Set("path", fmt.Sprintf("%s/%s", namespace, name)),
		)
	}

	err = c.stopSpaceDeploy(ctx, namespace, name, s)
	if err != nil {
		return fmt.Errorf("fail stop space %s/%s deploy error: %w", namespace, name, err)
	}
	return nil
}

func (c *spaceComponentImpl) stopSpaceDeploy(ctx context.Context, namespace, name string, s *database.Space) error {
	// get latest Deploy of space
	deploy, err := c.deployTaskStore.GetLatestDeployBySpaceID(ctx, s.ID)
	if err != nil {
		slog.Error("can't get space deployment", slog.Any("error", err), slog.Any("space id", s.ID))
		return fmt.Errorf("can't get space deployment,%w", err)
	}
	if deploy == nil {
		return fmt.Errorf("can't get space deployment")
	}

	dr := types.DeployRepo{
		SpaceID:   s.ID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
	}
	dr = c.updateDeployRepoByDeploy(dr, deploy)
	err = c.deployer.Stop(ctx, dr)
	if err != nil {
		return fmt.Errorf("can't stop space service deploy for service '%s', %w", deploy.SvcName, err)
	}

	err = c.deployTaskStore.StopDeploy(ctx, types.SpaceRepo, deploy.RepoID, deploy.UserID, deploy.ID)
	if err != nil {
		return fmt.Errorf("fail to update space deploy status to stopped for deploy ID '%d', %w", deploy.ID, err)
	}
	return nil
}

// FixHasEntryFile checks whether git repo has entry point file and update space's HasAppFile property in db
func (c *spaceComponentImpl) FixHasEntryFile(ctx context.Context, s *database.Space) *database.Space {
	hasAppFile := c.HasEntryFile(ctx, s)
	if s.HasAppFile != hasAppFile {
		s.HasAppFile = hasAppFile
		_ = c.spaceStore.Update(ctx, *s)
	}

	return s
}

func (c *spaceComponentImpl) status(ctx context.Context, s *database.Space) (types.SpaceStatus, error) {
	if !s.HasAppFile {
		if s.Sdk == types.NGINX.Name {
			return types.SpaceStatus{
				Status: SpaceStatusNoNGINXConf,
			}, nil
		}
		return types.SpaceStatus{
			Status: SpaceStatusNoAppFile,
		}, nil
	}
	// get latest Deploy for space by space id
	deploy, err := c.deployTaskStore.GetLatestDeployBySpaceID(ctx, s.ID)
	if err != nil || deploy == nil {
		return types.SpaceStatus{
			Status: SpaceStatusStopped,
		}, fmt.Errorf("failed to get latest space deploy by space id %d, error: %w", s.ID, err)
	}
	slog.Debug("space deploy", slog.Any("deploy", deploy))
	return types.SpaceStatus{
		SvcName:   deploy.SvcName,
		Status:    deployStatusCodeToString(deploy.Status),
		DeployID:  deploy.ID,
		ClusterID: deploy.ClusterID,
	}, nil
}

func (c *spaceComponentImpl) Status(ctx context.Context, namespace, name string) (string, string, error) {
	s, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return "", SpaceStatusStopped, fmt.Errorf("can't find space by path status, error: %w", err)
	}
	spaceStatus, err := c.status(ctx, s)
	return spaceStatus.SvcName, spaceStatus.Status, err
}

func (c *spaceComponentImpl) Logs(ctx context.Context, namespace, name string) (*deploy.MultiLogReader, error) {
	s, err := c.spaceStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("can't find space for logs, error: %w", err)
	}
	return c.deployer.Logs(ctx, types.DeployRepo{
		SpaceID:   s.ID,
		Namespace: namespace,
		Name:      name,
	})
}

// HasEntryFile checks whether space repo has entry point file to run with
func (c *spaceComponentImpl) HasEntryFile(ctx context.Context, space *database.Space) bool {
	namespace, name := space.Repository.NamespaceAndName()
	entryFile := types.EntryFileAppFile
	if space.Sdk == types.NGINX.Name {
		entryFile = types.EntryFileNginx
	} else if space.Sdk == types.DOCKER.Name {
		entryFile = types.EntryFileDockerfile
	}

	return c.hasEntryFile(ctx, namespace, name, entryFile)
}

func (c *spaceComponentImpl) hasEntryFile(ctx context.Context, namespace, name, entryFile string) bool {
	var req gitserver.GetRepoInfoByPathReq
	req.Namespace = namespace
	req.Name = name
	// root dir
	req.Path = ""
	req.RepoType = types.SpaceRepo
	files, err := c.git.GetRepoFileTree(ctx, req)
	if err != nil {
		slog.Error("check repo entry file existence failed", slog.Any("entryFile", entryFile), slog.Any("eror", err))
		return false
	}

	for _, f := range files {
		if f.Type == "file" && f.Path == entryFile {
			return true
		}
	}

	return false
}

func (c *spaceComponentImpl) mergeUpdateSpaceRequest(ctx context.Context, space *database.Space, req *types.UpdateSpaceReq) error {
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
		resource, err := c.spaceResourceStore.FindByID(ctx, *req.ResourceID)
		if err != nil {
			return fmt.Errorf("can't find space resource by id, resource id:%d, error:%w", *req.ResourceID, err)
		}
		space.Hardware = resource.Resources
		space.SKU = strconv.FormatInt(resource.ID, 10)
	}

	if req.Variables != nil {
		space.Variables = *req.Variables
	}

	if req.ClusterID != nil {
		space.ClusterID = *req.ClusterID
	}
	return nil
}

func (c *spaceComponentImpl) GetByID(ctx context.Context, spaceID int64) (*database.Space, error) {
	return c.spaceStore.ByID(ctx, spaceID)
}

func (c *spaceComponentImpl) MCPIndex(ctx context.Context, repoFilter *types.RepoFilter, per, page int) ([]*types.MCPService, int, error) {
	var (
		resSpaces []*types.MCPService
		err       error
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.SpaceRepo, repoFilter.Username, repoFilter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public space repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	spaces, err := c.spaceStore.ByRepoIDs(ctx, repoIDs)
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

		spaceStatus, _ := c.status(ctx, space)
		endpoint := c.getEndpoint(spaceStatus.SvcName, space)

		resSpaces = append(resSpaces, &types.MCPService{
			ID:           space.ID,
			Name:         space.Repository.Name,
			Description:  space.Repository.Description,
			Path:         space.Repository.Path,
			Env:          space.Env,
			License:      space.Repository.License,
			Private:      space.Repository.Private,
			CreatedAt:    space.Repository.CreatedAt,
			UpdatedAt:    space.Repository.UpdatedAt,
			Status:       spaceStatus.Status,
			RepositoryID: space.Repository.ID,
			SvcName:      spaceStatus.SvcName,
			Endpoint:     endpoint,
		})
	}
	return resSpaces, total, nil
}

func (c *spaceComponentImpl) getEndpoint(svcName string, space *database.Space) string {
	endpoint := ""
	if len(svcName) < 1 {
		return endpoint
	}

	if c.publicRootDomain == "" {
		if space.Sdk == types.STREAMLIT.Name || space.Sdk == types.GRADIO.Name {
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

	return endpoint
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
