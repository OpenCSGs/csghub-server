package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
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
	initCommitMessage = "initial commit"
)

func NewDatasetComponent(config *config.Config) (*DatasetComponent, error) {
	c := &DatasetComponent{}
	c.ds = database.NewDatasetStore()
	c.ns = database.NewNamespaceStore()
	c.us = database.NewUserStore()
	c.ts = database.NewTagStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
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
	client, err := oss.New(config.S3.Endpoint, config.S3.AccessKeyID, config.S3.AccessKeySecret)
	if err != nil {
		newError := fmt.Errorf("fail to init oss client for dataset,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.ossBucket, err = client.Bucket(config.S3.Bucket)
	if err != nil {
		newError := fmt.Errorf("fail to init oss bucket for dataset,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type DatasetComponent struct {
	ds        *database.DatasetStore
	ns        *database.NamespaceStore
	us        *database.UserStore
	ts        *database.TagStore
	gs        gitserver.GitServer
	tc        *TagComponent
	ossBucket *oss.Bucket
}

func (c *DatasetComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	slog.Debug("creating file get request", slog.String("namespace", req.NameSpace), slog.String("filepath", req.FilePath))
	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
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

	err = c.gs.CreateDatasetFile(req)
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
	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	return &resp, err
}
func (c *DatasetComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("update file get request", slog.String("namespace", req.NameSpace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
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

	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, err
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
	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update dataset file, cause: %w", err)
	}

	return resp, err
}

func (c *DatasetComponent) Create(ctx context.Context, req *types.CreateDatasetReq) (dataset *database.Dataset, err error) {
	namespace, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		if namespace.UserID != user.ID {
			return nil, errors.New("users do not have permission to create models in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, errors.New("users do not have permission to create models in this namespace")
		}
	}

	dataset, repo, err := c.gs.CreateDatasetRepo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git dataset repository, cause: %w", err)
	}

	dataset, err = c.ds.Create(ctx, dataset, repo, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(createGitattributesReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(createReadmeReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	return
}

func createGitattributesReq(r *types.CreateDatasetReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(datasetGitattributesContent)),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  ".gitattributes",
	}
}

func createReadmeReq(r *types.CreateDatasetReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(generateReadmeData(r.License))),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  "README.md",
	}
}

func generateReadmeData(license string) string {
	return `
---
license: ` + license + `
---
	`
}

func (c *DatasetComponent) Index(ctx context.Context, username, search, sort string, tags []database.TagReq, per, page int) ([]database.Dataset, int, error) {
	var user database.User
	var err error
	if username == "" {
		slog.Info("get datasets without current username")
	} else {
		user, err = c.us.FindByUsername(ctx, username)
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
	return datasets, total, nil
}

func (c *DatasetComponent) Update(ctx context.Context, req *types.UpdateDatasetReq) (*database.Dataset, error) {
	_, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.OriginName)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	err = c.gs.UpdateDatasetRepo(req.Namespace, req.OriginName, dataset, dataset.Repository, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update git dataset repository, error: %w", err)
	}

	err = c.ds.Update(ctx, dataset, dataset.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to update database dataset, error: %w", err)
	}

	return dataset, nil
}

func (c *DatasetComponent) Delete(ctx context.Context, namespace, name string) error {
	_, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find dataset, error: %w", err)
	}
	err = c.gs.DeleteDatasetRepo(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete git dataset repository, error: %w", err)
	}

	err = c.ds.Delete(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete database dataset, error: %w", err)
	}
	return nil
}

func (c *DatasetComponent) Detail(ctx context.Context, namespace, name string) (*types.DatasetDetail, error) {
	_, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	detail, err := c.gs.GetDatasetDetail(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset detail, error: %w", err)
	}

	return detail, nil
}

func (c *DatasetComponent) Show(ctx context.Context, namespace, name, current_user string) (*database.Dataset, error) {
	dataset, err := c.ds.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	if dataset.Private {
		if dataset.User.Username != current_user {
			return nil, fmt.Errorf("failed to find dataset, error: %w", errors.New("the private dataset is not accessible to the current user"))
		}
	}

	return dataset, nil
}

func (c *DatasetComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]*types.Commit, error) {
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	commits, err := c.gs.GetDatasetCommits(req.Namespace, req.Name, req.Ref, req.Per, req.Page)
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
	commit, err := c.gs.GetDatasetLastCommit(req.Namespace, req.Name, req.Ref)
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
	raw, err := c.gs.GetDatasetFileRaw(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get git dataset repository file raw, error: %w", err)
	}
	return raw, nil
}

func (c *DatasetComponent) DownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		reader io.ReadCloser
		url    string
	)
	dataset, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	if req.Lfs {
		objectKey := "lfs" + req.Path
		url, err = c.ossBucket.SignURL(objectKey, oss.HTTPGet, 600)
		if err != nil {
			return nil, url, err
		}
		return reader, url, nil
	} else {
		reader, err = c.gs.GetDatasetFileReader(req.Namespace, req.Name, req.Ref, req.Path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git dataset repository file, error: %w", err)
		}
		return reader, url, nil
	}
}

func (c *DatasetComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]*types.DatasetBranch, error) {
	_, err := c.ds.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	bs, err := c.gs.GetDatasetBranches(req.Namespace, req.Name, req.Per, req.Page)
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
	tree, err := c.gs.GetDatasetFileTree(req.Namespace, req.Name, req.Ref, req.Path)
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

	err = c.ds.UpdateRepoDownloads(ctx, dataset, req.Date, req.DownloadCount)
	if err != nil {
		return fmt.Errorf("failed to update dataset download count, error: %w", err)
	}
	return err
}
