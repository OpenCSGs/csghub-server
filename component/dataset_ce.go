//go:build !ee && !saas

package component

import (
	"context"
	"fmt"
	"path"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// extendDatasetImpl is the extended implementation of datasetComponentImpl for CE version
type extendDatasetImpl struct{}

func (c *datasetComponentImpl) addOpWeightToDataset(ctx context.Context, repoIDs []int64, resDatasets []*types.Dataset) {
}

func (c *datasetComponentImpl) BuyDataset(ctx context.Context, req *types.BuyDatasetReq) (*types.BuyDatasetResp, error) {
	return nil, nil
}

func (c *datasetComponentImpl) commonIndex(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Dataset, int, error) {
	var (
		err         error
		resDatasets []*types.Dataset
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.DatasetRepo, filter.Username, filter, per, page)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get public dataset repos, error: %w", err)
	}

	// Save total value to ensure we return it even if there's a panic later
	resultTotal := total
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	datasets, err := c.datasetStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get datasets by repo ids, error: %w", err)
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
		var (
			tags                []types.RepoTag
			mirrorTaskStatus    types.MirrorTaskStatus
			xnetMigrationStatus types.XnetMigrationTaskStatus
		)
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
		if dataset.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = dataset.Repository.Mirror.CurrentTask.Status
		}
		var xnetMigrationProgress int
		if dataset.Repository.CurrentXnetMigrationTaskID != 0 {
			task, err := c.xnetMigrationTaskStore.GetXnetMigrationTaskByID(ctx, dataset.Repository.CurrentXnetMigrationTaskID)
			if err == nil && task != nil {
				xnetMigrationStatus = task.Status
				if xnetMigrationStatus == types.XnetMigrationTaskStatusRunning {
					xnetMigrationProgress = c.getXnetMigrationProgress(ctx, repo)
				}
			}
		}

		resDatasets = append(resDatasets, &types.Dataset{
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
			MirrorTaskStatus:      mirrorTaskStatus,
			XnetMigrationStatus:   xnetMigrationStatus,
			XnetMigrationProgress: xnetMigrationProgress,
			// CE version doesn't support dataset purchase features
			// So we don't set DatasetType, RelatedDatasetID, Price, Forked, IsForSale, UserPurchased fields
		})
	}
	if needOpWeight {
		c.addOpWeightToDataset(ctx, repoIDs, resDatasets)
	}

	return resDatasets, resultTotal, nil
}

func (c *datasetComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.Dataset, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
	)
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
			ShowName:  tag.I18nKey, //ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, dataset.Repository.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check for the presence of the user likes, error: %w", err)
	}

	mirrorTaskStatus = c.repoComponent.GetMirrorTaskStatus(dataset.Repository)

	var xnetMigrationStatus types.XnetMigrationTaskStatus
	var xnetMigrationProgress int
	if dataset.Repository.CurrentXnetMigrationTaskID != 0 {
		task, err := c.xnetMigrationTaskStore.GetXnetMigrationTaskByID(ctx, dataset.Repository.CurrentXnetMigrationTaskID)
		if err == nil && task != nil {
			xnetMigrationStatus = task.Status
			if task.Status == types.XnetMigrationTaskStatusRunning {
				xnetMigrationProgress = c.getXnetMigrationProgress(ctx, dataset.Repository)
			}
		}
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
		MirrorTaskStatus:      mirrorTaskStatus,
		XnetEnabled:           dataset.Repository.XnetEnabled,
		XnetMigrationStatus:   xnetMigrationStatus,
		XnetMigrationProgress: xnetMigrationProgress,
		// CE version doesn't support dataset purchase features
		// So we don't set DatasetType, RelatedDatasetID, Price, Forked, IsForSale, UserPurchased fields
	}
	if permission.CanAdmin {
		resDataset.SensitiveCheckStatus = dataset.Repository.SensitiveCheckStatus.String()
	}

	if needOpWeight {
		c.addOpWeightToDataset(ctx, []int64{resDataset.RepositoryID}, []*types.Dataset{resDataset})
	}

	// add recom_scores to model
	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToDataset(ctx, resDataset.RepositoryID, resDataset, weightNames)
	}

	return resDataset, nil
}

func (c *datasetComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Dataset, int, error) {
	return c.commonIndex(ctx, filter, per, page, needOpWeight)
}

func (c *datasetComponentImpl) CreateFork(ctx context.Context, req types.CreateForkReq) (*types.Dataset, error) {
	return c.createFork(ctx, req)
}

func (c *datasetComponentImpl) Refork(ctx context.Context, req types.CreateForkReq) (*types.Dataset, error) {
	return nil, nil
}

// NewDatasetComponent creates a new dataset component for CE version
func NewDatasetComponent(config *config.Config) (DatasetComponent, error) {
	c := &datasetComponentImpl{
		tagStore:               database.NewTagStore(),
		datasetStore:           database.NewDatasetStore(),
		repoStore:              database.NewRepoStore(),
		namespaceStore:         database.NewNamespaceStore(),
		userStore:              database.NewUserStore(),
		userLikesStore:         database.NewUserLikesStore(),
		recomStore:             database.NewRecomStore(),
		xnetMigrationTaskStore: database.NewXnetMigrationTaskStore(),
		lfsMetaObjectStore:     database.NewLfsMetaObjectStore(),
		extendDatasetImpl:      extendDatasetImpl{},
	}
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

func (c *datasetComponentImpl) createFork(ctx context.Context, req types.CreateForkReq) (*types.Dataset, error) {
	// 1. Call repo component's CreateFork method
	repo, err := c.repoComponent.CreateFork(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create fork repo, error: %w", err)
	}

	// 2. Create dataset record with forked=true and update repo path
	dataset := database.Dataset{
		RepositoryID:  repo.ID,
		LastUpdatedAt: time.Now(),
		DatasetType:   "normal",
		Forked:        true,
	}

	finalPath := path.Join(req.TargetNamespace, req.TargetName)
	newDataset, err := c.datasetStore.CreateAndUpdateRepoPath(ctx, dataset, finalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataset record, error: %w", err)
	}

	// 3. Build and return types.Dataset
	resDataset := &types.Dataset{
		ID:               newDataset.ID,
		Name:             repo.Name,
		Nickname:         repo.Nickname,
		Description:      repo.Description,
		Likes:            repo.Likes,
		Downloads:        repo.DownloadCount,
		Path:             finalPath,
		RepositoryID:     repo.ID,
		Private:          repo.Private,
		CreatedAt:        repo.CreatedAt,
		UpdatedAt:        repo.UpdatedAt,
		DatasetType:      newDataset.DatasetType,
		RelatedDatasetID: newDataset.RelatedDatasetID,
		Price:            newDataset.Price,
		Forked:           newDataset.Forked,
	}

	return resDataset, nil
}
