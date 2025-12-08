package component

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/minio/minio-go/v7"
	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
	"opencsg.com/csghub-server/builder/deploy"
	deployStatus "opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/builder/multisync"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/mirror/cache"
	"opencsg.com/csghub-server/mq"
)

const (
	ErrNotFoundMessage    = "The target couldn't be found."
	ErrGetContentsOrList  = "GetContentsOrList"
	AdminSecret           = "gnuRYKce"
	GitAttributesFileName = ".gitattributes"
	GitIgnoreFileName     = ".gitignore"
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
	userResourcesStore     database.UserResourcesStore
	recomStore             database.RecomStore
	syncCache              cache.Cache
	multiSyncClient        multisync.Client
	sysMQ                  mq.MessageQueue
	mirrorTaskStore        database.MirrorTaskStore
	notificationSvcClient  rpc.NotificationSvcClient
	mirrorSvcClient        rpc.MirrorSvcClient
	xnetClient             rpc.XnetSvcClient
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
	RemoteTree(ctx context.Context, req *types.GetTreeRequest) (*types.GetRepoFileTreeResp, error)
	LogsTree(ctx context.Context, req *types.GetLogsTreeRequest) (*types.LogsTreeResp, error)
	UploadFile(ctx context.Context, req *types.CreateFileReq) error
	SDKListFiles(ctx context.Context, repoType types.RepositoryType, namespace, name, ref, userName string) (*types.SDKFiles, error)
	IsLfs(ctx context.Context, req *types.GetFileReq) (bool, int64, error)
	HeadDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (*types.File, *types.Commit, error)
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
	GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error)
	UpdateMirror(ctx context.Context, req types.UpdateMirrorReq) (*database.Mirror, error)
	DeleteMirror(ctx context.Context, req types.DeleteMirrorReq) error
	// get runtime framework list with type
	ListRuntimeFrameworkWithType(ctx context.Context, deployType int) ([]types.RuntimeFramework, error)
	// get runtime framework list
	ListRuntimeFramework(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFramework, error)
	ListRuntimeFrameworkV2(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFrameworkV2, error)
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
	MirrorProgress(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) (types.LFSSyncProgressResp, error)
	DeployUpdate(ctx context.Context, updateReq types.DeployActReq, req *types.DeployUpdateReq) error
	DeployStart(ctx context.Context, startReq types.DeployActReq) error
	AllFiles(ctx context.Context, req types.GetAllFilesReq) (*types.GetRepoFileTreeResp, error)
	GetUserRepoPermission(ctx context.Context, userName string, repo *database.Repository) (*types.UserRepoPermission, error)
	CheckCurrentUserPermission(ctx context.Context, userName string, namespace string, role membership.Role) (bool, error)
	GetNameSpaceInfo(ctx context.Context, path string) (*types.Namespace, error)
	RelatedRepos(ctx context.Context, repoID int64, currentUser string) (map[types.RepositoryType][]*database.Repository, error)
	VisiableToUser(ctx context.Context, repos []*database.Repository, currentUser string) ([]*database.Repository, error)
	GenerateEndpoint(ctx context.Context, deploy *database.Deploy) (string, string)
	FixRepoSource(ctx context.Context) error
	IsAdminRole(user database.User) bool
	CheckAccountAndResource(ctx context.Context, userName string, clusterID string, orderDetailID int64, resource *database.SpaceResource) error
	DiffBetweenTwoCommits(ctx context.Context, req types.GetDiffBetweenCommitsReq) ([]types.GiteaCallbackPushReq_Commit, error)
	RemoteDiff(ctx context.Context, req types.GetDiffBetweenCommitsReq) ([]types.RemoteDiffs, error)
	SendAssetManagementMsg(ctx context.Context, req types.RepoNotificationReq) error
	Preupload(ctx context.Context, req types.PreuploadReq) (*types.PreuploadResp, error)
	CommitFiles(ctx context.Context, req types.CommitFilesReq) error
	IsExists(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error)
	ValidateYaml(ctx context.Context, req types.ValidateYamlReq) error
	ParseNDJson(ctx *gin.Context) (*types.CommitFilesReq, error)
	IsSyncing(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error)
	ChangePath(ctx context.Context, req types.ChangePathReq) error
	BatchMigrateRepoToHashedPath(ctx context.Context, auto bool, batchSize int, lastID int64) (int64, error)
	GetMirrorTaskStatusAndSyncStatus(repo *database.Repository) (types.MirrorTaskStatus, types.RepositorySyncStatus)
	CheckDeployPermissionForUser(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error)
	IsXnetEnabled(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (*types.XetEnabled, error)
}

func NewRepoComponentImpl(config *config.Config) (*repoComponentImpl, error) {
	r, err := NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return r.(*repoComponentImpl), nil
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
				return nil, nil, errorx.ErrForbiddenMsg("users do not have permission to create repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, nil, errorx.ErrForbiddenMsg("users do not have permission to create repo in this namespace")
			}
		}
	}
	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	temPath := strings.SplitN(uuid.NewString(), "-", 2)
	dbRepo := database.Repository{
		UserID: user.ID,
		// Path:           path.Join(req.Namespace, req.Name),
		// GitPath:        fmt.Sprintf("%ss_%s/%s", string(req.RepoType), req.Namespace, req.Name),
		Path:           path.Join(temPath[0], temPath[1]),
		GitPath:        fmt.Sprintf("%ss_%s/%s", string(req.RepoType), temPath[0], temPath[1]),
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Private:        req.Private,
		License:        req.License,
		DefaultBranch:  req.DefaultBranch,
		RepositoryType: req.RepoType,
		StarCount:      req.StarCount,
		User:           user,
	}
	newDBRepo, err := c.repoStore.CreateRepo(ctx, dbRepo)
	if err != nil {
		return nil, nil, fmt.Errorf("fail to create database repo, error: %w", err)
	}

	err = c.recomStore.UpsertScore(ctx, []*database.RecomRepoScore{
		{
			RepositoryID: newDBRepo.ID,
			Score:        0,
			WeightName:   database.RecomWeightTotal,
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("fail to upsert recom repo score, error: %w", err)
	}

	gitRepoReq := gitserver.CreateRepoReq{
		Username:      req.Username,
		Namespace:     temPath[0],
		Name:          temPath[1],
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

	if len(req.CommitFiles) > 0 {
		var gitCommitFiles []gitserver.CommitFile
		for _, file := range req.CommitFiles {
			gitCommitFiles = append(gitCommitFiles, gitserver.CommitFile{
				Content: base64.StdEncoding.EncodeToString([]byte(file.Content)),
				Path:    file.Path,
				Action:  gitserver.CommitActionCreate,
			})
		}
		err = c.git.CommitFiles(ctx, gitserver.CommitFilesReq{
			Namespace: temPath[0],
			Name:      temPath[1],
			RepoType:  req.RepoType,
			Revision:  req.DefaultBranch,
			Username:  user.Username,
			Email:     user.Email,
			Message:   types.InitCommitMessage,
			Files:     gitCommitFiles,
		})
		if err != nil {
			return gitRepo, newDBRepo, fmt.Errorf("fail to commit files, error: %w", err)
		}
	}

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

	// Admin users have full permissions.
	if user.CanAdmin() {
		if req.Private != nil {
			repo.Private = *req.Private
		}
	} else {
		// Handle permissions for non-admin users.
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to update repo in this organization")
			}
			// Non-admins cannot change the privacy of an organization's repository.
			if req.Private != nil {
				return nil, errorx.ErrForbiddenMsg("only admins can change the privacy of an organization repository")
			}
		} else {
			// This is a user namespace.
			if namespace.Path != user.Username {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to update repo in this namespace")
			}
			// Users can change the privacy of their own repositories.
			if req.Private != nil {
				// Additional check if making the repository public.
				if !*req.Private {
					allow, reason := c.allowPublic(repo)
					if !allow {
						err := errors.New("cannot change repo to public: " + reason)
						return nil, errorx.CannotSetRepoVisibility(err, nil)
					}
				}
				repo.Private = *req.Private
			}
		}
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

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleAdmin)
			if err != nil {
				return nil, err
			}
			if !canWrite {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to delete repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, errorx.ErrForbiddenMsg("users do not have permission to delete repo in this namespace")
			}
		}
	}

	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("fail to find mirror, %w", err)
	}

	err = c.repoStore.CleanRelationsByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("fail to clean repo relations, %w", err)
	}

	if mirror != nil {
		err = c.mirrorSvcClient.CancelMirror(ctx, mirror.ID)
		if err != nil {
			return nil, fmt.Errorf("fail to cancel mirror, %w", err)
		}
	}

	err = c.git.DeleteRepo(ctx, repo.GitalyPath())
	if err != nil && status.Code(err) != codes.NotFound {
		slog.Error("fail to update repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to delete repo in git, error: %w", err)
	}

	err = c.repoStore.DeleteRepo(ctx, *repo)
	if err != nil {
		slog.Error("fail to delete repo in git ", slog.Any("req", req), slog.String("error", err.Error()))
		return nil, fmt.Errorf("fail to delete repo in database, error: %w", err)
	}
	repo.User = user
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
			// get user's orgs
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

// relatedRepos gets all repos related to the given repo, and return them by repo type
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

	relatedRepos, err := c.repoStore.FindByIds(ctx, relatedRepoIDs, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get relation to repositories by ids, error: %w", err)
	}

	relatedRepos, err = c.VisiableToUser(ctx, relatedRepos, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check related repositories visiable to user:%s, %w", currentUser, err)
	}
	res := make(map[types.RepositoryType][]*database.Repository)
	for _, repo := range relatedRepos {
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
		return nil, errorx.ErrUnauthorized
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

	if useLfs {
		objectKey := common.BuildLfsPath(repo.ID, req.Pointer.Oid, repo.Migrated)
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
	} else {
		if int64(len(req.Content)) > c.config.Git.MaxUnLfsFileSize {
			return nil, fmt.Errorf("file size exceeds the maximum allowed size for non-LFS files: %d > %d", len(req.Content), c.config.Git.MaxUnLfsFileSize)
		}
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
		return nil, err
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to update file in this repo")
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

	if useLfs {
		objectKey := common.BuildLfsPath(repo.ID, req.Pointer.Oid, repo.Migrated)
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
	} else {
		if int64(len(req.Content)) > c.config.Git.MaxUnLfsFileSize {
			return nil, fmt.Errorf("file size exceeds the maximum allowed size for non-LFS files: %d > %d", len(req.Content), c.config.Git.MaxUnLfsFileSize)
		}
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to delete file in this repo")
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
		return nil, nil, errorx.ErrUnauthorized
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get last commit in this repo")
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
		return "", errorx.ErrForbiddenMsg("users do not have permission to get file raw in this repo")
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
		return nil, 0, "", errorx.ErrForbiddenMsg("users do not have permission to download file in this repo")
	}

	err = c.repoStore.UpdateRepoFileDownloads(ctx, repo, time.Now(), 1)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to update %s file download count, error: %w", req.RepoType, err)
	}
	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}
	if req.Lfs {
		objectKey := common.BuildLfsPath(repo.ID, strings.ReplaceAll(req.Path, "/", ""), repo.Migrated)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get branches in this repo")
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get tags in this repo")
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
		return errorx.ErrForbiddenMsg("users do not have permission to update tags in this repo")
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get tree in this repo")
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get tree in this repo")
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get logs tree in this repo")
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
		return nil, errorx.ErrNotFound
	}

	canRead, err := c.AllowReadAccessRepo(ctx, repo, userName)
	if err != nil {
		return nil, err
	}
	if !canRead {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to access this repo")
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
		if e, ok := err.(errorx.CustomError); ok && e.Is(errorx.ErrGitFileNotFound) {
			return false, -1, errorx.ErrNotFound
		}
		slog.Error("failed to get %s file raw", string(req.RepoType), slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("path", req.Path))
		return false, -1, err
	}

	return strings.HasPrefix(content, types.LFSPrefix), int64(len(content)), nil
}

func (c *repoComponentImpl) HeadDownloadFile(ctx context.Context, req *types.GetFileReq, userName string) (*types.File, *types.Commit, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	canRead, err := c.AllowReadAccessRepo(ctx, repo, userName)
	if err != nil {
		return nil, nil, err
	}
	if !canRead {
		return nil, nil, errorx.ErrForbiddenMsg("users do not have permission to download file in this repo")
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
			return nil, nil, errorx.ErrNotFound
		}
		return nil, nil, fmt.Errorf("failed to download git %s repository file, error: %w", req.RepoType, err)
	}

	lastCommitReq := gitserver.GetRepoLastCommitReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		RepoType:  req.RepoType,
	}

	lastCommit, err := c.git.GetRepoLastCommit(ctx, lastCommitReq)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get last commit, error: %w", err)
	}
	return file, lastCommit, nil
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
		return nil, 0, "", errorx.ErrForbiddenMsg("users do not have permission to download file in this repo")
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
		objectKey := common.BuildLfsPath(repo.ID, file.LfsSHA256, repo.Migrated)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
		if err != nil {
			if err.Error() == ErrNotFoundMessage || err.Error() == ErrGetContentsOrList {
				return nil, 0, downloadUrl, errorx.ErrNotFound
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
				return nil, 0, downloadUrl, errorx.ErrNotFound
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
		objectKey := common.BuildLfsPath(repo.ID, file.LfsSHA256, repo.Migrated)
		reqParams := make(url.Values)
		if req.SaveAs != "" {
			// allow rename when download through content-disposition header
			reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
		}
		signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
		if err != nil {
			if err.Error() == ErrNotFoundMessage || err.Error() == ErrGetContentsOrList {
				return nil, 0, downloadUrl, errorx.ErrNotFound
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
				return nil, 0, downloadUrl, errorx.ErrNotFound
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get file info in this repo")
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
		Namespace:   req.Namespace,
		Name:        req.Name,
		Ref:         req.Ref,
		Path:        req.Path,
		RepoType:    req.RepoType,
		MaxFileSize: c.adjustMaxFileSize(req.MaxFileSize),
	}
	file, err := c.git.GetRepoFileContents(ctx, getFileContentReq)
	if err != nil {
		if errors.Is(err, errorx.ErrFileTooLarge) {
			// return basic file info, but tell client not to show the file content
			file.PreviewCode = types.FilePreviewCodeTooLarge
			return file, nil
		}
		return nil, fmt.Errorf("failed to get git model repository file info, error: %w", err)
	}
	decodedFileContent, _ := base64.StdEncoding.DecodeString(file.Content)
	file.PreviewCode = c.getFilePreviewCode(decodedFileContent)
	return file, nil
}

func (c *repoComponentImpl) getFilePreviewCode(fileContent []byte) types.FilePreviewCode {
	// detect the file content type like text/plain, image/jpeg, etc
	detectedType := http.DetectContentType(fileContent)
	switch {
	case strings.HasPrefix(detectedType, "text"):
		return types.FilePreviewCodeNormal
	default:
		return types.FilePreviewCodeNotText
	}
}

func (c *repoComponentImpl) adjustMaxFileSize(maxFileSize int64) int64 {
	// same with aliyun green check large content size
	const maxModerationContentSize = 100 * 9000
	if maxFileSize == 0 || maxFileSize > maxModerationContentSize {
		maxFileSize = maxModerationContentSize
	}
	return maxFileSize
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
	case types.MCPServerRepo:
		return types.MCPTagScope
	default:
		panic("convert repo type to tag scope failed, unknown repo type:" + repoType)
	}
}

func (c *repoComponentImpl) AllowReadAccessRepo(ctx context.Context, repo *database.Repository, username string) (bool, error) {
	if !repo.Private {
		return true, nil
	}

	if username == "" {
		return false, errorx.ErrUserNotFound
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
		return false, errorx.ErrUserNotFound
	}

	return c.CheckCurrentUserPermission(ctx, username, namespace, membership.RoleWrite)
}

func (c *repoComponentImpl) AllowAdminAccess(ctx context.Context, repoType types.RepositoryType, namespace, name, username string) (bool, error) {
	_, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if username == "" {
		return false, errorx.ErrUserNotFound
	}

	return c.CheckCurrentUserPermission(ctx, username, namespace, membership.RoleAdmin)
}

func (c *repoComponentImpl) GetUserRepoPermission(ctx context.Context, userName string, repo *database.Repository) (*types.UserRepoPermission, error) {
	if userName == "" {
		// anonymous user only has read permission to public repo
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
		// owner has full permission
		if userName == namespace {
			return &types.UserRepoPermission{
				CanRead:  true,
				CanWrite: true,
				CanAdmin: true,
			}, nil
		} else {
			// other user has read permission to pubic repo
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
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get commit in this repo")
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
	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath
	mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	mirror.RepositoryID = repo.ID
	mirror.Repository = repo

	sourceType, sourcePath, err := common.GetSourceTypeAndPathFromURL(req.SourceUrl)
	if err == nil {
		err = c.repoStore.UpdateSourcePath(ctx, repo.ID, sourcePath, sourceType)
		if err != nil {
			return nil, fmt.Errorf("failed to update source path in repo: %w", err)
		}
	}

	mirror.MirrorTaskID = taskId
	mirror.Priority = types.ASAPMirrorPriority

	reqMirror, err := c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror")
	}

	reqMirror.Status = types.MirrorQueued
	err = c.mirrorStore.Update(ctx, reqMirror)
	if err != nil {
		return nil, fmt.Errorf("failed to update mirror status: %w", err)
	}
	mt := database.MirrorTask{
		MirrorID: mirror.ID,
		Priority: mirror.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.Create(ctx, mt)
	if err != nil {
		return nil, fmt.Errorf("failed to create mirror task: %w", err)
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
		err := c.mirrorFromSaasSync(ctx, m, repo, namespace, name, repoType)
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
	mirror.Repository = repo
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
		return fmt.Errorf("failed to create mirror repo in git: %w", err)
	}

	mirror.MirrorTaskID = taskId

	m, err = c.mirrorStore.Create(ctx, &mirror)
	if err != nil {
		return fmt.Errorf("failed to create mirror: %w", err)
	}

	mt := database.MirrorTask{
		MirrorID: m.ID,
		Priority: m.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.CancelOtherTasksAndCreate(ctx, mt)
	if err != nil {
		return fmt.Errorf("failed to create mirror task: %w", err)
	}

	repo.SyncStatus = types.SyncStatusPending

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) mirrorFromSaasSync(ctx context.Context, mirror *database.Mirror, repo *database.Repository, namespace, name string, repoType types.RepositoryType) error {
	var err error
	mt := database.MirrorTask{
		MirrorID: mirror.ID,
		Priority: mirror.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.CancelOtherTasksAndCreate(ctx, mt)
	if err != nil {
		return fmt.Errorf("failed to create mirror task: %w", err)
	}

	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo sync status: %w", err)
	}

	return nil
}

func (c *repoComponentImpl) GetMirror(ctx context.Context, req types.GetMirrorReq) (*types.Mirror, error) {
	var (
		status      types.MirrorTaskStatus
		progress    int8
		lastMessage string
	)
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
	if mirror.CurrentTask != nil {
		status = mirror.CurrentTask.Status
		progress = int8(mirror.CurrentTask.Progress)
		lastMessage = mirror.CurrentTask.ErrorMessage
	}
	resMirror := &types.Mirror{
		ID:        mirror.ID,
		SourceUrl: mirror.SourceUrl,
		MirrorSource: types.MirrorSource{
			SourceName: mirror.MirrorSource.SourceName,
		},
		Username:        mirror.Username,
		AccessToken:     mirror.AccessToken,
		PushUrl:         mirror.PushUrl,
		PushUsername:    mirror.PushUsername,
		PushAccessToken: mirror.PushAccessToken,
		LastUpdatedAt:   mirror.LastUpdatedAt,
		SourceRepoPath:  mirror.SourceRepoPath,
		LocalRepoPath:   fmt.Sprintf("%ss/%s", mirror.Repository.RepositoryType, mirror.Repository.Path),
		LastMessage:     lastMessage,
		Status:          status,
		Progress:        progress,
		CreatedAt:       mirror.CreatedAt,
		UpdatedAt:       mirror.UpdatedAt,
	}
	return resMirror, nil
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

func (c *repoComponentImpl) ListRuntimeFrameworkV2(ctx context.Context, repoType types.RepositoryType, namespace, name string, deployType int) ([]types.RuntimeFrameworkV2, error) {
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
	var frameList []types.RuntimeFrameworkV2
	for _, modelFrame := range frames {
		systemDriverVersion := c.config.Runner.SystemCUDAVersion
		if systemDriverVersion != "" && modelFrame.ComputeType == string(types.ResourceTypeGPU) {
			frameDriverVersion, _ := version.NewVersion(modelFrame.DriverVersion)
			systemDriverVersion, _ := version.NewVersion(systemDriverVersion)
			// ignore unsupported driver version
			if frameDriverVersion.GreaterThan(systemDriverVersion) {
				continue
			}
		}
		exist, index := c.checkFrameNameExist(modelFrame.FrameName, frameList)
		if !exist {
			frameList = append(frameList, types.RuntimeFrameworkV2{
				FrameName: modelFrame.FrameName,
			})
			index = len(frameList) - 1
		}
		frameVersion := strings.Split(modelFrame.FrameImage, ":")[1]
		frameList[index].Versions = append(frameList[index].Versions, types.RuntimeFramework{
			ID:            modelFrame.ID,
			FrameName:     modelFrame.FrameName,
			FrameVersion:  frameVersion,
			FrameImage:    modelFrame.FrameImage,
			Enabled:       modelFrame.Enabled,
			ContainerPort: modelFrame.ContainerPort,
			EngineArgs:    modelFrame.EngineArgs,
			ComputeType:   modelFrame.ComputeType,
			DriverVersion: modelFrame.DriverVersion,
			Description:   modelFrame.Description,
			Type:          modelFrame.Type,
		})
		if !slices.Contains(frameList[index].ComputeTypes, modelFrame.ComputeType) {
			frameList[index].ComputeTypes = append(frameList[index].ComputeTypes, modelFrame.ComputeType)
		}

	}
	return frameList, nil
}

// check if the frame name is in the list
func (c *repoComponentImpl) checkFrameNameExist(frameName string, frameList []types.RuntimeFrameworkV2) (bool, int) {
	for index, frame := range frameList {
		if frameName == frame.FrameName {
			return true, index
		}
	}
	return false, 0
}

func (c *repoComponentImpl) CreateRuntimeFramework(ctx context.Context, req *types.RuntimeFrameworkReq) (*types.RuntimeFramework, error) {
	// found user id
	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for runtime framework, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, errorx.ErrForbiddenMsg("need admin permission for runtime framework")
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
		return nil, errorx.ErrForbiddenMsg("need admin permission for runtime framework")
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
		return errorx.ErrForbiddenMsg("need admin permission for runtime framework")
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
	if delReq.DeployType == types.ServerlessType {
		repo, err := c.repoStore.FindByPath(ctx, delReq.RepoType, delReq.Namespace, delReq.Name)
		if err != nil {
			return fmt.Errorf("fail to find repo for serverless, %w", err)
		}
		d, err := c.deployTaskStore.GetServerlessDeployByRepID(ctx, repo.ID)
		if err != nil {
			return fmt.Errorf("fail to get deploy for serverless, %w", err)
		}
		if d != nil {
			delReq.DeployID = d.ID
		} else {
			return fmt.Errorf("no deploy found for serverless type")
		}
	}
	user, deploy, err := c.CheckDeployPermissionForUser(ctx, delReq)
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
	if delReq.DeployType == types.ServerlessType {
		err = c.deployTaskStore.DeleteDeployNow(ctx, delReq.DeployID)
	} else {
		err = c.deployTaskStore.DeleteDeploy(ctx, types.RepositoryType(delReq.RepoType), deploy.RepoID, user.ID, delReq.DeployID)
	}

	if err != nil {
		return fmt.Errorf("fail to remove deploy instance, %w", err)
	}
	// release resource if it's a order case
	if deploy.OrderDetailID != 0 {
		ur, err := c.userResourcesStore.FindUserResourcesByOrderDetailId(ctx, deploy.UserUUID, deploy.OrderDetailID)
		if err != nil {
			return fmt.Errorf("fail to find user resource, %w", err)
		}
		ur.DeployId = 0
		err = c.userResourcesStore.UpdateDeployId(ctx, ur)
		if err != nil {
			return fmt.Errorf("fail to release resource, %w", err)
		}

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
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, detailReq)
	}
	if err != nil {
		return nil, err
	}

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

	_, code, _, err := c.deployer.Status(ctx, types.DeployRepo{
		DeployID:  deploy.ID,
		SpaceID:   deploy.SpaceID,
		ModelID:   deploy.ModelID,
		Namespace: detailReq.Namespace,
		Name:      detailReq.Name,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}, false)
	if err != nil {
		slog.Warn("fail to get deploy status", slog.Any("repo type", detailReq.RepoType), slog.Any("svc name", deploy.SvcName), slog.Any("error", err))
	}

	deploy.Status = code

	endpoint, _ := c.GenerateEndpoint(ctx, deploy)

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

	// Check if engine_args contains tool-call-parser parameter
	supportFunctionCall := strings.Contains(deploy.EngineArgs, "tool-call-parser")

	resDeploy := types.DeployRepo{
		DeployID:            deploy.ID,
		DeployName:          deploy.DeployName,
		RepoID:              deploy.RepoID,
		SvcName:             deploy.SvcName,
		Status:              deployStatusCodeToString(code),
		Hardware:            deploy.Hardware,
		Env:                 deploy.Env,
		RuntimeFramework:    deploy.RuntimeFramework,
		ImageID:             deploy.ImageID,
		MinReplica:          deploy.MinReplica,
		MaxReplica:          deploy.MaxReplica,
		GitBranch:           deploy.GitBranch,
		ClusterID:           deploy.ClusterID,
		SecureLevel:         deploy.SecureLevel,
		CreatedAt:           deploy.CreatedAt,
		UpdatedAt:           deploy.UpdatedAt,
		Endpoint:            endpoint,
		ActualReplica:       actualReplica,
		DesiredReplica:      desiredReplica,
		Instances:           instList,
		Private:             endpointPrivate,
		Path:                repoPath,
		ProxyEndpoint:       proxyEndPoint,
		SKU:                 deploy.SKU,
		Task:                string(deploy.Task),
		EngineArgs:          deploy.EngineArgs,
		Variables:           deploy.Variables,
		Entrypoint:          entrypoint,
		Reason:              deploy.Reason,
		Message:             deploy.Message,
		SupportFunctionCall: supportFunctionCall,
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
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, logReq)
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
	if deploy.RepoID != repoID {
		return false, errors.New("invalid deploy found")
	}
	if deploy.UserID == user.ID || c.IsAdminRole(user) || c.IsInSameOrg(ctx, user.ID, deploy.UserID) {
		return true, nil
	}
	return false, errorx.ErrForbiddenMsg("deploy was not created by user")
}

func (c *repoComponentImpl) checkAccessDeployForServerless(ctx context.Context, repoID int64, currentUser string, deploy *database.Deploy) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return false, fmt.Errorf("user %s does not exist", currentUser)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return false, errorx.ErrForbiddenMsg("need admin permission to see Serverless deploy instances")
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
		user, deploy, err = c.CheckDeployPermissionForUser(ctx, stopReq)
	}
	if err != nil {
		return fmt.Errorf("fail to check permission for stop deploy, %w", err)
	}
	// delete service
	deployRepo := types.DeployRepo{
		DeployID:      stopReq.DeployID,
		SpaceID:       deploy.SpaceID,
		ModelID:       deploy.ModelID,
		Namespace:     stopReq.Namespace,
		Name:          stopReq.Name,
		SvcName:       deploy.SvcName,
		ClusterID:     deploy.ClusterID,
		OrderDetailID: deploy.OrderDetailID,
		UserUUID:      user.UUID,
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
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		admin, err := c.CheckCurrentUserPermission(ctx, currentUser, namespace, membership.RoleAdmin)
		if err != nil {
			return fmt.Errorf("failed to check permission to create mirror, error: %w", err)
		}

		if !admin {
			return errorx.ErrForbiddenMsg("need be owner or admin role to sync mirror for this repo")
		}
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	mirror, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to find mirror, error: %w", err)
	}
	mirror.Priority = types.ASAPMirrorPriority
	mirror.Status = types.MirrorQueued
	err = c.mirrorStore.Update(ctx, mirror)
	if err != nil {
		return fmt.Errorf("failed to update mirror status: %w", err)
	}
	mt := database.MirrorTask{
		MirrorID: mirror.ID,
		Priority: mirror.Priority,
		Status:   types.MirrorQueued,
	}
	_, err = c.mirrorTaskStore.CancelOtherTasksAndCreate(ctx, mt)
	if err != nil {
		return fmt.Errorf("failed to create mirror task: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) MirrorProgress(ctx context.Context, repoType types.RepositoryType, namespace, name, currentUser string) (types.LFSSyncProgressResp, error) {
	var (
		progressResp types.LFSSyncProgressResp
		done         int
	)
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return progressResp, errors.New("user does not exist")
	}
	if !user.CanAdmin() {
		admin, err := c.CheckCurrentUserPermission(ctx, currentUser, namespace, membership.RoleAdmin)
		if err != nil {
			return progressResp, fmt.Errorf("failed to check permission to create mirror, error: %w", err)
		}

		if !admin {
			return progressResp, errorx.ErrForbiddenMsg("need be owner or admin role to check mirror progress for this repo")
		}
	}
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return progressResp, fmt.Errorf("failed to find repo, error: %w", err)
	}
	_, err = c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return progressResp, fmt.Errorf("failed to find mirror, error: %w", err)
	}
	lfsMetaObjects, err := c.lfsMetaObjectStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return progressResp, fmt.Errorf("failed to find lfs meta objects by repo id, error: %w", err)
	}
	repoPath := fmt.Sprintf("%ss/%s", repo.RepositoryType, repo.Path)
	for _, lfsMetaObject := range lfsMetaObjects {
		var resp types.SingleLFSProgress
		if lfsMetaObject.Existing {
			resp = types.SingleLFSProgress{
				Oid:      lfsMetaObject.Oid,
				Progress: 100,
				Size:     lfsMetaObject.Size,
			}
			done++
		} else {
			progress, _ := c.syncCache.GetLfsSyncFileProgress(ctx, repoPath, lfsMetaObject.Oid, strconv.Itoa(c.config.Mirror.PartSize))
			resp = types.SingleLFSProgress{
				Oid:      lfsMetaObject.Oid,
				Progress: progress,
				Size:     lfsMetaObject.Size,
			}
		}
		progressResp.Progress = append(progressResp.Progress, resp)
	}
	progressResp.Total = len(lfsMetaObjects)
	progressResp.Done = done

	return progressResp, nil
}

func (c *repoComponentImpl) CheckDeployPermissionForUser(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
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

	if deploy.UserID == user.ID || c.IsAdminRole(user) || c.IsInSameOrg(ctx, user.ID, deploy.UserID) {
		return &user, deploy, nil
	}
	return nil, nil, errorx.ErrForbiddenMsg("deploy was not created by user")
}

func (c *repoComponentImpl) checkDeployPermissionForServerless(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error) {
	user, err := c.userStore.FindByUsername(ctx, deployReq.CurrentUser)
	if err != nil {
		return nil, nil, fmt.Errorf("deploy permission check user failed, %w", err)
	}
	isAdmin := c.IsAdminRole(user)
	if !isAdmin {
		return nil, nil, errorx.ErrForbiddenMsg("need admin permission for Serverless deploy")
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
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, updateReq)
	}
	if err != nil {
		return fmt.Errorf("fail to check permission for update deploy, %w", err)
	}
	// check user balance if resource changed
	if req.ResourceID != nil {
		// don't support switch reserved resource
		if deploy.OrderDetailID != 0 {
			return fmt.Errorf("don't support switch reserved resource so far")
		}
		// resource available only if err is nil, err message should contain
		// the reason why resource is unavailable
		resource, err := c.spaceResourceStore.FindByID(ctx, *req.ResourceID)
		if err != nil {
			return fmt.Errorf("cannot find available resource, %w", err)
		}
		err = c.CheckAccountAndResource(ctx, updateReq.CurrentUser, resource.ClusterID, deploy.OrderDetailID, resource)
		if err != nil {
			return err
		}
		if req.RuntimeFrameworkID == nil {
			frame, err := c.runtimeFrameworksStore.FindEnabledByName(ctx, deploy.RuntimeFramework)
			if err != nil {
				return fmt.Errorf("cannot find available runtime framework by name , %w", err)
			}
			// update runtime image once user changed cpu to gpu
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
		_, deploy, err = c.CheckDeployPermissionForUser(ctx, startReq)
	}

	if err != nil {
		return fmt.Errorf("failed to check permission for start deploy, %w", err)
	}
	// check user balance
	resourceId, err := strconv.ParseInt(deploy.SKU, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse resource id, %w", err)
	}
	resource, err := c.spaceResourceStore.FindByID(ctx, resourceId)
	if err != nil {
		return fmt.Errorf("failed to find resource, %w", err)
	}
	// check resource available
	err = c.CheckAccountAndResource(ctx, startReq.CurrentUser, deploy.ClusterID, deploy.OrderDetailID, resource)
	if err != nil {
		return err
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
		// check deploy status
		_, status, _, err := c.deployer.Status(ctx, deployRepo, false)
		if err != nil {
			return fmt.Errorf("failed to get deploy status, %w", err)
		}

		// if deploy is in running status, return error
		const deployStatusRunning = 4
		if status == deployStatusRunning {
			return errors.New("stop deploy first")
		}

		// if deploy exists but not running, stop it first
		err = c.deployer.Stop(ctx, deployRepo)
		if err != nil {
			return fmt.Errorf("failed to stop existing deploy, %w", err)
		}
	}

	// start deploy
	err = c.deployer.StartDeploy(ctx, deploy)
	if err != nil {
		return fmt.Errorf("fail to start deploy, %w", err)
	}

	return err
}

func (c *repoComponentImpl) AllFiles(ctx context.Context, req types.GetAllFilesReq) (*types.GetRepoFileTreeResp, error) {
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
			return nil, errorx.ErrForbiddenMsg("users do not have permission to get all files for this repo")
		}
	}
	resp, err := c.git.GetTree(ctx, types.GetTreeRequest{
		Ref:       req.Ref,
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Limit:     req.Limit,
		Cursor:    req.Cursor,
		Path:      req.Path,
	})
	if err != nil {
		slog.Error("fail to get all files of repository", slog.Any("repoType", req.RepoType), slog.String("namespace", req.Namespace), slog.String("name", req.Name), slog.String("error", err.Error()))
		return nil, err
	}
	return resp, nil
}

func (c *repoComponentImpl) IsAdminRole(user database.User) bool {
	slog.Debug("Check if user is admin", slog.Any("user", user))
	return user.CanAdmin()
}

func (c *repoComponentImpl) IsInSameOrg(ctx context.Context, currentID int64, ownerID int64) bool {
	orgs, _ := c.orgStore.GetSharedOrgIDs(ctx, []int64{currentID, ownerID})
	return len(orgs) > 0
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

func (c *repoComponentImpl) FixRepoSource(ctx context.Context) error {
	batch, batchSize := 1, 1000
	for {
		var updateRepos []*database.Repository
		repos, err := c.repoStore.FindMirrorReposWithBatch(ctx, batchSize, batch)
		if err != nil {
			return err
		}
		if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			sourceType, sourcePath, err := common.GetSourceTypeAndPathFromURL(repo.Mirror.SourceUrl)
			if err == nil {
				repo.UpdateSourceBySourceTypeAndSourcePath(sourceType, sourcePath)
				updateRepos = append(updateRepos, &repo)
			}
		}
		if len(updateRepos) > 0 {
			err = c.repoStore.BulkUpdateSourcePath(ctx, updateRepos)
			if err != nil {
				return err
			}
		}
		batch++
	}
	return nil
}

func (c *repoComponentImpl) RemoteTree(ctx context.Context, req *types.GetTreeRequest) (*types.GetRepoFileTreeResp, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo == nil {
		return nil, fmt.Errorf("repo does not exist, error: %w", err)
	}

	if repo.Source == types.LocalSource {
		return nil, fmt.Errorf("repo is not a remote repo")
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get tree in this repo")
	}

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

func (c *repoComponentImpl) DiffBetweenTwoCommits(ctx context.Context, req types.GetDiffBetweenCommitsReq) ([]types.GiteaCallbackPushReq_Commit, error) {
	var rightCommit string
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, err: %w", err)
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get diff bewtween two commits in this repo")
	}

	rightCommit = req.RightCommitID
	if rightCommit == "" {
		lastCommit, err := c.git.GetRepoLastCommit(ctx, gitserver.GetRepoLastCommitReq{
			RepoType:  req.RepoType,
			Namespace: req.Namespace,
			Name:      req.Name,
			Ref:       repo.DefaultBranch,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get last commit, err: %w", err)
		}
		rightCommit = lastCommit.ID
	}

	diff, err := c.git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		RepoType:      req.RepoType,
		LeftCommitId:  req.LeftCommitID,
		RightCommitId: rightCommit,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get diff, err: %w", err)
	}

	return diff.Commits, nil
}

func (c *repoComponentImpl) Preupload(ctx context.Context, req types.PreuploadReq) (*types.PreuploadResp, error) {
	var (
		resp               types.PreuploadResp
		filePathOidMapping = make(map[string]*types.File)
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, err: %w", err)
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return nil, errorx.ErrForbiddenMsg("users do not have permission to get diff bewtween two commits in this repo")
	}

	existFiles, err := getAllFiles(ctx, req.Namespace, req.Name, "", req.RepoType, req.Revision, c.git.GetTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo files, err: %w", err)
	}

	for _, file := range existFiles {
		filePathOidMapping[file.Path] = file
	}

	gFile, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Revision,
		Path:      GitAttributesFileName,
	})
	if err != nil && status.Code(err) != codes.InvalidArgument {
		return nil, fmt.Errorf("failed to get gitattributes file, err: %w", err)
	}

	var sourceGContent string
	if gFile != nil {
		sourceGContent = gFile.Content
	}
	gContent, _ := base64.StdEncoding.DecodeString(sourceGContent)
	attributes := parseGitattributesContent(string(gContent))

	iFile, err := c.git.GetRepoFileContents(ctx, gitserver.GetRepoInfoByPathReq{
		RepoType:  req.RepoType,
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Revision,
		Path:      GitIgnoreFileName,
	})
	code := status.Code(err)
	if err != nil && code != codes.InvalidArgument {
		return nil, fmt.Errorf("failed to get .gitignore file, err: %w", err)
	}

	var sourceIContent string
	if iFile != nil {
		sourceIContent = iFile.Content
	}

	iContent, _ := base64.StdEncoding.DecodeString(sourceIContent)
	ig := ignore.CompileIgnoreLines(string(iContent))

	for _, file := range req.Files {
		var (
			uploadMode types.UploadMode
			oid        string
			isDir      bool
		)
		fileName := filepath.Base(file.Path)
		if shouldUseLFS(fileName, attributes) || file.Size > c.config.Git.MaxUnLfsFileSize {
			uploadMode = types.UploadModeLFS
		} else {
			uploadMode = types.UploadModeRegular
		}
		if file, ok := filePathOidMapping[file.Path]; ok {
			if file.Lfs {
				oid = file.LfsSHA256
			} else {
				oid = file.SHA
			}
		}
		for k := range filePathOidMapping {
			if strings.HasPrefix(k, fmt.Sprintf("%s/", file.Path)) {
				isDir = true
				break
			}
		}
		resp.Files = append(resp.Files, types.PreuploadRespFile{
			Path:         file.Path,
			UploadMode:   uploadMode,
			OID:          oid,
			ShouldIgnore: ig.MatchesPath(file.Path),
			IsDir:        isDir,
		})
	}

	return &resp, nil
}

func (c *repoComponentImpl) CommitFiles(ctx context.Context, req types.CommitFilesReq) error {
	var files []gitserver.CommitFile
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, err: %w", err)
	}
	if repo == nil {
		return errors.New("repo not found")
	}

	permission, err := c.GetUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return errorx.ErrForbiddenMsg("users do not have permission to get diff bewtween two commits in this repo")
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return fmt.Errorf("failed to find user, err: %w", err)
	}
	existFiles, err := c.git.GetRepoAllFiles(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
		Ref:       req.Revision,
	})
	if err != nil {
		return fmt.Errorf("failed to get repo files, err: %w", err)
	}

	for _, file := range req.Files {
		for _, existFile := range existFiles {
			if existFile.Path == file.Path {
				file.Action = types.CommitActionUpdate
			}
		}
		var action gitserver.CommitAction
		switch file.Action {
		case types.CommitActionCreate:
			action = gitserver.CommitActionCreate
		case types.CommitActionUpdate:
			action = gitserver.CommitActionUpdate
		case types.CommitActionDelete:
			action = gitserver.CommitActionDelete
		default:
			return fmt.Errorf("invalid action: %s", file.Action)
		}
		files = append(files, gitserver.CommitFile{
			Path:    file.Path,
			Content: cleanBase64(file.Content),
			Action:  action,
		})
	}

	err = c.git.CommitFiles(ctx, gitserver.CommitFilesReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		RepoType:  req.RepoType,
		Revision:  req.Revision,
		Username:  user.Username,
		Email:     user.Email,
		Message:   req.Message,
		Files:     files,
	})
	return err
}

func (c *repoComponentImpl) IsExists(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error) {
	_, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, err
	}
	return true, nil
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

func sanitizeBase64(input string) string {
	var sb strings.Builder
	for i := 0; i < len(input); i++ {
		c := input[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' {
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func cleanBase64(input string) string {
	cleaned := sanitizeBase64(input)

	cleaned = strings.ReplaceAll(cleaned, "=", "")
	if m := len(cleaned) % 4; m != 0 {
		cleaned += strings.Repeat("=", 4-m)
	}
	return cleaned
}

func (c *repoComponentImpl) SendAssetManagementMsg(ctx context.Context, req types.RepoNotificationReq) error {
	if req.RepoType == types.UnknownRepo {
		return fmt.Errorf("unknown repository")
	}
	if req.UserUUID == "" {
		return fmt.Errorf("no user UUID provided")
	}

	var repoUrl string
	if req.Operation == types.OperationCreate {
		repoUrl = GetRepoUrl(req.RepoType, req.RepoPath)
	}

	msg := types.NotificationMessage{
		UserUUIDs:        []string{req.UserUUID},
		NotificationType: types.NotificationAssetManagement,
		Template:         string(types.MessageScenarioAssetManagement),
		Payload: map[string]any{
			"repo_type": req.RepoType,
			"repo_path": req.RepoPath,
			"operation": req.Operation,
		},
	}

	if repoUrl != "" {
		msg.ClickActionURL = repoUrl
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message, err: %w", err)
	}

	notificationMsg := types.MessageRequest{
		Scenario:   types.MessageScenarioAssetManagement,
		Parameters: string(msgBytes),
		Priority:   types.MessagePriorityHigh,
	}

	var sendErr error
	retryCount := c.config.Notification.NotificationRetryCount
	for i := range retryCount {
		if sendErr = c.notificationSvcClient.Send(ctx, &notificationMsg); sendErr == nil {
			break
		}
		if i < retryCount-1 {
			slog.Warn("failed to send notification, retrying", "notification_msg", notificationMsg, "attempt", i+1, "error", sendErr.Error())
		}
	}

	if sendErr != nil {
		return fmt.Errorf("failed to send notification after %d attempts, err: %w", retryCount, sendErr)
	}

	return nil
}

func GetRepoUrl(repoType types.RepositoryType, repoPath string) string {
	if repoType == types.UnknownRepo || repoPath == "" {
		return ""
	}

	switch repoType {
	case types.ModelRepo:
		return fmt.Sprintf("/models/%s", repoPath)
	case types.DatasetRepo:
		return fmt.Sprintf("/datasets/%s", repoPath)
	case types.SpaceRepo:
		return fmt.Sprintf("/spaces/%s", repoPath)
	case types.CodeRepo:
		return fmt.Sprintf("/codes/%s", repoPath)
	case types.PromptRepo:
		return fmt.Sprintf("/prompts/%s", repoPath)
	case types.MCPServerRepo:
		return fmt.Sprintf("/mcp/servers/%s", repoPath)
	default:
		return ""
	}
}

// ValidateYaml
func (c *repoComponentImpl) ValidateYaml(ctx context.Context, req types.ValidateYamlReq) error {
	meta := metaText(req.Content)
	if len(meta) == 0 {
		return nil
	}

	categoryContents := make(map[string]any)
	// parse yaml string
	err := yaml.Unmarshal([]byte(meta), categoryContents)
	if err != nil {
		slog.Error("error unmarshall meta for tags", slog.Any("error", err), slog.String("meta", meta))
		return err
	}
	return nil
}

func metaText(readme string) string {
	splits := strings.Split(readme, "---")
	if len(splits) < 2 {
		return ""
	}

	return splits[1]
}

func (c *repoComponentImpl) ParseNDJson(ctx *gin.Context) (*types.CommitFilesReq, error) {
	req := &types.CommitFilesReq{}
	scanner := bufio.NewScanner(ctx.Request.Body)
	maxCapacity := int(c.config.Git.MaxUnLfsFileSize)
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines
		if line == "" {
			continue
		}
		var item types.FormField
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("invalid JSON on line %d: %v", lineNumber, err)
		}

		// Parse based on key type
		switch item.Key {
		case "header":
			headerBytes, err := json.Marshal(item.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal header value on line %d: %v", lineNumber, err)
			}

			var header types.CommitHeader
			if err := json.Unmarshal(headerBytes, &header); err != nil {
				return nil, fmt.Errorf("invalid header format on line %d: %v", lineNumber, err)
			}
			req.Message = header.Summary
		case "file":
			fileBytes, err := json.Marshal(item.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal file value on line %d: %v", lineNumber, err)
			}

			var file types.CommitFile
			if err := json.Unmarshal(fileBytes, &file); err != nil {
				return nil, fmt.Errorf("invalid file format on line %d: %v", lineNumber, err)
			}
			req.Files = append(req.Files, types.CommitFileReq{
				Path:    file.Path,
				Content: file.Content,
				Action:  types.CommitActionCreate,
			})
		case "lfsFile":
			fileBytes, err := json.Marshal(item.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal file value on line %d: %v", lineNumber, err)
			}
			var file types.CommitLFSFile
			if err := json.Unmarshal(fileBytes, &file); err != nil {
				return nil, fmt.Errorf("invalid file format on line %d: %v", lineNumber, err)
			}
			oid := fmt.Sprintf("%s:%s", file.Algo, file.OID)
			formattedStr := fmt.Sprintf("version %s\noid %s\nsize %d\n", "https://git-lfs.github.com/spec/v1", oid, file.Size)

			// Encode the string to Base64
			content := base64.StdEncoding.EncodeToString([]byte(formattedStr))
			req.Files = append(req.Files, types.CommitFileReq{
				Path:    file.Path,
				Content: content,
				Action:  types.CommitActionCreate,
			})

		case "deletedFolder":
			fileBytes, err := json.Marshal(item.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal file value on line %d: %v", lineNumber, err)
			}

			var file types.CommitFile
			if err := json.Unmarshal(fileBytes, &file); err != nil {
				return nil, fmt.Errorf("invalid file format on line %d: %v", lineNumber, err)
			}
			req.Files = append(req.Files, types.CommitFileReq{
				Path:   file.Path,
				Action: types.CommitActionDelete,
			})
		case "deletedFile":
			fileBytes, err := json.Marshal(item.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal file value on line %d: %v", lineNumber, err)
			}

			var file types.CommitFile
			if err := json.Unmarshal(fileBytes, &file); err != nil {
				return nil, fmt.Errorf("invalid file format on line %d: %v", lineNumber, err)
			}
			req.Files = append(req.Files, types.CommitFileReq{
				Path:   file.Path,
				Action: types.CommitActionDelete,
			})

		default:
			return nil, fmt.Errorf("unknown key type '%s' on line %d", item.Key, lineNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading NDJSON: %v", err)
	}
	return req, nil
}

func (c *repoComponentImpl) IsSyncing(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return false, fmt.Errorf("failed to find repo, error: %w", err)
	}

	m, err := c.mirrorStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to find mirror, error: %w", err)
	}

	if m != nil && m.CurrentTask != nil {
		if m.CurrentTask.Status == types.MirrorRepoSyncStart {
			return true, nil
		}
	}

	return false, nil
}

func (c *repoComponentImpl) ChangePath(ctx context.Context, req types.ChangePathReq) error {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	newNamespace, newName, err := common.GetNamespaceAndNameFromPath(req.NewPath)
	if err != nil {
		return fmt.Errorf("failed to get namespace and name from new path, error: %w", err)
	}

	// Check new path exists
	_, err = c.repoStore.FindByPath(ctx, req.RepoType, newNamespace, newName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check if new path exists, error: %w", err)
	}

	if err == nil {
		return errorx.BadRequest(errors.New("new path already exists"), errorx.Ctx().Set("new_path", req.NewPath))
	}

	if !repo.Hashed {
		// Migrate repo to hashed path
		// newPath := common.BuildHashedRelativePath(repo.ID)
		// err = c.git.CopyRepository(ctx, gitserver.CopyRepositoryReq{
		// 	RepoType:  req.RepoType,
		// 	Namespace: req.Namespace,
		// 	Name:      req.Name,
		// 	NewPath:   newPath + ".git",
		// })
		// if err != nil {
		// 	return fmt.Errorf("failed to move repo to hashed path, error: %w", err)
		// }
		return errorx.BadRequest(errors.New("repository not suported to change path"), errorx.Ctx())
	}

	repo.Path = req.NewPath
	repo.GitPath = fmt.Sprintf("%ss_%s/%s", req.RepoType, newNamespace, newName)
	repo.Hashed = true
	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo, error: %w", err)
	}

	return nil
}

func (c *repoComponentImpl) BatchMigrateRepoToHashedPath(ctx context.Context, auto bool, batchSize int, lastID int64) (int64, error) {
	if !c.config.Git.RepoDataMigrateEnable {
		return lastID, nil
	}

	var maxID int64
	maxID = lastID
	for {
		repos, err := c.repoStore.FindUnhashedRepos(ctx, batchSize, lastID)
		if err != nil {
			return lastID, fmt.Errorf("failed to find unhashed repos, error: %w", err)
		}

		if len(repos) == 0 {
			break
		}

		for _, repo := range repos {
			if repo.ID > maxID {
				maxID = repo.ID
			}
			err := c.migrateRepoToHashedPath(ctx, &repo)
			if err != nil {
				slog.Error("failed to migrate repo to hashed path", slog.Any("error", err))
			}
			slog.Info("migrated repo to hashed path", slog.Any("repo_type", repo.RepositoryType), slog.String("repo_path", repo.Path))
		}
		lastID = maxID
		if !auto {
			break
		}
	}
	return maxID, nil
}

func (c *repoComponentImpl) migrateRepoToHashedPath(ctx context.Context, repo *database.Repository) error {
	newPath := common.BuildHashedRelativePath(repo.ID)
	namespace, name := repo.NamespaceAndName()
	err := c.git.CopyRepository(ctx, gitserver.CopyRepositoryReq{
		Namespace: namespace,
		Name:      name,
		RepoType:  repo.RepositoryType,
		NewPath:   newPath,
	})
	if err != nil {
		return fmt.Errorf("failed to migrate %s repo %s to hashed path, error: %w", repo.RepositoryType, repo.Path, err)
	}

	repo.Hashed = true
	_, err = c.repoStore.UpdateRepo(ctx, *repo)
	if err != nil {
		return fmt.Errorf("failed to update repo, error: %w", err)
	}
	return nil
}

func (c *repoComponentImpl) GetMirrorTaskStatusAndSyncStatus(repo *database.Repository) (types.MirrorTaskStatus, types.RepositorySyncStatus) {
	var (
		syncStatus       types.RepositorySyncStatus
		mirrorTaskStatus types.MirrorTaskStatus
	)
	if repo.Mirror.ID != 0 {
		syncStatus = common.MirrorTaskStatusToRepoStatus(repo.Mirror.Status)
	}

	if repo.Mirror.CurrentTask != nil {
		syncStatus = common.MirrorTaskStatusToRepoStatus(repo.Mirror.CurrentTask.Status)
		mirrorTaskStatus = repo.Mirror.CurrentTask.Status
	}

	if syncStatus == "" {
		syncStatus = repo.SyncStatus
	}

	return mirrorTaskStatus, syncStatus
}

func (c *repoComponentImpl) RandomPath() []string {
	return strings.SplitN(uuid.NewString(), "-", 2)
}
