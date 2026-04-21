package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"slices"
	"time"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

// DatasetComponent defines the interface for dataset operations
type DatasetComponent interface {
	Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Dataset, int, error)
	Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight, needMultiSync bool) (*types.Dataset, error)
	GetByID(ctx context.Context, datasetID int64) (*types.Dataset, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgDatasets(ctx context.Context, req *types.OrgDatasetsReq) ([]types.Dataset, int, error)
	// CreateFork creates a fork of a dataset repository
	CreateFork(ctx context.Context, req types.CreateForkReq) (*types.Dataset, error)
	// Refork creates a fork of a dataset repository after user deletion, requires purchase check
	Refork(ctx context.Context, req types.CreateForkReq) (*types.Dataset, error)
	// BuyDataset buys a dataset
	BuyDataset(ctx context.Context, req *types.BuyDatasetReq) (*types.BuyDatasetResp, error)
}

// datasetComponentImpl is the base implementation of DatasetComponent
type datasetComponentImpl struct {
	config                 *config.Config
	repoComponent          RepoComponent
	tagStore               database.TagStore
	datasetStore           database.DatasetStore
	repoStore              database.RepoStore
	namespaceStore         database.NamespaceStore
	userStore              database.UserStore
	sensitiveComponent     SensitiveComponent
	gitServer              gitserver.GitServer
	userLikesStore         database.UserLikesStore
	userSvcClient          rpc.UserSvcClient
	recomStore             database.RecomStore
	xnetMigrationTaskStore database.XnetMigrationTaskStore
	lfsMetaObjectStore     database.LfsMetaObjectStore
	extendDatasetImpl
}

// getXnetMigrationProgress calculates the Xnet migration progress for a repository
func (c *datasetComponentImpl) getXnetMigrationProgress(ctx context.Context, repo *database.Repository) int {
	lfsMetaObjects, err := c.lfsMetaObjectStore.FindByRepoID(ctx, repo.ID)
	if err != nil || len(lfsMetaObjects) == 0 {
		return 0
	}

	var migratedCount int
	for _, obj := range lfsMetaObjects {
		if obj.XnetUsed {
			migratedCount++
		}
	}

	return (migratedCount * 100) / len(lfsMetaObjects)
}

func generateReadmeData(license string) string {
	return `
---
license: ` + license + `
---
	`
}

// Common methods for datasetComponentImpl

func (c *datasetComponentImpl) GetByID(ctx context.Context, datasetID int64) (*types.Dataset, error) {
	// Get dataset by ID
	dataset, err := c.datasetStore.ByID(ctx, datasetID)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset by ID, error: %w", err)
	}

	// Build and return types.Dataset
	return &types.Dataset{
		ID:               dataset.ID,
		Name:             dataset.Repository.Name,
		Nickname:         dataset.Repository.Nickname,
		Description:      dataset.Repository.Description,
		Likes:            dataset.Repository.Likes,
		Downloads:        dataset.Repository.DownloadCount,
		Path:             dataset.Repository.Path,
		RepositoryID:     dataset.Repository.ID,
		Private:          dataset.Repository.Private,
		CreatedAt:        dataset.CreatedAt,
		UpdatedAt:        dataset.Repository.UpdatedAt,
		DatasetType:      dataset.DatasetType,
		RelatedDatasetID: dataset.RelatedDatasetID,
		Price:            dataset.Price,
		Forked:           dataset.Forked,
	}, nil
}

func (c *datasetComponentImpl) Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("namespace does not exist, error: %w", err)
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("user does not exist, error: %w", err)
	}
	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.repoComponent.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create datasets in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to create datasets in this namespace")
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

	req.CommitFiles = []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: types.DatasetGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}
	_, dbRepo, commitFilesReq, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbDataset := database.Dataset{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	repoPath := path.Join(req.Namespace, req.Name)
	dataset, err := c.datasetStore.CreateAndUpdateRepoPath(ctx, dbDataset, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
	}

	_ = c.gitServer.CommitFiles(ctx, *commitFilesReq)

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
		URL:       dataset.Repository.Path,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.DatasetRepo,
			RepoPath:  dataset.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resDataset, nil
}

func (c *datasetComponentImpl) validateDatasetUpdate(ctx context.Context, req *types.UpdateDatasetReq) error {
	if req.DatasetType == "commercial" {
		if req.RelatedDatasetID <= 0 {
			return errorx.BadRequest(errors.New("related_dataset_id is required for commercial dataset"), errorx.Ctx().Set("related_dataset_id", req.RelatedDatasetID))
		}
		if req.Price <= 0 {
			return errorx.BadRequest(errors.New("price must be greater than 0 for commercial dataset"), errorx.Ctx().Set("price", req.Price))
		}
		// Check if related_dataset_id exists
		_, err := c.datasetStore.ByID(ctx, req.RelatedDatasetID)
		if err != nil {
			return errorx.BadRequest(errors.New("related_dataset_id does not exist"), errorx.Ctx().Set("related_dataset_id", req.RelatedDatasetID))
		}
	}
	return nil
}

func (c *datasetComponentImpl) Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error) {
	req.RepoType = types.DatasetRepo

	// Validation logic
	if err := c.validateDatasetUpdate(ctx, req); err != nil {
		return nil, err
	}

	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	dataset, err := c.datasetStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	// Update dataset fields
	if req.DatasetType != "" {
		dataset.DatasetType = req.DatasetType
	}
	if req.RelatedDatasetID > 0 {
		dataset.RelatedDatasetID = req.RelatedDatasetID
	}
	if req.Price > 0 {
		dataset.Price = req.Price
	}

	// Update dataset timestamp
	err = c.datasetStore.Update(ctx, *dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to update database dataset, error: %w", err)
	}

	resDataset := &types.Dataset{
		ID:               dataset.ID,
		Name:             dbRepo.Name,
		Nickname:         dbRepo.Nickname,
		Description:      dbRepo.Description,
		Likes:            dbRepo.Likes,
		Downloads:        dbRepo.DownloadCount,
		Path:             dbRepo.Path,
		RepositoryID:     dbRepo.ID,
		Private:          dbRepo.Private,
		CreatedAt:        dataset.CreatedAt,
		UpdatedAt:        dataset.UpdatedAt,
		DatasetType:      dataset.DatasetType,
		RelatedDatasetID: dataset.RelatedDatasetID,
		Price:            dataset.Price,
		Forked:           dataset.Forked,
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
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of dataset, error: %w", err)
	}

	err = c.datasetStore.Delete(ctx, *dataset)
	if err != nil {
		return fmt.Errorf("failed to delete database dataset, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.DatasetRepo,
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
		return nil, 0, fmt.Errorf("failed to get user datasets, error: %w", err)
	}

	for _, data := range datasets {
		resDatasets = append(resDatasets, types.Dataset{
			ID:               data.ID,
			Name:             data.Repository.Name,
			Nickname:         data.Repository.Nickname,
			Description:      data.Repository.Description,
			Likes:            data.Repository.Likes,
			Downloads:        data.Repository.DownloadCount,
			Path:             data.Repository.Path,
			RepositoryID:     data.RepositoryID,
			Private:          data.Repository.Private,
			CreatedAt:        data.CreatedAt,
			UpdatedAt:        data.Repository.UpdatedAt,
			DatasetType:      data.DatasetType,
			RelatedDatasetID: data.RelatedDatasetID,
			Price:            data.Price,
			Forked:           data.Forked,
		})
	}

	return resDatasets, total, nil
}

func (c *datasetComponentImpl) updateWeightToDataset(dataset *types.Dataset, newScore types.WeightScore) {
	for i := range len(dataset.Scores) {
		if dataset.Scores[i].WeightName == newScore.WeightName {
			dataset.Scores[i].Score = newScore.Score
			return
		}
	}
	dataset.Scores = append(dataset.Scores, newScore)
}

func (c *datasetComponentImpl) addWeightsToDataset(ctx context.Context, repoID int64, resDatasets *types.Dataset, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		if resDatasets.Scores == nil {
			resDatasets.Scores = make([]types.WeightScore, 0)
		}
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				c.updateWeightToDataset(resDatasets, score)
			}
		}
	}
}
