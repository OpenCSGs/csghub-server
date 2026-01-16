package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/redis/go-redis/v9"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

const HashedRepoPathPrefix = "@hashed_repos"

var RepositorySourceAndPrefixMapping = map[types.RepositorySource]string{
	types.HuggingfaceSource: types.HuggingfacePrefix,
	types.OpenCSGSource:     types.OpenCSGPrefix,
	types.LocalSource:       "",
}

var (
	redisClient cache.RedisClient
	redisOnce   sync.Once
)

type repoStoreImpl struct {
	config              *config.Config
	db                  *DB
	DbDriver            string
	SearchConfiguration string
	cache               cache.RedisClient
}

type RepoStore interface {
	// CreateRepoTx(ctx context.Context, tx bun.Tx, input Repository) (*Repository, error)
	CreateRepo(ctx context.Context, input Repository) (*Repository, error)
	UpdateRepo(ctx context.Context, input Repository) (*Repository, error)
	DeleteRepo(ctx context.Context, input Repository) error
	Find(ctx context.Context, owner, repoType, repoName string) (*Repository, error)
	FindById(ctx context.Context, id int64) (*Repository, error)
	FindByIds(ctx context.Context, ids []int64, opts ...SelectOption) ([]*Repository, error)
	FindByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Repository, error)
	FindByGitPath(ctx context.Context, path string) (*Repository, error)
	FindByGitPaths(ctx context.Context, paths []string, opts ...SelectOption) ([]*Repository, error)
	Exists(ctx context.Context, repoType types.RepositoryType, namespace string, name string) (bool, error)
	All(ctx context.Context) ([]*Repository, error)
	UpdateRepoFileDownloads(ctx context.Context, repo *Repository, date time.Time, clickDownloadCount int64) (err error)
	UpdateRepoCloneDownloads(ctx context.Context, repo *Repository, date time.Time, cloneCount int64) (err error)
	UpdateDownloads(ctx context.Context, repo *Repository) error
	Tags(ctx context.Context, repoID int64) (tags []Tag, err error)
	TagsWithCategory(ctx context.Context, repoID int64, category string) (tags []Tag, err error)
	// TagIDs get tag ids by repo id, if category is not empty, return only tags of the category
	TagIDs(ctx context.Context, repoID int64, category string) (tagIDs []int64, err error)
	SetUpdateTimeByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string, update time.Time) error
	PublicToUser(ctx context.Context, repoType types.RepositoryType, userIDs []int64, filter *types.RepoFilter, per, page int, isAdmin bool) (repos []*Repository, count int, err error)
	IsMirrorRepo(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error)
	ListRepoByDeployType(ctx context.Context, repoType types.RepositoryType, userID int64, search, sort string, deployType, per, page int) (repos []*Repository, count int, err error)
	WithMirror(ctx context.Context, per, page int) (repos []Repository, count int, err error)
	CleanRelationsByRepoID(ctx context.Context, repoId int64) error
	BatchCreateRepoTags(ctx context.Context, repoTags []RepositoryTag) error
	DeleteAllFiles(ctx context.Context, repoID int64) error
	DeleteAllTags(ctx context.Context, repoID int64) error
	UpdateOrCreateRepo(ctx context.Context, input Repository) (*Repository, error)
	UpdateLicenseByTag(ctx context.Context, repoID int64) error
	CountByRepoType(ctx context.Context, repoType types.RepositoryType) (int, error)
	GetRepoWithoutRuntimeByID(ctx context.Context, rfID int64, paths []string, batchSize, batch int) ([]Repository, error)
	GetRepoWithRuntimeByID(ctx context.Context, rfID int64, paths []string) ([]Repository, error)
	BatchGet(ctx context.Context, lastRepoID int64, batch int, filter *types.BatchGetFilter) ([]Repository, error)
	FindWithBatch(ctx context.Context, batchSize, batch int, repoTypes ...types.RepositoryType) ([]Repository, error)
	ByUser(ctx context.Context, userID int64, batchSize, batch int) ([]Repository, error)
	FindByRepoSourceWithBatch(ctx context.Context, repoSource types.RepositorySource, batchSize, batch int) ([]Repository, error)
	FindMirrorReposByUserAndSource(ctx context.Context, userID int64, source string, batchSize, batch int) ([]Repository, error)
	UpdateSourcePath(ctx context.Context, repoID int64, sourcePath, sourceType string) error
	FindMirrorReposWithBatch(ctx context.Context, batchSize, batch int) ([]Repository, error)
	BulkUpdateSourcePath(ctx context.Context, repos []*Repository) error
	FindByMirrorSourceURL(ctx context.Context, sourceURL string) (*Repository, error)
	FindWithMirror(ctx context.Context, repoType types.RepositoryType, owner, repoName string) (*Repository, error)
	RefreshLFSObjectsSize(ctx context.Context, id int64) error
	FindMirrorFinishedPrivateModelRepo(ctx context.Context) ([]*Repository, error)
	BatchUpdate(ctx context.Context, repos []*Repository) error
	FindByRepoTypeAndPaths(ctx context.Context, repoType types.RepositoryType, path []string) ([]Repository, error)
	FindUnhashedRepos(ctx context.Context, batchSize int, lastID int64) ([]Repository, error)
	UpdateRepoSensitiveCheckStatus(ctx context.Context, repoID int64, status types.SensitiveCheckStatus) error
	GetReposBySearch(ctx context.Context, search string, repoType types.RepositoryType, page, pageSize int) ([]*Repository, int, error)
}

func (s *repoStoreImpl) UpdateRepoSensitiveCheckStatus(ctx context.Context, repoID int64, status types.SensitiveCheckStatus) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&Repository{}).
		Set("sensitive_check_status = ?", status).
		Where("id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func newRepoStoreInstance(db *DB) RepoStore {
	cfg, _ := config.LoadConfig()

	redisOnce.Do(func() {
		var err error
		redisClient, err = cache.NewCache(context.Background(), cache.RedisConfig{
			Addr:     cfg.Redis.Endpoint,
			Username: cfg.Redis.User,
			Password: cfg.Redis.Password,
		})
		if err != nil {
			slog.Error("failed to init redis client", "error", err)
		}
	})

	return &repoStoreImpl{
		config:              cfg,
		db:                  db,
		DbDriver:            cfg.Database.Driver,
		SearchConfiguration: cfg.Database.SearchConfiguration,
		cache:               redisClient,
	}
}

func NewRepoStore() RepoStore {
	return newRepoStoreInstance(defaultDB)
}

// for testing with mock db
func NewRepoStoreWithDB(db *DB) RepoStore {
	return newRepoStoreInstance(db)
}

// for testing with mock cache
func NewRepoStoreWithCache(config *config.Config, db *DB, cache cache.RedisClient) RepoStore {
	return &repoStoreImpl{
		config:              config,
		db:                  db,
		DbDriver:            config.Database.Driver,
		SearchConfiguration: config.Database.SearchConfiguration,
		cache:               cache,
	}
}

type Repository struct {
	ID          int64  `bun:",pk,autoincrement" json:"id"`
	UserID      int64  `bun:",notnull" json:"user_id"`
	User        User   `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Path        string `bun:",notnull" json:"path"`
	GitPath     string `bun:",notnull" json:"git_path"`
	Name        string `bun:",notnull" json:"name"`
	Nickname    string `bun:",notnull" json:"nickname"`
	Description string `bun:",nullzero" json:"description"`
	Private     bool   `bun:",notnull" json:"private"`
	// Depreated
	Labels  string `bun:",nullzero" json:"labels"`
	License string `bun:",nullzero" json:"license"`
	// Depreated
	Readme               string                     `bun:",nullzero" json:"readme"`
	DefaultBranch        string                     `bun:",notnull" json:"default_branch"`
	LfsFiles             []LfsFile                  `bun:"rel:has-many,join:id=repository_id" json:"-"`
	Likes                int64                      `bun:",nullzero" json:"likes"`
	DownloadCount        int64                      `bun:",nullzero" json:"download_count"`
	Downloads            []RepositoryDownload       `bun:"rel:has-many,join:id=repository_id" json:"downloads"`
	Tags                 []Tag                      `bun:"m2m:repository_tags,join:Repository=Tag" json:"tags"`
	Metadata             Metadata                   `bun:"rel:has-one,join:id=repository_id" json:"metadata"`
	Mirror               Mirror                     `bun:"rel:has-one,join:id=repository_id" json:"mirror"`
	RepositoryType       types.RepositoryType       `bun:",notnull" json:"repository_type"`
	HTTPCloneURL         string                     `bun:",nullzero" json:"http_clone_url"`
	SSHCloneURL          string                     `bun:",nullzero" json:"ssh_clone_url"`
	Source               types.RepositorySource     `bun:",nullzero,default:'local'" json:"source"`
	SyncStatus           types.RepositorySyncStatus `bun:",nullzero" json:"sync_status"`
	SensitiveCheckStatus types.SensitiveCheckStatus `bun:",default:0" json:"sensitive_check_status"`
	MSPath               string                     `bun:",nullzero" json:"ms_path"`
	CSGPath              string                     `bun:",nullzero" json:"csg_path"`
	HFPath               string                     `bun:",nullzero" json:"hf_path"`
	GithubPath           string                     `bun:",nullzero" json:"github_path"`
	LFSObjectsSize       int64                      `bun:",nullzero" json:"lfs_objects_size"`
	StarCount            int                        `bun:",nullzero" json:"star_count"`
	DeletedAt            time.Time                  `bun:",soft_delete,nullzero"`
	Migrated             bool                       `bun:"," json:"migrated"`
	Hashed               bool                       `bun:"," json:"hashed"`
	XnetEnabled          bool                       `bun:"," json:"xnet_enabled"`

	// updated_at timestamp will be updated only if files changed
	times
}

// NamespaceAndName returns namespace and name by parsing repository path
func (r Repository) NamespaceAndName() (namespace string, name string) {
	fields := strings.Split(r.Path, "/")
	return fields[0], fields[1]
}

func (r Repository) OriginName() string {
	oriName := r.Name
	if r.HFPath != "" {
		oriName = strings.Split(r.HFPath, "/")[1]
	} else if r.MSPath != "" {
		oriName = strings.Split(r.MSPath, "/")[1]
	}
	return oriName
}

func (r Repository) OriginPath() string {
	oriPath := r.Path
	if r.HFPath != "" {
		oriPath = r.HFPath
	} else if r.MSPath != "" {
		oriPath = r.MSPath
	}
	return oriPath
}

func (r Repository) OriginNamespaceAndName() (string, string) {
	originPath := r.Path
	if r.HFPath != "" {
		originPath = r.HFPath
	} else if r.MSPath != "" {
		originPath = r.MSPath
	}
	fields := strings.Split(originPath, "/")
	return fields[0], fields[1]
}

func (r Repository) Archs() (archs []string) {
	if r.Metadata.Architecture != "" {
		archs = append(archs, r.Metadata.Architecture)
	}
	if r.Metadata.ClassName != "" {
		archs = append(archs, r.Metadata.ClassName)
	}
	if r.Metadata.ModelType != "" {
		archs = append(archs, r.Metadata.ModelType)
	}
	return archs
}
func (r Repository) Format() string {
	for _, tag := range r.Tags {
		if tag.Category == "framework" {
			return tag.Name
		}
	}
	//handle some old repo has no gguf tag
	if strings.Contains(strings.ToLower(r.Name), "gguf") {
		return "gguf"
	}
	return string(types.Unknown)
}

func (r Repository) Task() string {
	for _, tag := range r.Tags {
		if tag.Category == "task" {
			return tag.Name
		}
	}
	return string(types.Unknown)
}

func (r *Repository) UpdateSourceBySourceTypeAndSourcePath(sourceType, sourcePath string) {
	switch sourceType {
	case enum.HFSource:
		r.HFPath = sourcePath
	case enum.MSSource:
		r.MSPath = sourcePath
	case enum.CSGSource:
		r.CSGPath = sourcePath
	}
}

func (r *Repository) SetSyncStatus(syncStatus types.RepositorySyncStatus) {
	r.SyncStatus = syncStatus
}

type RepositoryTag struct {
	ID           int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64       `bun:",notnull" json:"repository_id"`
	TagID        int64       `bun:",notnull" json:"tag_id"`
	Repository   *Repository `bun:"rel:belongs-to,join:repository_id=id"`
	Tag          *Tag        `bun:"rel:belongs-to,join:tag_id=id"`
	/*
		for meta tags parsed from README.md file, count is alway 1

		for Library tags, count means how many a kind of library file (e.g. *.ONNX file) exists in the repository
	*/
	Count int32 `bun:",default:1" json:"count"`
}

func (r Repository) PathWithOutPrefix() string {
	return strings.TrimPrefix(r.Path, RepositorySourceAndPrefixMapping[r.Source])

}

func (r Repository) IsOpenCSGRepo() bool {
	return r.Source == types.OpenCSGSource
}

func (r Repository) GitalyPath() string {
	if r.Hashed {
		sha256Path := SHA256(strconv.FormatInt(r.ID, 10))
		return fmt.Sprintf("%s/%s/%s/%s.git", HashedRepoPathPrefix, sha256Path[0:2], sha256Path[2:4], sha256Path)
	}
	splitPath := strings.Split(r.Path, "/")
	return strings.ToLower(fmt.Sprintf("%ss", r.RepositoryType)+"_"+splitPath[0]+"/"+splitPath[1]) + ".git"
}

func SHA256(s string) string {
	hash := sha256.New()
	hash.Write([]byte(s))
	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

// func (s *repoStoreImpl) CreateRepoTx(ctx context.Context, tx bun.Tx, input Repository) (*Repository, error) {
// 	res, err := tx.NewInsert().Model(&input).Exec(ctx)
// 	if err := assertAffectedOneRow(res, err); err != nil {
// 		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
// 	}

// 	return &input, nil
// }

func (s *repoStoreImpl) CreateRepo(ctx context.Context, input Repository) (*Repository, error) {
	input.Migrated = true
	input.Hashed = true
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		err = errorx.HandleDBError(err, errorx.Ctx().Set("path", input.Path))
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *repoStoreImpl) UpdateRepo(ctx context.Context, input Repository) (*Repository, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("path", input.Path))
	return &input, err
}

func (s *repoStoreImpl) DeleteRepo(ctx context.Context, input Repository) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().ForceDelete().Exec(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("path", input.Path))
	return err
}

func (s *repoStoreImpl) Find(ctx context.Context, owner, repoType, repoName string) (*Repository, error) {
	var err error
	repo := &Repository{}
	err = s.db.Operator.Core.
		NewSelect().
		Model(repo).
		Where("repository_type = ? AND LOWER(path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", owner, repoName)).
		Limit(1).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("repo_type", repoType).
		Set("path", fmt.Sprintf("%s/%s", owner, repoName)))
	return repo, err
}

func (s *repoStoreImpl) FindWithMirror(ctx context.Context, repoType types.RepositoryType, owner, repoName string) (*Repository, error) {
	var err error
	repo := &Repository{}
	err = s.db.Operator.Core.
		NewSelect().
		Model(repo).
		Relation("Mirror").
		Where("repository_type = ? AND LOWER(path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", owner, repoName)).
		Limit(1).
		Scan(ctx)
	err = errorx.HandleDBError(err,
		errorx.Ctx().
			Set("repo_type", repoType).
			Set("path", fmt.Sprintf("%s/%s", owner, repoName)),
	)
	return repo, err
}

func (s *repoStoreImpl) FindById(ctx context.Context, id int64) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("id =?", id).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().Set("repo_id", id))
	return resRepo, err
}

func (s *repoStoreImpl) FindByIds(ctx context.Context, ids []int64, opts ...SelectOption) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	q := s.db.Operator.Core.
		NewSelect()
	for _, opt := range opts {
		opt.Appply(q)
	}
	err := q.
		Model(&repos).
		Where("id in (?)", bun.In(ids)).
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return repos, err
}

func (s *repoStoreImpl) FindByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Relation("Tags").
		Relation("Metadata").
		Where("repository_type = ? AND LOWER(path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", namespace, name)).
		Limit(1).
		Scan(ctx)
	err = errorx.HandleDBError(err,
		errorx.Ctx().
			Set("repo_type", repoType).
			Set("path", fmt.Sprintf("%s/%s", namespace, name)),
	)
	if err != nil {
		return nil, err
	}
	return resRepo, err
}

func (s *repoStoreImpl) FindByGitPath(ctx context.Context, path string) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("LOWER(git_path) = LOWER(?)", path).
		Scan(ctx)
	err = errorx.HandleDBError(err,
		errorx.Ctx().
			Set("git_path", path),
	)
	return resRepo, err
}

func (s *repoStoreImpl) FindByGitPaths(ctx context.Context, paths []string, opts ...SelectOption) ([]*Repository, error) {
	for i := range paths {
		paths[i] = strings.ToLower(paths[i])
	}
	repos := make([]*Repository, 0)
	q := s.db.Operator.Core.
		NewSelect()
	for _, opt := range opts {
		opt.Appply(q)
	}
	err := q.Model(&repos).
		Where("LOWER(git_path) in (?)", bun.In(paths)).
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return repos, err
}

func (s *repoStoreImpl) Exists(ctx context.Context, repoType types.RepositoryType, namespace string, name string) (bool, error) {
	isExist, err := s.db.Operator.Core.NewSelect().Model((*Repository)(nil)).
		Where("repository_type = ? AND LOWER(path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", namespace, name)).
		Exists(ctx)
	return isExist, errorx.HandleDBError(err, errorx.Ctx().
		Set("repo_type", repoType).
		Set("path", fmt.Sprintf("%s/%s", namespace, name)),
	)
}

func (s *repoStoreImpl) All(ctx context.Context) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	err := s.db.Operator.Core.
		NewSelect().
		Model(&repos).
		Scan(ctx)
	err = errorx.HandleDBError(err, nil)
	return repos, err
}

func (s *repoStoreImpl) UpdateRepoFileDownloads(ctx context.Context, repo *Repository, date time.Time, clickDownloadCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), repo.ID).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path).
		Set("date", date),
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	} else if errors.Is(err, sql.ErrNoRows) {
		rd.ClickDownloadCount = clickDownloadCount
		rd.Date = date
		rd.RepositoryID = repo.ID
		err = s.db.Operator.Core.NewInsert().
			Model(rd).
			Scan(ctx)
		if err != nil {
			return
		}
	} else {
		rd.ClickDownloadCount = rd.ClickDownloadCount + clickDownloadCount
		query := s.db.Operator.Core.NewUpdate().
			Model(rd).
			WherePK()
		slog.Debug(query.String())

		_, err = query.Exec(ctx)
		if err != nil {
			return
		}
	}
	err = s.UpdateDownloads(ctx, repo)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path),
	)
	return
}

func (s *repoStoreImpl) UpdateRepoCloneDownloads(ctx context.Context, repo *Repository, date time.Time, cloneCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), repo.ID).
		Scan(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path).
		Set("date", date),
	)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	} else if errors.Is(err, sql.ErrNoRows) {
		rd.CloneCount = cloneCount
		rd.Date = date
		rd.RepositoryID = repo.ID
		err = s.db.Operator.Core.NewInsert().
			Model(rd).
			Scan(ctx)
		if err != nil {
			return
		}
	} else {
		rd.CloneCount = cloneCount
		query := s.db.Operator.Core.NewUpdate().
			Model(rd).
			WherePK()
		slog.Debug(query.String())

		_, err = query.Exec(ctx)
		if err != nil {
			return
		}
	}
	err = s.UpdateDownloads(ctx, repo)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path),
	)
	return
}

func (s *repoStoreImpl) UpdateDownloads(ctx context.Context, repo *Repository) error {
	var downloadCount int64
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("(SUM(clone_count)+SUM(click_download_count)) AS total_count").
		Model(&RepositoryDownload{}).
		Where("repository_id=?", repo.ID).
		Scan(ctx, &downloadCount)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path),
	)
	if err != nil {
		return err
	}
	repo.DownloadCount = downloadCount
	_, err = s.db.Operator.Core.NewUpdate().
		Model(repo).
		WherePK().
		Exec(ctx)
	return errorx.HandleDBError(err, errorx.Ctx().
		Set("path", repo.Path),
	)
}

func (s *repoStoreImpl) Tags(ctx context.Context, repoID int64) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&RepositoryTag{}).
		Join("JOIN tags ON repository_tag.tag_id = tags.id").
		Where("repository_tag.repository_id = ?", repoID).
		Where("repository_tag.count > 0")
	err = query.Scan(ctx, &tags)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("id", repoID),
	)
	return
}
func (s *repoStoreImpl) TagsWithCategory(ctx context.Context, repoID int64, category string) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&RepositoryTag{}).
		Join("JOIN tags ON repository_tag.tag_id = tags.id").
		Where("repository_tag.repository_id = ?", repoID).
		Where("repository_tag.count > 0").
		Where("tags.category = ?", category)
	err = query.Scan(ctx, &tags)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("id", repoID),
	)
	return
}

// TagIDs get tag ids by repo id, if category is not empty, return only tags of the category
func (s *repoStoreImpl) TagIDs(ctx context.Context, repoID int64, category string) (tagIDs []int64, err error) {
	query := s.db.Operator.Core.NewSelect().
		Model(&RepositoryTag{}).
		Join("JOIN tags ON repository_tag.tag_id = tags.id").
		Where("repository_id = ?", repoID)
	if len(category) > 0 {
		query.Where("tags.category = ?", category)
	}
	query.Column("repository_tag.tag_id")
	err = query.Scan(ctx, &tagIDs)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("id", repoID),
	)
	return tagIDs, err
}

func (s *repoStoreImpl) SetUpdateTimeByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string, update time.Time) error {
	repo := new(Repository)
	repo.UpdatedAt = update
	_, err := s.db.Operator.Core.NewUpdate().Model(repo).
		Column("updated_at").
		Where("repository_type = ? AND LOWER(path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", namespace, name)).
		Exec(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("repo_type", repoType).
		Set("path", fmt.Sprintf("%s/%s", namespace, name)),
	)
	return err
}

func (s *repoStoreImpl) PublicToUser(ctx context.Context, repoType types.RepositoryType, userIDs []int64, filter *types.RepoFilter, per, page int, isAdmin bool) (repos []*Repository, count int, err error) {
	q := s.db.Operator.Core.
		NewSelect().
		Column("repository.*").
		Model(&repos).
		Relation("Tags")

	q.Where("repository.repository_type = ?", repoType)

	if !isAdmin {
		if len(userIDs) > 0 {
			q.Where("repository.private = ? or repository.user_id in (?)", false, bun.In(userIDs))
		} else {
			q.Where("repository.private = ?", false)
		}
	}

	if filter.Source != "" {
		if filter.Source == "local" {
			q.Join("LEFT JOIN mirrors ON mirrors.repository_id = repository.id").
				Join("LEFT JOIN mirror_tasks ON mirror_tasks.mirror_id = mirrors.id").
				Where("mirror_tasks.status = ? or repository.source = ?", types.MirrorLfsSyncFinished, "local")
		} else {
			q.Where("repository.source = ?", filter.Source)
		}
	}

	// model tree filter
	if filter.Tree != nil {
		q.Where("repository.id IN (SELECT target_repo_id FROM model_trees WHERE source_repo_id = ? and relation = ?)", filter.Tree.RepoId, filter.Tree.Relation)
	}
	// list serverless
	if filter.ListServerless {
		q.Where("repository.id IN (SELECT repo_id FROM deploys WHERE type = ? and status = ?)", types.ServerlessType, common.Running)
	}

	if len(filter.SpaceSDK) > 0 {
		q.Where("spaces.sdk = ?", filter.SpaceSDK)
	}

	if len(filter.Tags) > 0 {
		for i, tag := range filter.Tags {
			var asRepoTag = fmt.Sprintf("%s%d", "rt", i)
			var asTag = fmt.Sprintf("%s%d", "ts", i)
			q.Join(fmt.Sprintf("JOIN repository_tags AS %s ON repository.id = %s.repository_id", asRepoTag, asRepoTag)).
				Join(fmt.Sprintf("JOIN tags AS %s ON %s.tag_id = %s.id", asTag, asRepoTag, asTag))
			if tag.Category != "" {
				q.Where(fmt.Sprintf("%s.category = ?", asTag), tag.Category)
			}
			if tag.Name != "" {
				q.Where(fmt.Sprintf("%s.name = ?", asTag), tag.Name)
			}
			if tag.Group != "" {
				q.Where(fmt.Sprintf("%s.group = ?", asTag), tag.Group)
			}
		}
		q.Distinct()
	}

	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Search != "" {
		filter.Search = strings.ToLower(filter.Search) // search is case insensitive in our query, and convert to lower can improve the cache hit rate
		repos, count, err = s.SearchRepoWithCache(ctx, q, repoType, filter, per, page)
		err = errorx.HandleDBError(err, errorx.Ctx().
			Set("repo_type", repoType).
			Set("filter", filter),
		)
		return
	}

	if filter.Sort == "trending" {
		q.Join("LEFT JOIN recom_repo_scores ON repository.id = recom_repo_scores.repository_id")
		q.Where("recom_repo_scores.weight_name = ?", RecomWeightTotal)
		q.ColumnExpr(`COALESCE(recom_repo_scores.score, 0) AS popularity`)
	}
	q.Order(sortBy[filter.Sort])

	count, err = q.Count(ctx)
	err = errorx.HandleDBError(err, errorx.Ctx().
		Set("repo_type", repoType).
		Set("filter", filter),
	)
	if err != nil {
		return
	}

	err = q.Limit(per).Offset((page - 1) * per).Scan(ctx)

	return
}

func (s *repoStoreImpl) getCacheKey(q *bun.SelectQuery, repoType types.RepositoryType, filter *types.RepoFilter) (string, error) {
	h := xxhash.New()
	_, err := h.Write([]byte(q.String()))
	if err != nil {
		slog.Error("failed to write query to hash", "error", err)
		return "", err
	}
	filter.Sort = "" // sort in filter is useless in search, so we don't need to add it to the hash
	_, err = fmt.Fprintf(h, "%+v", filter)
	if err != nil {
		slog.Error("failed to write filter to hash", "error", err)
		return "", err
	}
	return fmt.Sprintf("repo:search:%s:%x", repoType, h.Sum64()), nil
}

func paginateRows(rows []*Repository, per, page int) []*Repository {
	begin := (page - 1) * per
	if begin >= len(rows) {
		return []*Repository{}
	}
	end := min(begin+per, len(rows))
	return rows[begin:end]
}

func (s *repoStoreImpl) SearchRepoWithCache(ctx context.Context, q *bun.SelectQuery, repoType types.RepositoryType, filter *types.RepoFilter, per, page int) ([]*Repository, int, error) {
	cacheKey, err := s.getCacheKey(q, repoType, filter)
	if err != nil {
		slog.Error("failed to get cache key", "error", err)
		return nil, 0, err
	}
	cacheExist, err := s.cache.Exists(ctx, cacheKey)
	if err != nil {
		slog.Warn("failed to check cache exist", "error", err)
	}

	if cacheExist == 0 {
		var err error
		rows := []*Repository{}
		if s.DbDriver == "pg" {
			q = buildVectorSearchQuery(q, filter, s.db.BunDB, s.SearchConfiguration, s.config.Search.RepoSearchLimit)
			err = q.Scan(ctx, &rows)
		} else {
			buildLikeQuery(q, filter, s.config.Search.RepoSearchLimit)
			err = q.Model(&rows).Scan(ctx)
		}

		if err != nil {
			slog.Error("failed to scan search result", "error", err)
			err = errorx.HandleDBError(err, errorx.Ctx().
				Set("repo_type", repoType).
				Set("filter", filter),
			)
			return nil, 0, err
		}
		if len(rows) > 0 {
			zMembers := make([]redis.Z, 0, len(rows))
			for i, row := range rows {
				// Use negative index to maintain DESC order (highest rank first)
				zMembers = append(zMembers, redis.Z{
					Score:  float64(-i),
					Member: row.ID,
				})
			}
			err = s.cache.ZAdd(ctx, cacheKey, zMembers...)
			if err != nil {
				slog.Warn("failed to add search result to cache", "error", err)
				return paginateRows(rows, per, page), len(rows), nil
			}
			err = s.cache.Expire(ctx, cacheKey, time.Duration(s.config.Search.RepoSearchCacheTTL)*time.Second)
			if err != nil {
				slog.Warn("failed to expire cache", "error", err)
				if err = s.cache.Del(ctx, cacheKey); err != nil {
					slog.Warn("failed to delete cache", "error", err)
				}
				return paginateRows(rows, per, page), len(rows), nil
			}
		}
	}

	total, err := s.cache.ZCard(ctx, cacheKey)
	if err != nil {
		slog.Error("failed to get cache card", "error", err)
		return nil, 0, err
	}
	count := int(total)
	if count == 0 {
		return nil, 0, nil
	}

	start := int64((page - 1) * per)
	end := start + int64(per) - 1
	idStrs, err := s.cache.ZRevRange(ctx, cacheKey, start, end)
	if err != nil {
		slog.Error("failed to get cache range", "error", err)
		return nil, 0, err
	}

	ids := make([]int64, 0, len(idStrs))
	for _, idStr := range idStrs {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			slog.Error("failed to parse id", "error", err)
			return nil, 0, err
		}
		ids = append(ids, id)
	}

	repos := make([]*Repository, 0, len(ids))

	err = s.db.Operator.Core.NewSelect().
		Column("repository.*").
		Model(&repos).
		Relation("Tags").
		Where("repository.id IN (?)", bun.In(ids)).
		Scan(ctx)
	if err != nil {
		slog.Error("failed to find repos", "error", err)
		err = errorx.HandleDBError(err, errorx.Ctx().Set("ids", ids))
		return nil, 0, err
	}

	repoMap := make(map[int64]*Repository, len(repos))
	for _, repo := range repos {
		repoMap[repo.ID] = repo
	}

	orderedRepos := make([]*Repository, 0, len(ids))
	for _, id := range ids {
		if repo, exists := repoMap[id]; exists {
			orderedRepos = append(orderedRepos, repo)
		}
	}

	return orderedRepos, count, nil
}

func buildLikeQuery(q *bun.SelectQuery, filter *types.RepoFilter, limit int) {
	q.Where(
		`LOWER(repository.path) LIKE ? 
		 OR LOWER(repository.hf_path) LIKE ?
		 OR LOWER(repository.ms_path) LIKE ?
		 OR LOWER(repository.description) LIKE ? 
		 OR LOWER(repository.nickname) LIKE ?`,
		"%"+filter.Search+"%",
		"%"+filter.Search+"%",
		"%"+filter.Search+"%",
		"%"+filter.Search+"%",
		"%"+filter.Search+"%",
	)
	q.Limit(limit)
}

// buildSearchRepoQuery constructs a repository query statement, automatically selecting full-text search or LIKE query based on the database environment.
func buildVectorSearchQuery(
	q *bun.SelectQuery,
	filter *types.RepoFilter,
	db *bun.DB,
	searchConfiguration string,
	limit int,
) *bun.SelectQuery {
	input := filter.Search

	tsQuerySub := `
		(
			SELECT array_to_string(
				ARRAY(
					SELECT (trim(item) || ':*')
					FROM unnest(
						string_to_array(
							plainto_tsquery(?, regexp_replace(?, '[-/]+', ' ', 'g'))::text,
							'&'
						)
					) AS item
				),
				'&'
			)::tsquery
		)
	`
	q.Where(`repository.search_vector @@ `+tsQuerySub, searchConfiguration, input)
	q.Limit(limit)

	oq := db.NewSelect().
		TableExpr("(?) AS r", q).
		Column("r.*").
		ColumnExpr(`
			ts_rank_cd(
				r.search_vector,
				`+tsQuerySub+`,
				32
			) AS rank
		`, searchConfiguration, input)
	oq.OrderExpr("rank DESC")

	return oq
}

func (s *repoStoreImpl) IsMirrorRepo(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error) {
	var result struct {
		Exists bool `bun:"exists"`
	}

	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("EXISTS(SELECT 1 FROM mirrors WHERE mirrors.repository_id = repositories.id) AS exists").
		Table("repositories").
		Where("repositories.repository_type = ? AND LOWER(repositories.path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", namespace, name)).
		Limit(1).
		Scan(ctx, &result)
	if err != nil {
		return false, err
	}

	return result.Exists, nil
}

func (s *repoStoreImpl) ListRepoByDeployType(ctx context.Context, repoType types.RepositoryType, userID int64, search, sort string, deployType, per, page int) (repos []*Repository, count int, err error) {
	queryArchs := "SELECT architecture_name FROM runtime_architectures WHERE runtime_framework_id IN (SELECT id FROM runtime_frameworks WHERE type=?)"
	var architectureNames []string
	err = s.db.BunDB.NewRaw(queryArchs, deployType).
		Scan(ctx, &architectureNames)
	if err != nil {
		return
	}

	q := s.db.Operator.Core.
		NewSelect().
		Column("repository.*").
		Model(&repos).
		Relation("Tags").
		Relation("Metadata")

	q.Where("metadata.architecture IN (?) or metadata.model_type IN (?) or metadata.class_name IN (?)", bun.In(architectureNames), bun.In(architectureNames), bun.In(architectureNames))

	q.Where("repository.repository_type = ?", repoType)
	q.Where("repository.private = ? or repository.user_id = ?", false, userID)
	if search != "" {
		search = strings.ToLower(search)
		q.Where(
			"LOWER(repository.path) like ? or LOWER(repository.description) like ? or LOWER(repository.nickname) like ?",
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
			fmt.Sprintf("%%%s%%", search),
		)
	}
	count, err = q.Count(ctx)
	if err != nil {
		return
	}

	orderBy := "path"

	if sort != "" {
		if sort == "trending" {
			subQuery := s.db.Operator.Core.NewSelect().
				Model((*RecomRepoScore)(nil)).
				Column("score", "repository_id").
				Where("weight_name = ?", RecomWeightTotal)
			q.Join("LEFT JOIN (" + subQuery.String() + ") AS recom_repo_scores ON repository.id = recom_repo_scores.repository_id")
			q.ColumnExpr(`COALESCE(recom_repo_scores.score, 0) AS popularity`)
		}
		sortByStr, exits := sortBy[sort]
		if exits {
			orderBy = sortByStr
		}
	}

	err = q.Order(orderBy).
		Limit(per).Offset((page - 1) * per).
		Scan(ctx)

	return
}

func (s *repoStoreImpl) WithMirror(ctx context.Context, per, page int) (repos []Repository, count int, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&repos).
		Relation("Mirror").
		Where("mirror.id is not null")
	count, err = q.Count(ctx)
	if err != nil {
		return
	}
	err = q.Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)

	if err != nil {
		return
	}

	return
}

func (s *repoStoreImpl) CleanRelationsByRepoID(ctx context.Context, repoId int64) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.Exec("delete from repositories_runtime_frameworks where repo_id=?", repoId); err != nil {
			return err
		}

		if _, err := tx.Exec("delete from user_likes where repo_id=?", repoId); err != nil {
			return err
		}

		if _, err := tx.Exec("delete from lfs_meta_objects where repository_id=?", repoId); err != nil {
			return err
		}

		// delete mirrors
		var mirrorIDs []int64
		if err := tx.NewSelect().Model(&Mirror{}).Column("id").Where("repository_id=?", repoId).Scan(ctx, &mirrorIDs); err != nil {
			return err
		}

		if len(mirrorIDs) > 0 {
			if _, err := tx.Exec("delete from mirrors where repository_id=?", repoId); err != nil {
				return err
			}

			if _, err := tx.Exec("delete from mirror_tasks where mirror_id in (?)", bun.In(mirrorIDs)); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (s *repoStoreImpl) BatchCreateRepoTags(ctx context.Context, repoTags []RepositoryTag) error {
	result, err := s.db.Operator.Core.NewInsert().
		Model(&repoTags).
		Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(repoTags)), result, err)
}

func (s *repoStoreImpl) DeleteAllFiles(ctx context.Context, repoID int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&File{}).
		Where("repository_id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *repoStoreImpl) DeleteAllTags(ctx context.Context, repoID int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&RepositoryTag{}).
		Where("repository_id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *repoStoreImpl) UpdateOrCreateRepo(ctx context.Context, input Repository) (*Repository, error) {
	input.UpdatedAt = time.Now()
	_, err := s.db.Core.NewUpdate().
		Model(&input).
		Where("path = ? and repository_type = ?", input.Path, input.RepositoryType).
		Returning("*").
		Exec(ctx, &input)
	if err == nil {
		return &input, nil
	}

	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *repoStoreImpl) UpdateLicenseByTag(ctx context.Context, repoID int64) error {
	var tag Tag
	err := s.db.Core.NewSelect().
		Model(&tag).
		Join("join repository_tags on tag.id = repository_tags.tag_id").
		Join("join repositories on repositories.id = repository_tags.repository_id").
		Where("repository_tags.repository_id = ? and tag.category = ?", repoID, "license").
		Scan(ctx)
	if err != nil {
		return err
	}
	if tag.Name != "" {
		repo, err := s.FindById(ctx, repoID)
		if err != nil {
			return err
		}
		repo.License = tag.Name
		_, err = s.UpdateRepo(ctx, *repo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *repoStoreImpl) CountByRepoType(ctx context.Context, repoType types.RepositoryType) (int, error) {
	return s.db.Core.NewSelect().Model(&Repository{}).Where("repository_type = ?", repoType).Count(ctx)
}

func (s *repoStoreImpl) GetRepoWithoutRuntimeByID(ctx context.Context, rfID int64, paths []string, batchSize, batch int) ([]Repository, error) {
	var res []Repository
	q := s.db.Operator.Core.NewSelect().Model(&res).Relation("Tags")
	if len(paths) > 0 {
		q.Where("path in (?)", bun.In(paths))
	}
	q.Where("repository_type = ?", types.ModelRepo).
		Where("id not in (select repo_id from repositories_runtime_frameworks where runtime_framework_id = ?)", rfID)
	err := q.Order("id desc").
		Limit(batchSize).
		Offset(batchSize * batch).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select repos without runtime failed, %w", err)
	}
	return res, nil
}

func (s *repoStoreImpl) GetRepoWithRuntimeByID(ctx context.Context, rfID int64, paths []string) ([]Repository, error) {
	var res []Repository
	q := s.db.Operator.Core.NewSelect().Model(&res).Relation("Tags")
	if len(paths) > 0 {
		q.Where("path in (?)", bun.In(paths))
	}
	err := q.Where("repository_type = ?", types.ModelRepo).
		Where("id in (select repo_id from repositories_runtime_frameworks where runtime_framework_id = ?)", rfID).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select repos with runtime failed, %w", err)
	}
	return res, nil
}

func (s *repoStoreImpl) BatchGet(ctx context.Context, lastRepoID int64, batch int, filter *types.BatchGetFilter) ([]Repository, error) {
	var res []Repository
	q := s.db.Operator.Core.NewSelect().Model(&res)
	if lastRepoID > 0 {
		q.Where("id > ?", lastRepoID)
	}

	// Apply filters only if filter is provided and fields have meaningful values
	if filter != nil {
		// Apply repository type filter only if specified
		if filter.RepoType != "" {
			q.Where("repository_type = ?", filter.RepoType)
		}

		// Apply sensitive check status filter only if specified (pointer is not nil)
		if filter.SensitiveCheckStatus != nil {
			q.Where("sensitive_check_status = ?", *filter.SensitiveCheckStatus)
		}
	}

	err := q.Order("id ASC").
		Limit(batch).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("select repos failed, last_repo_id: %d, batch: %d, %w", lastRepoID, batch, err)
	}
	return res, nil
}

func (s *repoStoreImpl) FindWithBatch(ctx context.Context, batchSize, batch int, repoTypes ...types.RepositoryType) ([]Repository, error) {
	var res []Repository
	q := s.db.Operator.Core.NewSelect().
		Model(&res).
		Relation("Tags")
	if len(repoTypes) > 0 {
		q.Where("repository_type in (?)", bun.In(repoTypes))
	}
	err := q.Order("id desc").
		Limit(batchSize).
		Offset(batchSize * batch).
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) ByUser(ctx context.Context, userID int64, batchSize, batch int) ([]Repository, error) {
	var repos []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&repos).
		Where("user_id = ?", userID).
		Order("id desc").
		Limit(batchSize).
		Offset(batch * batchSize).
		Scan(ctx)
	return repos, err
}

func (s *repoStoreImpl) FindByRepoSourceWithBatch(ctx context.Context, repoSource types.RepositorySource, batchSize, batch int) ([]Repository, error) {
	var res []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Where("source = ?", repoSource).
		Order("id desc").
		Limit(batchSize).
		Offset(batchSize * batch).
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) FindMirrorReposByUserAndSource(ctx context.Context, userID int64, source string, batchSize, batch int) ([]Repository, error) {
	var res []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Relation("Mirror").
		Relation("Mirror.MirrorSource").
		Where("mirror__mirror_source.source_name = ? and repository.user_id = ?", source, userID).
		Order("id desc").
		Limit(batchSize).
		Offset(batchSize * (batch - 1)).
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) FindMirrorReposWithBatch(ctx context.Context, batchSize, batch int) ([]Repository, error) {
	var res []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Relation("Mirror").
		Where("mirror.id is not null").
		Order("id desc").
		Limit(batchSize).
		Offset(batchSize * (batch - 1)).
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) UpdateSourcePath(ctx context.Context, repoID int64, sourcePath, sourceType string) error {
	var field string
	switch sourceType {
	case enum.CSGSource:
		field = "csg_path"
	case enum.HFSource:
		field = "hf_path"
	case enum.MSSource:
		field = "ms_path"
	case enum.GitHubSource:
		field = "github_path"
	default:
		return fmt.Errorf("unknown source type: %s", sourceType)
	}

	_, err := s.db.Operator.Core.NewUpdate().
		Model(&Repository{}).
		Set(field+" = ?", sourcePath).
		Where("id = ?", repoID).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *repoStoreImpl) BulkUpdateSourcePath(ctx context.Context, repos []*Repository) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&repos).
		Column("csg_path", "hf_path", "ms_path").
		Bulk().
		Exec(ctx)
	return err
}
func (s *repoStoreImpl) FindByMirrorSourceURL(ctx context.Context, sourceURL string) (*Repository, error) {
	var res Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Relation("Mirror").
		Where("mirror.source_url = ?", sourceURL).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *repoStoreImpl) RefreshLFSObjectsSize(ctx context.Context, repoID int64) error {
	var totalSize int64
	err := s.db.Operator.Core.NewSelect().
		Model(&LfsMetaObject{}).
		ColumnExpr("COALESCE(SUM(size), 0)").
		Where("repository_id = ?", repoID).
		Scan(ctx, &totalSize)

	if err != nil {
		return err
	}
	_, err = s.db.Operator.Core.NewUpdate().
		Model(&Repository{}).
		Set("lfs_objects_size = ?", totalSize).
		Where("id = ?", repoID).
		Exec(ctx)
	return err
}

func (s *repoStoreImpl) FindMirrorFinishedPrivateModelRepo(ctx context.Context) ([]*Repository, error) {
	var res []*Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Join("JOIN mirrors ON mirrors.repository_id = repository.id").
		Join("JOIN mirror_tasks ON mirror_tasks.mirror_id = mirrors.id").
		Where(
			"repository.repository_type = ? and mirror_tasks.status = ? and repository.sensitive_check_status = ? and repository.private = true",
			types.ModelRepo, types.MirrorLfsSyncFinished, types.SensitiveCheckPass).
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) BatchUpdate(ctx context.Context, repos []*Repository) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&repos).
		Bulk().
		Exec(ctx)
	return err
}

func (s *repoStoreImpl) FindByRepoTypeAndPaths(ctx context.Context, repoType types.RepositoryType, paths []string) ([]Repository, error) {
	var res []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Relation("Mirror").
		Where("repository_type = ? and path in (?)", repoType, bun.In(paths)).
		Order("created_at DESC").
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) FindUnhashedRepos(ctx context.Context, batchSize int, lastID int64) ([]Repository, error) {
	var res []Repository
	err := s.db.Operator.Core.NewSelect().
		Model(&res).
		Where("hashed = ? and id > ?", false, lastID).
		Limit(batchSize).
		Order("id ASC").
		Scan(ctx)
	return res, err
}

func (s *repoStoreImpl) GetReposBySearch(ctx context.Context, search string, repoType types.RepositoryType, page, pageSize int) ([]*Repository, int, error) {
	var (
		res   []*Repository
		count int
		err   error
	)
	count, err = s.db.Operator.Core.NewSelect().
		Model(&res).
		Where("path like ? and repository_type = ?", fmt.Sprintf("%%%s%%", search), repoType).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		ScanAndCount(ctx)
	return res, count, err
}
