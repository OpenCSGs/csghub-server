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
	"slices"
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

const ErrNotFoundMessage = "The target couldn't be found."

type RepoComponent struct {
	tc        *TagComponent
	user      *database.UserStore
	org       *database.OrgStore
	namespace *database.NamespaceStore
	repo      *database.RepoStore
	rel       *database.RepoRelationsStore
	git       gitserver.GitServer
	s3Client  *minio.Client
	msc       *MemberComponent
	lfsBucket string
	uls       *database.UserLikesStore
}

func NewRepoComponent(config *config.Config) (*RepoComponent, error) {
	c := &RepoComponent{}
	c.namespace = database.NewNamespaceStore()
	c.user = database.NewUserStore()
	c.org = database.NewOrgStore()
	c.repo = database.NewRepoStore()
	c.rel = database.NewRepoRelationsStore()
	c.uls = database.NewUserLikesStore()
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
	newDBRepo.User = user

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

// relatedRepos gets all repos related to the given repo, and return them by repo type
func (c *RepoComponent) relatedRepos(ctx context.Context, repoID int64, currentUser string) (map[string][]*database.Repository, error) {
	fromRelations, err := c.rel.From(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo relation from, error: %w", err)
	}
	var toRepoIDs []int64
	for _, rel := range fromRelations {
		toRepoIDs = append(toRepoIDs, rel.ToRepoID)
	}

	toRelations, err := c.rel.To(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo relation to, error: %w", err)
	}
	var fromRepoIDs []int64
	for _, rel := range toRelations {
		fromRepoIDs = append(fromRepoIDs, rel.FromRepoID)
	}

	//combine from and to related repos and then remove duplicates
	var relatedRepoIDs []int64
	relatedRepoIDs = append(relatedRepoIDs, toRepoIDs...)
	relatedRepoIDs = append(relatedRepoIDs, fromRepoIDs...)
	slices.Sort(relatedRepoIDs)
	relatedRepoIDs = slices.Compact(relatedRepoIDs)

	var opts []database.SelectOption
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private", "name",
		"nickname", "description", "download_count", "updated_at"))

	relatedRepos, err := c.repo.FindByIds(ctx, relatedRepoIDs, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get relation to repositories by ids, error: %w", err)
	}

	relatedRepos, err = c.visiableToUser(ctx, relatedRepos, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check related repositories visiable to user:%s, %w", currentUser, err)
	}
	res := make(map[string][]*database.Repository)
	for _, repo := range relatedRepos {
		res[string(repo.RepositoryType)] = append(res[string(repo.RepositoryType)], repo)
	}
	return res, nil
}

func (c *RepoComponent) visiableToUser(ctx context.Context, repos []*database.Repository, currentUser string) ([]*database.Repository, error) {
	var res []*database.Repository
	for _, repo := range repos {
		if repo.Private {
			if len(currentUser) == 0 {
				continue
			}
			namespace, _ := repo.NamespaceAndName()
			canRead, err := c.checkCurrentUserPermission(ctx, currentUser, namespace, membership.RoleRead)
			if err != nil {
				return nil, err
			}
			if canRead {
				res = append(res, repo)
			}
		} else {
			res = append(res, repo)
		}
	}
	return res, nil
}

func (c *RepoComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	slog.Debug("creating file get request", slog.String("namespace", req.NameSpace), slog.String("filepath", req.FilePath))
	var err error
	var user database.User
	user, err = c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	req.Email = user.Email

	_, err = c.namespace.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		err = c.createReadmeFile(ctx, req)
	} else {
		err = c.createLibraryFile(ctx, req)
	}

	if err != nil {
		slog.Error("failed to create repo file", slog.String("file", req.FilePath), slog.Any("error", err), slog.String("namespace", req.NameSpace), slog.String("name", req.Name))
	}

	err = c.repo.SetUpdateTimeByPath(ctx, req.RepoType, req.NameSpace, req.Name, time.Now())
	if err != nil {
		slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(req.RepoType)), slog.String("namespace", req.NameSpace), slog.String("name", req.Name))
	}

	var resp types.CreateFileResp
	return &resp, nil
}

func (c *RepoComponent) createReadmeFile(ctx context.Context, req *types.CreateFileReq) error {
	var (
		err error
	)
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.git.CreateRepoFile(req)
	if err != nil {
		return fmt.Errorf("failed to create %s file, cause: %w", req.RepoType, err)
	}

	return err
}

func (c *RepoComponent) createLibraryFile(ctx context.Context, req *types.CreateFileReq) error {
	var (
		err error
	)

	err = c.tc.UpdateLibraryTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, "", req.FilePath)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to set %s's tags", req.RepoType), slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return fmt.Errorf("failed to set %s's tags, cause: %w", req.RepoType, err)
	}
	err = c.git.CreateRepoFile(req)
	if err != nil {
		return err
	}

	return err
}

func (c *RepoComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("update file get request", slog.String("namespace", req.NameSpace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var err error
	var user database.User
	user, err = c.user.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	req.Email = user.Email

	_, err = c.namespace.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	err = c.git.UpdateRepoFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update %s file, cause: %w", req.RepoType, err)
	}

	// TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		err = c.updateReadmeFile(ctx, req)
	} else {
		slog.Debug("file is not readme", slog.String("filePath", req.FilePath), slog.String("originPath", req.OriginPath))
		err = c.updateLibraryFile(ctx, req)
	}
	if err != nil {
		slog.Error("failed to update file", slog.String("file", req.FilePath), slog.Any("error", err), slog.String("namespace", req.NameSpace), slog.String("name", req.Name))
	}

	err = c.repo.SetUpdateTimeByPath(ctx, req.RepoType, req.NameSpace, req.Name, time.Now())
	if err != nil {
		slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(req.RepoType)), slog.String("namespace", req.NameSpace), slog.String("name", req.Name))
	}

	resp := new(types.UpdateFileResp)
	return resp, nil
}

func (c *RepoComponent) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) error {
	var err error

	isFileRenamed := req.FilePath != req.OriginPath
	// need to handle tag change only if file renamed
	if isFileRenamed {
		err = c.tc.UpdateLibraryTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, req.OriginPath, req.FilePath)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set %s's tags", req.RepoType), slog.String("namespace", req.NameSpace),
				slog.String("name", req.Name), slog.Any("error", err))
			return fmt.Errorf("failed to set %s's tags, cause: %w", req.RepoType, err)
		}
	}

	return err
}

func (c *RepoComponent) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) error {
	slog.Debug("file is readme", slog.String("content", req.Content))
	var err error

	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tc.UpdateMetaTags(ctx, getTagScopeByRepoType(req.RepoType), req.NameSpace, req.Name, string(contentDecoded))
	if err != nil {
		return fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	return err
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
	f, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: req.NameSpace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      req.FilePath,
		RepoType:  req.RepoType,
	})
	if err != nil {
		_, err = c.CreateFile(ctx, req)
		if err != nil {
			return fmt.Errorf("fail to create file for file uploading, %w", err)
		}
		return nil
	}
	var updateFileReq types.UpdateFileReq

	updateFileReq.Username = req.Username
	updateFileReq.Message = req.Message
	updateFileReq.Branch = req.Branch
	updateFileReq.Content = req.Content
	updateFileReq.NameSpace = req.NameSpace
	updateFileReq.Name = req.Name
	updateFileReq.FilePath = req.FilePath
	//we need file sha, not commit SHA
	updateFileReq.SHA = f.SHA
	updateFileReq.RepoType = req.RepoType

	_, err = c.UpdateFile(ctx, &updateFileReq)

	return err
}

func (c *RepoComponent) SDKListFiles(ctx *gin.Context, repoType types.RepositoryType, namespace, name string) (*types.SDKFiles, error) {
	var sdkFiles []types.SDKFile
	repo, err := c.repo.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return nil, ErrNotFound
	}

	currentUser, exists := ctx.Get("currentUser")
	// TODO: Use user access token to check permissions
	if repo.Private {
		if !exists {
			return nil, ErrUnauthorized
		}
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser.(string), namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, ErrUnauthorized
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
		if err.Error() == ErrNotFoundMessage {
			return false, ErrNotFound
		}
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
	if repo.Private {
		if !exists {
			return nil, ErrUnauthorized
		}
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser.(string), req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, err
		}
		if !canRead {
			return nil, ErrUnauthorized
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
		if err.Error() == ErrNotFoundMessage {
			return nil, ErrNotFound
		}
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
	if repo.Private {
		if !exists {
			return nil, "", ErrUnauthorized
		}
		canRead, err := c.checkCurrentUserPermission(ctx, currentUser.(string), req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, "", err
		}
		if !canRead {
			return nil, "", ErrUnauthorized
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
			if err.Error() == ErrNotFoundMessage {
				return nil, downloadUrl, ErrNotFound
			}
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
			if err.Error() == ErrNotFoundMessage {
				return nil, downloadUrl, ErrNotFound
			}
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

func (c *RepoComponent) AllowReadAccessRepo(ctx context.Context, repo *database.Repository, username string) (bool, error) {
	if !repo.Private {
		return true, nil
	}

	if username == "" {
		return false, errors.New("user not found, please login first")
	}

	namespace, _ := repo.NamespaceAndName()
	return c.checkCurrentUserPermission(ctx, username, namespace, membership.RoleRead)
}

func (c *RepoComponent) AllowReadAccess(ctx context.Context, namespace, name, username string) (bool, error) {
	repo, err := c.repo.FindByPath(ctx, types.SpaceRepo, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	return c.AllowReadAccessRepo(ctx, repo, username)
}

func (c *RepoComponent) AllowWriteAccess(ctx context.Context, namespace, name, username string) (bool, error) {
	repo, err := c.repo.FindByPath(ctx, types.SpaceRepo, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if !repo.Private {
		return true, nil
	}

	if username == "" {
		return false, errors.New("user not found, please login first")
	}

	return c.checkCurrentUserPermission(ctx, username, namespace, membership.RoleWrite)
}

func (c *RepoComponent) AllowAdminAccess(ctx context.Context, namespace, name, username string) (bool, error) {
	repo, err := c.repo.FindByPath(ctx, types.SpaceRepo, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if !repo.Private {
		return true, nil
	}

	if username == "" {
		return false, errors.New("user not found, please login first")
	}

	return c.checkCurrentUserPermission(ctx, username, namespace, membership.RoleAdmin)
}

func (c *RepoComponent) checkCurrentUserPermission(ctx context.Context, userName string, namespace string, role membership.Role) (bool, error) {
	ns, err := c.namespace.FindByPath(ctx, namespace)
	if err != nil {
		return false, err
	}

	if ns.NamespaceType == "user" {
		return userName == namespace, nil
	} else {
		r, err := c.msc.GetMemberRole(ctx, namespace, userName)
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
