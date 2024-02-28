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
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/inference"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
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
*.gguf filter=lfs diff=lfs merge=lfs -text
*.ggml filter=lfs diff=lfs merge=lfs -text
*.pdparams filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
`
const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

func NewModelComponent(config *config.Config) (*ModelComponent, error) {
	c := &ModelComponent{}
	c.us = database.NewUserStore()
	c.ms = database.NewModelStore()
	c.os = database.NewOrgStore()
	c.ns = database.NewNamespaceStore()
	var err error
	c.gs, err = git.NewGitServer(config)
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
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for model,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.lfsBucket = config.S3.Bucket

	c.infer = inference.NewInferClient(config.Inference.ServerAddr)

	c.msc, err = NewMemberComponent(config)
	if err != nil {
		newError := fmt.Errorf("fail to create membership component,error:%w", err)
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
	infer     inference.App
	tc        *TagComponent
	s3Client  *minio.Client
	lfsBucket string

	msc *MemberComponent
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
		canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
		if err != nil {
			return nil, err
		}
		if !canWrite {
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
	m, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	detail, err := c.gs.GetModelDetail(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model detail, error: %w", err)
	}
	detail.Downloads = int(m.Downloads)

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

	// TODO:check sensitive content of file
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

	err = c.gs.CreateModelFile(req)
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
	err = c.gs.CreateModelFile(req)
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
	err = c.gs.UpdateModelFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create model file, error: %w", err)
	}
	// TODO:check sensitive content of file

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
	// need to handle tag change only if file renamed
	if isFileRenamed {
		err = c.tc.UpdateLibraryTags(ctx, database.ModelTagScope, req.NameSpace, req.Name, req.OriginPath, req.FilePath)
		if err != nil {
			slog.Error("failed to set model's tags", slog.String("namespace", req.NameSpace),
				slog.String("name", req.Name), slog.Any("error", err))
			return nil, fmt.Errorf("failed to set model's tags, cause: %w", err)
		}
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

func (c *ModelComponent) FileInfo(ctx context.Context, req *types.GetFileReq) (*types.File, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	file, err := c.gs.GetModelFileContents(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository file info, error: %w", err)
	}
	return file, nil
}

func (c *ModelComponent) DownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		reader      io.ReadCloser
		downloadUrl string
	)
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find model, error: %w", err)
	}
	err = c.ms.UpdateRepoFileDownloads(ctx, model, time.Now(), 1)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update model file download count, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
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
		reader, err = c.gs.GetModelFileReader(req.Namespace, req.Name, req.Ref, req.Path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git model repository file, error: %w", err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *ModelComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]*types.ModelBranch, error) {
	_, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
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
		return nil, fmt.Errorf("failed to find model, error: %w", err)
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

	err = c.ms.UpdateRepoCloneDownloads(ctx, model, req.Date, req.CloneCount)
	if err != nil {
		return fmt.Errorf("failed to update model download count, error: %w", err)
	}
	return err
}

func (c *ModelComponent) UploadFile(ctx context.Context, req *types.CreateFileReq) error {
	parentPath := filepath.Dir(req.FilePath)
	if parentPath == "." {
		parentPath = "/"
	}
	tree, err := c.gs.GetModelFileTree(req.NameSpace, req.Name, req.Branch, parentPath)
	if err != nil {
		slog.Error("Error getting mode file tree: %w", err, slog.String("model", fmt.Sprintf("%s/%s", req.NameSpace, req.Name)), slog.String("file_path", req.FilePath))
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

func (c *ModelComponent) SDKListFiles(ctx *gin.Context, namespace, name string) (*types.SDKFiles, error) {
	var (
		sdkFiles    []types.SDKFile
		tags        []string
		currentUser any
		exists      bool
	)
	model, err := c.ms.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	currentUser, exists = ctx.Get("currentUser")

	// TODO: Use user access token to check permissions
	if model.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}

	filePaths, err := getFilePaths(namespace, name, "", c.gs.GetModelFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all model files, error: %w", err)
	}

	for _, tag := range model.Repository.Tags {
		tags = append(tags, tag.Name)
	}

	for _, filePath := range filePaths {
		sdkFiles = append(sdkFiles, types.SDKFile{Filename: filePath})
	}
	return &types.SDKFiles{
		ID:        fmt.Sprintf("%s/%s", namespace, name),
		Siblings:  sdkFiles,
		Private:   model.Private,
		Downloads: model.Downloads,
		Likes:     model.Likes,
		Tags:      tags,
		SHA:       model.Repository.DefaultBranch,
	}, nil
}

func getFilePaths(namespace, repoName, folder string, gsTree func(namespce, repoName, ref, path string) ([]*types.File, error)) ([]string, error) {
	var filePaths []string
	gitFiles, err := gsTree(namespace, repoName, "", folder)
	if err != nil {
		slog.Error("Failed to get repo file contents", slog.String("path", folder), slog.Any("error", err))
		return filePaths, err
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			subFileNames, err := getFilePaths(namespace, repoName, file.Path, gsTree)
			if err != nil {
				return filePaths, err
			}
			filePaths = append(filePaths, subFileNames...)
		} else {
			filePaths = append(filePaths, file.Path)
		}
	}
	return filePaths, nil
}

func (c *ModelComponent) IsLfs(ctx context.Context, req *types.GetFileReq) (bool, error) {
	content, err := c.gs.GetModelFileRaw(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		slog.Error("failed to get model file raw", slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("path", req.Path))
		return false, err
	}

	return strings.HasPrefix(content, LFSPrefix), nil
}

func (c *ModelComponent) HeadDownloadFile(ctx *gin.Context, req *types.GetFileReq) (*types.File, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")

	// TODO: Use user access token to check permissions
	if model.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}

	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	file, err := c.gs.GetModelFileContents(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to download git dataset repository file, error: %w", err)
	}
	return file, nil
}

func (c *ModelComponent) SDKDownloadFile(ctx *gin.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var downloadUrl string
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find model, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")

	// TODO: Use user access token to check permissions
	if model.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, "", err
		}
		if !canRead {
			return nil, "", fmt.Errorf("permission denied")
		}
	}
	if req.Ref == "" {
		req.Ref = model.Repository.DefaultBranch
	}
	if req.Lfs {
		file, err := c.gs.GetModelFileContents(req.Namespace, req.Name, req.Ref, req.Path)
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
		reader, err := c.gs.GetModelFileReader(req.Namespace, req.Name, req.Ref, req.Path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git model repository file, error: %w", err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *ModelComponent) Predict(ctx context.Context, req *types.ModelPredictReq) (*types.ModelPredictResp, error) {
	model, err := c.ms.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find model, error: %w", err)
	}

	mid := inference.ModelID{
		Owner:   model.User.Username,
		Name:    model.Name,
		Version: req.Version,
	}
	inferReq := &inference.PredictRequest{
		Prompt: req.Input,
	}
	inferResp, err := c.infer.Predict(mid, inferReq)
	if err != nil {
		slog.Error("failed to predict", slog.Any("req", *inferReq), slog.Any("model", mid), slog.String("error", err.Error()))
		return nil, err
	}
	resp := &types.ModelPredictResp{
		Content: inferResp.GeneratedText,
	}
	return resp, nil
}

func (c *ModelComponent) checkCurrentUserPermission(ctx context.Context, currentUser any, namespace string, role membership.Role) (bool, error) {
	cu, ok := currentUser.(string)
	if !ok {
		return false, fmt.Errorf("error parsing current user from context")
	}

	ns, err := c.ns.FindByPath(ctx, namespace)
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
		switch role {
		case membership.RoleAdmin:
			return r.CanAdmin(), nil
		case membership.RoleWrite:
			return r.CanWrite(), nil
		case membership.RoleRead:
			return r.CanRead(), nil
		default:
			return false, fmt.Errorf("unknown role %s", role)
		}
	}
}
