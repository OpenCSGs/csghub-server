package component

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
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

	"github.com/minio/minio-go/v7"
	"github.com/spf13/cast"
	"opencsg.com/csghub-server/builder/deploy"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/queue"
)

const (
	ErrNotFoundMessage    = "The target couldn't be found."
	ErrGetContentsOrList  = "GetContentsOrList"
	AdminSecret           = "gnuRYKce"
	GitAttributesFileName = ".gitattributes"
	DefaultTreeLimit      = 500
	MaxTreeLimit          = 10000
	DefaultLogTreeLimit   = 25
	MaxLogTreeLimit       = 100
)

type repoComponentImpl struct {
	tagComponent           TagComponent
	userStore              database.UserStore
	orgStore               database.OrgStore
	namespaceStore         database.NamespaceStore
	repoStore              database.RepoStore
	repoFileStore          database.RepoFileStore
	repoRelationsStore     database.RepoRelationsStore
	mirrorStore            database.MirrorStore
	git                    gitserver.GitServer
	s3Client               s3.Client
	userSvcClient          rpc.UserSvcClient
	lfsBucket              string
	userLikesStore         database.UserLikesStore
	mirrorServer           mirrorserver.MirrorServer
	runtimeFrameworksStore database.RuntimeFrameworksStore
	deployTaskStore        database.DeployTaskStore
	deployer               deploy.Deployer
	publicRootDomain       string
	serverBaseUrl          string
	clusterInfoStore       database.ClusterInfoStore
	mirrorSourceStore      database.MirrorSourceStore
	tokenStore             database.AccessTokenStore
	syncVersionStore       database.SyncVersionStore
	syncClientSettingStore database.SyncClientSettingStore
	fileStore              database.FileStore
	config                 *config.Config
	accountingComponent    AccountingComponent
	spaceResourceStore     database.SpaceResourceStore
	lfsMetaObjectStore     database.LfsMetaObjectStore
	recomStore             database.RecomStore
	mq                     queue.PriorityQueue
}

type RepoComponent interface {
	CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, error)
	UpdateRepo(ctx context.Context, req types.UpdateRepoReq) (*database.Repository, error)
	DeleteRepo(ctx context.Context, req types.DeleteRepoReq) (*database.Repository, error)
	// PublicToUser gets visible repos of the given user and user's orgs
	PublicToUser(ctx context.Context, repoType types.RepositoryType, userName string, filter *types.RepoFilter, per, page int) (repos []*database.Repository, count int, err error)
	CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error)
	UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error)
	DeleteFile(ctx context.Context, req *types.DeleteFileReq) (*types.DeleteFileResp, error)
	Commits(ctx context.Context, req *types.GetCommitsReq) ([]types.Commit, *types.RepoPageOpts, error)
	LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error)
	FileRaw(ctx context.Context, req *types.GetFileReq) (string, error)
	DownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (io.ReadCloser, int64, string, error)
	InternalDownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, int64, string, error)
	Branches(ctx context.Context, req *types.GetBranchesReq) ([]types.Branch, error)
	Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error)
	UpdateTags(ctx context.Context, namespace, name string, repoType types.RepositoryType, category, currentUser string, tags []string) error
	Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error)
	TreeV2(ctx context.Context, req *types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)
	LogsTree(ctx context.Context, req *types.GetLogsTreeRequest) (*types.LogsTreeResp, error)
	UploadFile(ctx context.Context, req *types.CreateFileReq) error
	SDKListFiles(ctx context.Context, repoType types.RepositoryType, namespace, name, ref, userName string) (*types.SDKFiles, error)
	IsLfs(ctx context.Context, req *types.GetFileReq) (bool, int64, error)
	HeadDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (*types.File, error)
	SDKDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (io.ReadCloser, int64, string, error)
	// UpdateDownloads increase clone download count for repo by given count
	UpdateDownloads(ctx context.Context, req *types.UpdateDownloadsReq) error
	// IncrDownloads increase the click download count for repo by 1
	IncrDownloads(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
	FileInfo(ctx context.Context, req *types.GetFileReq) (*types.File, error)
	AllowReadAccessRepo(ctx context.Context, repo *database.Repository, username string) (bool, error)
	AllowReadAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error)
	AllowWriteAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error)
	AllowAdminAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error)
	GetCommitWithDiff(ctx context.Context, req *types.GetCommitsReq) (*types.CommitResponse, error)
	CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error)
	MirrorFromSaas(ctx context.Context, namespace, name, currentUser string, repoType types.RepositoryType) error
	GetMirror(ctx context.Context, req types.GetMirrorReq) (*database.Mirror, error)
	UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error)
	DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error
	// get runtime framework list with type
	ListRuntimeFrameworkWithType(ctx context.Context, deployType int) ([]types.RuntimeFramework, error)
	// get runtime framework list
	ListRuntimeFramework(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFramework, error)
	CreateRuntimeFramework(ctx context.Context, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error)
	UpdateRuntimeFramework(ctx context.Context, id int64, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error)
	DeleteRuntimeFramework(ctx context.Context, currentUser string, id int64) error
	ListDeploy(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) ([]types.DeployRepo, error)
	DeleteDeploy(ctx context.Context, delReq types.DeployActReq) error
	DeployDetail(ctx context.Context, detailReq types.DeployActReq) (*types.DeployRepo, error)
	DeployInstanceLogs(ctx context.Context, logReq types.DeployActReq) (*deploy.MultiLogReader, error)
	// check access repo permission by repo id
	AllowAccessByRepoID(ctx context.Context, repoID int64, username string) (bool, error)
	// check access endpoint for rproxy
	AllowAccessEndpoint(ctx context.Context, currentUser string, deploy *database.Deploy) (bool, error)
	// check access deploy permission
	AllowAccessDeploy(ctx context.Context, req types.DeployActReq) (bool, error)
	DeployStop(ctx context.Context, stopReq types.DeployActReq) error
	AllowReadAccessByDeployID(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string, deployID int64) (bool, error)
	DeployStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployID int64) (types.ModelStatusEventData, error)
	GetDeployBySvcName(ctx context.Context, svcName string) (*database.Deploy, error)
	SyncMirror(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) error
	DeployUpdate(ctx context.Context, updateReq types.DeployActReq, req *types.DeployUpdateReq) error
	DeployStart(ctx context.Context, startReq types.DeployActReq) error
	AllFiles(ctx context.Context, req types.GetAllFilesReq) ([]*types.File, error)
	GetUserRepoPermission(ctx context.Context, userName string, repo *database.Repository) (*types.UserRepoPermission, error)
	CheckCurrentUserPermission(ctx context.Context, userName string, namespace string, role membership.Role) (bool, error)
	GetNameSpaceInfo(ctx context.Context, path string) (*types.Namespace, error)
	RelatedRepos(ctx context.Context, repoID int64, currentUser string) (map[types.RepositoryType][]*database.Repository, error)
	VisiableToUser(ctx context.Context, repos []*database.Repository, currentUser string) ([]*database.Repository, error)
	GenerateEndpoint(ctx context.Context, deploy *database.Deploy) (string, string)
	IsAdminRole(user database.User) bool
}

func NewRepoComponentImpl(config *config.Config) (*repoComponentImpl, error) {
	r, err := NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return r.(*repoComponentImpl), nil
}

func NewRepoComponent(config *config.Config) (RepoComponent, error) {
	c := &repoComponentImpl{}
	c.namespaceStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.orgStore = database.NewOrgStore()
	c.repoStore = database.NewRepoStore()
	c.repoFileStore = database.NewRepoFileStore()
	c.repoRelationsStore = database.NewRepoRelationsStore()
	c.userLikesStore = database.NewUserLikesStore()
	c.mirrorStore = database.NewMirrorStore()
	c.mirrorSourceStore = database.NewMirrorSourceStore()
	c.tokenStore = database.NewAccessTokenStore()
	c.syncVersionStore = database.NewSyncVersionStore()
	c.syncClientSettingStore = database.NewSyncClientSettingStore()
	c.fileStore = database.NewFileStore()
	var err error
	c.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	mq, err := queue.GetPriorityQueueInstance()
	if err != nil {
		return nil, fmt.Errorf("failed to get priority queue: %v", err)
	}
	c.mq = mq
	c.mirrorServer, err = git.NewMirrorServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git mirror server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.tagComponent, err = NewTagComponent(config)
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
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	c.deployTaskStore = database.NewDeployTaskStore()
	c.deployer = deploy.NewDeployer()
	c.publicRootDomain = config.Space.PublicRootDomain
	c.serverBaseUrl = config.APIServer.PublicDomain
	c.clusterInfoStore = database.NewClusterInfoStore()
	c.runtimeFrameworksStore = database.NewRuntimeFrameworksStore()
	c.accountingComponent, err = NewAccountingComponent(config)
	if err != nil {
		return nil, err
	}
	c.spaceResourceStore = database.NewSpaceResourceStore()
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.recomStore = database.NewRecomStore()
	c.config = config
	return c, nil
}

func (c *repoComponentImpl) CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, error) {
	// Name validation
	valid, err := common.IsValidName(req.Name)
	if !valid {
		return nil, nil, fmt.Errorf("repo name is invalid, error: %w", err)
	}

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, nil, errors.New("user does not exist")
	}

	if user.Email == "" {
		return nil, nil, fmt.Errorf("please set your email first")
	}

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, nil, err
			}
			if !canWrite {
				return nil, nil, ErrForbiddenMsg("users do not have permission to create repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, nil, ErrForbiddenMsg("users do not have permission to create repo in this namespace")
			}
		}
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	gitRepoReq := gitserver.CreateRepoReq{
		Username:      req.Username,
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      req.Nickname,
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
	newDBRepo, err := c.repoStore.CreateRepo(ctx, dbRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to create database repo, error: %w", err)
	}
	newDBRepo.User = user

	return gitRepo, newDBRepo, nil
}

func (c *repoComponentImpl) UpdateRepo(ctx context.Context, req types.UpdateRepoReq) (*database.Repository, error) {
	repo, err := c.repoStore.Find(ctx, req.Namespace, string(req.RepoType), req.Name)
	if err != nil {
		return nil, errors.New("repository does not exist")
	}

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, ErrForbiddenMsg("users do not have permission to update repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, ErrForbiddenMsg("users do not have permission to update repo in this namespace")
			}
		}
	}

	if req.Private != nil {
		repo.Private = *req.Private
	}
	if req.Nickname != nil {
		repo.Nickname = *req.Nickname
	}
	if req.Description != nil {
		repo.Description = *req.Description
	}

	gitRepoReq := gitserver.UpdateRepoReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		Nickname:      repo.Nickname,
		Description:   repo.Description,
		DefaultBranch: repo.DefaultBranch,
		Private:       repo.Private,
		RepoType:      req.RepoType,
	}
	_, err = c.git.UpdateRepo(ctx, gitRepoReq)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in git, error: %w", err)
	}

	resRepo, err := c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to update repo in database, error: %w", err)
	}

	return resRepo, nil
}

func (c *repoComponentImpl) DeleteRepo(ctx context.Context, req types.DeleteRepoReq) (*database.Repository, error) {
	repo, err := c.repoStore.Find(ctx, req.Namespace, string(req.RepoType), req.Name)
	if err != nil {
		return nil, errors.New("repository does not exist")
	}

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	if namespace.NamespaceType == database.OrgNamespace {
		canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleAdmin)
		if err != nil {
			return nil, err
		}
		if !canWrite {
			return nil, ErrForbiddenMsg("users do not have permission to delete repo in this organization")
		}
	} else {
		if namespace.Path != user.Username {
			return nil, ErrForbiddenMsg("users do not have permission to delete repo in this namespace")
		}
	}

	err = c.repoStore.CleanRelationsByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to clean repo relations, %w", err)
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

	err = c.repoStore.DeleteRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to delete repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to delete repo in database, error: %w", err)
	}

	return repo, nil
}

// PublicToUser gets visible repos of the given user and user's orgs
func (c *repoComponentImpl) PublicToUser(ctx context.Context, repoType types.RepositoryType, userName string, filter *types.RepoFilter, per, page int) (repos []*database.Repository, count int, err error) {
	var repoOwnerIDs []int64
	var isAdmin bool

	if len(userName) > 0 {
		// get user orgs from user service
		user, err := c.userSvcClient.GetUserInfo(ctx, userName, userName)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get user info, error: %w", err)
		}

		dbUser := &database.User{
			RoleMask: strings.Join(user.Roles, ","),
		}

		isAdmin = dbUser.CanAdmin()

		if !isAdmin {
			repoOwnerIDs = append(repoOwnerIDs, user.ID)
			//get user's orgs
			for _, org := range user.Orgs {
				repoOwnerIDs = append(repoOwnerIDs, org.UserID)
			}
		}
	}
	repos, count, err = c.repoStore.PublicToUser(ctx, repoType, repoOwnerIDs, filter, per, page, isAdmin)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user public repos, error: %w", err)
	}

	return repos, count, nil
}

// RelatedRepos gets all repos related to the given repo, and return them by repo type
func (c *repoComponentImpl) RelatedRepos(ctx context.Context, repoID int64, currentUser string) (map[types.RepositoryType][]*database.Repository, error) {
	fromRelations, err := c.repoRelationsStore.From(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo relation from, error: %w", err)
	}
	var toRepoIDs []int64
	for _, rel := range fromRelations {
		toRepoIDs = append(toRepoIDs, rel.ToRepoID)
	}

	toRelations, err := c.repoRelationsStore.To(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo relation to, error: %w", err)
	}
	var fromRepoIDs []int64
	for _, rel := range toRelations {
		fromRepoIDs = append(fromRepoIDs, rel.FromRepoID)
	}

	// combine from and to related repos and then remove duplicates
	var relatedRepoIDs []int64
	relatedRepoIDs = append(relatedRepoIDs, toRepoIDs...)
	relatedRepoIDs = append(relatedRepoIDs, fromRepoIDs...)
	slices.Sort(relatedRepoIDs)
	relatedRepoIDs = slices.Compact(relatedRepoIDs)

	var opts []database.SelectOption
	opts = append(opts, database.Columns("id", "repository_type", "path", "user_id", "private", "name",
		"nickname", "description", "download_count", "updated_at"))

	RelatedRepos, err := c.repoStore.FindByIds(ctx, relatedRepoIDs, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get relation to repositories by ids, error: %w", err)
	}

	RelatedRepos, err = c.VisiableToUser(ctx, RelatedRepos, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check related repositories visiable to user:%s, %w", currentUser, err)
	}
	res := make(map[types.RepositoryType][]*database.Repository)
	for _, repo := range RelatedRepos {
		res[repo.RepositoryType] = append(res[repo.RepositoryType], repo)
	}
	return res, nil
}

func (c *repoComponentImpl) VisiableToUser(ctx context.Context, repos []*database.Repository, currentUser string) ([]*database.Repository, error) {
	var res []*database.Repository
	for _, repo := range repos {
		if repo.Private {
			if len(currentUser) == 0 {
				continue
			}
			namespace, _ := repo.NamespaceAndName()
			canRead, err := c.CheckCurrentUserPermission(ctx, currentUser, namespace, membership.RoleRead)
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

func (c *repoComponentImpl) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	slog.Debug("creating file get request", slog.String("namespace", req.Namespace), slog.String("filepath", req.FilePath))
	var (
		err    error
		user   database.User
		useLfs bool
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return nil, ErrUnauthorized
	}

	user, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	req.Email = user.Email

	_, err = c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		useLfs, req = c.checkIfShouldUseLfs(ctx, req)
	}

	// TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		err = c.createReadmeFile(ctx, req)
	} else {
		err = c.createLibraryFile(ctx, req)
	}

	if err != nil {
		slog.Error("failed to create repo file", slog.String("file", req.FilePath), slog.Any("error", err), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	if useLfs {
		objectKey := filepath.Join("lfs", req.Pointer.RelativePath())
		uploadInfo, err := c.s3Client.PutObject(ctx, c.config.S3.Bucket, objectKey, bytes.NewReader(req.OriginalContent), req.Pointer.Size, minio.PutObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to upload to Minio: %w", err)
		}

		if uploadInfo.Size != req.Pointer.Size {
			return nil, fmt.Errorf("uploaded file size does not match expected size: %d != %d", uploadInfo.Size, req.Pointer.Size)
		}
		_, err = c.lfsMetaObjectStore.UpdateOrCreate(ctx, database.LfsMetaObject{
			Oid:          req.Pointer.Oid,
			Size:         req.Pointer.Size,
			RepositoryID: repo.ID,
			Existing:     true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create LFS meta object: %w", err)
		}
	}

	err = c.repoStore.SetUpdateTimeByPath(ctx, req.RepoType, req.Namespace, req.Name, time.Now())
	if err != nil {
		slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(req.RepoType)), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	var resp types.CreateFileResp
	return &resp, nil
}

func (c *repoComponentImpl) createReadmeFile(ctx context.Context, req *types.CreateFileReq) error {
	var err error
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(req.Content)
	_, err = c.tagComponent.UpdateMetaTags(ctx, getTagScopeByRepoType(req.RepoType), req.Namespace, req.Name, string(contentDecoded))
	if err != nil {
		return fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.git.CreateRepoFile(req)
	if err != nil {
		return fmt.Errorf("failed to create %s file, cause: %w", req.RepoType, err)
	}

	return err
}

func (c *repoComponentImpl) createLibraryFile(ctx context.Context, req *types.CreateFileReq) error {
	var err error
	err = c.tagComponent.UpdateLibraryTags(ctx, getTagScopeByRepoType(req.RepoType), req.Namespace, req.Name, "", req.FilePath)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to set %s's tags", req.RepoType), slog.String("namespace", req.Namespace),
			slog.String("name", req.Name), slog.Any("error", err))
		return fmt.Errorf("failed to set %s's tags, cause: %w", req.RepoType, err)
	}
	err = c.git.CreateRepoFile(req)
	if err != nil {
		return err
	}

	return err
}

func (c *repoComponentImpl) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("update file get request", slog.String("namespace", req.Namespace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var (
		err    error
		user   database.User
		useLfs bool
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return nil, ErrForbiddenMsg("users do not have permission to update file in this repo")
	}

	user, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	req.Email = user.Email

	_, err = c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		useLfs, req = c.checkIfShouldUseLfsUpdate(ctx, req)
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
		slog.Error("failed to update file", slog.String("file", req.FilePath), slog.Any("error", err), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	if useLfs {
		objectKey := filepath.Join("lfs", req.Pointer.RelativePath())
		uploadInfo, err := c.s3Client.PutObject(ctx, c.config.S3.Bucket, objectKey, bytes.NewReader(req.OriginalContent), req.Pointer.Size, minio.PutObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to upload to Minio: %w", err)
		}

		if uploadInfo.Size != req.Pointer.Size {
			return nil, fmt.Errorf("uploaded file size does not match expected size: %d != %d", uploadInfo.Size, req.Pointer.Size)
		}
		_, err = c.lfsMetaObjectStore.UpdateOrCreate(ctx, database.LfsMetaObject{
			Oid:          req.Pointer.Oid,
			Size:         req.Pointer.Size,
			RepositoryID: repo.ID,
			Existing:     true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create LFS meta object: %w", err)
		}
	}

	err = c.repoStore.SetUpdateTimeByPath(ctx, req.RepoType, req.Namespace, req.Name, time.Now())
	if err != nil {
		slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(req.RepoType)), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	resp := new(types.UpdateFileResp)
	return resp, nil
}

func (c *repoComponentImpl) DeleteFile(ctx context.Context, req *types.DeleteFileReq) (*types.DeleteFileResp, error) {
	slog.Debug("delete file get request", slog.String("namespace", req.Namespace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var (
		err  error
		user database.User
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return nil, ErrForbiddenMsg("users do not have permission to delete file in this repo")
	}

	user, err = c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	req.Email = user.Email

	_, err = c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	err = c.git.DeleteRepoFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to delete %s file, cause: %w", req.RepoType, err)
	}

	// TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		err = c.deleteReadmeFile(ctx, req)
	} else {
		slog.Debug("file is not readme", slog.String("filePath", req.FilePath), slog.String("originPath", req.OriginPath))
		err = c.deleteLibraryFile(ctx, req)
	}

	if err != nil {
		slog.Error("failed to delete file", slog.String("file", req.FilePath), slog.Any("error", err), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	err = c.repoStore.SetUpdateTimeByPath(ctx, req.RepoType, req.Namespace, req.Name, time.Now())
	if err != nil {
		slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(req.RepoType)), slog.String("namespace", req.Namespace), slog.String("name", req.Name))
	}

	resp := new(types.DeleteFileResp)
	return resp, nil
}

func (c *repoComponentImpl) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) error {
	err := c.changeLibraryFile(ctx, req.FilePath, req.OriginPath, req.Namespace, req.Name, req.RepoType)
	return err
}

func (c *repoComponentImpl) deleteLibraryFile(ctx context.Context, req *types.DeleteFileReq) error {
	err := c.changeLibraryFile(ctx, req.FilePath, req.OriginPath, req.Namespace, req.Name, req.RepoType)
	return err
}

func (c *repoComponentImpl) changeLibraryFile(ctx context.Context, filePath, originPath, namespace, name string, repoType types.RepositoryType) error {
	var err error

	isFileRenamed := filePath != originPath
	// need to handle tag change only if file renamed
	if isFileRenamed {
		err = c.tagComponent.UpdateLibraryTags(ctx, getTagScopeByRepoType(repoType), namespace, name, originPath, filePath)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to set %s's tags", repoType), slog.String("namespace", namespace),
				slog.String("name", name), slog.Any("error", err))
			return fmt.Errorf("failed to set %s's tags, cause: %w", repoType, err)
		}
	}

	return err
}

func (c *repoComponentImpl) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) error {
	slog.Debug("file is readme", slog.String("content", req.Content))
	err := c.changeReadmeFile(ctx, req.Content, req.Namespace, req.Name, req.RepoType)
	if err != nil {
		return fmt.Errorf("failed to update meta tags for update readme, cause: %w", err)
	}
	return err
}

func (c *repoComponentImpl) deleteReadmeFile(ctx context.Context, req *types.DeleteFileReq) error {
	err := c.changeReadmeFile(ctx, req.Content, req.Namespace, req.Name, req.RepoType)
	if err != nil {
		return fmt.Errorf("failed to update meta tags for delete readme, cause: %w", err)
	}
	return err
}

func (c *repoComponentImpl) changeReadmeFile(ctx context.Context, content, namespace, name string, repoType types.RepositoryType) error {
	contentDecoded, _ := base64.RawStdEncoding.DecodeString(content)
	_, err := c.tagComponent.UpdateMetaTags(ctx, getTagScopeByRepoType(repoType), namespace, name, string(contentDecoded))
	if err != nil {
		return fmt.Errorf("failed to update meta tags, cause: %w", err)
	}
	return err
}

func (c *repoComponentImpl) Commits(ctx context.Context, req *types.GetCommitsReq) ([]types.Commit, *types.RepoPageOpts, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, nil, ErrUnauthorized
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
	commits, pageOpt, err := c.git.GetRepoCommits(ctx, getCommitsReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get git %s repository commits, error: %w", req.RepoType, err)
	}
	return commits, pageOpt, nil
}

func (c *repoComponentImpl) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get last commit in this repo")
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

func (c *repoComponentImpl) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil || repo == nil {
		return "", fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return "", ErrForbiddenMsg("users do not have permission to get file raw in this repo")
	}

	if repo.Source != types.LocalSource && strings.ToLower(req.Path) == "readme.md" {
		_, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return repo.Readme, nil
			}
		}
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

func (c *repoComponentImpl) DownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (io.ReadCloser, int64, string, error) {
	var (
		reader      io.ReadCloser
		downloadUrl string
		size        int64
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, 0, "", ErrForbiddenMsg("users do not have permission to download file in this repo")
	}

	err = c.repoStore.UpdateRepoFileDownloads(ctx, repo, time.Now(), 1)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to update %s file download count, error: %w", req.RepoType, err)
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
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, ossFileExpire, reqParams)
		if err != nil {
			return nil, 0, downloadUrl, err
		}
		return nil, 0, signedUrl.String(), nil
	} else {
		getFileReaderReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  req.RepoType,
		}
		reader, size, err = c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			return nil, 0, "", fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
		}
		return reader, size, downloadUrl, nil
	}
}

func (c *repoComponentImpl) Branches(ctx context.Context, req *types.GetBranchesReq) ([]types.Branch, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get branches in this repo")
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
		if repo.Source != types.LocalSource {
			return []types.Branch{}, nil
		}
		return nil, fmt.Errorf("failed to get git %s repository branches, error: %w", req.RepoType, err)
	}
	return bs, nil
}

func (c *repoComponentImpl) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find %s, error: %w", req.RepoType, err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get tags in this repo")
	}

	tags, err := c.repoStore.Tags(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s tags, error: %w", req.RepoType, err)
	}
	return tags, nil
}

func (c *repoComponentImpl) UpdateTags(ctx context.Context, namespace, name string, repoType types.RepositoryType, category, currentUser string, tags []string) error {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, currentUser, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return ErrForbiddenMsg("users do not have permission to update tags in this repo")
	}

	tagScope := getTagScopeByRepoType(repoType)
	err = c.tagComponent.UpdateRepoTagsByCategory(ctx, tagScope, repo.ID, category, tags)

	return err
}

func (c *repoComponentImpl) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if repo == nil {
		return nil, fmt.Errorf("repo does not exist, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get tree in this repo")
	}

	if repo.Source != types.LocalSource {
		_, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if req.Path == "" {
					req.Path = "/"
				}
				files, err := c.fileStore.FindByParentPath(ctx, repo.ID, req.Path, nil)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return []*types.File{}, nil
					} else {
						return nil, err
					}
				}
				var resFiles []*types.File
				for _, f := range files {
					resFiles = append(resFiles, &types.File{
						Name: f.Name,
						Path: f.Path,
						Size: f.Size,
						Commit: types.Commit{
							Message:       f.LastCommitMessage,
							CommitterDate: f.LastCommitDate,
						},
					})
				}
				return resFiles, nil
			}
		}
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

func (c *repoComponentImpl) TreeV2(ctx context.Context, req *types.GetTreeRequest) (*types.GetRepoFileTreeResp, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("repo does not exist, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get tree in this repo")
	}

	if req.Limit == 0 {
		req.Limit = DefaultTreeLimit
	} else if req.Limit > MaxTreeLimit {
		req.Limit = MaxTreeLimit
	}

	if repo.Source != types.LocalSource {
		_, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if req.Path == "" {
					req.Path = "/"
				}

				offset := 0
				if req.Cursor != "" {
					r, err := base64.StdEncoding.DecodeString(req.Cursor)
					if err == nil {
						offset = cast.ToInt(string(r))
					}
				}
				files, err := c.fileStore.FindByParentPath(
					ctx, repo.ID, req.Path, &types.OffsetPagination{
						Limit:  req.Limit,
						Offset: offset,
					},
				)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return nil, nil
					} else {
						return nil, err
					}
				}
				var resFiles []*types.File
				for _, f := range files {
					resFiles = append(resFiles, &types.File{
						Name: f.Name,
						Path: f.Path,
						Size: f.Size,
						Commit: types.Commit{
							Message:       f.LastCommitMessage,
							CommitterDate: f.LastCommitDate,
						},
					})
				}

				cursor := ""
				if len(files) == req.Limit {
					cursor = base64.StdEncoding.EncodeToString(
						[]byte(cast.ToString(offset + req.Limit)),
					)
				}
				return &types.GetRepoFileTreeResp{
					Files:  resFiles,
					Cursor: cursor,
				}, nil
			}
		}
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	tree, err := c.git.GetTree(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository file tree, error: %w", req.RepoType, err)
	}
	return tree, nil
}

func (c *repoComponentImpl) LogsTree(ctx context.Context, req *types.GetLogsTreeRequest) (*types.LogsTreeResp, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("repo does not exist, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get logs tree in this repo")
	}

	if req.Limit == 0 {
		req.Limit = DefaultLogTreeLimit
	} else if req.Limit > MaxLogTreeLimit {
		req.Limit = MaxLogTreeLimit
	}

	if repo.Source != types.LocalSource {
		_, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if req.Path == "" {
					req.Path = "/"
				}
				files, err := c.fileStore.FindByParentPath(
					ctx, repo.ID, req.Path, &types.OffsetPagination{
						Offset: req.Offset,
						Limit:  req.Limit,
					},
				)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return nil, nil
					} else {
						return nil, err
					}
				}
				var commits []*types.CommitForTree
				for _, f := range files {
					commits = append(commits, &types.CommitForTree{
						Message:       f.LastCommitMessage,
						CommitterDate: f.LastCommitDate,
					})
				}
				return &types.LogsTreeResp{
					Commits: commits,
				}, nil
			}
		}
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}

	tree, err := c.git.GetLogsTree(ctx, *req)
	if err != nil {
		return nil, fmt.Errorf("failed to get git %s repository file tree, error: %w", req.RepoType, err)
	}
	return tree, nil
}

func (c *repoComponentImpl) UploadFile(ctx context.Context, req *types.CreateFileReq) error {
	f, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
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
	updateFileReq.Namespace = req.Namespace
	updateFileReq.Name = req.Name
	updateFileReq.FilePath = req.FilePath
	// we need file sha, not commit SHA
	updateFileReq.SHA = f.SHA
	updateFileReq.RepoType = req.RepoType
	updateFileReq.OriginalContent = req.OriginalContent
	updateFileReq.CurrentUser = req.CurrentUser

	_, err = c.UpdateFile(ctx, &updateFileReq)

	return err
}

func (c *repoComponentImpl) SDKListFiles(ctx context.Context, repoType types.RepositoryType, namespace, name, ref, userName string) (*types.SDKFiles, error) {
	var sdkFiles []types.SDKFile
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil || repo == nil {
		return nil, ErrNotFound
	}

	canRead, err := c.AllowReadAccessRepo(ctx, repo, userName)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrForbiddenMsg("users do not have permission to access this repo")
	}

	if ref == "" {
		ref = repo.DefaultBranch
	}

	filePaths, err := GetFilePaths(ctx, namespace, name, "", repoType, ref, c.git.GetTree)
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

func (c *repoComponentImpl) IsLfs(ctx context.Context, req *types.GetFileReq) (bool, int64, error) {
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
			return false, -1, ErrNotFound
		}
		slog.Error("failed to get %s file raw", string(req.RepoType), slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("path", req.Path))
		return false, -1, err
	}

	return strings.HasPrefix(content, LFSPrefix), int64(len(content)), nil
}

func (c *repoComponentImpl) HeadDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (*types.File, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	canRead, err := c.AllowReadAccessRepo(ctx, repo, userName)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, ErrForbiddenMsg("users do not have permission to download file in this repo")
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
		slog.Error("err.Error()", slog.Any("err.Error()", err.Error()))
		if err.Error() == ErrNotFoundMessage || err.Error() == ErrGetContentsOrList {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
	}
	return file, nil
}

func (c *repoComponentImpl) SDKDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (io.ReadCloser, int64, string, error) {
	var downloadUrl string
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to find repo, error: %w", err)
	}

	canRead, err := c.AllowReadAccessRepo(ctx, repo, userName)
	if err != nil {
		return nil, 0, "", err
	}
	if !canRead {
		return nil, 0, "", ErrForbiddenMsg("users do not have permission to download file in this repo")
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
			return nil, 0, "", err
		}
		objectKey := file.LfsRelativePath
		objectKey = path.Join("lfs", objectKey)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, ossFileExpire, reqParams)
		if err != nil {
			if err.Error() == ErrNotFoundMessage || err.Error() == ErrGetContentsOrList {
				return nil, 0, downloadUrl, ErrNotFound
			}
			return nil, 0, downloadUrl, err
		}
		return nil, 0, signedUrl.String(), nil
	} else {
		getFileReaderReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  req.RepoType,
		}
		reader, size, err := c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			if err.Error() == ErrNotFoundMessage {
				return nil, 0, downloadUrl, ErrNotFound
			}
			return nil, 0, "", fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
		}
		return reader, size, downloadUrl, nil
	}
}

func (c *repoComponentImpl) InternalDownloadFile(ctx context.Context, req *types.GetFileReq) (io.ReadCloser, int64, string, error) {
	var downloadUrl string
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to find repo, error: %w", err)
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
			return nil, 0, "", err
		}
		objectKey := file.LfsRelativePath
		objectKey = path.Join("lfs", objectKey)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
		if err != nil {
			if err.Error() == ErrNotFoundMessage || err.Error() == ErrGetContentsOrList {
				return nil, 0, downloadUrl, ErrNotFound
			}
			return nil, 0, downloadUrl, err
		}
		return nil, 0, signedUrl.String(), nil

	} else {
		getFileReaderReq := gitserver.GetRepoInfoByPathReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       req.Ref,
			Path:      req.Path,
			RepoType:  req.RepoType,
		}
		reader, size, err := c.git.GetRepoFileReader(ctx, getFileReaderReq)
		if err != nil {
			if err.Error() == ErrNotFoundMessage {
				return nil, 0, downloadUrl, ErrNotFound
			}
			return nil, 0, "", fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
		}
		return reader, size, downloadUrl, nil
	}
}

// UpdateDownloads increase clone download count for repo by given count
func (c *repoComponentImpl) UpdateDownloads(ctx context.Context, req *types.UpdateDownloadsReq) error {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find %s, error: %w", req.RepoType, err)
	}

	err = c.repoStore.UpdateRepoCloneDownloads(ctx, repo, req.Date, req.CloneCount)
	if err != nil {
		return fmt.Errorf("failed to update %s download count, error: %w", req.RepoType, err)
	}
	return err
}

// IncrDownloads increase the click download count for repo by 1
func (c *repoComponentImpl) IncrDownloads(ctx context.Context, repoType types.RepositoryType, namespace, name string) error {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find %s, error: %w", repoType, err)
	}

	err = c.repoStore.UpdateRepoFileDownloads(ctx, repo, time.Now(), 1)
	if err != nil {
		return fmt.Errorf("failed to incr download count, error: %w", err)
	}
	return err
}

func (c *repoComponentImpl) FileInfo(ctx context.Context, req *types.GetFileReq) (*types.File, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get file info in this repo")
	}

	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}

	if repo.Source != types.LocalSource && strings.ToLower(req.Path) == "readme.md" {
		_, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				content := base64.StdEncoding.EncodeToString([]byte(repo.Readme))
				return &types.File{
					Name:    "readme.md",
					Path:    "readme.md",
					Content: content,
				}, nil
			}
		}
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

func getTagScopeByRepoType(repoType types.RepositoryType) types.TagScope {
	switch repoType {
	case types.ModelRepo:
		return types.ModelTagScope
	case types.DatasetRepo:
		return types.DatasetTagScope
	case types.CodeRepo:
		return types.CodeTagScope
	case types.SpaceRepo:
		return types.SpaceTagScope
	case types.PromptRepo:
		return types.PromptTagScope
	default:
		panic("convert repo type to tag scope failed, unknown repo type:" + repoType)
	}
}

func (c *repoComponentImpl) AllowReadAccessRepo(ctx context.Context, repo *database.Repository, username string) (bool, error) {
	if !repo.Private {
		return true, nil
	}

	if username == "" {
		return false, ErrUserNotFound
	}

	namespace, _ := repo.NamespaceAndName()
	return c.CheckCurrentUserPermission(ctx, username, namespace, membership.RoleRead)
}

func (c *repoComponentImpl) AllowReadAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	return c.AllowReadAccessRepo(ctx, repo, username)
}

func (c *repoComponentImpl) AllowWriteAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error) {
	_, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if username == "" {
		return false, ErrUserNotFound
	}

	return c.CheckCurrentUserPermission(ctx, username, namespace, membership.RoleWrite)
}

func (c *repoComponentImpl) AllowAdminAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error) {
	_, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if username == "" {
		return false, ErrUserNotFound
	}

	return c.CheckCurrentUserPermission(ctx, username, namespace, membership.RoleAdmin)
}

func (c *repoComponentImpl) GetUserRepoPermission(ctx context.Context, userName string, repo *database.Repository) (*types.UserRepoPermission, error) {
	if userName == "" {
		//anonymous user only has read permission to public repo
		return &types.UserRepoPermission{CanRead: !repo.Private, CanWrite: false, CanAdmin: false}, nil
	}

	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return nil, fmt.Errorf("failed to find user '%s' when get user repo permission, error: %w", userName, err)
	}
	if user.CanAdmin() {
		return &types.UserRepoPermission{CanRead: true, CanWrite: true, CanAdmin: true}, nil
	}

	namespace, _ := repo.NamespaceAndName()
	ns, err := c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace '%s' when get user repo permission, error: %w", namespace, err)
	}

	if ns.NamespaceType == "user" {
		//owner has full permission
		if userName == namespace {
			return &types.UserRepoPermission{
				CanRead:  true,
				CanWrite: true,
				CanAdmin: true,
			}, nil
		} else {
			//other user has read permission to pubic repo
			return &types.UserRepoPermission{
				CanRead: !repo.Private, CanWrite: false, CanAdmin: false,
			}, nil
		}
	} else {
		r, err := c.userSvcClient.GetMemberRole(ctx, namespace, userName)
		if err != nil {
			return nil, fmt.Errorf("failed to get user '%s' member role of org '%s' when get user repo permission, error: %w", userName, namespace, err)
		}

		return &types.UserRepoPermission{
			CanRead:  r.CanRead() || !repo.Private,
			CanWrite: r.CanWrite(),
			CanAdmin: r.CanAdmin(),
		}, nil
	}
}

func (c *repoComponentImpl) CheckCurrentUserPermission(ctx context.Context, userName string, namespace string, role membership.Role) (bool, error) {
	ns, err := c.namespaceStore.FindByPath(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("fail to find namespace '%s', err:%w", namespace, err)
	}

	u, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("fail to find user '%s', err:%w", userName, err)
	}
	if u.CanAdmin() {
		return true, nil
	}

	if ns.NamespaceType == "user" {
		return userName == namespace, nil
	} else {
		r, err := c.userSvcClient.GetMemberRole(ctx, namespace, userName)
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

func (c *repoComponentImpl) GetCommitWithDiff(ctx context.Context, req *types.GetCommitsReq) (*types.CommitResponse, error) {
	// get commit diff by commit id
	if req.Ref == "" {
		return nil, fmt.Errorf("failed to find request commit id")
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrForbiddenMsg("users do not have permission to get commit in this repo")
	}
	getCommitReq := gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace, // user name or org name
		Name:      req.Name,      // repo name
		Ref:       req.Ref,       // commit id
		RepoType:  req.RepoType,  // repo type
	}
	resp, errGSC := c.git.GetSingleCommit(ctx, getCommitReq)
	if errGSC != nil {
		return nil, fmt.Errorf("failed to get git %s repository %s commit id '%s', error: %w", req.RepoType, req.Name, req.Ref, errGSC)
	}

	return resp, nil
}

func (c *repoComponentImpl) CreateMirror(ctx context.Context, req types.CreateMirrorReq) (*database.Mirror, error) {
	var (
		mirror database.Mirror
		taskId int64
	)
	admin, err := c.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to create mirror for this repo")
	}

	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	exists, err := c.mirrorStore.IsExist(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("mirror already exists")
	}
	mirrorSource, err := c.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, err: %w, id: %d", err, req.MirrorSourceID)
	}
	pushAccessToken, err := c.tokenStore.GetUserGitToken(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find access token, error: %w", err)
	}
	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	mirror.RepositoryID = repo.ID

	sourceType, sourcePath, err := common.GetSourceTypeAndPathFromURL(req.SourceUrl)
	if err == nil {
		err = c.repoStore.UpdateSourcePath(ctx, repo.ID, sourceType, sourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to update source path in repo: %v", err)
		}
	}

	if c.config.Saas {
		if c.config.GitServer.Type == types.GitServerTypeGitea {
			mirror.PushUsername = req.CurrentUser
			mirror.PushAccessToken = pushAccessToken.Token
			taskId, err = c.mirrorServer.CreateMirrorRepo(ctx, mirrorserver.CreateMirrorRepoReq{
				Namespace:   "root",
				Name:        mirror.LocalRepoPath,
				CloneUrl:    mirror.SourceUrl,
				Username:    mirror.Username,
				AccessToken: mirror.AccessToken,
				Private:     false,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create pull mirror in mirror server: %v", err)
			}
		}
	} else {
		if c.config.GitServer.Type == types.GitServerTypeGitea {
			err = c.git.MirrorSync(ctx, gitserver.MirrorSyncReq{
				Namespace:   req.Namespace,
				Name:        req.Name,
				CloneUrl:    mirror.SourceUrl,
				Username:    mirror.Username,
				AccessToken: mirror.AccessToken,
				RepoType:    req.RepoType,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create pull mirror in mirror server: %v", err)
			}
		}
	}

	mirror.MirrorTaskID = taskId

	reqMirror, err := c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror")
	}

	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		c.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID:  reqMirror.ID,
			Priority:  queue.PriorityMap[reqMirror.Priority],
			CreatedAt: mirror.CreatedAt.Unix(),
		})
		reqMirror.Status = types.MirrorWaiting
		err = c.mirrorStore.Update(ctx, reqMirror)
		if err != nil {
			return nil, fmt.Errorf("failed to update mirror status: %v", err)
		}
	}

	return reqMirror, nil
}

func (c *repoComponentImpl) MirrorFromSaas(ctx context.Context, namespace, name, currentUser string, repoType types.RepositoryType) error {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	m, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to find mirror, error: %w", err)
		}
	}
	if m != nil {
		err := c.mirrorFromSaasSync(ctx, m, namespace, name, repoType)
		if err != nil {
			return fmt.Errorf("failed to trigger mirror sync, error: %w", err)
		}
		return nil
	}
	var mirror database.Mirror
	syncVersion, err := c.syncVersionStore.FindByRepoTypeAndPath(ctx, repo.PathWithOutPrefix(), repoType)
	if err != nil {
		return fmt.Errorf("failed to find sync version, error: %w", err)
	}
	mirrorSource := &database.MirrorSource{}
	if syncVersion.SourceID == types.SyncVersionSourceOpenCSG {
		mirrorSource.SourceName = types.OpenCSGPrefix
	} else if syncVersion.SourceID == types.SyncVersionSourceHF {
		mirrorSource.SourceName = types.HuggingfacePrefix
	}

	mirrorSource.SourceName = types.OpenCSGPrefix
	syncClientSetting, err := c.syncClientSettingStore.First(ctx)
	if err != nil {
		return fmt.Errorf("failed to find sync client setting, error: %w", err)
	}

	sourceUrl := common.TrimPrefixCloneURLBySourceID(c.config.MultiSync.SaasSyncDomain, string(repoType), namespace, name, syncVersion.SourceID)
	mirror.SourceUrl = sourceUrl
	mirror.MirrorSourceID = mirrorSource.ID
	mirror.RepositoryID = repo.ID
	mirror.Username = currentUser
	mirror.AccessToken = c.config.Mirror.Token
	mirror.SourceRepoPath = fmt.Sprintf("%s/%s", namespace, name)

	taskId, err := c.git.CreateMirrorRepo(ctx, gitserver.CreateMirrorRepoReq{
		Namespace:   namespace,
		Name:        name,
		CloneUrl:    mirror.SourceUrl,
		RepoType:    repoType,
		MirrorToken: syncClientSetting.Token,
		Private:     false,
	})
	if err != nil {
		return fmt.Errorf("failed to create pull mirror in mirror server: %v", err)
	}

	mirror.MirrorTaskID = taskId

	_, err = c.mirrorStore.Create(ctx, &mirror)

	if err != nil {
		return fmt.Errorf("failed to create mirror")
	}

	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		c.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID:    mirror.ID,
			Priority:    queue.Priority(mirror.Priority),
			CreatedAt:   mirror.CreatedAt.Unix(),
			MirrorToken: syncClientSetting.Token,
		})
		repo.SyncStatus = types.SyncStatusPending
	} else {
		repo.SyncStatus = types.SyncStatusInProgress
	}

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) mirrorFromSaasSync(ctx context.Context, mirror *database.Mirror, namespace, name string, repoType types.RepositoryType) error {
	var err error
	syncClientSetting, err := c.syncClientSettingStore.First(ctx)
	if err != nil {
		return fmt.Errorf("failed to find sync client setting, error: %w", err)
	}
	repo, err := c.repoStore.FindById(ctx, mirror.RepositoryID)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		err = c.git.MirrorSync(ctx, gitserver.MirrorSyncReq{
			Namespace:   namespace,
			Name:        name,
			RepoType:    repoType,
			MirrorToken: syncClientSetting.Token,
		})
		if err != nil {
			return fmt.Errorf("failed to sync mirror, error: %w", err)
		}
	}
	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		c.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID:    mirror.ID,
			Priority:    queue.Priority(mirror.Priority),
			CreatedAt:   mirror.CreatedAt.Unix(),
			MirrorToken: syncClientSetting.Token,
		})
		repo.SyncStatus = types.SyncStatusPending
	}

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) GetMirror(ctx context.Context, req types.GetMirrorReq) (*database.Mirror, error) {
	admin, err := c.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to get mirror for this repo")
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	return mirror, nil
}

func (c *repoComponentImpl) UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error) {
	admin, err := c.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return nil, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return nil, fmt.Errorf("users do not have permission to update mirror for this repo")
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	mirrorSource, err := c.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mirror source, err: %w, id: %d", err, req.MirrorSourceID)
	}

	pushAccessToken, err := c.tokenStore.GetUserGitToken(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find access token, error: %w", err)
	}

	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = req.PushUrl
	mirror.AccessToken = req.AccessToken
	mirror.PushUsername = req.CurrentUser
	mirror.PushAccessToken = pushAccessToken.Token
	mirror.SourceRepoPath = req.SourceRepoPath
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	err = c.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror, error: %w", err)
	}
	return mirror, nil
}

func (c *repoComponentImpl) DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error {
	admin, err := c.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return fmt.Errorf("users do not have permission to delete mirror for this repo")
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	err = c.mirrorStore.Delete(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to delete mirror, error: %w", err)
	}
	return nil
}

// get runtime framework list with type
func (c *repoComponentImpl) ListRuntimeFrameworkWithType(ctx context.Context, deployType int) ([]types.RuntimeFramework, error) {
	frames, err := c.runtimeFrameworksStore.List(ctx, deployType)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	var frameList []types.RuntimeFramework
	for _, frame := range frames {
		frameList = append(frameList, types.RuntimeFramework{
			ID:            frame.ID,
			FrameName:     frame.FrameName,
			FrameVersion:  frame.FrameVersion,
			FrameImage:    frame.FrameImage,
			Enabled:       frame.Enabled,
			ContainerPort: frame.ContainerPort,
			Type:          frame.Type,
			EngineArgs:    frame.EngineArgs,
			ComputeType:   frame.ComputeType,
			DriverVersion: frame.DriverVersion,
		})
	}
	return frameList, nil
}

// get runtime framework list
func (c *repoComponentImpl) ListRuntimeFramework(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFramework, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	archs := repo.Archs()
	originName := repo.OriginName()
	format := repo.Format()
	frames, err := c.runtimeFrameworksStore.ListByArchsNameAndType(ctx, originName, format, archs, deployType)
	if err != nil {
		return nil, fmt.Errorf("failed to list runtime frameworks, error: %w", err)
	}
	var frameList []types.RuntimeFramework
	for _, modelFrame := range frames {
		frameList = append(frameList, types.RuntimeFramework{
			ID:            modelFrame.ID,
			FrameName:     modelFrame.FrameName,
			FrameVersion:  modelFrame.FrameVersion,
			FrameImage:    modelFrame.FrameImage,
			Enabled:       modelFrame.Enabled,
			ContainerPort: modelFrame.ContainerPort,
			EngineArgs:    modelFrame.EngineArgs,
			ComputeType:   modelFrame.ComputeType,
			DriverVersion: modelFrame.DriverVersion,
			Description:   modelFrame.Description,
			Type:          modelFrame.Type,
		})

	}
	return frameList, nil
}

func (c *repoComponentImpl) CreateRuntimeFramework(ctx context.Context, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, ErrForbiddenMsg("need admin permission for runtime framework")
	}
	newFrame := database.RuntimeFramework{
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		ComputeType:   req.ComputeType,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}
	_, err = c.runtimeFrameworksStore.Add(ctx, newFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime framework, error: %w", err)
	}
	frame := &types.RuntimeFramework{
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
		ComputeType:   req.ComputeType,
		DriverVersion: req.DriverVersion,
	}
	return frame, nil
}

func (c *repoComponentImpl) UpdateRuntimeFramework(ctx context.Context, id int64, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, ErrForbiddenMsg("need admin permission for runtime framework")
	}
	newFrame := database.RuntimeFramework{
		ID:            id,
		FrameName:     req.FrameName,
		FrameVersion:  req.FrameVersion,
		FrameImage:    req.FrameImage,
		Enabled:       req.Enabled,
		ContainerPort: req.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}
	frame, err := c.runtimeFrameworksStore.Update(ctx, newFrame)
	if err != nil {
		return nil, fmt.Errorf("failed to update runtime frameworks, error: %w", err)
	}
	return &types.RuntimeFramework{
		ID:            frame.ID,
		FrameName:     frame.FrameName,
		FrameVersion:  frame.FrameVersion,
		FrameImage:    frame.FrameImage,
		Enabled:       frame.Enabled,
		ContainerPort: frame.ContainerPort,
		Type:          req.Type,
		EngineArgs:    req.EngineArgs,
	}, nil
}

func (c *repoComponentImpl) DeleteRuntimeFramework(ctx context.Context, currentUser string, id int64) error {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return ErrForbiddenMsg("need admin permission for runtime framework")
	}
	frame, err := c.runtimeFrameworksStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find runtime frameworks, error: %w", err)
	}
	err = c.runtimeFrameworksStore.Delete(ctx, *frame)
	return err
}

func (c *repoComponentImpl) ListDeploy(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) ([]types.DeployRepo, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		slog.Error("Failed to query deploy", slog.Any("error", err), slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		return nil, errors.New("invalid repository for query parameters")
	}
	if repo == nil {
		slog.Error("nothing found for deploys", slog.Any("repotype", repoType), slog.Any("namespace", namespace), slog.Any("name", name))
		return nil, errors.New("nothing found for deploys")
	}
	deploys, err := c.deployTaskStore.ListDeploy(ctx, repoType, repo.ID, user.ID)
	if err != nil {
		return nil, errors.New("fail to list user deploys")
	}
	var resDeploys []types.DeployRepo
	for _, deploy := range deploys {
		resDeploys = append(resDeploys, types.DeployRepo{
			DeployID:         deploy.ID,
			DeployName:       deploy.DeployName,
			RepoID:           deploy.RepoID,
			SvcName:          deploy.SvcName,
			Status:           deployStatusCodeToString(deploy.Status),
			Hardware:         deploy.Hardware,
			Env:              deploy.Env,
			RuntimeFramework: deploy.RuntimeFramework,
			ImageID:          deploy.ImageID,
			MinReplica:       deploy.MinReplica,
			MaxReplica:       deploy.MaxReplica,
			GitBranch:        deploy.GitBranch,
			ClusterID:        deploy.ClusterID,
			SecureLevel:      deploy.SecureLevel,
			CreatedAt:        deploy.CreatedAt,
			UpdatedAt:        deploy.UpdatedAt,
			Task:             string(deploy.Task),
			EngineArgs:       deploy.EngineArgs,
		})
	}
	return resDeploys, nil
}

func (c *repoComponentImpl) DeleteDeploy(ctx context.Context, delReq types.DeployActReq) error {
	user, deploy, err := c.checkDeployPermissionForUser(ctx, delReq)
	if err != nil {
		return err
	}
	// delete service
	deployRepo := types.DeployRepo{
		SpaceID:   0,
		DeployID:  delReq.DeployID,
		Namespace: delReq.Namespace,
		Name:      delReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	// purge service
	err = c.deployer.Purge(ctx, deployRepo)
	if err != nil {
		// fail to purge deploy instance, maybe service is gone
		slog.Warn("purge deploy instance", slog.Any("error", err))
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		// fail to check service
		return err
	}

	if exist {
		// fail to delete service
		return errors.New("fail to delete service")
	}

	// update database deploy
	err = c.deployTaskStore.DeleteDeploy(ctx, types.RepositoryType(delReq.RepoType), deploy.RepoID, user.ID, delReq.DeployID)
	if err != nil {
		return fmt.Errorf("fail to remove deploy instance, %w", err)
	}

	return err
}

func (c *repoComponentImpl) DeployDetail(ctx context.Context, detailReq types.DeployActReq) (*types.DeployRepo, error) {
	var (
		deploy *database.Deploy
		err    error
	)
	if detailReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, detailReq)
	} else {
		_, deploy, err = c.checkDeployPermissionForUser(ctx, detailReq)
	}
	if err != nil {
		return nil, err
	}

	endpoint, _ := c.GenerateEndpoint(ctx, deploy)

	req := types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: detailReq.Namespace,
		Name:      detailReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	actualReplica, desiredReplica, instList, err := c.deployer.GetReplica(ctx, req)
	if err != nil {
		slog.Warn("fail to get deploy replica", slog.Any("repotype", detailReq.RepoType), slog.Any("req", req), slog.Any("error", err))
	}
	endpointPrivate := true
	if deploy.SecureLevel == types.EndpointPublic {
		endpointPrivate = false
	}
	proxyEndPoint := ""
	if deploy.Type == types.FinetuneType {
		proxyEndPoint = endpoint + "/proxy/7860/"
	}
	repoPath := strings.TrimPrefix(deploy.GitPath, string(detailReq.RepoType)+"s_")

	varMap, err := common.JsonStrToMap(deploy.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert variables to map, error: %w", err)
	}
	var entrypoint string
	val, exist := varMap[types.GGUFEntryPoint]
	if exist {
		entrypoint = val
	}

	resDeploy := types.DeployRepo{
		DeployID:         deploy.ID,
		DeployName:       deploy.DeployName,
		RepoID:           deploy.RepoID,
		SvcName:          deploy.SvcName,
		Status:           deployStatusCodeToString(deploy.Status),
		Hardware:         deploy.Hardware,
		Env:              deploy.Env,
		RuntimeFramework: deploy.RuntimeFramework,
		ImageID:          deploy.ImageID,
		MinReplica:       deploy.MinReplica,
		MaxReplica:       deploy.MaxReplica,
		GitBranch:        deploy.GitBranch,
		ClusterID:        deploy.ClusterID,
		SecureLevel:      deploy.SecureLevel,
		CreatedAt:        deploy.CreatedAt,
		UpdatedAt:        deploy.UpdatedAt,
		Endpoint:         endpoint,
		ActualReplica:    actualReplica,
		DesiredReplica:   desiredReplica,
		Instances:        instList,
		Private:          endpointPrivate,
		Path:             repoPath,
		ProxyEndpoint:    proxyEndPoint,
		SKU:              deploy.SKU,
		Task:             string(deploy.Task),
		EngineArgs:       deploy.EngineArgs,
		Variables:        deploy.Variables,
		Entrypoint:       entrypoint,
		Reason:           deploy.Reason,
		Message:          deploy.Message,
	}

	return &resDeploy, nil
}

// generate endpoint
func (c *repoComponentImpl) GenerateEndpoint(ctx context.Context, deploy *database.Deploy) (string, string) {
	var endpoint string
	provider := ""
	cls, err := c.clusterInfoStore.ByClusterID(ctx, deploy.ClusterID)
	zone := ""
	if err != nil {
		slog.Warn("Get cluster with error", slog.Any("error", err))
	} else {
		zone = cls.Zone
		provider = cls.Provider
	}
	if len(deploy.SvcName) > 0 && deploy.Status == deployStatus.Running {
		// todo: zone.provider.endpoint to support multi-zone, multi-provider
		regionDomain := ""
		if len(zone) > 0 && len(provider) > 0 {
			regionDomain = fmt.Sprintf(".%s.%s", zone, provider)
		}
		if c.publicRootDomain == "" {
			endpoint, _ = url.JoinPath(c.serverBaseUrl, "endpoint", deploy.SvcName)
			endpoint = strings.Replace(endpoint, "http://", "", 1)
			endpoint = strings.Replace(endpoint, "https://", "", 1)
		} else {
			endpoint = fmt.Sprintf("%s%s.%s", deploy.SvcName, regionDomain, c.publicRootDomain)
		}

	}

	return endpoint, provider
}

func deployStatusCodeToString(code int) string {
	// DeployBuildPending    = 10
	// DeployBuildInProgress = 11
	// DeployBuildFailed     = 12
	// DeployBuildSucceed    = 13
	// DeployBuildSkip       = 14
	//
	// DeployPrepareToRun = 20
	// DeployStartUp      = 21
	// DeployRunning      = 22
	// DeployRunTimeError = 23
	// DeployStopped      = 26
	// DeployRunDeleted   = 27 // end user trigger delete action for deploy

	// simplified status for frontend show
	var txt string
	switch code {
	case 10:
		txt = SpaceStatusStopped
	case 11:
		txt = SpaceStatusBuilding
	case 12:
		txt = SpaceStatusBuildFailed
	case 13:
		txt = SpaceStatusDeploying
	case 20:
		txt = SpaceStatusDeploying
	case 21:
		txt = SpaceStatusDeployFailed
	case 22:
		txt = SpaceStatusDeploying
	case 23:
		txt = SpaceStatusRunning
	case 24:
		txt = SpaceStatusRuntimeError
	case 25:
		txt = SpaceStatusSleeping
	case 26:
		txt = SpaceStatusStopped
	case 27:
		txt = RepoStatusDeleted
	default:
		txt = SpaceStatusStopped
	}
	return txt
}

func (c *repoComponentImpl) DeployInstanceLogs(ctx context.Context, logReq types.DeployActReq) (*deploy.MultiLogReader, error) {
	var (
		deploy *database.Deploy
		err    error
	)
	if logReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, logReq)
	} else {
		_, deploy, err = c.checkDeployPermissionForUser(ctx, logReq)
	}

	if err != nil {
		return nil, err
	}
	return c.deployer.InstanceLogs(ctx, types.DeployRepo{
		DeployID:     deploy.ID,
		SpaceID:      deploy.SpaceID,
		ModelID:      deploy.ModelID,
		Namespace:    logReq.Namespace,
		Name:         logReq.Name,
		ClusterID:    deploy.ClusterID,
		SvcName:      deploy.SvcName,
		InstanceName: logReq.InstanceName,
	})
}

// check access repo permission by repo id
func (c *repoComponentImpl) AllowAccessByRepoID(ctx context.Context, repoID int64, username string) (bool, error) {
	r, err := c.repoStore.FindById(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("failed to get repository by repo_id: %d, %w", repoID, err)
	}
	if r == nil {
		return false, fmt.Errorf("invalid repository by repo_id: %d", repoID)
	}
	fields := strings.Split(r.Path, "/")
	return c.AllowReadAccess(ctx, r.RepositoryType, fields[0], fields[1], username)
}

// check access endpoint for rproxy
func (c *repoComponentImpl) AllowAccessEndpoint(ctx context.Context, currentUser string, deploy *database.Deploy) (bool, error) {
	if deploy.SecureLevel == types.EndpointPublic {
		// public endpoint
		return true, nil
	}
	return c.checkAccessDeployForUser(ctx, deploy.RepoID, currentUser, deploy)
}

// check access deploy permission
func (c *repoComponentImpl) AllowAccessDeploy(ctx context.Context, req types.DeployActReq) (bool, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return false, fmt.Errorf("failed to find %s repo %s/%s", req.RepoType, req.Namespace, req.Name)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, req.DeployID)
	if err != nil {
		return false, fmt.Errorf("fail to get deploy by ID: %v, %w", req.DeployID, err)
	}
	if deploy == nil {
		return false, fmt.Errorf("deploy not found by ID: %v", req.DeployID)
	}
	if req.DeployType == types.ServerlessType {
		return c.checkAccessDeployForServerless(ctx, repo.ID, req.CurrentUser, deploy)
	} else {
		return c.checkAccessDeployForUser(ctx, repo.ID, req.CurrentUser, deploy)
	}
}

// common check function for apiserver and rproxy
func (c *repoComponentImpl) checkAccessDeployForUser(ctx context.Context, repoID int64, currentUser string, deploy *database.Deploy) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, errors.New("user does not exist")
	}
	if deploy.UserID != user.ID {
		// deny access due to deploy was not created by
		return false, ErrForbiddenMsg("deploy was not created by user")
	}
	if deploy.RepoID != repoID {
		// deny access for invalid repo
		return false, errors.New("invalid deploy found")
	}
	return true, nil
}

func (c *repoComponentImpl) checkAccessDeployForServerless(ctx context.Context, repoID int64, currentUser string, deploy *database.Deploy) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, fmt.Errorf("user %s does not exist", currentUser)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return false, ErrForbiddenMsg("need admin permission to see Serverless deploy instances")
	}
	if deploy.RepoID != repoID {
		// deny access for invalid repo
		return false, errors.New("invalid deploy found")
	}
	return true, nil
}

func (c *repoComponentImpl) DeployStop(ctx context.Context, stopReq types.DeployActReq) error {
	var (
		user   *database.User
		deploy *database.Deploy
		err    error
	)
	if stopReq.DeployType == types.ServerlessType {
		user, deploy, err = c.checkDeployPermissionForServerless(ctx, stopReq)
	} else {
		user, deploy, err = c.checkDeployPermissionForUser(ctx, stopReq)
	}
	if err != nil {
		return fmt.Errorf("fail to check permission for stop deploy, %w", err)
	}
	// delete service
	deployRepo := types.DeployRepo{
		DeployID:  stopReq.DeployID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: stopReq.Namespace,
		Name:      stopReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	err = c.deployer.Stop(ctx, deployRepo)
	if err != nil {
		// fail to stop deploy instance, maybe service is gone
		slog.Warn("stop deploy instance with error", slog.Any("error", err), slog.Any("stopReq", stopReq))
	}

	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		// fail to check service
		return err
	}

	if exist {
		// fail to delete service
		return errors.New("fail to stop deploy instance")
	}

	// update database deploy to stopped
	err = c.deployTaskStore.StopDeploy(ctx, stopReq.RepoType, deploy.RepoID, user.ID, stopReq.DeployID)
	if err != nil {
		return fmt.Errorf("fail to stop deploy instance, %w", err)
	}

	return err
}

func (c *repoComponentImpl) AllowReadAccessByDeployID(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string, deployID int64) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, errors.New("user does not exist")
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployID)
	if err != nil {
		return false, err
	}
	if deploy == nil {
		return false, errors.New("fail to get deploy by ID")
	}
	if deploy.UserID != user.ID {
		return false, errors.New("deploy was not created by user")
	}
	if deploy.RepoID != repo.ID {
		return false, errors.New("found incorrect repo")
	}
	return c.AllowReadAccessRepo(ctx, repo, currentUser)
}

func (c *repoComponentImpl) DeployStatus(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployID int64) (types.ModelStatusEventData, error) {
	var status types.ModelStatusEventData
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployID)
	if err != nil {
		status.Status = SpaceStatusStopped
		return status, err
	}
	// request deploy status by deploy id
	_, code, instances, err := c.deployer.Status(ctx, types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: namespace,
		Name:      name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}, true)
	if err != nil {
		slog.Error("error happen when get deploy status", slog.Any("error", err), slog.String("path", deploy.GitPath))
		status.Status = SpaceStatusStopped
		status.Details = instances
		return status, err
	}
	status.Status = deployStatusCodeToString(code)
	status.Details = instances
	status.Message = deploy.Message
	status.Reason = deploy.Reason
	return status, nil
}

func (c *repoComponentImpl) GetDeployBySvcName(ctx context.Context, svcName string) (*database.Deploy, error) {
	d, err := c.deployTaskStore.GetDeployBySvcName(ctx, svcName)
	if err != nil {
		return nil, fmt.Errorf("failed to get deploy by svc name:%s, %w", svcName, err)
	}
	if d == nil {
		return nil, fmt.Errorf("do not found deploy by svc name:%s", svcName)
	}
	return d, nil
}

func (c *repoComponentImpl) SyncMirror(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) error {
	admin, err := c.CheckCurrentUserPermission(ctx, currentUser, namespace, membership.RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
	}

	if !admin {
		return ErrForbiddenMsg("need be owner or admin role to sync mirror for this repo")
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	mirror.Priority = types.HighMirrorPriority
	if c.config.GitServer.Type == types.GitServerTypeGitea {
		err = c.mirrorServer.MirrorSync(ctx, mirrorserver.MirrorSyncReq{
			Namespace: "root",
			Name:      mirror.LocalRepoPath,
		})
		if err != nil {
			return fmt.Errorf("failed to sync mirror, error: %w", err)
		}
	} else if c.config.GitServer.Type == types.GitServerTypeGitaly {
		c.mq.PushRepoMirror(&queue.MirrorTask{
			MirrorID:  mirror.ID,
			Priority:  queue.PriorityMap[mirror.Priority],
			CreatedAt: mirror.CreatedAt.Unix(),
		})
		mirror.Status = types.MirrorWaiting
		err = c.mirrorStore.Update(ctx, mirror)
		if err != nil {
			return fmt.Errorf("failed to update mirror status: %v", err)
		}
	}

	return nil
}

func (c *repoComponentImpl) checkDeployPermissionForUser(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy permission check user failed, %w", err)
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployReq.DeployID)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get user deploy %v, %w", deployReq.DeployID, err)
	}
	if deploy == nil {
		return nil, nil, fmt.Errorf("do not found user deploy %v", deployReq.DeployID)
	}
	if deploy.UserID != user.ID {
		return nil, nil, ErrForbiddenMsg("deploy was not created by user")
	}
	return &user, deploy, nil
}

func (c *repoComponentImpl) checkDeployPermissionForServerless(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy permission check user failed, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, nil, ErrForbiddenMsg("need admin permission for Serverless deploy")
	}
	deploy, err := c.deployTaskStore.GetDeployByID(ctx, deployReq.DeployID)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get serverless deploy:%v, %w", deployReq.DeployID, err)
	}
	if deploy == nil {
		return nil, nil, fmt.Errorf("do not found serverless deploy %v", deployReq.DeployID)
	}
	return &user, deploy, nil
}

func (c *repoComponentImpl) DeployUpdate(ctx context.Context, updateReq types.DeployActReq, req *types.DeployUpdateReq) error {
	var (
		deploy *database.Deploy
		err    error
	)
	if updateReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, updateReq)
	} else {
		_, deploy, err = c.checkDeployPermissionForUser(ctx, updateReq)
	}
	if err != nil {
		return fmt.Errorf("fail to check permission for update deploy, %w", err)
	}
	if req.ResourceID != nil {
		if req.RuntimeFrameworkID == nil {
			frame, err := c.runtimeFrameworksStore.FindEnabledByName(ctx, deploy.RuntimeFramework)
			if err != nil {
				return fmt.Errorf("cannot find available runtime framework by name %s, %w", deploy.RuntimeFramework, err)
			}
			//update runtime image once user changed cpu to gpu
			req.RuntimeFrameworkID = &frame.ID
		}
	}

	if req.ClusterID != nil {
		_, err = c.clusterInfoStore.ByClusterID(ctx, *req.ClusterID)
		if err != nil {
			return fmt.Errorf("invalid cluster %v, %w", *req.ClusterID, err)
		}
	}

	// check service
	deployRepo := types.DeployRepo{
		DeployID:  updateReq.DeployID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: updateReq.Namespace,
		Name:      updateReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return fmt.Errorf("check deploy exists, err: %w", err)
	}

	if needRestartDeploy(req) && exist {
		// deploy instance is running
		return errors.New("stop deploy first")
	}

	if req.EngineArgs != nil {
		_, err = common.JsonStrToMap(*req.EngineArgs)
		if err != nil {
			return fmt.Errorf("invalid engine args, %w", err)
		}
	}

	// update inference service and keep deploy_id and svc_name unchanged
	err = c.deployer.UpdateDeploy(ctx, req, deploy)
	return err
}

func needRestartDeploy(req *types.DeployUpdateReq) bool {
	if req.ClusterID != nil || req.RuntimeFrameworkID != nil || req.ResourceID != nil ||
		req.MaxReplica != nil || req.MinReplica != nil || req.Env != nil ||
		req.EngineArgs != nil || req.Variables != nil || req.Entrypoint != nil {
		return true
	}
	return false
}

func (c *repoComponentImpl) DeployStart(ctx context.Context, startReq types.DeployActReq) error {
	var (
		deploy *database.Deploy
		err    error
	)
	if startReq.DeployType == types.ServerlessType {
		_, deploy, err = c.checkDeployPermissionForServerless(ctx, startReq)
	} else {
		_, deploy, err = c.checkDeployPermissionForUser(ctx, startReq)
	}

	if err != nil {
		return fmt.Errorf("failed to check permission for start deploy, %w", err)
	}
	// check service
	deployRepo := types.DeployRepo{
		DeployID:  startReq.DeployID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: startReq.Namespace,
		Name:      startReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}
	exist, err := c.deployer.Exist(ctx, deployRepo)
	if err != nil {
		return err
	}

	if exist {
		// deploy instance is running
		return errors.New("stop deploy first")
	}

	// start deploy
	err = c.deployer.StartDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("fail to start deploy, %w", err)
	}

	return err
}

func (c *repoComponentImpl) AllFiles(ctx context.Context, req types.GetAllFilesReq) ([]*types.File, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("failed to find repo")
	}
	if repo.Private {
		read, err := c.CheckCurrentUserPermission(ctx, req.CurrentUser, req.Namespace, membership.RoleRead)
		if err != nil {
			return nil, fmt.Errorf("failed to check permission to get all files, error: %w", err)
		}

		if !read {
			return nil, ErrForbiddenMsg("users do not have permission to get all files for this repo")
		}
	}
	allFiles, err := getAllFiles(ctx, req.Namespace, req.Name, "", req.RepoType, req.Ref, c.git.GetTree)
	if err != nil {
		slog.Error("fail to get all files of repository", slog.Any("repoType", req.RepoType), slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("error", err.Error()))
		return nil, err
	}
	return allFiles, nil
}

func (c *repoComponentImpl) IsAdminRole(user database.User) bool {
	slog.Debug("Check if user is admin", slog.Any("user", user))
	return user.CanAdmin()
}

func (c *repoComponentImpl) GetNameSpaceInfo(ctx context.Context, path string) (*types.Namespace, error) {
	nsResp, err := c.userSvcClient.GetNameSpaceInfo(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace infor from user service, path: %s, error: %w", path, err)
	}
	ns := &types.Namespace{
		Path:   nsResp.Path,
		Avatar: nsResp.Avatar,
		Type:   nsResp.Type,
	}
	return ns, nil
}

func (c *repoComponentImpl) checkIfShouldUseLfs(ctx context.Context, req *types.CreateFileReq) (bool, *types.CreateFileReq) {
	gFile, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      GitAttributesFileName,
	})
	if err != nil {
		return false, req
	}
	fileName := filepath.Base(req.FilePath)
	decodedBytes, _ := base64.StdEncoding.DecodeString(gFile.Content)
	attributes := parseGitattributesContent(string(decodedBytes))
	useLfs := shouldUseLFS(fileName, attributes)
	if !useLfs {
		return false, req
	}
	pointer, pointerFile := generateLFSPointerFromContent([]byte(req.OriginalContent))
	req.Content = pointerFile
	req.Pointer = pointer
	return true, req
}

func (c *repoComponentImpl) checkIfShouldUseLfsUpdate(ctx context.Context, req *types.UpdateFileReq) (bool, *types.UpdateFileReq) {
	gFile, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Branch,
		Path:      GitAttributesFileName,
	})
	if err != nil {
		return false, req
	}
	fileName := filepath.Base(req.FilePath)
	decodedBytes, _ := base64.StdEncoding.DecodeString(gFile.Content)
	attributes := parseGitattributesContent(string(decodedBytes))
	useLfs := shouldUseLFS(fileName, attributes)
	if !useLfs {
		return false, req
	}
	pointer, pointerFile := generateLFSPointerFromContent([]byte(req.OriginalContent))
	req.Content = pointerFile
	req.Pointer = pointer
	return true, req
}

func parseGitattributesContent(content string) map[string][]string {
	attributes := make(map[string][]string)
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			attributes[parts[0]] = parts[1:]
		}
	}

	return attributes
}

func shouldUseLFS(filePath string, attributes map[string][]string) bool {
	filePath = strings.ToLower(filePath)
	for pattern, attrs := range attributes {
		matched, _ := filepath.Match(pattern, filePath)
		if matched {
			for _, attr := range attrs {
				if attr == "filter=lfs" || attr == "lfs" {
					return true
				}
			}
		}
	}
	return false
}

func generateLFSPointerFromContent(content []byte) (*types.Pointer, string) {
	hash := sha256.New()

	hash.Write(content)
	oid := fmt.Sprintf("sha256:%x", hash.Sum(nil))

	fileSize := int64(len(content))

	lfsPointerContent := fmt.Sprintf(`version https://git-lfs.github.com/spec/v1
oid %s
size %d
`, oid, fileSize)
	encodingContent := base64.StdEncoding.EncodeToString([]byte(lfsPointerContent))
	pointer := types.Pointer{
		Oid:  strings.Split(oid, ":")[1],
		Size: fileSize,
	}

	return &pointer, encodingContent
}
