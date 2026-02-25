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
	"github.com/minio/minio-go/v7"
	ignore "github.com/sabhiram/go-gitignore"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"
	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
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
	multiSyncClient        multisync.Client
	sysMQ                  mq.MessageQueue
	mirrorTaskStore        database.MirrorTaskStore
	notificationSvcClient  rpc.NotificationSvcClient
	mirrorSvcClient        rpc.MirrorSvcClient
	pendingDeletion        database.PendingDeletionStore
	xnetClient             rpc.XnetSvcClient
	extendRepoImpl
}

type RepoComponent interface {
	CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, *gitserver.CommitFilesReq, error)
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
	GetMirrorTaskStatus(repo *database.Repository) types.MirrorTaskStatus
	CheckDeployPermissionForUser(ctx context.Context, deployReq types.DeployActReq) (*database.User, *database.Deploy, error)
	DeletePendingDeletion(ctx context.Context) error
	GetRepos(ctx context.Context, search, currentUser string, repoType types.RepositoryType) ([]string, error)
	advancedRepoInterface
	communityRepoInterface
}

func NewRepoComponentImpl(config *config.Config) (*repoComponentImpl, error) {
	r, err := NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return r.(*repoComponentImpl), nil
}

func (c *repoComponentImpl) CreateRepo(ctx context.Context, req types.CreateRepoReq) (*gitserver.CreateRepoResp, *database.Repository, *gitserver.CommitFilesReq, error) {
	var commitFilesReq *gitserver.CommitFilesReq
	// Name validation
	valid, err := common.IsValidName(req.Name)
	if !valid {
		slog.ErrorContext(ctx, "repo name is invalid", slog.Any("error", err))
		return nil, nil, commitFilesReq, errorx.ErrRepoNameInvalid
	}

	namespace, err := c.namespaceStore.FindByPath(ctx, req.Namespace)
	if err != nil {
		slog.ErrorContext(ctx, "namespace does not exist", slog.Any("error", err))
		return nil, nil, commitFilesReq, errorx.ErrNamespaceNotFound
	}

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		slog.ErrorContext(ctx, "user does not exist", slog.Any("error", err))
		return nil, nil, commitFilesReq, errorx.ErrUserNotFound
	}

	if user.Email == "" {
		slog.ErrorContext(ctx, "user email is empty", slog.Any("user", user))
		return nil, nil, commitFilesReq, errorx.ErrUserEmailEmpty
	}

	if !user.CanAdmin() {
		if namespace.NamespaceType == database.OrgNamespace {
			canWrite, err := c.CheckCurrentUserPermission(ctx, req.Username, req.Namespace, membership.RoleWrite)
			if err != nil {
				return nil, nil, commitFilesReq, err
			}
			if !canWrite {
				return nil, nil, commitFilesReq, errorx.ErrForbiddenMsg("users do not have permission to create repo in this organization")
			}
		} else {
			if namespace.Path != user.Username {
				return nil, nil, commitFilesReq, errorx.ErrForbiddenMsg("users do not have permission to create repo in this namespace")
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
		return nil, nil, commitFilesReq, fmt.Errorf("fail to create database repo, error: %w", err)
	}

	err = c.recomStore.UpsertScore(ctx, []*database.RecomRepoScore{
		{
			RepositoryID: newDBRepo.ID,
			Score:        0,
			WeightName:   database.RecomWeightTotal,
		},
	})
	if err != nil {
		return nil, nil, commitFilesReq, fmt.Errorf("fail to upsert recom repo score, error: %w", err)
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
		return nil, nil, commitFilesReq, fmt.Errorf("fail to create repo in git, error: %w", err)
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
		commitFilesReq = &gitserver.CommitFilesReq{
			Namespace: req.Namespace,
			Name:      req.Name,
			RepoType:  req.RepoType,
			Revision:  req.DefaultBranch,
			Username:  user.Username,
			Email:     user.Email,
			Message:   types.InitCommitMessage,
			Files:     gitCommitFiles,
		}
	}

	return gitRepo, newDBRepo, commitFilesReq, nil
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
		if req.XnetEnabled != nil {
			repo.XnetEnabled = *req.XnetEnabled
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
		if status.Code(err) == codes.NotFound {
			return "", errorx.ErrNotFound
		}
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
		return nil, 0, "", fmt.Errorf("failed to update %s file download count, error: %w", fmt.Sprintf("%s/%s/%s", req.RepoType, req.Namespace, req.Name), err)
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
		if repo.XnetEnabled {
			signedUrl, err := c.xnetClient.PresignedGetObject(ctx, objectKey, types.OssFileExpire, reqParams)
			if err != nil {
				return nil, 0, downloadUrl, err
			}
			downloadUrl = signedUrl.String()
		} else {
			signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
			if err != nil {
				return nil, 0, downloadUrl, err
			}
			downloadUrl = signedUrl.String()
		}
		return nil, 0, downloadUrl, nil
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

	if file != nil && repo.XnetEnabled && file.Lfs {
		file.XnetEnabled = true
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

	err = c.repoStore.UpdateRepoFileDownloads(ctx, repo, time.Now(), 1)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to update %s file download count, error: %w", fmt.Sprintf("%s/%s/%s", req.RepoType, req.Namespace, req.Name), err)
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
		if repo.XnetEnabled {
			signedUrl, err := c.xnetClient.PresignedGetObject(ctx, objectKey, types.OssFileExpire, reqParams)
			if err != nil {
				return nil, 0, downloadUrl, err
			}
			downloadUrl = signedUrl.String()
		} else {
			signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.lfsBucket, objectKey, types.OssFileExpire, reqParams)
			if err != nil {
				return nil, 0, downloadUrl, err
			}
			downloadUrl = signedUrl.String()
		}
		return nil, 0, downloadUrl, nil
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
	if req.MirrorSourceID != 0 {
		mirrorSource, err := c.mirrorSourceStore.Get(ctx, req.MirrorSourceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get mirror source, err: %w, id: %d", err, req.MirrorSourceID)
		}
		mirror.LocalRepoPath = fmt.Sprintf("%s_%s_%s_%s", mirrorSource.SourceName, req.RepoType, req.Namespace, req.Name)
	}

	mirror.Interval = req.Interval
	mirror.SourceUrl = req.SourceUrl
	mirror.MirrorSourceID = req.MirrorSourceID
	mirror.Username = req.Username
	mirror.PushUrl = repo.HTTPCloneURL
	mirror.AccessToken = req.AccessToken
	mirror.SourceRepoPath = req.SourceRepoPath

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
	var (
		files    []gitserver.CommitFile
		lfsFiles []types.Pointer
	)
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
		cleanedContent := cleanBase64(file.Content)

		files = append(files, gitserver.CommitFile{
			Path:    file.Path,
			Content: cleanedContent,
			Action:  action,
		})
		content, err := base64.StdEncoding.DecodeString(cleanedContent)
		if err != nil {
			return fmt.Errorf("failed to decode content, err: %w", err)
		}
		p, err := gitaly.ReadPointerFromBuffer(content)
		if err != nil {
			continue
		}
		lfsFiles = append(lfsFiles, p)
	}

	for _, lfsFile := range lfsFiles {
		if repo.XnetEnabled {
			lfsExistReq := &types.XetFileExistsReq{
				RepoID:    strconv.FormatInt(repo.ID, 10),
				ObjectKey: lfsFile.Oid,
			}
			if lfsFileExist, err := c.xnetClient.FileExists(ctx, lfsExistReq); err != nil || !lfsFileExist {
				return fmt.Errorf("failed to request xnet, exist:%t err: %w", lfsFileExist, err)
			}
		}
		_, err := c.lfsMetaObjectStore.UpdateOrCreate(ctx, database.LfsMetaObject{
			Oid:          lfsFile.Oid,
			Size:         lfsFile.Size,
			Existing:     true,
			RepositoryID: repo.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to update or create lfs meta object, err: %w", err)
		}
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
		return fmt.Sprintf("/prompts/library/%s", repoPath)
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

func (c *repoComponentImpl) GetMirrorTaskStatus(repo *database.Repository) types.MirrorTaskStatus {
	var (
		mirrorTaskStatus types.MirrorTaskStatus
	)

	if repo.Mirror.CurrentTask != nil {
		mirrorTaskStatus = repo.Mirror.CurrentTask.Status
	}

	return mirrorTaskStatus
}

func (c *repoComponentImpl) RandomPath() []string {
	return strings.SplitN(uuid.NewString(), "-", 2)
}

func (c *repoComponentImpl) GetRepos(ctx context.Context, search, currentUser string, repoType types.RepositoryType) ([]string, error) {
	var repoPaths []string
	repos, _, err := c.repoStore.GetReposBySearch(ctx, search, repoType, 1, 10)
	if err != nil {
		return repoPaths, fmt.Errorf("failed to get repos, error: %w", err)
	}
	for _, repo := range repos {
		repoPaths = append(repoPaths, repo.Path)
	}
	return repoPaths, nil
}
