package component

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type MCPServerComponent interface {
	Create(ctx context.Context, req *types.CreateMCPServerReq) (*types.MCPServer, error)
	Delete(ctx context.Context, req *types.UpdateMCPServerReq) error
	Update(ctx context.Context, req *types.UpdateMCPServerReq) (*types.MCPServer, error)
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.MCPServer, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.MCPServer, int, error)
	Properties(ctx context.Context, req *types.MCPPropertyFilter) ([]types.MCPServerProperties, int, error)
	OrgMCPServers(ctx context.Context, req *types.OrgMCPsReq) ([]types.MCPServer, int, error)
	Deploy(ctx context.Context, req *types.DeployMCPServerReq) (*types.Space, error)
}

type mcpServerComponentImpl struct {
	config             *config.Config
	repoComponent      RepoComponent
	repoStore          database.RepoStore
	gitServer          gitserver.GitServer
	userSvcClient      rpc.UserSvcClient
	mcpServerStore     database.MCPServerStore
	userLikesStore     database.UserLikesStore
	recomStore         database.RecomStore
	spaceStore         database.SpaceStore
	spaceResourceStore database.SpaceResourceStore
	tokenStore         database.AccessTokenStore
	namespaceStore     database.NamespaceStore
}

func NewMCPServerComponent(config *config.Config) (MCPServerComponent, error) {
	var err error
	m := &mcpServerComponentImpl{}
	m.config = config
	m.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component for mcp, error: %w", err)
	}
	m.repoStore = database.NewRepoStore()
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server for mcp, error: %w", err)
	}
	m.gitServer = gs
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	m.userSvcClient = rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	m.mcpServerStore = database.NewMCPServerStore()
	m.userLikesStore = database.NewUserLikesStore()
	m.recomStore = database.NewRecomStore()
	m.spaceStore = database.NewSpaceStore()
	m.spaceResourceStore = database.NewSpaceResourceStore()
	m.tokenStore = database.NewAccessTokenStore()
	m.namespaceStore = database.NewNamespaceStore()
	return m, nil
}

func (m *mcpServerComponentImpl) Create(ctx context.Context, req *types.CreateMCPServerReq) (*types.MCPServer, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	req.RepoType = types.MCPServerRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname
	_, dbRepo, err := m.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, fmt.Errorf("fail to create mcp repo cause: %w", err)
	}

	input := database.MCPServer{
		RepositoryID:  dbRepo.ID,
		Configuration: req.Configuration,
		Repository:    dbRepo,
	}

	mcpServer, err := m.mcpServerStore.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("fail to create mcp server cause: %w", err)
	}

	err = m.createMCPServerRepoFiles(req, dbRepo)
	if err != nil {
		return nil, fmt.Errorf("fail to create mcp server repo files cause: %w", err)
	}

	for _, tag := range mcpServer.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	res := &types.MCPServer{
		ID:           mcpServer.ID,
		Name:         mcpServer.Repository.Name,
		Nickname:     mcpServer.Repository.Nickname,
		Description:  mcpServer.Repository.Description,
		Likes:        mcpServer.Repository.Likes,
		Downloads:    mcpServer.Repository.DownloadCount,
		Path:         mcpServer.Repository.Path,
		RepositoryID: mcpServer.RepositoryID,
		Repository:   common.BuildCloneInfo(m.config, mcpServer.Repository),
		Private:      mcpServer.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: mcpServer.CreatedAt,
		UpdatedAt: mcpServer.UpdatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.MCPServerRepo,
			RepoPath:  mcpServer.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = m.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return res, nil
}

func (m *mcpServerComponentImpl) createMCPServerRepoFiles(req *types.CreateMCPServerReq, dbRepo *database.Repository) error {
	// Create README.md file
	err := m.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.ReadmeFileName,
	}, types.MCPServerRepo))
	if err != nil {
		return fmt.Errorf("failed to create mcp server repo README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	// err = m.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
	// 	Username:  dbRepo.User.Username,
	// 	Email:     dbRepo.User.Email,
	// 	Message:   types.InitCommitMessage,
	// 	Branch:    req.DefaultBranch,
	// 	Content:   codeGitattributesContent,
	// 	NewBranch: req.DefaultBranch,
	// 	Namespace: req.Namespace,
	// 	Name:      req.Name,
	// 	FilePath:  types.GitattributesFileName,
	// }, types.MCPServerRepo))
	// if err != nil {
	// 	return fmt.Errorf("failed to create mcp server repo .gitattributes file, cause: %w", err)
	// }
	return nil
}

func (m *mcpServerComponentImpl) Delete(ctx context.Context, req *types.UpdateMCPServerReq) error {
	mcpServer, err := m.mcpServerStore.ByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find mcp server %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	permission, err := m.repoComponent.GetUserRepoPermission(ctx, req.Username, mcpServer.Repository)
	if err != nil {
		return fmt.Errorf("failed to get user %s permission for repo %s/%s, error: %w", req.Username, req.Namespace, req.Name, err)
	}

	if !permission.CanAdmin {
		return errorx.ErrForbidden
	}

	deleteRepoReq := types.DeleteRepoReq{
		Username:  req.Username,
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  types.MCPServerRepo,
	}
	repo, err := m.repoComponent.DeleteRepo(ctx, deleteRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of mcp server %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	err = m.mcpServerStore.Delete(ctx, *mcpServer)
	if err != nil {
		return fmt.Errorf("failed to delete mcp server, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.MCPServerRepo,
			RepoPath:  repo.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = m.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil

}

func (m *mcpServerComponentImpl) Update(ctx context.Context, req *types.UpdateMCPServerReq) (*types.MCPServer, error) {
	mcpServer, err := m.mcpServerStore.ByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find mcp server %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	permission, err := m.repoComponent.GetUserRepoPermission(ctx, req.Username, mcpServer.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s permission for repo %s/%s, error: %w", req.Namespace, req.Namespace, req.Name, err)
	}
	if !permission.CanAdmin {
		return nil, errorx.ErrForbidden
	}

	req.RepoType = types.MCPServerRepo
	dbRepo, err := m.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update mcp server repo %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	mcpServer = m.updateMCPServerInfo(mcpServer, req)

	res, err := m.mcpServerStore.Update(ctx, *mcpServer)
	if err != nil {
		return nil, fmt.Errorf("failed to update mcp server by , error: %w", err)
	}

	resCode := &types.MCPServer{
		ID:              res.ID,
		Name:            dbRepo.Name,
		Nickname:        dbRepo.Nickname,
		Description:     dbRepo.Description,
		Likes:           dbRepo.Likes,
		Downloads:       dbRepo.DownloadCount,
		Path:            dbRepo.Path,
		RepositoryID:    dbRepo.ID,
		Private:         dbRepo.Private,
		CreatedAt:       res.CreatedAt,
		UpdatedAt:       res.UpdatedAt,
		Configuration:   res.Configuration,
		Schema:          res.Schema,
		ProgramLanguage: res.ProgramLanguage,
		RunMode:         res.RunMode,
		InstallDepsCmds: res.InstallDepsCmds,
		BuildCmds:       res.BuildCmds,
		LaunchCmds:      res.LaunchCmds,
	}

	return resCode, nil
}

func (m *mcpServerComponentImpl) updateMCPServerInfo(mcpServer *database.MCPServer, req *types.UpdateMCPServerReq) *database.MCPServer {
	if req.Configuration != nil {
		mcpServer.Configuration = *req.Configuration
	}
	if req.ProgramLanguage != nil {
		mcpServer.ProgramLanguage = *req.ProgramLanguage
	}
	if req.RunMode != nil {
		mcpServer.RunMode = *req.RunMode
	}
	if req.InstallDepsCmds != nil {
		mcpServer.InstallDepsCmds = *req.InstallDepsCmds
	}
	if req.BuildCmds != nil {
		mcpServer.BuildCmds = *req.BuildCmds
	}
	if req.LaunchCmds != nil {
		mcpServer.LaunchCmds = *req.LaunchCmds
	}
	return mcpServer
}

func (m *mcpServerComponentImpl) Show(ctx context.Context, namespace string, name string, currentUser string, needOpWeight, needMultiSync bool) (*types.MCPServer, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
		syncStatus       types.RepositorySyncStatus
	)
	mcpServer, err := m.mcpServerStore.ByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find mcp server %s/%s, error: %w", namespace, name, err)
	}

	permission, err := m.repoComponent.GetUserRepoPermission(ctx, currentUser, mcpServer.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s permission for repo %s/%s, error: %w", currentUser, namespace, name, err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := m.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace for %s, error: %w", namespace, err)
	}

	for _, tag := range mcpServer.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := m.userLikesStore.IsExist(ctx, currentUser, mcpServer.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes, error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus, syncStatus = m.repoComponent.GetMirrorTaskStatusAndSyncStatus(mcpServer.Repository)

	res := &types.MCPServer{
		ID:            mcpServer.ID,
		Name:          mcpServer.Repository.Name,
		Nickname:      mcpServer.Repository.Nickname,
		Description:   mcpServer.Repository.Description,
		Likes:         mcpServer.Repository.Likes,
		Downloads:     mcpServer.Repository.DownloadCount,
		Path:          mcpServer.Repository.Path,
		RepositoryID:  mcpServer.Repository.ID,
		DefaultBranch: mcpServer.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(m.config, mcpServer.Repository),
		Tags:          tags,
		User: types.User{
			Username: mcpServer.Repository.User.Username,
			Nickname: mcpServer.Repository.User.NickName,
			Email:    mcpServer.Repository.User.Email,
		},
		Private:       mcpServer.Repository.Private,
		CreatedAt:     mcpServer.CreatedAt,
		UpdatedAt:     mcpServer.Repository.UpdatedAt,
		UserLikes:     likeExists,
		Source:        mcpServer.Repository.Source,
		SyncStatus:    syncStatus,
		License:       mcpServer.Repository.License,
		CanWrite:      permission.CanWrite,
		CanManage:     permission.CanAdmin,
		Namespace:     ns,
		ToolsNum:      mcpServer.ToolsNum,
		Configuration: mcpServer.Configuration,
		Schema:        mcpServer.Schema,
		GithubPath:    mcpServer.Repository.GithubPath,
		StarNum:       mcpServer.Repository.StarCount,
		MultiSource: types.MultiSource{
			HFPath:  mcpServer.Repository.HFPath,
			MSPath:  mcpServer.Repository.MSPath,
			CSGPath: mcpServer.Repository.CSGPath,
		},
		ProgramLanguage:  mcpServer.ProgramLanguage,
		RunMode:          mcpServer.RunMode,
		InstallDepsCmds:  mcpServer.InstallDepsCmds,
		BuildCmds:        mcpServer.BuildCmds,
		LaunchCmds:       mcpServer.LaunchCmds,
		MirrorTaskStatus: mirrorTaskStatus,
	}
	if permission.CanAdmin {
		res.SensitiveCheckStatus = mcpServer.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		m.addOpWeightToMCPs(ctx, []int64{res.RepositoryID}, []*types.MCPServer{res})
	}
	// add recom_scores to model
	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		m.addWeightsToMCP(ctx, res.RepositoryID, res, weightNames)
	}
	return res, nil
}

func (m *mcpServerComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per int, page int, needOpWeight bool) ([]*types.MCPServer, int, error) {
	var (
		err     error
		resMCPs []*types.MCPServer
	)
	repos, total, err := m.repoComponent.PublicToUser(ctx, types.MCPServerRepo, filter.Username, filter, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get public mcp repos error:%w", err)
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	mcps, err := m.mcpServerStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get mcps by repo ids error: %w", err)
	}

	// loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var mcpServer *database.MCPServer
		for _, m := range mcps {
			if m.RepositoryID == repo.ID {
				mcpServer = &m
				break
			}
		}
		if mcpServer == nil {
			continue
		}
		var tags []types.RepoTag
		for _, tag := range repo.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		resMCPs = append(resMCPs, &types.MCPServer{
			ID:           mcpServer.ID,
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Likes:        repo.Likes,
			Downloads:    repo.DownloadCount,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
			CreatedAt:    mcpServer.CreatedAt,
			Tags:         tags,
			UpdatedAt:    repo.UpdatedAt,
			Source:       repo.Source,
			SyncStatus:   repo.SyncStatus,
			License:      repo.License,
			Repository:   common.BuildCloneInfo(m.config, mcpServer.Repository),
			GithubPath:   mcpServer.Repository.GithubPath,
			ToolsNum:     mcpServer.ToolsNum,
			StarNum:      repo.StarCount,
		})
	}
	if needOpWeight {
		m.addOpWeightToMCPs(ctx, repoIDs, resMCPs)
	}
	return resMCPs, total, nil
}

func (m *mcpServerComponentImpl) Properties(ctx context.Context, req *types.MCPPropertyFilter) ([]types.MCPServerProperties, int, error) {
	var (
		isAdmin      bool
		repoOwnerIDs []int64
	)
	if len(req.CurrentUser) > 0 {
		// get user orgs from user service
		user, err := m.userSvcClient.GetUserInfo(ctx, req.CurrentUser, req.CurrentUser)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user info for list mcp tools, error: %w", err)
		}

		dbUser := &database.User{
			RoleMask: strings.Join(user.Roles, ","),
		}

		isAdmin = dbUser.CanAdmin()

		if !isAdmin {
			repoOwnerIDs = append(repoOwnerIDs, user.ID)
			//get user's orgs
			for _, org := range user.Orgs {
				repoOwnerIDs = append(repoOwnerIDs, org.UserID)
			}
		}
	}

	req.IsAdmin = isAdmin
	req.UserIDs = repoOwnerIDs

	slog.Debug("get user info to list tools", slog.Any("req", req), slog.Any("isadmin", req.IsAdmin), slog.Any("userids", req.UserIDs))
	res, total, err := m.mcpServerStore.ListProperties(ctx, req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list mcp tools, error: %w", err)
	}
	var properties []types.MCPServerProperties
	for _, r := range res {

		var tags []types.RepoTag
		for _, tag := range r.MCPServer.Repository.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}

		properties = append(properties, types.MCPServerProperties{
			ID:           r.ID,
			MCPServerID:  r.MCPServerID,
			RepositoryID: r.MCPServer.Repository.ID,
			Kind:         r.Kind,
			Name:         r.Name,
			Description:  r.Description,
			Schema:       r.Schema,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
			RepoPath:     r.MCPServer.Repository.Path,
			Tags:         tags,
		})
	}
	return properties, total, nil
}

func (m *mcpServerComponentImpl) OrgMCPServers(ctx context.Context, req *types.OrgMCPsReq) ([]types.MCPServer, int, error) {
	var resp []types.MCPServer
	var err error

	r := membership.RoleUnknown

	if req.CurrentUser != "" {
		r, err = m.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.Warn("faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}

	onlyPublic := !r.CanRead()

	mcps, total, err := m.mcpServerStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get org %s mcp servers, error:%w", req.Namespace, err)
	}

	for _, mcpServer := range mcps {
		resp = append(resp, types.MCPServer{
			ID:           mcpServer.ID,
			Name:         mcpServer.Repository.Name,
			Nickname:     mcpServer.Repository.Nickname,
			Description:  mcpServer.Repository.Description,
			Likes:        mcpServer.Repository.Likes,
			Downloads:    mcpServer.Repository.DownloadCount,
			Path:         mcpServer.Repository.Path,
			RepositoryID: mcpServer.RepositoryID,
			Private:      mcpServer.Repository.Private,
			CreatedAt:    mcpServer.CreatedAt,
			UpdatedAt:    mcpServer.Repository.UpdatedAt,
			Source:       mcpServer.Repository.Source,
			SyncStatus:   mcpServer.Repository.SyncStatus,
			License:      mcpServer.Repository.License,
			GithubPath:   mcpServer.Repository.GithubPath,
			ToolsNum:     mcpServer.ToolsNum,
			StarNum:      mcpServer.Repository.StarCount,
		})
	}

	return resp, total, nil
}

func (c *mcpServerComponentImpl) addWeightsToMCP(ctx context.Context, repoID int64, resMCPServer *types.MCPServer, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resMCPServer.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resMCPServer.Scores = append(resMCPServer.Scores, score)
			}
		}
	}
}

func (m *mcpServerComponentImpl) Deploy(ctx context.Context, req *types.DeployMCPServerReq) (*types.Space, error) {
	mcpServer, err := m.mcpServerStore.ByPath(ctx, req.MCPRepo.Namespace, req.MCPRepo.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find mcp server %s/%s, error: %w",
			req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}

	permission, err := m.repoComponent.GetUserRepoPermission(ctx, req.CurrentUser, mcpServer.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s permission for repo %s/%s, error: %w",
			req.CurrentUser, req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	token, err := m.tokenStore.GetUserGitToken(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s git access token, error: %w", req.CurrentUser, err)
	}

	resource, err := m.spaceResourceStore.FindByID(ctx, req.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find space resource by id %d for deploy mcp server, %w", req.ResourceID, err)
	}
	err = m.repoComponent.CheckAccountAndResource(ctx, req.CurrentUser, req.ClusterID, 0, resource)
	if err != nil {
		return nil, fmt.Errorf("failed to verify resource %s is available for MCP server deployment, %w", resource.Name, err)
	}

	namespace, err := m.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace %s, error: %w", req.Namespace, err)
	}

	user, err := m.userSvcClient.GetUserInfo(ctx, req.CurrentUser, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s info for deploy mcp server, error: %w", req.CurrentUser, err)
	}

	if user.Email == "" {
		return nil, errorx.ErrNoEmail
	}

	dbUser := database.User{}
	dbUser.SetRoles(user.Roles)

	if !dbUser.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := m.repoComponent.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, fmt.Errorf("failed to check user %s permission for namespace %s, error: %w", req.CurrentUser, req.Namespace, err)
			}
			if !canWrite {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create repo in this namespace")
			}
		}
	}

	spaceEnv, err := mergeMCPSpaceDeployEnv(req.Env, m.config)
	if err != nil {
		return nil, fmt.Errorf("fail to merge env for deploy mcp server %s/%s, %w", req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}

	req.RepoType = types.SpaceRepo
	req.License = mcpServer.Repository.License

	dbRepoReq := database.Repository{
		UserID:         user.ID,
		Path:           path.Join(req.Namespace, req.Name),
		GitPath:        common.BuildRelativePath(fmt.Sprintf("%ss", string(req.RepoType)), req.Namespace, req.Name),
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        req.License,
		DefaultBranch:  req.DefaultBranch,
		RepositoryType: req.RepoType,
	}
	dbRepo, err := m.repoStore.CreateRepo(ctx, dbRepoReq)
	if err != nil {
		return nil, fmt.Errorf("fail to create space repo %s/%s to deploy mcp server %s/%s, %w",
			req.Namespace, req.Name, req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}

	dbSpace := database.Space{
		RepositoryID:  dbRepo.ID,
		Sdk:           types.MCPSERVER.Name,
		SdkVersion:    "",
		CoverImageUrl: req.CoverImageUrl,
		Env:           spaceEnv,
		Hardware:      resource.Resources,
		Secrets:       "",
		Variables:     "",
		Template:      "",
		SKU:           strconv.FormatInt(resource.ID, 10), // space resource id
		ClusterID:     req.ClusterID,
	}

	resSpace, err := m.spaceStore.Create(ctx, dbSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to create space %s/%s to deploy mcp server %s/%s, error: %w",
			req.Namespace, req.Name, req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}

	cloneURL, err := getSourceRepoCloneURL(m.config.APIServer.PublicDomain, req.CurrentUser, token.Token, mcpServer.Repository.Path)
	if err != nil {
		return nil, fmt.Errorf("fail to get source repo clone url for mcp server %s/%s, error: %w",
			req.MCPRepo.Namespace, req.MCPRepo.Name, err)
	}
	slog.Debug("clone mcp server repo to space repo", slog.Any("cloneURL", cloneURL))
	cloneReq := gitserver.CreateMirrorRepoReq{
		RepoType:  req.RepoType,  // copy to repo type
		Namespace: req.Namespace, // copy to space namespace
		Name:      req.Name,      // copy to space repo name
		CloneUrl:  cloneURL,
	}
	taskId, err := m.gitServer.CreateMirrorRepo(ctx, cloneReq)
	if err != nil {
		return nil, fmt.Errorf("fail to clone mcp server %s/%s to space %s/%s, error: %w",
			req.MCPRepo.Namespace, req.MCPRepo.Name, req.Namespace, req.Name, err)
	}

	slog.Debug("mirror space repo from mcp server repo", slog.Any("req", req), slog.Any("taskId", taskId),
		slog.Any("cloneReq", cloneReq), slog.Any("req", req), slog.Any("error", err))

	err = m.updateSpaceMetaTag(req, user)
	if err != nil {
		slog.Warn("fail to set mcpserver tag for space", slog.Any("req", req), slog.Any("user", user.Username), slog.Any("error", err))
	}

	err = m.createDeployDefaultFiles(req, mcpServer, user)
	if err != nil {
		return nil, fmt.Errorf("fail to create default files for space %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	space := &types.Space{
		ID:            resSpace.ID,
		Creator:       req.CurrentUser,
		License:       req.License,
		RepositoryID:  dbRepo.ID,
		Path:          dbRepo.Path,
		Private:       dbRepo.Private,
		Name:          dbRepo.Name,
		Nickname:      dbRepo.Nickname,
		Sdk:           resSpace.Sdk,
		SdkVersion:    resSpace.SdkVersion,
		Env:           resSpace.Env,
		Hardware:      resource.Resources,
		CoverImageUrl: resSpace.CoverImageUrl,
		SKU:           resSpace.SKU,
		CreatedAt:     resSpace.CreatedAt,
	}
	return space, nil
}

func getSourceRepoCloneURL(address, userName, token, repoPath string) (string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("invalid clone url prefix, error: %w", err)
	}
	credential := fmt.Sprintf("%s:%s", userName, token)

	cloneURL := fmt.Sprintf("%s://%s@%s:%s/%ss/%s.git",
		u.Scheme,
		credential,
		u.Hostname(),
		u.Port(),
		types.MCPServerRepo,
		repoPath)

	return cloneURL, nil
}

func (m *mcpServerComponentImpl) createDeployDefaultFiles(req *types.DeployMCPServerReq,
	mcpServer *database.MCPServer, user *rpc.User) error {
	templatePath, err := getSpaceTemplatePath("mcp_deploy")
	if err != nil {
		return fmt.Errorf("check mcp deploy template path %s error: %w", templatePath, err)
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

	err = m.uploadTemplateFiles(entries, req, templatePath, mcpServer, user)
	if err != nil {
		return fmt.Errorf("fail to upload space mcp server template files error: %w", err)
	}
	return nil
}

func (m *mcpServerComponentImpl) uploadTemplateFiles(entries []os.DirEntry, req *types.DeployMCPServerReq,
	templatePath string, mcpServer *database.MCPServer, user *rpc.User) error {
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		fileName := entry.Name()

		content, err := os.ReadFile(filepath.Join(templatePath, fileName))
		if err != nil {
			return fmt.Errorf("failed to read %s/%s file for %s/%s space, cause: %w",
				templatePath, fileName, req.Namespace, req.Name, err)
		}

		fileReq := types.CreateFileParams{
			Username:  user.Username,
			Email:     user.Email,
			Message:   types.InitCommitMessage,
			Branch:    req.DefaultBranch,
			Content:   string(content),
			NewBranch: req.DefaultBranch,
			Namespace: req.Namespace,
			Name:      req.Name,
			FilePath:  fileName,
		}
		err = m.gitServer.CreateRepoFile(buildCreateFileReq(&fileReq, types.SpaceRepo))
		if err != nil {
			return fmt.Errorf("failed to create %s file for %s/%s space, cause: %w", fileName, req.Namespace, req.Name, err)
		}
	}

	config := types.MCPSpaceConfig{
		ProgramLanguage: mcpServer.ProgramLanguage,
		RunMode:         mcpServer.RunMode,
		InstallDepsCmds: mcpServer.InstallDepsCmds,
		BuildCmds:       mcpServer.BuildCmds,
		LaunchCmds:      mcpServer.LaunchCmds,
	}

	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mcp space config for %s/%s space, cause: %w", req.Namespace, req.Name, err)
	}

	fileReq := types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   string(content),
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.MCPSpaceConfFileName,
	}

	err = m.gitServer.CreateRepoFile(buildCreateFileReq(&fileReq, types.SpaceRepo))
	if err != nil {
		return fmt.Errorf("failed to create %s file for %s/%s space, cause: %w", types.MCPSpaceConfFileName,
			req.Namespace, req.Name, err)
	}

	return nil
}

func (m *mcpServerComponentImpl) updateSpaceMetaTag(req *types.DeployMCPServerReq, user *rpc.User) error {
	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.DefaultBranch,
		Path:      types.REPOCARD_FILENAME,
		RepoType:  types.SpaceRepo,
	}
	metaMap, splits, err := GetMetaMapFromReadMe(m.gitServer, getFileContentReq)
	if err != nil {
		return fmt.Errorf("failed parse meta from readme, cause: %w", err)
	}
	metaMap["mcpservers"] = []string{fmt.Sprintf("%s/%s", req.MCPRepo.Namespace, req.MCPRepo.Name)}
	output, err := GetOutputForReadme(metaMap, splits)
	if err != nil {
		return fmt.Errorf("failed generate output for readme, cause: %w", err)
	}

	var readmeReq types.UpdateFileReq
	readmeReq.Branch = types.MainBranch
	readmeReq.Message = "update mcp server tag"
	readmeReq.FilePath = types.REPOCARD_FILENAME
	readmeReq.RepoType = types.SpaceRepo
	readmeReq.Namespace = req.Namespace
	readmeReq.Name = req.Name
	readmeReq.Username = user.Username
	readmeReq.Email = user.Email
	readmeReq.Content = base64.StdEncoding.EncodeToString([]byte(output))

	err = m.gitServer.UpdateRepoFile(&readmeReq)
	if err != nil {
		return fmt.Errorf("failed to set mcp server tag to %s file for repo %s/%s, cause: %w",
			readmeReq.FilePath, req.Namespace, req.Name, err)
	}

	return nil
}

func mergeMCPSpaceDeployEnv(env string, config *config.Config) (string, error) {
	newEnvs := ""
	envMap := make(map[string]string)
	if len(env) > 0 {
		err := json.Unmarshal([]byte(env), &envMap)
		if err != nil {
			return "", fmt.Errorf("invalid json format env %s for deploy mcp space, cause: %w", env, err)
		}
	}

	if len(config.Space.PYPIIndexURL) > 0 {
		envMap[types.MCPSpacePypiKey] = config.Space.PYPIIndexURL
	}

	if len(envMap) < 1 {
		return "", nil
	}

	data, err := json.Marshal(envMap)
	if err != nil {
		return "", fmt.Errorf("fail to marshal env map for deploy mcp space, cause: %w", err)
	}

	newEnvs = string(data)
	return newEnvs, nil
}
