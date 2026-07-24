package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type mirrorStoreImpl struct {
	db *DB
}

type MirrorStore interface {
	IsExist(ctx context.Context, repoID int64) (exists bool, err error)
	IsRepoExist(ctx context.Context, repoType types.RepositoryType, namespace, name string) (exists bool, err error)
	FindByRepoID(ctx context.Context, repoID int64) (*Mirror, error)
	FindByID(ctx context.Context, ID int64, forUpdate ...bool) (*Mirror, error)
	FindByIDs(ctx context.Context, IDs []int64) ([]Mirror, error)
	FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error)
	IndexSyncWithPagination(ctx context.Context, query MirrorSyncListQuery) ([]Mirror, int, error)
	FindWithMapping(ctx context.Context, repoType types.RepositoryType, namespace, name string, mapping types.Mapping) (*Repository, error)
	Create(ctx context.Context, mirror *Mirror) (*Mirror, error)
	NoPushMirror(ctx context.Context) ([]Mirror, error)
	PushedMirror(ctx context.Context) ([]Mirror, error)
	Update(ctx context.Context, mirror *Mirror) (err error)
	Unfinished(ctx context.Context) ([]Mirror, error)
	Finished(ctx context.Context) ([]Mirror, error)
	ToSyncRepo(ctx context.Context) ([]Mirror, error)
	ToSyncLfs(ctx context.Context) ([]Mirror, error)
	IndexWithPagination(ctx context.Context, per, page int, filter types.MirrorFilter, hasRepo bool) (mirrors []Mirror, count int, err error)
	StatusCount(ctx context.Context) ([]MirrorStatusCount, error)
	UpdateMirrorAndRepository(ctx context.Context, mirror *Mirror, repo *Repository) error
	FindBySourceURLs(ctx context.Context, sourceURLs []string) ([]Mirror, error)
	BatchUpdate(ctx context.Context, mirrors []Mirror) error
	BatchCreate(ctx context.Context, mirrors []Mirror) error
	ToBeScheduled(ctx context.Context) ([]Mirror, error)
	Delete(ctx context.Context, mirror *Mirror) error
	DeleteWithTaskCancelTx(ctx context.Context, mirrorID int64, jobCancelClient MirrorJobCancelClient) error
}

// MirrorSyncListQuery contains database-level mirror sync filters and pagination.
type MirrorSyncListQuery struct {
	Page     int
	Per      int
	Search   string
	Statuses []types.MirrorTaskStatus
}

func NewMirrorStore() MirrorStore {
	return &mirrorStoreImpl{
		db: defaultDB,
	}
}

func NewMirrorStoreWithDB(db *DB) MirrorStore {
	return &mirrorStoreImpl{
		db: db,
	}
}

type Mirror struct {
	ID int64 `bun:",pk,autoincrement" json:"id"`
	// Interval is retained only for database schema compatibility.
	Interval       string       `bun:",notnull" json:"-"`
	SourceUrl      string       `bun:",notnull" json:"source_url"`
	MirrorSourceID int64        `bun:",notnull" json:"mirror_source_id"`
	MirrorSource   MirrorSource `bun:"rel:belongs-to,join:mirror_source_id=id" json:"mirror_source"`
	//source user name
	Username string `bun:",nullzero" json:"-"`
	// source access token
	AccessToken            string                 `bun:",nullzero" json:"-"`
	PushUrl                string                 `bun:",nullzero" json:"-"`
	PushUsername           string                 `bun:",nullzero" json:"-"`
	PushAccessToken        string                 `bun:",nullzero" json:"-"`
	RepositoryID           int64                  `bun:",notnull" json:"repository_id"`
	Repository             *Repository            `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	LastUpdatedAt          time.Time              `bun:",nullzero" json:"last_updated_at"`
	SourceRepoPath         string                 `bun:",nullzero" json:"source_repo_path"`
	LocalRepoPath          string                 `bun:",nullzero" json:"local_repo_path"`
	LastMessage            string                 `bun:",nullzero" json:"last_message"`
	MirrorTaskID           int64                  `bun:",nullzero" json:"mirror_task_id"`
	MirrorTasks            []*MirrorTask          `bun:"rel:has-many,join:mirror_task_id=id" json:"mirror_task"`
	PushMirrorCreated      bool                   `bun:",nullzero,default:false" json:"push_mirror_created"`
	Status                 types.MirrorTaskStatus `bun:",nullzero" json:"status"`
	Progress               int8                   `bun:",nullzero" json:"progress"`
	NextExecutionTimestamp time.Time              `bun:",nullzero" json:"next_execution_timestamp"`
	Priority               types.MirrorPriority   `bun:"mirror_priority,notnull,default:0" json:"priority"`
	RetryCount             int                    `bun:",nullzero" json:"retry_count"`
	RemoteUpdatedAt        time.Time              `bun:",nullzero" json:"remote_updated_at"`
	CurrentTaskID          int64                  `bun:",nullzero" json:"current_task_id"`
	CurrentTask            *MirrorTask            `bun:"rel:has-one,join:current_task_id=id" json:"current_task"`

	times
}

type MirrorStatusCount struct {
	Status types.MirrorTaskStatus `bun:"status"`
	Count  int                    `bun:"count"`
}

func (m *Mirror) RepoPath() string {
	if m.Repository != nil {
		return fmt.Sprintf("%ss/%s", m.Repository.RepositoryType, m.Repository.Path)
	}
	return ""
}

func (m *Mirror) SetStatus(status types.MirrorTaskStatus) {
	m.Status = status
}

func (s *mirrorStoreImpl) IsExist(ctx context.Context, repoID int64) (exists bool, err error) {
	var mirror Mirror
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&mirror).
		Where("repository_id=?", repoID).
		Exists(ctx)
	return
}
func (s *mirrorStoreImpl) IsRepoExist(ctx context.Context, repoType types.RepositoryType, namespace, name string) (exists bool, err error) {
	var repo Repository
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&repo).
		Where("path=?", fmt.Sprintf("%s/%s", namespace, name)).
		Where("repository_type=?", repoType).
		Exists(ctx)
	return
}

func (s *mirrorStoreImpl) FindByRepoID(ctx context.Context, repoID int64) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Relation("CurrentTask").
		Relation("Repository").
		Relation("MirrorTasks").
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *mirrorStoreImpl) FindByID(ctx context.Context, ID int64, forUpdate ...bool) (*Mirror, error) {
	var mirror Mirror
	query := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Relation("Repository").
		Where("mirror.id=?", ID)

	// Check if forUpdate is true
	if len(forUpdate) > 0 && forUpdate[0] {
		query = query.For("UPDATE OF mirror")
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

// IndexSyncWithPagination applies database filters and orders current tasks by latest update before loading one page.
func (s *mirrorStoreImpl) IndexSyncWithPagination(ctx context.Context, query MirrorSyncListQuery) ([]Mirror, int, error) {
	var mirrors []Mirror
	q := s.newMirrorSyncSelect(&mirrors, query.Search).
		OrderExpr("current_task.updated_at DESC NULLS LAST, mirror.id DESC")
	if len(query.Statuses) > 0 {
		q = q.Where("current_task.status IN (?)", bun.In(query.Statuses))
	}
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	if err := q.Limit(query.Per).Offset((query.Page - 1) * query.Per).Scan(ctx); err != nil {
		return nil, 0, err
	}
	return mirrors, count, nil
}

// newMirrorSyncSelect applies relations and static search shared by mirror sync queries.
func (s *mirrorStoreImpl) newMirrorSyncSelect(mirrors *[]Mirror, search string) *bun.SelectQuery {
	q := s.db.Operator.Core.NewSelect().
		Model(mirrors).
		Relation("Repository").
		Relation("CurrentTask")
	if search == "" {
		return q
	}
	pattern := fmt.Sprintf("%%%s%%", strings.ToLower(search))
	return q.Where(
		"LOWER(mirror.source_url) LIKE ? OR LOWER(mirror.username) LIKE ? OR LOWER(repository.path) LIKE ? OR LOWER(mirror.local_repo_path) LIKE ?",
		pattern, pattern, pattern, pattern,
	)
}

func (s *mirrorStoreImpl) FindByIDs(ctx context.Context, IDs []int64) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("mirror.id in (?)", bun.In(IDs)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Join("JOIN repositories AS r ON mirror.repository_id = r.id ").
		Where("r.repository_type = ? AND LOWER(r.path) = LOWER(?)", repoType, fmt.Sprintf("%s/%s", namespace, name)).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *mirrorStoreImpl) FindWithMapping(ctx context.Context, repoType types.RepositoryType, namespace, name string, mapping types.Mapping) (*Repository, error) {
	resRepo := new(Repository)
	query := s.db.Operator.Core.
		NewSelect().
		Model(resRepo)
	path := fmt.Sprintf("%s/%s", namespace, name)
	query.Where("repository_type = ?", repoType)
	switch mapping {
	case types.HFMapping:
		//compatiebility with old data
		//TODO: remove path after sdk 0.4.6
		query.Where("hf_path = ? or path = ?", path, path)
	case types.ModelScopeMapping:
		query.Where("ms_path = ?", path, path)
	case types.AutoMapping:
		query.Where("hf_path = ? or ms_path = ? or path = ?", path, path, path)
	default:
		// for csg path
		query.Where("path = ?", path)
	}
	err := query.Order("created_at desc").Limit(1).Scan(ctx)
	return resRepo, err
}

func (s *mirrorStoreImpl) Create(ctx context.Context, mirror *Mirror) (*Mirror, error) {
	err := s.db.Operator.Core.NewInsert().
		Model(mirror).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirror, nil
}

func (s *mirrorStoreImpl) NoPushMirror(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Where("push_mirror_created = ?", false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) PushedMirror(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("push_mirror_created = ?", true).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) Update(ctx context.Context, mirror *Mirror) (err error) {
	err = assertAffectedOneRow(s.db.Operator.Core.NewUpdate().
		Model(mirror).
		WherePK().
		Exec(ctx),
	)

	return
}

func (s *mirrorStoreImpl) Delete(ctx context.Context, mirror *Mirror) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err = tx.
			NewDelete().
			Model(mirror).
			WherePK().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.
			NewDelete().
			Model(&MirrorTask{}).
			Where("mirror_id = ?", mirror.ID).
			Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	return nil
}

// DeleteWithTaskCancelTx cancels all workhub jobs for a mirror and deletes its mirror data atomically.
func (s *mirrorStoreImpl) DeleteWithTaskCancelTx(ctx context.Context, mirrorID int64, jobCancelClient MirrorJobCancelClient) (err error) {
	err = s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var mirror Mirror
		if err := tx.NewSelect().
			Model(&mirror).
			Where("id = ?", mirrorID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return err
		}

		var tasks []MirrorTask
		if err := tx.NewSelect().
			Model(&tasks).
			Where("mirror_id = ?", mirror.ID).
			For("UPDATE").
			Scan(ctx); err != nil {
			return err
		}

		for _, task := range tasks {
			if err := cancelMirrorTaskJobsTx(ctx, tx.Tx, task, jobCancelClient); err != nil {
				return err
			}
		}

		if mirror.RepositoryID != 0 && shouldCancelRepoSyncOnMirrorDelete(mirror, tasks) {
			if err := updateRepoSyncStatus(ctx, tx, mirror.RepositoryID, types.SyncStatusCanceled); err != nil {
				return err
			}
		}

		if _, err := tx.NewDelete().
			Model(&MirrorTask{}).
			Where("mirror_id = ?", mirror.ID).
			Exec(ctx); err != nil {
			return err
		}

		_, err := tx.NewDelete().
			Model(&mirror).
			WherePK().
			Exec(ctx)
		return err
	})
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}

	return nil
}

func shouldCancelRepoSyncOnMirrorDelete(mirror Mirror, tasks []MirrorTask) bool {
	for _, task := range tasks {
		if task.Status != "" && !isMirrorTaskTerminalStatus(task.Status) {
			return true
		}
	}
	return mirror.Status != "" && !isMirrorTaskTerminalStatus(mirror.Status)
}

func (s *mirrorStoreImpl) Unfinished(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("status != ? OR status IS NULL", types.MirrorLfsSyncFinished).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) Finished(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("status = ?", types.MirrorLfsSyncFinished).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) ToSyncRepo(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where(
			"next_execution_timestamp < ? or status in (?,?,?)",
			time.Now(),
			types.MirrorLfsSyncFailed,
			types.MirrorRepoSyncFailed,
			types.MirrorQueued,
		).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) ToSyncLfs(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("next_execution_timestamp < ? or status = ?", time.Now(), types.MirrorRepoSyncFinished).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) IndexWithPagination(ctx context.Context, per, page int, filter types.MirrorFilter, hasRepo bool) (mirrors []Mirror, count int, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Relation("CurrentTask").
		Order("id desc")

	if hasRepo {
		q = q.Where("repository.id is not null")
	}
	if filter.Search != "" {
		q = q.Where("LOWER(repository.path) like ? or LOWER(mirror.source_url) like ? or LOWER(mirror.local_repo_path) like ?",
			fmt.Sprintf("%%%s%%", strings.ToLower(filter.Search)),
			fmt.Sprintf("%%%s%%", strings.ToLower(filter.Search)),
			fmt.Sprintf("%%%s%%", strings.ToLower(filter.Search)),
		)
	}
	if filter.Status != nil {
		q = q.Where("current_task.status = ? or (current_task.status IS NULL and mirror.status = ?)", *filter.Status, *filter.Status)
	}
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

func (s *mirrorStoreImpl) StatusCount(ctx context.Context) ([]MirrorStatusCount, error) {
	var statusCounts []MirrorStatusCount
	err := s.db.Operator.Core.NewSelect().
		Model((*Mirror)(nil)).
		Column("status").
		ColumnExpr("COUNT(*) AS count").
		Group("status").
		Scan(ctx, &statusCounts)
	return statusCounts, err
}

func (s *mirrorStoreImpl) UpdateMirrorAndRepository(ctx context.Context, mirror *Mirror, repo *Repository) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().Model(mirror).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update mirror: %v", err)
		}
		_, err = tx.NewUpdate().Model(repo).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update repository: %v", err)
		}
		return nil
	})
	return err
}

func (s *mirrorStoreImpl) FindBySourceURLs(ctx context.Context, sourceURLs []string) ([]Mirror, error) {
	var mirrors []Mirror
	_, err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Where("source_url in (?)", bun.In(sourceURLs)).
		Exec(ctx, &mirrors)
	return mirrors, err
}

func (s *mirrorStoreImpl) BatchUpdate(ctx context.Context, mirrors []Mirror) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&mirrors).
		Column("remote_updated_at", "mirror_priority", "username", "access_token").
		Bulk().
		Exec(ctx)
	return err
}

func (s *mirrorStoreImpl) BatchCreate(ctx context.Context, mirrors []Mirror) error {
	_, err := s.db.Operator.Core.NewInsert().
		Model(&mirrors).
		Exec(ctx)
	return err
}

func (s *mirrorStoreImpl) ToBeScheduled(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("MirrorTasks").
		Where(
			"mirror.remote_updated_at > mirror.updated_at or (mirror.repository_id = 0 and mirror.status = ?)",
			types.MirrorQueued,
		).
		Distinct().
		Order("mirror_priority asc").
		Limit(100).
		Scan(ctx)
	return mirrors, err
}
