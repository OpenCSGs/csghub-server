package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

const (
	initCommitMessage     = "initial commit"
	ossFileExpire         = 259200 * time.Second
	gitattributesFileName = ".gitattributes"
)

type DatasetComponent interface {
	Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Dataset, int, error)
	Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string) (*types.Dataset, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgDatasets(ctx context.Context, req *types.OrgDatasetsReq) ([]types.Dataset, int, error)
}

func NewDatasetComponent(config *config.Config) (DatasetComponent, error) {
	c := &datasetComponentImpl{}
	c.tagStore = database.NewTagStore()
	c.datasetStore = database.NewDatasetStore()
	c.repoStore = database.NewRepoStore()
	c.namespaceStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.userLikesStore = database.NewUserLikesStore()
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create repo component, error: %w", err)
	}
	c.sensitiveComponent, err = NewSensitiveComponent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sensitive component, error: %w", err)
	}
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.gitServer = gs
	c.config = config
	return c, nil
}

type datasetComponentImpl struct {
	config             *config.Config
	repoComponent      RepoComponent
	tagStore           database.TagStore
	datasetStore       database.DatasetStore
	repoStore          database.RepoStore
	namespaceStore     database.NamespaceStore
	userStore          database.UserStore
	sensitiveComponent SensitiveComponent
	gitServer          gitserver.GitServer
	userLikesStore     database.UserLikesStore
	userSvcClient      rpc.UserSvcClient
}

func (c *datasetComponentImpl) Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errors.New("users do not have permission to create datasets in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errors.New("users do not have permission to create datasets in this namespace")
			}
		}
	}

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}

	req.RepoType = types.DatasetRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname
	_, dbRepo, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbDataset := database.Dataset{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	dataset, err := c.datasetStore.Create(ctx, dbDataset)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
	}

	// Create README.md file
	err = c.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  types.ReadmeFileName,
	}, types.DatasetRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.gitServer.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   types.InitCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   types.DatasetGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  gitattributesFileName,
	}, types.DatasetRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	for _, tag := range dataset.Repository.Tags {
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

	resDataset := &types.Dataset{
		ID:           dataset.ID,
		Name:         dataset.Repository.Name,
		Nickname:     dataset.Repository.Nickname,
		Description:  dataset.Repository.Description,
		Likes:        dataset.Repository.Likes,
		Downloads:    dataset.Repository.DownloadCount,
		Path:         dataset.Repository.Path,
		RepositoryID: dataset.RepositoryID,
		Repository:   common.BuildCloneInfo(c.config, dataset.Repository),
		Private:      dataset.Repository.Private,
		User: types.User{
			Username: user.Username,
			Nickname: user.NickName,
			Email:    user.Email,
		},
		Tags:      tags,
		CreatedAt: dataset.CreatedAt,
		UpdatedAt: dataset.UpdatedAt,
	}

	return resDataset, nil
}

func generateReadmeData(license string) string {
	return `
---
license: ` + license + `
---
	`
}

func (c *datasetComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Dataset, int, error) {
	return c.commonIndex(ctx, filter, per, page)
}

func (c *datasetComponentImpl) commonIndex(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Dataset, int, error) {
	var (
		err         error
		resDatasets []types.Dataset
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.DatasetRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public dataset repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	datasets, err := c.datasetStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get datasets by repo ids,error:%w", err)
		return nil, 0, newError
	}

	// loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var dataset *database.Dataset
		for _, d := range datasets {
			if repo.ID == d.RepositoryID {
				dataset = &d
				break
			}
		}
		if dataset == nil {
			continue
		}
		var tags []types.RepoTag
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
		resDatasets = append(resDatasets, types.Dataset{
			ID:           dataset.ID,
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Likes:        repo.Likes,
			Downloads:    repo.DownloadCount,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
			Tags:         tags,
			CreatedAt:    dataset.CreatedAt,
			UpdatedAt:    repo.UpdatedAt,
			Source:       repo.Source,
			SyncStatus:   repo.SyncStatus,
			License:      repo.License,
			Repository:   common.BuildCloneInfo(c.config, dataset.Repository),
			User: types.User{
				Username: dataset.Repository.User.Username,
				Nickname: dataset.Repository.User.NickName,
				Email:    dataset.Repository.User.Email,
				Avatar:   dataset.Repository.User.Avatar,
			},
			MultiSource: types.MultiSource{
				HFPath:  dataset.Repository.HFPath,
				MSPath:  dataset.Repository.MSPath,
				CSGPath: dataset.Repository.CSGPath,
			},
		})
	}

	return resDatasets, total, nil
}

func (c *datasetComponentImpl) Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error) {
	req.RepoType = types.DatasetRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	dataset, err := c.datasetStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	// update times of dateset
	err = c.datasetStore.Update(ctx, *dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to update database dataset, error: %w", err)
	}

	resDataset := &types.Dataset{
		ID:           dataset.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    dataset.CreatedAt,
		UpdatedAt:    dataset.UpdatedAt,
	}

	return resDataset, nil
}

func (c *datasetComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	dataset, err := c.datasetStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find dataset, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.DatasetRepo,
	}
	_, err = c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of dataset, error: %w", err)
	}

	err = c.datasetStore.Delete(ctx, *dataset)
	if err != nil {
		return fmt.Errorf("failed to delete database dataset, error: %w", err)
	}
	return nil
}

func (c *datasetComponentImpl) Show(ctx context.Context, namespace, name, currentUser string) (*types.Dataset, error) {
	var tags []types.RepoTag
	dataset, err := c.datasetStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, dataset.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for dataset, error: %w", err)
	}

	for _, tag := range dataset.Repository.Tags {
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

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, dataset.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	resDataset := &types.Dataset{
		ID:            dataset.ID,
		Name:          dataset.Repository.Name,
		Nickname:      dataset.Repository.Nickname,
		Description:   dataset.Repository.Description,
		Likes:         dataset.Repository.Likes,
		Downloads:     dataset.Repository.DownloadCount,
		Path:          dataset.Repository.Path,
		RepositoryID:  dataset.Repository.ID,
		DefaultBranch: dataset.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(c.config, dataset.Repository),
		Tags:          tags,
		User: types.User{
			Username: dataset.Repository.User.Username,
			Nickname: dataset.Repository.User.NickName,
			Email:    dataset.Repository.User.Email,
			Avatar:   dataset.Repository.User.Avatar,
		},
		Private:             dataset.Repository.Private,
		CreatedAt:           dataset.CreatedAt,
		UpdatedAt:           dataset.Repository.UpdatedAt,
		UserLikes:           likeExists,
		Source:              dataset.Repository.Source,
		SyncStatus:          dataset.Repository.SyncStatus,
		License:             dataset.Repository.License,
		MirrorLastUpdatedAt: dataset.Repository.Mirror.LastUpdatedAt,
		CanWrite:            permission.CanWrite,
		CanManage:           permission.CanAdmin,
		Namespace:           ns,
		MultiSource: types.MultiSource{
			HFPath:  dataset.Repository.HFPath,
			MSPath:  dataset.Repository.MSPath,
			CSGPath: dataset.Repository.CSGPath,
		},
	}
	if permission.CanAdmin {
		resDataset.SensitiveCheckStatus = dataset.Repository.SensitiveCheckStatus.String()
	}

	return resDataset, nil
}

func (c *datasetComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	dataset, err := c.datasetStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, dataset.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrForbidden
	}

	return c.getRelations(ctx, dataset.RepositoryID, currentUser)
}

func (c *datasetComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
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

func (c *datasetComponentImpl) OrgDatasets(ctx context.Context, req *types.OrgDatasetsReq) ([]types.Dataset, int, error) {
	var resDatasets []types.Dataset
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
	datasets, total, err := c.datasetStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get user datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range datasets {
		resDatasets = append(resDatasets, types.Dataset{
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

	return resDatasets, total, nil
}
