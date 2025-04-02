package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

var RepositorySourceAndPrefixMapping = map[types.RepositorySource]string{
	types.HuggingfaceSource: types.HuggingfacePrefix,
	types.OpenCSGSource:     types.OpenCSGPrefix,
	types.LocalSource:       "",
}

type repoStoreImpl struct {
	db *DB
}

type RepoStore interface {
	CreateRepoTx(ctx context.Context, tx bun.Tx, input Repository) (*Repository, error)
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
	BatchGet(ctx context.Context, repoType types.RepositoryType, lastRepoID int64, batch int) ([]Repository, error)
	FindWithBatch(ctx context.Context, batchSize, batch int, repoTypes ...types.RepositoryType) ([]Repository, error)
	ByUser(ctx context.Context, userID int64) ([]Repository, error)
	FindByRepoSourceWithBatch(ctx context.Context, repoSource types.RepositorySource, batchSize, batch int) ([]Repository, error)
	UpdateSourcePath(ctx context.Context, repoID int64, sourcePath, sourceType string) error
	FindMirrorReposWithBatch(ctx context.Context, batchSize, batch int) ([]Repository, error)
	BulkUpdateSourcePath(ctx context.Context, repos []*Repository) error
}

func NewRepoStore() RepoStore {
	return &repoStoreImpl{
		db: defaultDB,
	}
}

func NewRepoStoreWithDB(db *DB) RepoStore {
	return &repoStoreImpl{
		db: db,
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
			if tag.Name == "gguf" {
				return tag.Name
			}
			if tag.Name == "onnx" {
				return tag.Name
			}
			if tag.Name == "safetensors" {
				return tag.Name
			}
		}
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

func (s *repoStoreImpl) CreateRepoTx(ctx context.Context, tx bun.Tx, input Repository) (*Repository, error) {
	res, err := tx.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *repoStoreImpl) CreateRepo(ctx context.Context, input Repository) (*Repository, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *repoStoreImpl) UpdateRepo(ctx context.Context, input Repository) (*Repository, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *repoStoreImpl) DeleteRepo(ctx context.Context, input Repository) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *repoStoreImpl) Find(ctx context.Context, owner, repoType, repoName string) (*Repository, error) {
	var err error
	repo := &Repository{}
	err = s.db.Operator.Core.
		NewSelect().
		Model(repo).
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, owner, repoName)).
		Limit(1).
		Scan(ctx)
	return repo, err
}

func (s *repoStoreImpl) FindById(ctx context.Context, id int64) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("id =?", id).
		Scan(ctx)
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
	return repos, err
}

func (s *repoStoreImpl) FindByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Relation("Tags").
		Relation("Metadata").
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Limit(1).
		Scan(ctx)
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
	return repos, err
}

func (s *repoStoreImpl) Exists(ctx context.Context, repoType types.RepositoryType, namespace string, name string) (bool, error) {
	return s.db.Operator.Core.NewSelect().Model((*Repository)(nil)).
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Exists(ctx)
}

func (s *repoStoreImpl) All(ctx context.Context) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	err := s.db.Operator.Core.
		NewSelect().
		Model(&repos).
		Scan(ctx)
	return repos, err
}

func (s *repoStoreImpl) UpdateRepoFileDownloads(ctx context.Context, repo *Repository, date time.Time, clickDownloadCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), repo.ID).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
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
	if err != nil {
		return
	}

	return
}

func (s *repoStoreImpl) UpdateRepoCloneDownloads(ctx context.Context, repo *Repository, date time.Time, cloneCount int64) (err error) {
	rd := new(RepositoryDownload)
	err = s.db.Operator.Core.NewSelect().
		Model(rd).
		Where("date = ? AND repository_id = ?", date.Format("2006-01-02"), repo.ID).
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
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
	if err != nil {
		return
	}

	return
}

func (s *repoStoreImpl) UpdateDownloads(ctx context.Context, repo *Repository) error {
	var downloadCount int64
	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("(SUM(clone_count)+SUM(click_download_count)) AS total_count").
		Model(&RepositoryDownload{}).
		Where("repository_id=?", repo.ID).
		Scan(ctx, &downloadCount)
	if err != nil {
		return err
	}
	repo.DownloadCount = downloadCount
	_, err = s.db.Operator.Core.NewUpdate().
		Model(repo).
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *repoStoreImpl) Tags(ctx context.Context, repoID int64) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&RepositoryTag{}).
		Join("JOIN tags ON repository_tag.tag_id = tags.id").
		Where("repository_tag.repository_id = ?", repoID).
		Where("repository_tag.count > 0")
	err = query.Scan(ctx, &tags)
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
	return tagIDs, err
}

func (s *repoStoreImpl) SetUpdateTimeByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string, update time.Time) error {
	repo := new(Repository)
	repo.UpdatedAt = update
	_, err := s.db.Operator.Core.NewUpdate().Model(repo).
		Column("updated_at").
		Where("LOWER(git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Exec(ctx)
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
		q.Where("repository.source = ?", filter.Source)
	}

	if filter.Search != "" {
		filter.Search = strings.ToLower(filter.Search)
		q.Where(
			"LOWER(repository.path) like ? or LOWER(repository.description) like ? or LOWER(repository.nickname) like ?",
			fmt.Sprintf("%%%s%%", filter.Search),
			fmt.Sprintf("%%%s%%", filter.Search),
			fmt.Sprintf("%%%s%%", filter.Search),
		)
	}
	// list serverless
	if filter.ListServerless {
		q.Where("repository.id IN (SELECT repo_id FROM deploys WHERE type = ? and status = ?)", types.ServerlessType, common.Running)
	}

	if len(filter.SpaceSDK) > 0 {
		q.Join("LEFT JOIN spaces AS spaces ON repository.id = spaces.repository_id").Where("spaces.sdk = ?", filter.SpaceSDK)
	}

	if len(filter.Tags) > 0 {
		for i, tag := range filter.Tags {
			var asRepoTag = fmt.Sprintf("%s%d", "rt", i)
			var asTag = fmt.Sprintf("%s%d", "ts", i)
			q.Join(fmt.Sprintf("JOIN repository_tags AS %s ON repository.id = %s.repository_id", asRepoTag, asRepoTag)).
				Join(fmt.Sprintf("JOIN tags AS %s ON %s.tag_id = %s.id", asTag, asRepoTag, asTag))
			q.Where(fmt.Sprintf("%s.category = ? AND %s.name = ?", asTag, asTag), tag.Category, tag.Name)
		}

	}

	count, err = q.Count(ctx)
	if err != nil {
		return
	}

	if filter.Sort == "trending" {
		q.Join("Left Join recom_repo_scores on repository.id = recom_repo_scores.repository_id")
		q.Join("Left Join recom_op_weights on repository.id = recom_op_weights.repository_id")
		q.ColumnExpr(`COALESCE(recom_repo_scores.score, 0)+COALESCE(recom_op_weights.weight, 0) AS popularity`)
	}

	err = q.Order(sortBy[filter.Sort]).
		Limit(per).Offset((page - 1) * per).
		Scan(ctx)

	return
}

func (s *repoStoreImpl) IsMirrorRepo(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error) {
	var result struct {
		Exists bool `bun:"exists"`
	}

	err := s.db.Operator.Core.NewSelect().
		ColumnExpr("EXISTS(SELECT 1 FROM mirrors WHERE mirrors.repository_id = repositories.id) AS exists").
		Table("repositories").
		Where("LOWER(repositories.git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
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

func (s *repoStoreImpl) BatchGet(ctx context.Context, repoType types.RepositoryType, lastRepoID int64, batch int) ([]Repository, error) {
	var res []Repository
	q := s.db.Operator.Core.NewSelect().Model(&res)
	if lastRepoID > 0 {
		q.Where("id > ?", lastRepoID)
	}
	err := q.Where("repository_type = ? and sensitive_check_status = ?", repoType, types.SensitiveCheckPending).
		Order("id ASC").
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

func (s *repoStoreImpl) ByUser(ctx context.Context, userID int64) ([]Repository, error) {
	var repos []Repository
	err := s.db.Operator.Core.NewSelect().Model(&repos).Where("user_id = ?", userID).Scan(ctx)
	return repos, err
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
