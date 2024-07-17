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
	"opencsg.com/csghub-server/common/types"
)

var RepositorySourceAndPrefixMapping = map[types.RepositorySource]string{
	types.HuggingfaceSource: types.HuggingfacePrefix,
	types.OpenCSGSource:     types.OpenCSGPrefix,
	types.LocalSource:       "",
}

type RepoStore struct {
	db *DB
}

func NewRepoStore() *RepoStore {
	return &RepoStore{
		db: defaultDB,
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
	Readme         string                     `bun:",nullzero" json:"readme"`
	DefaultBranch  string                     `bun:",notnull" json:"default_branch"`
	LfsFiles       []LfsFile                  `bun:"rel:has-many,join:id=repository_id" json:"-"`
	Likes          int64                      `bun:",nullzero" json:"likes"`
	DownloadCount  int64                      `bun:",nullzero" json:"download_count"`
	Downloads      []RepositoryDownload       `bun:"rel:has-many,join:id=repository_id" json:"downloads"`
	Tags           []Tag                      `bun:"m2m:repository_tags,join:Repository=Tag" json:"tags"`
	Mirror         Mirror                     `bun:"rel:has-one,join:id=repository_id" json:"mirror"`
	RepositoryType types.RepositoryType       `bun:",notnull" json:"repository_type"`
	HTTPCloneURL   string                     `bun:",nullzero" json:"http_clone_url"`
	SSHCloneURL    string                     `bun:",nullzero" json:"ssh_clone_url"`
	Source         types.RepositorySource     `bun:",nullzero,default:'local'" json:"source"`
	SyncStatus     types.RepositorySyncStatus `bun:",nullzero" json:"sync_status"`
	// updated_at timestamp will be updated only if files changed
	times
}

// NamespaceAndName returns namespace and name by parsing repository path
func (r Repository) NamespaceAndName() (namespace string, name string) {
	fields := strings.Split(r.Path, "/")
	return fields[0], fields[1]
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

func (s *RepoStore) CreateRepoTx(ctx context.Context, tx bun.Tx, input Repository) (*Repository, error) {
	res, err := tx.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *RepoStore) CreateRepo(ctx context.Context, input Repository) (*Repository, error) {
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create repository in tx failed,error:%w", err)
	}

	return &input, nil
}

func (s *RepoStore) UpdateRepo(ctx context.Context, input Repository) (*Repository, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)

	return &input, err
}

func (s *RepoStore) DeleteRepo(ctx context.Context, input Repository) error {
	_, err := s.db.Core.NewDelete().Model(&input).WherePK().Exec(ctx)

	return err
}

func (s *RepoStore) Find(ctx context.Context, owner, repoType, repoName string) (*Repository, error) {
	var err error
	repo := &Repository{}
	err = s.db.Operator.Core.
		NewSelect().
		Model(repo).
		Where("git_path =?", fmt.Sprintf("%ss_%s/%s", repoType, owner, repoName)).
		Limit(1).
		Scan(ctx)
	return repo, err
}

func (s *RepoStore) FindById(ctx context.Context, id int64) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("id =?", id).
		Scan(ctx)
	return resRepo, err
}

func (s *RepoStore) FindByIds(ctx context.Context, ids []int64, opts ...SelectOption) ([]*Repository, error) {
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

func (s *RepoStore) FindByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("git_path =?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return resRepo, err
}

func (s *RepoStore) FindByGitPath(ctx context.Context, path string) (*Repository, error) {
	resRepo := new(Repository)
	err := s.db.Operator.Core.
		NewSelect().
		Model(resRepo).
		Where("git_path =?", path).
		Scan(ctx)
	return resRepo, err
}

func (s *RepoStore) FindByGitPaths(ctx context.Context, paths []string, opts ...SelectOption) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	q := s.db.Operator.Core.
		NewSelect()
	for _, opt := range opts {
		opt.Appply(q)
	}
	err := q.Model(&repos).
		Where("git_path in (?)", bun.In(paths)).
		Scan(ctx)
	return repos, err
}

func (s *RepoStore) Exists(ctx context.Context, repoType types.RepositoryType, namespace string, name string) (bool, error) {
	return s.db.Operator.Core.NewSelect().Model((*Repository)(nil)).
		Where("git_path =?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Exists(ctx)
}

func (s *RepoStore) All(ctx context.Context) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	err := s.db.Operator.Core.
		NewSelect().
		Model(&repos).
		Scan(ctx)
	return repos, err
}

func (s *RepoStore) UpdateRepoFileDownloads(ctx context.Context, repo *Repository, date time.Time, clickDownloadCount int64) (err error) {
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

func (s *RepoStore) UpdateRepoCloneDownloads(ctx context.Context, repo *Repository, date time.Time, cloneCount int64) (err error) {
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

func (s *RepoStore) UpdateDownloads(ctx context.Context, repo *Repository) error {
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

func (s *RepoStore) Tags(ctx context.Context, repoID int64) (tags []Tag, err error) {
	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("tags.*").
		Model(&RepositoryTag{}).
		Join("JOIN tags ON repository_tag.tag_id = tags.id").
		Where("repository_tag.repository_id = ?", repoID).
		Where("repository_tag.count > 0")
	err = query.Scan(ctx, &tags)
	return
}

// TagIDs get tag ids by repo id, if category is not empty, return only tags of the category
func (s *RepoStore) TagIDs(ctx context.Context, repoID int64, category string) (tagIDs []int64, err error) {
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

func (s *RepoStore) SetUpdateTimeByPath(ctx context.Context, repoType types.RepositoryType, namespace, name string, update time.Time) error {
	repo := new(Repository)
	repo.UpdatedAt = update
	_, err := s.db.Operator.Core.NewUpdate().Model(repo).
		Column("updated_at").
		Where("git_path =?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Exec(ctx)
	return err
}

func (s *RepoStore) PublicToUser(ctx context.Context, repoType types.RepositoryType, userID int64, filter *types.RepoFilter, per, page int) (repos []*Repository, count int, err error) {
	q := s.db.Operator.Core.
		NewSelect().
		Column("repository.*").
		Model(&repos).
		Relation("Tags")

	q.Where("repository.repository_type = ?", repoType)
	q.Where("repository.private = ? or repository.user_id = ?", false, userID)

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
	if len(filter.Tags) > 0 {
		q.Join("JOIN repository_tags ON repository.id = repository_tags.repository_id").
			Join("JOIN tags ON repository_tags.tag_id = tags.id")
		for _, tag := range filter.Tags {
			q.Where("tags.category = ? AND tags.name = ?", tag.Category, tag.Name)
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

func (s *RepoStore) IsMirrorRepo(ctx context.Context, repoType types.RepositoryType, namespace, name string) (bool, error) {
	var result struct {
		Exists bool `bun:"exists"`
	}

	query := s.db.Operator.Core.NewSelect().
		ColumnExpr("EXISTS(SELECT 1 FROM mirrors WHERE mirrors.repository_id = repositories.id) AS exists").
		Table("repositories").
		Where("repositories.git_path = ?", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
		Limit(1)

	err := query.Scan(ctx, &result)
	if err != nil {
		return false, err
	}

	return result.Exists, nil
}

func (s *RepoStore) ListRepoPublicToUserByRepoIDs(ctx context.Context, repoType types.RepositoryType, userID int64, search, sort string, per, page int, repoIDs []int64) (repos []*Repository, count int, err error) {
	q := s.db.Operator.Core.
		NewSelect().
		Column("repository.*").
		Model(&repos).
		Relation("Tags")

	q.Where("repository.repository_type = ?", repoType)
	q.Where("repository.private = ? or repository.user_id = ?", false, userID)
	q.Where("id in (?)", bun.In(repoIDs))

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
			q.Join("Left Join recom_repo_scores on repository.id = recom_repo_scores.repository_id")
			q.Join("Left Join recom_op_weights on repository.id = recom_op_weights.repository_id")
			q.ColumnExpr(`COALESCE(recom_repo_scores.score, 0)+COALESCE(recom_op_weights.weight, 0) AS popularity`)
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

func (s *RepoStore) WithMirror(ctx context.Context, per, page int) (repos []Repository, count int, err error) {
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

func (s *RepoStore) CleanRelationsByRepoID(ctx context.Context, repoId int64) error {
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

func (s *RepoStore) BatchCreateRepoTags(ctx context.Context, repoTags []RepositoryTag) error {
	result, err := s.db.Operator.Core.NewInsert().
		Model(&repoTags).
		Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(repoTags)), result, err)
}

func (s *RepoStore) DeleteAllFiles(ctx context.Context, repoID int64) error {
	err := s.db.Operator.Core.NewDelete().
		Model(&File{}).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *RepoStore) DeleteAllTags(ctx context.Context, repoID int64) error {
	err := s.db.Operator.Core.NewDelete().
		Model(&RepositoryTag{}).
		Where("repository_id = ?", repoID).
		Scan(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (s *RepoStore) UpdateOrCreateRepo(ctx context.Context, input Repository) (*Repository, error) {
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

func (s *RepoStore) CountByRepoType(ctx context.Context, repoType types.RepositoryType) (int, error) {
	return s.db.Core.NewSelect().Model(&Repository{}).Where("repository_type = ?", repoType).Count(ctx)
}
