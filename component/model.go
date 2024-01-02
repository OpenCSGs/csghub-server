package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"path/filepath"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
)

const modelGitattributesContent = `*.7z filter=lfs diff=lfs merge=lfs -text
*.arrow filter=lfs diff=lfs merge=lfs -text
*.bin filter=lfs diff=lfs merge=lfs -text
*.bz2 filter=lfs diff=lfs merge=lfs -text
*.ckpt filter=lfs diff=lfs merge=lfs -text
*.ftz filter=lfs diff=lfs merge=lfs -text
*.gz filter=lfs diff=lfs merge=lfs -text
*.h5 filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
*.lfs.* filter=lfs diff=lfs merge=lfs -text
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

`

func NewModelComponent(config *config.Config) (*ModelComponent, error) {
	c := &ModelComponent{}
	c.us = database.NewUserStore()
	c.ms = database.NewModelStore()
	c.os = database.NewOrgStore()
	c.ns = database.NewNamespaceStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
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

type ModelComponent struct {
	us        *database.UserStore
	ms        *database.ModelStore
	os        *database.OrgStore
	ns        *database.NamespaceStore
	gs        gitserver.GitServer
	tc        *TagComponent
	ossBucket *oss.Bucket
}

func (c *ModelComponent) Index(ctx context.Context, username, search, sort string, ragReqs []database.TagReq, per, page int) ([]database.Model, int, error) {
	var user database.User
	var err error
	if username == "" {
		slog.Info("get models without current username")
	} else {
		user, err = c.us.FindByUsername(ctx, username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}
	}
	models, total, err := c.ms.PublicToUser(ctx, &user, search, sort, ragReqs, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public models,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}
	return models, total, nil
}

func (c *ModelComponent) Create(ctx context.Context, req *types.CreateModelReq) (*database.Model, error) {
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

	model, repo, err := c.gs.CreateModelRepo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git model repository, error: %w", err)
	}

	model, err = c.ms.Create(ctx, model, repo, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create database model, error: %w", err)
	}

	err = c.gs.CreateModelFile(createModelGitattributesReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	err = c.gs.CreateModelFile(createModelReadmeReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	return model, nil
}

func createModelGitattributesReq(r *types.CreateModelReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(modelGitattributesContent)),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  ".gitattributes",
	}
}

func createModelReadmeReq(r *types.CreateModelReq, user database.User) *types.CreateFileReq {
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

func (c *ModelComponent) Update(ctx context.Context, req *types.UpdateModelReq) (*database.Model, error) {
	_, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	model, err := c.ms.FindByPath(ctx, req.Namespace, req.OriginName)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	err = c.gs.UpdateModelRepo(req.Namespace, req.OriginName, model, model.Repository, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update git model repository, error: %w", err)
	}

	err = c.ms.Update(ctx, model, model.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to update database model, error: %w", err)
	}

	return model, nil
}

func (c *ModelComponent) Delete(ctx context.Context, namespace, name string) error {
	_, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}
	err = c.gs.DeleteModelRepo(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete git model repository, error: %w", err)
	}

	err = c.ms.Delete(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete database model, error: %w", err)
	}
	return nil
}

func (c *ModelComponent) Detail(ctx context.Context, namespace, name string) (*types.ModelDetail, error) {
	_, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	detail, err := c.gs.GetModelDetail(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model detail, error: %w", err)
	}

	return detail, nil
}

func (c *ModelComponent) Show(ctx context.Context, namespace, name, current_user string) (*database.Model, error) {
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	if model.Private {
		if model.User.Username != current_user {
			return nil, fmt.Errorf("failed to find model, error: %w", errors.New("the private model is not accessible to the current user"))
		}
	}

	return model, nil
}

func (c *ModelComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	_, err := c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find username, error: %w", err)
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

func (c *ModelComponent) createReadmeFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, database.ModelTagScope, req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create model file, cause: %w", err)
	}

	return &resp, err
}

func (c *ModelComponent) createLibraryFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)

	err = c.tc.UpdateLibraryTags(ctx, database.ModelTagScope, req.NameSpace, req.Name, "", req.FilePath)
	if err != nil {
		slog.Error("failed to set model's tags", slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set model's tags, cause: %w", err)
	}
	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	return &resp, err
}

func (c *ModelComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	_, err := c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find username, error: %w", err)
	}
	err = c.gs.UpdateModelFile(req.NameSpace, req.Name, req.FilePath, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create model file, error: %w", err)
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

func (c *ModelComponent) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	var err error
	resp := &types.UpdateFileResp{}

	isFileRenamed := req.FilePath != req.OriginPath
	//need to handle tag change only if file renamed
	if isFileRenamed {
		c.tc.UpdateLibraryTags(ctx, database.ModelTagScope, req.NameSpace, req.Name, req.OriginPath, req.FilePath)
		if err != nil {
			slog.Error("failed to set model's tags", slog.String("namespace", req.NameSpace),
				slog.String("name", req.Name), slog.Any("error", err))
			return nil, fmt.Errorf("failed to set model's tags, cause: %w", err)
		}
	}

	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *ModelComponent) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("file is readme", slog.String("content", req.Content))
	var err error
	resp := new(types.UpdateFileResp)

	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, database.ModelTagScope, req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}
	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update model file, cause: %w", err)
	}

	return resp, err
}

func (c *ModelComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]*types.Commit, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	commits, err := c.gs.GetModelCommits(req.Namespace, req.Name, req.Ref, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository commits, error: %w", err)
	}
	return commits, nil
}

func (c *ModelComponent) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	commit, err := c.gs.GetModelLastCommit(req.Namespace, req.Name, req.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository last commit, error: %w", err)
	}
	return commit, nil
}

func (c *ModelComponent) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	raw, err := c.gs.GetModelFileRaw(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get git model repository file raw, error: %w", err)
	}
	return raw, nil
}

func (c *ModelComponent) DownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		reader io.ReadCloser
		url    string
	)
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	if req.Lfs {
		objectKey := path.Join("lfs", req.Path)
		if req.SaveAs != "" {
			opt := oss.ContentDisposition(fmt.Sprintf(`attachment; filename="%s"`, req.SaveAs))
			c.ossBucket.SetObjectMeta(objectKey, opt)
		}
		url, err = c.ossBucket.SignURL(objectKey, oss.HTTPGet, ossFileExpireSeconds)
		if err != nil {
			return nil, url, err
		}
		return nil, url, nil
	} else {
		reader, err = c.gs.GetModelFileReader(req.Namespace, req.Name, req.Ref, req.Path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git dataset repository file, error: %w", err)
		}
		return reader, url, nil
	}
}

func (c *ModelComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]*types.ModelBranch, error) {
	_, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	bs, err := c.gs.GetModelBranches(req.Namespace, req.Name, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository branches, error: %w", err)
	}
	return bs, nil
}

func (c *ModelComponent) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	_, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	tags, err := c.ms.Tags(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get model tags, error: %w", err)
	}
	return tags, nil
}

func (c *ModelComponent) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	tree, err := c.gs.GetModelFileTree(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository file tree, error: %w", err)
	}
	return tree, nil
}

func (c *ModelComponent) UpdateDownloads(ctx context.Context, req *types.UpdateDownloadsReq) error {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find model, error: %w", err)
	}

	err = c.ms.UpdateRepoDownloads(ctx, model, req.Date, req.DownloadCount)
	if err != nil {
		return fmt.Errorf("failed to update model download count, error: %w", err)
	}
	return err
}
