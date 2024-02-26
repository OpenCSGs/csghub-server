package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
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
	c.ds = database.NewDatasetStore()
	c.namespace = database.NewNamespaceStore()
	c.user = database.NewUserStore()
	c.org = database.NewOrgStore()
	c.repo = database.NewRepoStore()
	c.ts = database.NewTagStore()
	var err error
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.tc, err = NewTagComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create tag component,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for dataset,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.lfsBucket = config.S3.Bucket
	c.msc, err = NewMemberComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create membership component,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type DatasetComponent struct {
	repoComponent
	ds        *database.DatasetStore
	ts        *database.TagStore
	tc        *TagComponent
	s3Client  *minio.Client
	lfsBucket string
	msc       *MemberComponent
}

func (c *DatasetComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	slog.Debug("creating file get request", slog.String("namespace", req.NameSpace), slog.String("filepath", req.FilePath))
	var err error
	_, err = c.namespace.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	//TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		return c.createReadmeFile(ctx, req)
	} else {
		return c.createLibraryFile(ctx, req)
	}
}

func (c *DatasetComponent) createReadmeFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, database.DatasetTagScope, req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.git.CreateRepoFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataset file, cause: %w", err)
	}

	return &resp, err
}

func (c *DatasetComponent) createLibraryFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)

	err = c.tc.UpdateLibraryTags(ctx, database.DatasetTagScope, req.NameSpace, req.Name, "", req.FilePath)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}
	err = c.git.CreateRepoFile(req)
	if err != nil {
		return nil, err
	}

	return &resp, err
}
func (c *DatasetComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("update file get request", slog.String("namespace", req.NameSpace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var err error
	_, err = c.namespace.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}

	err = c.git.UpdateRepoFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update dataset file, cause: %w", err)
	}
	//TODO:check sensitive content of file

	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		return c.updateReadmeFile(ctx, req)
	} else {
		slog.Debug("file is not readme", slog.String("filePath", req.FilePath), slog.String("originPath", req.OriginPath))
		return c.updateLibraryFile(ctx, req)
	}
}

func (c *DatasetComponent) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	var err error
	resp := &types.UpdateFileResp{}

	isFileRenamed := req.FilePath != req.OriginPath
	//need to handle tag change only if file renamed
	if isFileRenamed {
		c.tc.UpdateLibraryTags(ctx, database.DatasetTagScope, req.NameSpace, req.Name, req.OriginPath, req.FilePath)
		if err != nil {
			slog.Error("failed to set dataset's tags", slog.String("namespace", req.NameSpace),
				slog.String("name", req.Name), slog.Any("error", err))
			return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
		}
	}

	return resp, err
}

func (c *DatasetComponent) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("file is readme", slog.String("content", req.Content))
	var err error
	resp := new(types.UpdateFileResp)

	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, database.DatasetTagScope, req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	return resp, err
}

func (c *DatasetComponent) Create(ctx context.Context, req *types.CreateDatasetReq) (*types.Dataset, error) {
	var nickname string
	req.RepoType = types.DatasetRepo
	req.Readme = generateReadmeData(req.License)
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}
	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	dbDataset := database.Dataset{
		Repository:   dbRepo,
		UrlSlug:      nickname,
		RepositoryID: dbRepo.ID,
	}

	dataset, err := c.ds.Create(ctx, dbDataset)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
	}

	user, err := c.user.FindByID(ctx, int(dbRepo.UserID))
	if err != nil {
		return nil, fmt.Errorf("failed to find user by id, cause: %w", err)
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

	resDataset := &types.Dataset{
		ID:           dataset.ID,
		Name:         dataset.Repository.Name,
		Nickname:     dataset.UrlSlug,
		Description:  dataset.Repository.Description,
		Likes:        dataset.Likes,
		Downloads:    dataset.Downloads,
		Path:         dataset.Repository.Path,
		RepositoryID: dataset.RepositoryID,
		Private:      dataset.Repository.Private,
		Username:     user.Username,
		CreatedAt:    dataset.CreatedAt,
		UpdatedAt:    dataset.UpdatedAt,
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
		resDatasets = append(resDatasets, types.Dataset{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.UrlSlug,
			Description:  data.Repository.Description,
			Likes:        data.Likes,
			Downloads:    data.Downloads,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
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

	dataset.UrlSlug = req.Nickname

	err = c.ds.Update(ctx, *dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to update database dataset, error: %w", err)
	}

	resDataset := &types.Dataset{
		ID:           dataset.ID,
		Name:         dbRepo.Name,
		Nickname:     dataset.UrlSlug,
		Description:  dbRepo.Description,
		Likes:        dataset.Likes,
		Downloads:    dataset.Downloads,
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

func (c *DatasetComponent) Show(ctx context.Context, namespace, name, current_user string) (*database.Dataset, error) {
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	if dataset.Repository.Private {
		if dataset.Repository.User.Username != current_user {
			return nil, fmt.Errorf("failed to find dataset, error: %w", errors.New("the private dataset is not accessible to the current user"))
		}
	}

	return dataset, nil
}

func (c *DatasetComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]types.Commit, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	getCommitsReq := gitserver.GetRepoCommitsReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Per:       req.Per,
		Page:      req.Page,
		RepoType:  types.DatasetRepo,
	}
	commits, err := c.git.GetRepoCommits(ctx, getCommitsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository commits, error: %w", err)
	}
	return commits, nil
}

func (c *DatasetComponent) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	getLastCommitReq := gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		RepoType:  types.DatasetRepo,
	}
	commit, err := c.git.GetRepoLastCommit(ctx, getLastCommitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository last commit, error: %w", err)
	}
	return commit, nil
}

func (c *DatasetComponent) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	raw, err := c.git.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		return "", fmt.Errorf("failed to get git dataset repository file raw, error: %w", err)
	}
	return raw, nil
}

func (c *DatasetComponent) DownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		reader      io.ReadCloser
		downloadUrl string
	)
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find dataset, error: %w", err)
	}
	err = c.ds.UpdateRepoFileDownloads(ctx, dataset, time.Now(), 1)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update dataset file download count, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	if req.Lfs {
		objectKey := path.Join("lfs", req.Path)

		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, ossFileExpireSeconds, reqParams)
		if err != nil {
			return nil, downloadUrl, err
		}
		return nil, signedUrl.String(), nil
	} else {
		getFileReaderReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  types.DatasetRepo,
		}
		reader, err = c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git dataset repository file, error: %w", err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *DatasetComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]types.Branch, error) {
	_, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	getBranchesReq := gitserver.GetBranchesReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Per:       req.Per,
		Page:      req.Page,
		RepoType:  types.DatasetRepo,
	}
	bs, err := c.git.GetRepoBranches(ctx, getBranchesReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository branches, error: %w", err)
	}
	return bs, nil
}

func (c *DatasetComponent) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	_, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	tags, err := c.ds.Tags(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset tags, error: %w", err)
	}
	return tags, nil
}

func (c *DatasetComponent) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	tree, err := c.git.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository file tree, error: %w", err)
	}
	return tree, nil
}

func (c *DatasetComponent) UpdateDownloads(ctx context.Context, req *types.UpdateDownloadsReq) error {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find dataset, error: %w", err)
	}

	err = c.ds.UpdateRepoCloneDownloads(ctx, dataset, req.Date, req.CloneCount)
	if err != nil {
		return fmt.Errorf("failed to update dataset download count, error: %w", err)
	}
	return err
}

func (c *DatasetComponent) UploadFile(ctx context.Context, req *types.CreateFileReq) error {
	parentPath := filepath.Dir(req.FilePath)
	if parentPath == "." {
		parentPath = "/"
	}
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.NameSpace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      parentPath,
		RepoType:  types.DatasetRepo,
	}
	tree, err := c.git.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		slog.Error("Error getting dataset file tree: %w", err, slog.String("dataset", fmt.Sprintf("%s/%s", req.NameSpace, req.Name)), slog.String("file_path", req.FilePath))
		return err
	}
	file, exists := fileIsExist(tree, req.FilePath)
	if !exists {
		_, err = c.CreateFile(ctx, req)
		if err != nil {
			return err
		}
		return nil
	}
	var updateFileReq types.UpdateFileReq

	updateFileReq.Username = req.Username
	updateFileReq.Email = req.Email
	updateFileReq.Message = req.Message
	updateFileReq.Branch = req.Branch
	updateFileReq.Content = req.Content
	updateFileReq.NameSpace = req.NameSpace
	updateFileReq.Name = req.Name
	updateFileReq.FilePath = req.FilePath
	updateFileReq.SHA = file.SHA

	_, err = c.UpdateFile(ctx, &updateFileReq)

	return err
}

func fileIsExist(tree []*types.File, path string) (*types.File, bool) {
	for _, f := range tree {
		if f.Path == path {
			return f, true
		}
	}
	return nil, false
}

func (c *DatasetComponent) SDKListFiles(ctx *gin.Context, namespace, name string) (*types.SDKFiles, error) {
	var sdkFiles []types.SDKFile
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if dataset.Repository.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, namespace)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}

	filePaths, err := getFilePaths(namespace, name, "", types.DatasetRepo, c.git.GetRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all dataset files, error: %w", err)
	}

	for _, filePath := range filePaths {
		sdkFiles = append(sdkFiles, types.SDKFile{Filename: filePath})
	}
	return &types.SDKFiles{
		ID:        fmt.Sprintf("%s/%s", namespace, name),
		Siblings:  sdkFiles,
		Private:   dataset.Repository.Private,
		Downloads: dataset.Downloads,
		Likes:     dataset.Likes,
		Tags:      []string{},
		SHA:       dataset.Repository.DefaultBranch,
	}, nil
}

func (c *DatasetComponent) IsLfs(ctx context.Context, req *types.GetFileReq) (bool, error) {
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	content, err := c.git.GetRepoFileRaw(ctx, getFileRawReq)

	if err != nil {
		slog.Error("failed to get dataset file raw", slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("path", req.Path))
		return false, err
	}

	return strings.HasPrefix(content, LFSPrefix), nil
}

func (c *DatasetComponent) HeadDownloadFile(ctx *gin.Context, req *types.GetFileReq) (*types.File, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if dataset.Repository.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  types.DatasetRepo,
	}
	file, err := c.git.GetRepoFileContents(ctx, getFileContentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to download git dataset repository file, error: %w", err)
	}
	return file, nil
}

func (c *DatasetComponent) SDKDownloadFile(ctx *gin.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		downloadUrl string
	)
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find dataset, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if dataset.Repository.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace)
		if err != nil {
			return nil, "", err
		}
		if !canRead {
			return nil, "", fmt.Errorf("permission denied")
		}
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	if req.Lfs {
		getFileContentReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  types.DatasetRepo,
		}
		file, err := c.git.GetRepoFileContents(ctx, getFileContentReq)
		if err != nil {
			return nil, "", err
		}
		objectKey := file.LfsRelativePath
		objectKey = path.Join("lfs", objectKey)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, ossFileExpireSeconds, reqParams)
		if err != nil {
			return nil, downloadUrl, err
		}
		return nil, signedUrl.String(), nil
	} else {
		getFileReaderReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  types.DatasetRepo,
		}
		reader, err := c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git dataset repository file, error: %w", err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *DatasetComponent) checkCurrentUserPermission(ctx context.Context, currentUser any, namespace string) (bool, error) {
	cu, ok := currentUser.(string)
	if !ok {
		return false, fmt.Errorf("error parsing current user from context")
	}

	ns, err := c.namespace.FindByPath(ctx, namespace)
	if err != nil {
		return false, err
	}

	if ns.NamespaceType == "user" {
		return cu == namespace, nil
	} else {
		r, err := c.msc.GetMemberRole(ctx, namespace, cu)
		if err != nil {
			return false, err
		}
		return r.CanRead(), nil
	}
}
