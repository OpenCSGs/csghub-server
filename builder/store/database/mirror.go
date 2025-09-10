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
	FindByID(ctx context.Context, ID int64) (*Mirror, error)
	FindByIDs(ctx context.Context, IDs []int64) ([]Mirror, error)
	FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error)
	FindWithMapping(ctx context.Context, repoType types.RepositoryType, namespace, name string, mapping types.Mapping) (*Repository, error)
	Create(ctx context.Context, mirror *Mirror) (*Mirror, error)
	NoPushMirror(ctx context.Context) ([]Mirror, error)
	PushedMirror(ctx context.Context) ([]Mirror, error)
	Update(ctx context.Context, mirror *Mirror) (err error)
	Unfinished(ctx context.Context) ([]Mirror, error)
	Finished(ctx context.Context) ([]Mirror, error)
	ToSyncRepo(ctx context.Context) ([]Mirror, error)
	ToSyncLfs(ctx context.Context) ([]Mirror, error)
	IndexWithPagination(ctx context.Context, per, page int, search string, hasRepo bool) (mirrors []Mirror, count int, err error)
	StatusCount(ctx context.Context) ([]MirrorStatusCount, error)
	UpdateMirrorAndRepository(ctx context.Context, mirror *Mirror, repo *Repository) error
	FindBySourceURLs(ctx context.Context, sourceURLs []string) ([]Mirror, error)
	BatchUpdate(ctx context.Context, mirrors []Mirror) error
	BatchCreate(ctx context.Context, mirrors []Mirror) error
	ToBeScheduled(ctx context.Context) ([]Mirror, error)
	Delete(ctx context.Context, mirror *Mirror) error
	Recover(ctx context.Context) error
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
	ID             int64        `bun:",pk,autoincrement" json:"id"`
	Interval       string       `bun:",notnull" json:"interval"`
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
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
}

func (s *mirrorStoreImpl) FindByID(ctx context.Context, ID int64) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Relation("Repository").
		Where("mirror.id=?", ID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &mirror, nil
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
		_, err = s.db.Operator.Core.
			NewDelete().
			Model(mirror).
			WherePK().
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = s.db.Operator.Core.
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

func (s *mirrorStoreImpl) IndexWithPagination(ctx context.Context, per, page int, search string, hasRepo bool) (mirrors []Mirror, count int, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Relation("MirrorSource").
		Relation("CurrentTask").
		Order("id desc")

	if hasRepo {
		q = q.Where("repository.id is not null")
	}
	if search != "" {
		q = q.Where("LOWER(repository.path) like ? or LOWER(mirror.source_url) like ? or LOWER(mirror.local_repo_path) like ?",
			fmt.Sprintf("%%%s%%", strings.ToLower(search)),
			fmt.Sprintf("%%%s%%", strings.ToLower(search)),
			fmt.Sprintf("%%%s%%", strings.ToLower(search)),
		)
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
		Column("remote_updated_at", "mirror_priority").
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
			"mirror.remote_updated_at > mirror.updated_at",
		).
		Distinct().
		Order("mirror_priority desc").
		Limit(100).
		Scan(ctx)
	return mirrors, err
}

func (s *mirrorStoreImpl) Recover(ctx context.Context) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*Mirror)(nil)).
		Set("status = ?", types.MirrorQueued).
		Set("updated_at = ?", time.Now()).
		Where("status in (?)", bun.In([]types.MirrorTaskStatus{types.MirrorLfsSyncStart, types.MirrorRepoSyncStart})).
		Exec(ctx)
	return err
}
