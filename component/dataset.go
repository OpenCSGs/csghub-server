package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
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
		Repository: types.Repository{
			HTTPCloneURL: dataset.Repository.HTTPCloneURL,
			SSHCloneURL:  dataset.Repository.SSHCloneURL,
		},
		Private: dataset.Repository.Private,
		User: types.User{
			Username: user.Username,
			Nickname: user.Name,
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

func (c *DatasetComponent) Index(ctx context.Context, username, search, sort string, tags []database.TagReq, per, page int) ([]types.Dataset, int, error) {
	var (
		user        database.User
		err         error
		resDatasets []types.Dataset
	)
	if username == "" {
		slog.Info("get datasets without current username")
	} else {
		user, err = c.user.FindByUsername(ctx, username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}
	}
	datasets, total, err := c.ds.PublicToUser(ctx, &user, search, sort, tags, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range datasets {
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
			Tags:         tags,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
		})
	}

	return resDatasets, total, nil
}

func (c *DatasetComponent) Update(ctx context.Context, req *types.UpdateDatasetReq) (*types.Dataset, error) {
	req.RepoType = types.DatasetRepo
	dbRepo, err := c.UpdateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

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

func (c *DatasetComponent) Show(ctx context.Context, namespace, name, current_user string) (*types.Dataset, error) {
	var tags []types.RepoTag
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	if dataset.Repository.Private {
		if dataset.Repository.User.Username != current_user {
			return nil, fmt.Errorf("failed to find dataset, error: %w", errors.New("the private dataset is not accessible to the current user"))
		}
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
		ID:            dataset.ID,
		Name:          dataset.Repository.Name,
		Nickname:      dataset.Repository.Nickname,
		Description:   dataset.Repository.Description,
		Likes:         dataset.Repository.Likes,
		Downloads:     dataset.Repository.DownloadCount,
		Path:          dataset.Repository.Path,
		RepositoryID:  dataset.Repository.ID,
		DefaultBranch: dataset.Repository.DefaultBranch,
		Repository: types.Repository{
			HTTPCloneURL: dataset.Repository.HTTPCloneURL,
			SSHCloneURL:  dataset.Repository.SSHCloneURL,
		},
		Tags: tags,
		User: types.User{
			Username: dataset.Repository.User.Username,
			Nickname: dataset.Repository.User.Name,
			Email:    dataset.Repository.User.Email,
		},
		Private:   dataset.Repository.Private,
		CreatedAt: dataset.CreatedAt,
		UpdatedAt: dataset.UpdatedAt,
	}

	return resDataset, nil
}

func fileIsExist(tree []*types.File, path string) (*types.File, bool) {
	for _, f := range tree {
		if f.Path == path {
			return f, true
		}
	}
	return nil, false
}
