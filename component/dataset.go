package component

import (
	"context"
	"errors"
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const datasetGitattributesContent = `*.7z filter=lfs diff=lfs merge=lfs -text
*.arrow filter=lfs diff=lfs merge=lfs -text
*.bin filter=lfs diff=lfs merge=lfs -text
*.bz2 filter=lfs diff=lfs merge=lfs -text
*.ckpt filter=lfs diff=lfs merge=lfs -text
*.ftz filter=lfs diff=lfs merge=lfs -text
*.gz filter=lfs diff=lfs merge=lfs -text
*.h5 filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
*.lfs.* filter=lfs diff=lfs merge=lfs -text
*.lz4 filter=lfs diff=lfs merge=lfs -text
*.mlmodel filter=lfs diff=lfs merge=lfs -text
*.model filter=lfs diff=lfs merge=lfs -text
*.msgpack filter=lfs diff=lfs merge=lfs -text
*.npy filter=lfs diff=lfs merge=lfs -text
*.npz filter=lfs diff=lfs merge=lfs -text
*.onnx filter=lfs diff=lfs merge=lfs -text
*.ot filter=lfs diff=lfs merge=lfs -text
*.parquet filter=lfs diff=lfs merge=lfs -text
*.pb filter=lfs diff=lfs merge=lfs -text
*.pickle filter=lfs diff=lfs merge=lfs -text
*.pkl filter=lfs diff=lfs merge=lfs -text
*.pt filter=lfs diff=lfs merge=lfs -text
*.pth filter=lfs diff=lfs merge=lfs -text
*.rar filter=lfs diff=lfs merge=lfs -text
*.safetensors filter=lfs diff=lfs merge=lfs -text
saved_model/**/* filter=lfs diff=lfs merge=lfs -text
*.tar.* filter=lfs diff=lfs merge=lfs -text
*.tar filter=lfs diff=lfs merge=lfs -text
*.tflite filter=lfs diff=lfs merge=lfs -text
*.tgz filter=lfs diff=lfs merge=lfs -text
*.wasm filter=lfs diff=lfs merge=lfs -text
*.xz filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
*.zst filter=lfs diff=lfs merge=lfs -text
*tfevents* filter=lfs diff=lfs merge=lfs -text
# Audio files - uncompressed
*.pcm filter=lfs diff=lfs merge=lfs -text
*.sam filter=lfs diff=lfs merge=lfs -text
*.raw filter=lfs diff=lfs merge=lfs -text
# Audio files - compressed
*.aac filter=lfs diff=lfs merge=lfs -text
*.flac filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
# Image files - uncompressed
*.bmp filter=lfs diff=lfs merge=lfs -text
*.gif filter=lfs diff=lfs merge=lfs -text
*.png filter=lfs diff=lfs merge=lfs -text
*.tiff filter=lfs diff=lfs merge=lfs -text
# Image files - compressed
*.jpg filter=lfs diff=lfs merge=lfs -text
*.jpeg filter=lfs diff=lfs merge=lfs -text
*.webp filter=lfs diff=lfs merge=lfs -text

`

const (
	initCommitMessage     = "initial commit"
	ossFileExpireSeconds  = 259200 * time.Second
	readmeFileName        = "README.md"
	gitattributesFileName = ".gitattributes"
)

func NewDatasetComponent(config *config.Config) (*DatasetComponent, error) {
	c := &DatasetComponent{}
	c.ts = database.NewTagStore()
	c.ds = database.NewDatasetStore()
	c.rs = database.NewRepoStore()
	var err error
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

type DatasetComponent struct {
	*RepoComponent
	ts *database.TagStore
	ds *database.DatasetStore
	rs *database.RepoStore
}

func (c *DatasetComponent) Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
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
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbDataset := database.Dataset{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	dataset, err := c.ds.Create(ctx, dbDataset)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
	}

	// Create README.md file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  readmeFileName,
	}, types.DatasetRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   datasetGitattributesContent,
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
			ShowName:  tag.ShowName,
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

func (c *DatasetComponent) Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Dataset, int, error) {
	var (
		err         error
		resDatasets []types.Dataset
	)
	repos, total, err := c.PublicToUser(ctx, types.DatasetRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public dataset repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	datasets, err := c.ds.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get datasets by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
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
				ShowName:  tag.ShowName,
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
		})
	}

	return resDatasets, total, nil
}

func (c *DatasetComponent) Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error) {
	req.RepoType = types.DatasetRepo
	dbRepo, err := c.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	dataset, err := c.ds.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	// update times of dateset
	err = c.ds.Update(ctx, *dataset)
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

func (c *DatasetComponent) Delete(ctx context.Context, namespace, name, currentUser string) error {
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find dataset, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.DatasetRepo,
	}
	_, err = c.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of dataset, error: %w", err)
	}

	err = c.ds.Delete(ctx, *dataset)
	if err != nil {
		return fmt.Errorf("failed to delete database dataset, error: %w", err)
	}
	return nil
}

func (c *DatasetComponent) Show(ctx context.Context, namespace, name, currentUser string) (*types.Dataset, error) {
	var tags []types.RepoTag
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	permission, err := c.getUserRepoPermission(ctx, currentUser, dataset.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	ns, err := c.getNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for dataset, error: %w", err)
	}

	for _, tag := range dataset.Repository.Tags {
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

	likeExists, err := c.uls.IsExist(ctx, currentUser, dataset.Repository.ID)
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
		},
		Private:    dataset.Repository.Private,
		CreatedAt:  dataset.CreatedAt,
		UpdatedAt:  dataset.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     dataset.Repository.Source,
		SyncStatus: dataset.Repository.SyncStatus,
		License:    dataset.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
	}

	return resDataset, nil
}

func (c *DatasetComponent) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset repo, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, dataset.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	return c.getRelations(ctx, dataset.RepositoryID, currentUser)
}

func (c *DatasetComponent) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.relatedRepos(ctx, repoID, currentUser)
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
