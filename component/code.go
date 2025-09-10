package component

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
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

const codeGitattributesContent = modelGitattributesContent

type CodeComponent interface {
	Create(ctx context.Context, req *types.CreateCodeReq) (*types.Code, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Code, int, error)
	Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Code, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgCodes(ctx context.Context, req *types.OrgCodesReq) ([]types.Code, int, error)
}

func NewCodeComponent(config *config.Config) (CodeComponent, error) {
	c := &codeComponentImpl{}
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	c.codeStore = database.NewCodeStore()
	c.repoStore = database.NewRepoStore()
	c.recomStore = database.NewRecomStore()
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}
	c.gitServer = gs
	c.config = config
	c.userLikesStore = database.NewUserLikesStore()
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return c, nil
}

type codeComponentImpl struct {
	config         *config.Config
	repoComponent  RepoComponent
	codeStore      database.CodeStore
	repoStore      database.RepoStore
	userLikesStore database.UserLikesStore
	gitServer      gitserver.GitServer
	userSvcClient  rpc.UserSvcClient
	recomStore     database.RecomStore
}

func (c *codeComponentImpl) Create(ctx context.Context, req *types.CreateCodeReq) (*types.Code, error) {
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

	req.RepoType = types.CodeRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname
	_, dbRepo, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbCode := database.Code{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	code, err := c.codeStore.Create(ctx, dbCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create database code, cause: %w", err)
	}

	// Create README.md file
	err = c.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.ReadmeFileName,
	}, types.CodeRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   codeGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.GitattributesFileName,
	}, types.CodeRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	for _, tag := range code.Repository.Tags {
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

	resCode := &types.Code{
		ID:           code.ID,
		Name:         code.Repository.Name,
		Nickname:     code.Repository.Nickname,
		Description:  code.Repository.Description,
		Likes:        code.Repository.Likes,
		Downloads:    code.Repository.DownloadCount,
		Path:         code.Repository.Path,
		RepositoryID: code.RepositoryID,
		Repository:   common.BuildCloneInfo(c.config, code.Repository),
		Private:      code.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: code.CreatedAt,
		UpdatedAt: code.UpdatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.CodeRepo,
			RepoPath:  code.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resCode, nil
}

func (c *codeComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Code, int, error) {
	var (
		err      error
		resCodes []*types.Code
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.CodeRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public code repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	codes, err := c.codeStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get codes by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var code *database.Code
		for _, c := range codes {
			if c.RepositoryID == repo.ID {
				code = &c
				break
			}
		}
		var (
			tags             []types.RepoTag
			mirrorTaskStatus types.MirrorTaskStatus
		)
		for _, tag := range repo.Tags {
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
		if code.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = code.Repository.Mirror.CurrentTask.Status
		}
		resCodes = append(resCodes, &types.Code{
			ID:               code.ID,
			Name:             repo.Name,
			Nickname:         repo.Nickname,
			Description:      repo.Description,
			Likes:            repo.Likes,
			Downloads:        repo.DownloadCount,
			Path:             repo.Path,
			RepositoryID:     repo.ID,
			Private:          repo.Private,
			CreatedAt:        code.CreatedAt,
			UpdatedAt:        repo.UpdatedAt,
			Tags:             tags,
			Source:           repo.Source,
			SyncStatus:       repo.SyncStatus,
			License:          repo.License,
			MirrorTaskStatus: mirrorTaskStatus,
		})
	}
	slog.Info("code.index")
	if needOpWeight {
		c.addOpWeightToCodes(ctx, repoIDs, resCodes)
	}

	return resCodes, total, nil
}

func (c *codeComponentImpl) Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error) {
	req.RepoType = types.CodeRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	code, err := c.codeStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	//update times of code
	err = c.codeStore.Update(ctx, *code)
	if err != nil {
		return nil, fmt.Errorf("failed to update database code repo, error: %w", err)
	}

	resCode := &types.Code{
		ID:           code.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    code.CreatedAt,
		UpdatedAt:    code.UpdatedAt,
	}

	return resCode, nil
}

func (c *codeComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find code, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.CodeRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of code, error: %w", err)
	}

	err = c.codeStore.Delete(ctx, *code)
	if err != nil {
		return fmt.Errorf("failed to delete database code, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.CodeRepo,
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

func (c *codeComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Code, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
		syncStatus       types.RepositorySyncStatus
	)
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, code.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for code, error: %w", err)
	}

	for _, tag := range code.Repository.Tags {
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

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, code.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus, syncStatus = c.repoComponent.GetMirrorTaskStatusAndSyncStatus(code.Repository)

	resCode := &types.Code{
		ID:            code.ID,
		Name:          code.Repository.Name,
		Nickname:      code.Repository.Nickname,
		Description:   code.Repository.Description,
		Likes:         code.Repository.Likes,
		Downloads:     code.Repository.DownloadCount,
		Path:          code.Repository.Path,
		RepositoryID:  code.Repository.ID,
		DefaultBranch: code.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(c.config, code.Repository),
		Tags:          tags,
		User: types.User{
			Username: code.Repository.User.Username,
			Nickname: code.Repository.User.NickName,
			Email:    code.Repository.User.Email,
		},
		Private:    code.Repository.Private,
		CreatedAt:  code.CreatedAt,
		UpdatedAt:  code.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     code.Repository.Source,
		SyncStatus: syncStatus,
		License:    code.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
		MultiSource: types.MultiSource{
			HFPath:  code.Repository.HFPath,
			MSPath:  code.Repository.MSPath,
			CSGPath: code.Repository.CSGPath,
		},
		MirrorTaskStatus: mirrorTaskStatus,
		//RecomOpWeight: ,
	}
	if permission.CanAdmin {
		resCode.SensitiveCheckStatus = code.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		c.addOpWeightToCodes(ctx, []int64{resCode.RepositoryID}, []*types.Code{resCode})
	}

	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToCode(ctx, resCode.RepositoryID, resCode, weightNames)
	}
	return resCode, nil
}

func (c *codeComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, code.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrForbidden
	}

	return c.getRelations(ctx, code.RepositoryID, currentUser)
}

func (c *codeComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.repoComponent.RelatedRepos(ctx, repoID, currentUser)
	if err != nil {
		return nil, err
	}
	rels := new(types.Relations)
	modelRepos := res[types.ModelRepo]
	for _, repo := range modelRepos {
		rels.Models = append(rels.Models, &types.Model{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}

	return rels, nil
}

func (c *codeComponentImpl) OrgCodes(ctx context.Context, req *types.OrgCodesReq) ([]types.Code, int, error) {
	var resCodes []types.Code
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
	codes, total, err := c.codeStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org codes,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range codes {
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

func (c *codeComponentImpl) addWeightsToCode(ctx context.Context, repoID int64, resCode *types.Code, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resCode.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resCode.Scores = append(resCode.Scores, score)
			}
		}
	}
}
