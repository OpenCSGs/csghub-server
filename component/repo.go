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
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type RepoComponent struct {
	tc        *TagComponent
	user      *database.UserStore
	org       *database.OrgStore
	namespace *database.NamespaceStore
	repo      *database.RepoStore
	ds        *database.DatasetStore
	ms        *database.ModelStore
	cs        *database.CodeStore
	git       gitserver.GitServer
	s3Client  *minio.Client
	msc       *MemberComponent
	lfsBucket string
}

func NewRepoComponent(config *config.Config) (*RepoComponent, error) {
	c := &RepoComponent{}
	c.cs = database.NewCodeStore()
	c.namespace = database.NewNamespaceStore()
	c.user = database.NewUserStore()
	c.org = database.NewOrgStore()
	c.repo = database.NewRepoStore()
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
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
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

func (c *RepoComponent) CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, error) {
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
		if err != nil {
			return nil, nil, err
		}
		if !canWrite {
			return nil, nil, fmt.Errorf("users do not have permission to create %s in this organization", req.RepoType)
		}
	} else {
		if namespace.Path != user.Username {
			return nil, nil, fmt.Errorf("users do not have permission to create %s in this namespace", req.RepoType)
		}
	}

	gitRepoReq := gitserver.CreateRepoReq{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Name,
		License:       req.License,
		DefaultBranch: req.DefaultBranch,
		// Readme:        "Please introduce your space.",
		Readme:   req.Readme,
		Private:  req.Private,
		RepoType: req.RepoType,
	}
	gitRepo, err := c.git.CreateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to create repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, nil, fmt.Errorf("fail to create repo in git, error: %w", err)
	}

	dbRepo := database.Repository{
		UserID:         user.ID,
		Path:           path.Join(req.Namespace, req.Name),
		GitPath:        gitRepo.GitPath,
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        req.License,
		DefaultBranch:  gitRepo.DefaultBranch,
		RepositoryType: req.RepoType,
		HTTPCloneURL:   gitRepo.HttpCloneURL,
		SSHCloneURL:    gitRepo.SshCloneURL,
	}
	newDBRepo, err := c.repo.CreateRepo(ctx, dbRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to create database repo, error: %w", err)
	}

	return gitRepo, newDBRepo, nil
}

func (c *RepoComponent) UpdateRepo(ctx context.Context, req types.CreateRepoReq) (*database.Repository, error) {
	repo, err := c.repo.Find(ctx, req.Namespace, string(req.RepoType), req.Name)
	if err != nil {
		return nil, errors.New("repository does not exist")
	}

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
			return nil, errors.New("users do not have permission to update repo in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, errors.New("users do not have permission to update repo in this namespace")
		}
	}

	gitRepoReq := gitserver.UpdateRepoReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
		Description:   req.Description,
		DefaultBranch: req.DefaultBranch,
		Private:       req.Private,
		RepoType:      req.RepoType,
	}
	gitRepo, err := c.git.UpdateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in git, error: %w", err)
	}

	repo.Description = gitRepo.Description
	repo.Private = gitRepo.Private
	repo.DefaultBranch = gitRepo.DefaultBranch
	repo.Nickname = req.Nickname

	resRepo, err := c.repo.UpdateRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in database, error: %w", err)
	}

	return resRepo, nil
}

func (c *RepoComponent) DeleteRepo(ctx context.Context, req types.DeleteRepoReq) (*database.Repository, error) {
	repo, err := c.repo.Find(ctx, req.Namespace, string(req.RepoType), req.Name)
	if err != nil {
		return nil, errors.New("repository does not exist")
	}

	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		canWrite, err := c.checkCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleAdmin)
		if err != nil {
			return nil, err
		}
		if !canWrite {
			return nil, errors.New("users do not have permission to delete repo in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, errors.New("users do not have permission to delete repo in this namespace")
		}
	}

	deleteRepoReq := gitserver.DeleteRepoReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
	}
	err = c.git.DeleteRepo(ctx, deleteRepoReq)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to delete repo in git, error: %w", err)
	}

	err = c.repo.DeleteRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to delete repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to delete repo in database, error: %w", err)
	}

	return repo, nil
}

func (c *RepoComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
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
	// TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		return c.createReadmeFile(ctx, req)
	} else {
		return c.createLibraryFile(ctx, req)
	}
}

func (c *RepoComponent) createReadmeFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.git.CreateRepoFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s file, cause: %w", req.RepoType, err)
	}

	return &resp, err
}

func (c *RepoComponent) createLibraryFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var (
		err  error
		resp types.CreateFileResp
	)

	err = c.tc.UpdateLibraryTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, "", req.FilePath)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to set %s's tags", req.RepoType), slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set %s's tags, cause: %w", req.RepoType, err)
	}
	err = c.git.CreateRepoFile(req)
	if err != nil {
		return nil, err
	}

	return &resp, err
}

func (c *RepoComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
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
		return nil, fmt.Errorf("failed to update %s file, cause: %w", req.RepoType, err)
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

func (c *RepoComponent) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	var err error
	resp := &types.UpdateFileResp{}

	isFileRenamed := req.FilePath != req.OriginPath
	// need to handle tag change only if file renamed
	if isFileRenamed {
		c.tc.UpdateLibraryTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, req.OriginPath, req.FilePath)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set %s's tags", req.RepoType), slog.String("namespace", req.NameSpace),
				slog.String("name", req.Name), slog.Any("error", err))
			return nil, fmt.Errorf("failed to set %s's tags, cause: %w", req.RepoType, err)
		}
	}

	return resp, err
}

func (c *RepoComponent) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("file is readme", slog.String("content", req.Content))
	var err error
	resp := new(types.UpdateFileResp)

	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	return resp, err
}

func (c *RepoComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]types.Commit, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	getCommitsReq := gitserver.GetRepoCommitsReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Per:       req.Per,
		Page:      req.Page,
		RepoType:  req.RepoType,
	}
	commits, err := c.git.GetRepoCommits(ctx, getCommitsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository commits, error: %w", req.RepoType, err)
	}
	return commits, nil
}

func (c *RepoComponent) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	getLastCommitReq := gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		RepoType:  req.RepoType,
	}
	commit, err := c.git.GetRepoLastCommit(ctx, getLastCommitReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository last commit, error: %w", req.RepoType, err)
	}
	return commit, nil
}

func (c *RepoComponent) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to find repo, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	raw, err := c.git.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		return "", fmt.Errorf("failed to get git %s repository file raw, error: %w", req.RepoType, err)
	}
	return raw, nil
}

func (c *RepoComponent) DownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var (
		reader      io.ReadCloser
		downloadUrl string
	)
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find repo, error: %w", err)
	}
	err = c.repo.UpdateRepoFileDownloads(ctx, repo, time.Now(), 1)
	if err != nil {
		return nil, "", fmt.Errorf("failed to update %s file download count, error: %w", req.RepoType, err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
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
			RepoType:  req.RepoType,
		}
		reader, err = c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *RepoComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]types.Branch, error) {
	_, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	getBranchesReq := gitserver.GetBranchesReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Per:       req.Per,
		Page:      req.Page,
		RepoType:  req.RepoType,
	}
	bs, err := c.git.GetRepoBranches(ctx, getBranchesReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository branches, error: %w", req.RepoType, err)
	}
	return bs, nil
}

func (c *RepoComponent) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	_, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s, error: %w", req.RepoType, err)
	}
	tags, err := c.repo.Tags(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s tags, error: %w", req.RepoType, err)
	}
	return tags, nil
}

func (c *RepoComponent) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	tree, err := c.git.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository file tree, error: %w", req.RepoType, err)
	}
	return tree, nil
}

func (c *RepoComponent) UploadFile(ctx context.Context, req *types.CreateFileReq) error {
	parentPath := filepath.Dir(req.FilePath)
	if parentPath == "." {
		parentPath = "/"
	}
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.NameSpace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      parentPath,
		RepoType:  req.RepoType,
	}
	tree, err := c.git.GetRepoFileTree(ctx, getRepoFileTree)
	if err != nil {
		slog.Error("Error getting %s file tree: %w", req.RepoType, err, slog.String("dataset", fmt.Sprintf("%s/%s", req.NameSpace, req.Name)), slog.String("file_path", req.FilePath))
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

func (c *RepoComponent) SDKListFiles(ctx *gin.Context, repoType types.RepositoryType, namespace, name string) (*types.SDKFiles, error) {
	var sdkFiles []types.SDKFile
	repo, err := c.repo.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if repo.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}

	filePaths, err := getFilePaths(namespace, name, "", repoType, c.git.GetRepoFileTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get all %s files, error: %w", repoType, err)
	}

	for _, filePath := range filePaths {
		sdkFiles = append(sdkFiles, types.SDKFile{Filename: filePath})
	}
	return &types.SDKFiles{
		ID:        fmt.Sprintf("%s/%s", namespace, name),
		Siblings:  sdkFiles,
		Private:   repo.Private,
		Downloads: repo.DownloadCount,
		Likes:     repo.Likes,
		Tags:      []string{},
		SHA:       repo.DefaultBranch,
	}, nil
}

func (c *RepoComponent) IsLfs(ctx context.Context, req *types.GetFileReq) (bool, error) {
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	content, err := c.git.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		slog.Error("failed to get %s file raw", string(req.RepoType), slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("path", req.Path))
		return false, err
	}

	return strings.HasPrefix(content, LFSPrefix), nil
}

func (c *RepoComponent) HeadDownloadFile(ctx *gin.Context, req *types.GetFileReq) (*types.File, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if repo.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, fmt.Errorf("permission denied")
		}
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	file, err := c.git.GetRepoFileContents(ctx, getFileContentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
	}
	return file, nil
}

func (c *RepoComponent) SDKDownloadFile(ctx *gin.Context, req *types.GetFileReq) (io.ReadCloser, string, error) {
	var downloadUrl string
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find repo, error: %w", err)
	}
	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if repo.Private && exists {
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, "", err
		}
		if !canRead {
			return nil, "", fmt.Errorf("permission denied")
		}
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	if req.Lfs {
		getFileContentReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  req.RepoType,
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
			RepoType:  req.RepoType,
		}
		reader, err := c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			return nil, "", fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
		}
		return reader, downloadUrl, nil
	}
}

func (c *RepoComponent) UpdateDownloads(ctx context.Context, req *types.UpdateDownloadsReq) error {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find %s, error: %w", req.RepoType, err)
	}

	err = c.repo.UpdateRepoCloneDownloads(ctx, repo, req.Date, req.CloneCount)
	if err != nil {
		return fmt.Errorf("failed to update %s download count, error: %w", req.RepoType, err)
	}
	return err
}

func (c *RepoComponent) FileInfo(ctx context.Context, req *types.GetFileReq) (*types.File, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}

	getFileContentReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	file, err := c.git.GetRepoFileContents(ctx, getFileContentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get git model repository file info, error: %w", err)
	}
	return file, nil
}

func getTagScopeByRepoType(repoType types.RepositoryType) database.TagScope {
	if repoType == types.ModelRepo {
		return database.ModelTagScope
	} else {
		return database.DatasetTagScope
	}
}

func (c *RepoComponent) checkCurrentUserPermission(ctx context.Context, currentUser any, namespace string, role membership.Role) (bool, error) {
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
