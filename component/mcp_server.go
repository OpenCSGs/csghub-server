package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type MCPServerComponent interface {
	Create(ctx context.Context, req *types.CreateMCPServerReq) (*types.MCPServer, error)
	Delete(ctx context.Context, req *types.UpdateMCPServerReq) error
	Update(ctx context.Context, req *types.UpdateMCPServerReq) (*types.MCPServer, error)
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool) (*types.MCPServer, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.MCPServer, int, error)
	Properties(ctx context.Context, req *types.MCPPropertyFilter) ([]types.MCPServerProperties, int, error)
}

type mcpServerComponentImpl struct {
	config         *config.Config
	repoComponent  RepoComponent
	repoStore      database.RepoStore
	gitServer      gitserver.GitServer
	userSvcClient  rpc.UserSvcClient
	mcpServerStore database.MCPServerStore
	userLikesStore database.UserLikesStore
	recomStore     database.RecomStore
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
			ShowName:  tag.ShowName,
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
		return ErrForbidden
	}

	deleteRepoReq := types.DeleteRepoReq{
		Username:  req.Username,
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  types.MCPServerRepo,
	}
	_, err = m.repoComponent.DeleteRepo(ctx, deleteRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of mcp server %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	err = m.mcpServerStore.Delete(ctx, *mcpServer)
	if err != nil {
		return fmt.Errorf("failed to delete mcp server, error: %w", err)
	}
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
		return nil, ErrForbidden
	}

	req.RepoType = types.MCPServerRepo
	dbRepo, err := m.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update mcp server repo %s/%s, error: %w", req.Namespace, req.Name, err)
	}

	if req.Configuration != nil {
		mcpServer.Configuration = *req.Configuration
	}

	res, err := m.mcpServerStore.Update(ctx, *mcpServer)
	if err != nil {
		return nil, fmt.Errorf("failed to update mcp server by , error: %w", err)
	}

	resCode := &types.MCPServer{
		ID:            res.ID,
		Name:          dbRepo.Name,
		Nickname:      dbRepo.Nickname,
		Description:   dbRepo.Description,
		Likes:         dbRepo.Likes,
		Downloads:     dbRepo.DownloadCount,
		Path:          dbRepo.Path,
		RepositoryID:  dbRepo.ID,
		Private:       dbRepo.Private,
		CreatedAt:     res.CreatedAt,
		UpdatedAt:     res.UpdatedAt,
		Configuration: res.Configuration,
		Schema:        res.Schema,
	}

	return resCode, nil
}

func (m *mcpServerComponentImpl) Show(ctx context.Context, namespace string, name string, currentUser string, needOpWeight bool) (*types.MCPServer, error) {
	var tags []types.RepoTag
	mcpServer, err := m.mcpServerStore.ByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find mcp server %s/%s, error: %w", namespace, name, err)
	}

	permission, err := m.repoComponent.GetUserRepoPermission(ctx, currentUser, mcpServer.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user %s permission for repo %s/%s, error: %w", currentUser, namespace, name, err)
	}
	if !permission.CanRead {
		return nil, ErrForbidden
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
			ShowName:  tag.ShowName,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := m.userLikesStore.IsExist(ctx, currentUser, mcpServer.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes, error:%w", err)
		return nil, newError
	}

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
		SyncStatus:    mcpServer.Repository.SyncStatus,
		License:       mcpServer.Repository.License,
		CanWrite:      permission.CanWrite,
		CanManage:     permission.CanAdmin,
		Namespace:     ns,
		ToolsNum:      mcpServer.ToolsNum,
		Configuration: mcpServer.Configuration,
		Schema:        mcpServer.Schema,
	}
	if permission.CanAdmin {
		res.SensitiveCheckStatus = mcpServer.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		m.addOpWeightToMCPs(ctx, []int64{res.RepositoryID}, []*types.MCPServer{res})
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
		return nil, 0, fmt.Errorf("failed to get public mcp repos,error:%w", err)
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
				ShowName:  tag.ShowName,
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
				ShowName:  tag.ShowName,
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
