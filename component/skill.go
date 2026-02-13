package component

import (
	"context"
	"fmt"
	"log/slog"
	"path"
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

const skillGitattributesContent = modelGitattributesContent

type SkillComponent interface {
	Create(ctx context.Context, req *types.CreateSkillReq) (*types.Skill, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Skill, int, error)
	Update(ctx context.Context, req *types.UpdateSkillReq) (*types.Skill, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Skill, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgSkills(ctx context.Context, req *types.OrgSkillsReq) ([]types.Skill, int, error)
}

func NewSkillComponent(config *config.Config) (SkillComponent, error) {
	c := &skillComponentImpl{}
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	c.skillStore = database.NewSkillStore()
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

type skillComponentImpl struct {
	config         *config.Config
	repoComponent  RepoComponent
	skillStore     database.SkillStore
	repoStore      database.RepoStore
	userLikesStore database.UserLikesStore
	gitServer      gitserver.GitServer
	userSvcClient  rpc.UserSvcClient
	recomStore     database.RecomStore
}

func (c *skillComponentImpl) Create(ctx context.Context, req *types.CreateSkillReq) (*types.Skill, error) {
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

	req.RepoType = types.SkillRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname
	req.CommitFiles = []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: skillGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}
	_, dbRepo, commitFilesReq, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbSkill := database.Skill{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	repoPath := path.Join(req.Namespace, req.Name)
	skill, err := c.skillStore.CreateAndUpdateRepoPath(ctx, dbSkill, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database skill, cause: %w", err)
	}

	_ = c.gitServer.CommitFiles(ctx, *commitFilesReq)

	for _, tag := range skill.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	resSkill := &types.Skill{
		ID:           skill.ID,
		Name:         skill.Repository.Name,
		Nickname:     skill.Repository.Nickname,
		Description:  skill.Repository.Description,
		Likes:        skill.Repository.Likes,
		Downloads:    skill.Repository.DownloadCount,
		Path:         skill.Repository.Path,
		RepositoryID: skill.RepositoryID,
		Repository:   common.BuildCloneInfo(c.config, skill.Repository),
		Private:      skill.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: skill.CreatedAt,
		UpdatedAt: skill.UpdatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SkillRepo,
			RepoPath:  skill.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resSkill, nil
}

func (c *skillComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Skill, int, error) {
	var (
		err       error
		resSkills []*types.Skill
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.SkillRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public skill repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	skills, err := c.skillStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get skills by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var skill *database.Skill
		for _, s := range skills {
			if s.RepositoryID == repo.ID {
				skill = &s
				break
			}
		}
		if skill == nil {
			continue
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
				ShowName:  tag.I18nKey,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		if skill.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = skill.Repository.Mirror.CurrentTask.Status
		}
		resSkills = append(resSkills, &types.Skill{
			ID:               skill.ID,
			Name:             repo.Name,
			Nickname:         repo.Nickname,
			Description:      repo.Description,
			Likes:            repo.Likes,
			Downloads:        repo.DownloadCount,
			Path:             repo.Path,
			RepositoryID:     repo.ID,
			Private:          repo.Private,
			CreatedAt:        skill.CreatedAt,
			UpdatedAt:        repo.UpdatedAt,
			Tags:             tags,
			Source:           repo.Source,
			SyncStatus:       repo.SyncStatus,
			License:          repo.License,
			MirrorTaskStatus: mirrorTaskStatus,
		})
	}
	slog.Info("skill.index")
	if needOpWeight {
		c.addOpWeightToSkills(ctx, repoIDs, resSkills)
	}

	return resSkills, total, nil
}

func (c *skillComponentImpl) Update(ctx context.Context, req *types.UpdateSkillReq) (*types.Skill, error) {
	req.RepoType = types.SkillRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	skill, err := c.skillStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill repo, error: %w", err)
	}

	//update times of skill
	err = c.skillStore.Update(ctx, *skill)
	if err != nil {
		return nil, fmt.Errorf("failed to update database skill repo, error: %w", err)
	}

	resSkill := &types.Skill{
		ID:           skill.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    skill.CreatedAt,
		UpdatedAt:    skill.UpdatedAt,
	}

	return resSkill, nil
}

func (c *skillComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find skill, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.SkillRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of skill, error: %w", err)
	}

	err = c.skillStore.Delete(ctx, *skill)
	if err != nil {
		return fmt.Errorf("failed to delete database skill, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SkillRepo,
			RepoPath:  repo.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.ErrorContext(ctx, "failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *skillComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Skill, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
	)
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, skill.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for skill, error: %w", err)
	}

	for _, tag := range skill.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, skill.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus = c.repoComponent.GetMirrorTaskStatus(skill.Repository)

	resSkill := &types.Skill{
		ID:            skill.ID,
		Name:          skill.Repository.Name,
		Nickname:      skill.Repository.Nickname,
		Description:   skill.Repository.Description,
		Likes:         skill.Repository.Likes,
		Downloads:     skill.Repository.DownloadCount,
		Path:          skill.Repository.Path,
		RepositoryID:  skill.Repository.ID,
		DefaultBranch: skill.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(c.config, skill.Repository),
		Tags:          tags,
		User: types.User{
			Username: skill.Repository.User.Username,
			Nickname: skill.Repository.User.NickName,
			Email:    skill.Repository.User.Email,
		},
		Private:    skill.Repository.Private,
		CreatedAt:  skill.CreatedAt,
		UpdatedAt:  skill.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     skill.Repository.Source,
		SyncStatus: skill.Repository.SyncStatus,
		License:    skill.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
		MultiSource: types.MultiSource{
			HFPath:  skill.Repository.HFPath,
			MSPath:  skill.Repository.MSPath,
			CSGPath: skill.Repository.CSGPath,
		},
		MirrorTaskStatus: mirrorTaskStatus,
	}
	if permission.CanAdmin {
		resSkill.SensitiveCheckStatus = skill.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		c.addOpWeightToSkills(ctx, []int64{resSkill.RepositoryID}, []*types.Skill{resSkill})
	}

	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToSkill(ctx, resSkill.RepositoryID, resSkill, weightNames)
	}
	return resSkill, nil
}

func (c *skillComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, skill.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrForbidden
	}

	return c.getRelations(ctx, skill.RepositoryID, currentUser)
}

func (c *skillComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
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

func (c *skillComponentImpl) OrgSkills(ctx context.Context, req *types.OrgSkillsReq) ([]types.Skill, int, error) {
	var resSkills []types.Skill
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.ErrorContext(ctx, "faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	skills, total, err := c.skillStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org skills,error:%w", err)
		slog.ErrorContext(ctx, newError.Error())
		return nil, 0, newError
	}

	for _, data := range skills {
		resSkills = append(resSkills, types.Skill{
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

	return resSkills, total, nil
}

func (c *skillComponentImpl) addWeightsToSkill(ctx context.Context, repoID int64, resSkill *types.Skill, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resSkill.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resSkill.Scores = append(resSkill.Scores, score)
			}
		}
	}
}

func (c *skillComponentImpl) addOpWeightToSkills(ctx context.Context, repoIDs []int64, resSkills []*types.Skill) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, repoIDs)
	if err != nil {
		return
	}
	weightMap := make(map[int64]map[string]float64)
	for _, weight := range weights {
		if _, ok := weightMap[weight.RepositoryID]; !ok {
			weightMap[weight.RepositoryID] = make(map[string]float64)
		}
		weightMap[weight.RepositoryID][string(weight.WeightName)] = weight.Score
	}

	for _, skill := range resSkills {
		if weight, ok := weightMap[skill.RepositoryID]; ok {
			skill.RecomOpWeight = int(weight["op"])
		}
	}
}
