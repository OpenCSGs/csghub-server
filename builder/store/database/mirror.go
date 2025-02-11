package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/uptrace/bun"
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
	FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error)
	FindWithMapping(ctx context.Context, repoType types.RepositoryType, namespace, name string, mapping types.Mapping) (*Repository, error)
	Create(ctx context.Context, mirror *Mirror) (*Mirror, error)
	WithPagination(ctx context.Context) ([]Mirror, error)
	WithPaginationWithRepository(ctx context.Context) ([]Mirror, error)
	NoPushMirror(ctx context.Context) ([]Mirror, error)
	PushedMirror(ctx context.Context) ([]Mirror, error)
	Update(ctx context.Context, mirror *Mirror) (err error)
	Delete(ctx context.Context, mirror *Mirror) (err error)
	Unfinished(ctx context.Context) ([]Mirror, error)
	Finished(ctx context.Context) ([]Mirror, error)
	ToSyncRepo(ctx context.Context) ([]Mirror, error)
	ToSyncLfs(ctx context.Context) ([]Mirror, error)
	IndexWithPagination(ctx context.Context, per, page int, search string) (mirrors []Mirror, count int, err error)
	StatusCount(ctx context.Context) ([]MirrorStatusCount, error)
	UpdateMirrorAndRepository(ctx context.Context, mirror *Mirror, repo *Repository) error
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
	PushMirrorCreated      bool                   `bun:",nullzero,default:false" json:"push_mirror_created"`
	Status                 types.MirrorTaskStatus `bun:",nullzero" json:"status"`
	Progress               int8                   `bun:",nullzero" json:"progress"`
	NextExecutionTimestamp time.Time              `bun:",nullzero" json:"next_execution_timestamp"`
	Priority               types.MirrorPriority   `bun:"mirror_priority,notnull,default:0" json:"priority"`

	times
}

type MirrorStatusCount struct {
	Status types.MirrorTaskStatus `bun:"status"`
	Count  int                    `bun:"count"`
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

func (s *mirrorStoreImpl) FindByRepoPath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (*Mirror, error) {
	var mirror Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirror).
		Join("JOIN repositories AS r ON mirror.repository_id = r.id ").
		Where("LOWER(r.git_path) = LOWER(?)", fmt.Sprintf("%ss_%s/%s", repoType, namespace, name)).
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
	if mapping == types.HFMapping {
		//compatiebility with old data
		//TODO: remove path after sdk 0.4.6
		query.Where("hf_path = ? or path = ?", path, path)
	} else if mapping == types.ModelScopeMapping {
		query.Where("ms_path = ?", path, path)
	} else {
		// for csg path
		query.Where("path = ?", path)
	}
	err := query.Limit(1).Scan(ctx)
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

func (s *mirrorStoreImpl) WithPagination(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) WithPaginationWithRepository(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
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
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(mirror).
		WherePK().
		Exec(ctx)
	return
}

func (s *mirrorStoreImpl) Unfinished(ctx context.Context) ([]Mirror, error) {
	var mirrors []Mirror
	err := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Where("status != ? OR status IS NULL", types.MirrorFinished).
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
		Where("status = ?", types.MirrorFinished).
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
		Where(
			"next_execution_timestamp < ? or status in (?,?,?,?)",
			time.Now(),
			types.MirrorIncomplete,
			types.MirrorFailed,
			types.MirrorWaiting,
			types.MirrorRunning).
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
		Where("next_execution_timestamp < ? or status = ?", time.Now(), types.MirrorRepoSynced).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return mirrors, nil
}

func (s *mirrorStoreImpl) IndexWithPagination(ctx context.Context, per, page int, search string) (mirrors []Mirror, count int, err error) {
	q := s.db.Operator.Core.NewSelect().
		Model(&mirrors).
		Relation("Repository").
		Relation("MirrorSource")
	if search != "" {
		q = q.Where("LOWER(mirror.source_url) like ? or LOWER(mirror.local_repo_path) like ?",
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
