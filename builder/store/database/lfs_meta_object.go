package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/common/types"
)

type XnetDispatchStatus string

const (
	XnetDispatchPending     XnetDispatchStatus = "pending"
	XnetDispatchDispatched  XnetDispatchStatus = "dispatched"
	XnetDispatchCompleted   XnetDispatchStatus = "completed"
	XnetDispatchFailed      XnetDispatchStatus = "failed"
	XnetDispatchPaused      XnetDispatchStatus = "paused"
	XnetDispatchGlobalPause XnetDispatchStatus = "global_paused"
)

type XnetDispatchClaim struct {
	// Now is used only by the SQLite fallback. PostgreSQL always uses its own clock.
	Now           time.Time
	LeaseDuration time.Duration
	Limit         int
	MaxInFlight   int
	MaxAttempts   int
	RepositoryID  *int64
}

type XnetDispatchStatsQuery struct {
	// Now is used only by the SQLite fallback. PostgreSQL always uses its own clock.
	Now           time.Time
	LeaseDuration time.Duration
	MaxAttempts   int
	RepositoryID  *int64
}

type XnetDispatchExpiry struct {
	// Now is used only by the SQLite fallback. PostgreSQL always uses its own clock.
	Now           time.Time
	LeaseDuration time.Duration
	MaxAttempts   int
	Limit         int
}

type XnetDispatchResume struct {
	// Now is used only by the SQLite fallback. PostgreSQL always uses its own clock.
	Now           time.Time
	LeaseDuration time.Duration
	MaxAttempts   int
	RepositoryID  int64
}

type XnetDispatchStats struct {
	Active                int
	Retryable             int
	Exhausted             int
	Completed             int
	ExhaustedByRepository map[int64]int
}

type lfsMetaObjectStoreImpl struct {
	db *DB
}

type XnetDispatchFence interface {
	WithXnetDispatchPublishFence(ctx context.Context, claimed LfsMetaObject, publish func() error) (bool, error)
	WithXnetDispatchStopFence(ctx context.Context, repoID *int64, stop func() error) error
}

type xnetDispatchFenceImpl struct {
	db *DB
}

type XnetPublishRateLimiter interface {
	AcquireXnetPublishSlot(ctx context.Context, interval time.Duration) (wait time.Duration, acquired bool, err error)
}

type xnetPublishRateLimiterImpl struct {
	db *DB
}

type LfsMetaObjectStore interface {
	FindByOID(ctx context.Context, RepoId int64, Oid string) (*LfsMetaObject, error)
	FindByRepoID(ctx context.Context, repoID int64) ([]LfsMetaObject, error)
	Create(ctx context.Context, lfsObj LfsMetaObject) (*LfsMetaObject, error)
	RemoveByOid(ctx context.Context, oid string, repoID int64) error
	UpdateOrCreate(ctx context.Context, input LfsMetaObject) (*LfsMetaObject, error)
	// BulkUpdateOrCreate replaces repository LFS metadata and updates its total size atomically.
	BulkUpdateOrCreate(ctx context.Context, repoID int64, input []LfsMetaObject) error
	// BulkUpdateExistingByOIDs persists checked LFS object existence states without replacing metadata rows.
	BulkUpdateExistingByOIDs(ctx context.Context, repoID int64, existingOIDs, missingOIDs []string) error
	UpdateXnetUsed(ctx context.Context, repoID int64, oid string, xnetUsed bool) error
	ClaimXnetDispatchObjects(ctx context.Context, claim XnetDispatchClaim) ([]LfsMetaObject, error)
	ExpireExhaustedXnetDispatches(ctx context.Context, expiry XnetDispatchExpiry) ([]int64, error)
	CompleteXnetDispatch(ctx context.Context, repoID int64, oid string) error
	FailXnetDispatch(ctx context.Context, claimed LfsMetaObject) (bool, error)
	ResumeXnetDispatch(ctx context.Context, resume XnetDispatchResume) error
	ResumeAllXnetDispatch(ctx context.Context, resume XnetDispatchResume) error
	GetXnetDispatchStats(ctx context.Context, query XnetDispatchStatsQuery) (XnetDispatchStats, error)
	GetXnetDispatchState(ctx context.Context, query types.XnetDispatchStateQuery) (types.XnetDispatchState, error)
	CheckIfAllMigratedToXnet(ctx context.Context, repoID int64) (bool, error)
	ExistsByOidExclRepo(ctx context.Context, oid string, repoID int64) (bool, error)
	ExistsByOidInActiveRepoExclRepo(ctx context.Context, oid string, repoID int64) (bool, error)
}

func NewLfsMetaObjectStore() LfsMetaObjectStore {
	return &lfsMetaObjectStoreImpl{
		db: defaultDB,
	}
}

func NewLfsMetaObjectStoreWithDB(db *DB) LfsMetaObjectStore {
	return &lfsMetaObjectStoreImpl{
		db: db,
	}
}

func NewXnetDispatchFence() XnetDispatchFence {
	return &xnetDispatchFenceImpl{db: defaultDB}
}

func NewXnetDispatchFenceWithDB(db *DB) XnetDispatchFence {
	return &xnetDispatchFenceImpl{db: db}
}

func NewXnetPublishRateLimiter() XnetPublishRateLimiter {
	return &xnetPublishRateLimiterImpl{db: defaultDB}
}

func NewXnetPublishRateLimiterWithDB(db *DB) XnetPublishRateLimiter {
	return &xnetPublishRateLimiterImpl{db: db}
}

type LfsMetaObject struct {
	ID                   int64              `bun:",pk,autoincrement" json:"user_id"`
	Oid                  string             `bun:",notnull" json:"oid"`
	Size                 int64              `bun:",notnull" json:"size"`
	RepositoryID         int64              `bun:",notnull" json:"repository_id"`
	Repository           Repository         `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	Existing             bool               `bun:",notnull" json:"existing"`
	XnetUsed             bool               `bun:",notnull" json:"xnet_used"`
	XnetDispatchStatus   XnetDispatchStatus `bun:",notnull,default:'pending'" json:"xnet_dispatch_status"`
	XnetDispatchedAt     *time.Time         `bun:",nullzero" json:"xnet_dispatched_at,omitempty"`
	XnetNextAttemptAt    *time.Time         `bun:",nullzero" json:"xnet_next_attempt_at,omitempty"`
	XnetDispatchAttempts int                `bun:",notnull,default:0" json:"xnet_dispatch_attempts"`
	times
}

func (s *lfsMetaObjectStoreImpl) FindByOID(ctx context.Context, RepoId int64, Oid string) (*LfsMetaObject, error) {
	var lfsMetaObject LfsMetaObject
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsMetaObject).
		Where("oid=? and repository_id=?", Oid, RepoId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsMetaObject, nil
}

func (s *lfsMetaObjectStoreImpl) FindByRepoID(ctx context.Context, repoID int64) ([]LfsMetaObject, error) {
	var lfsMetaObjects []LfsMetaObject
	err := s.db.Operator.Core.NewSelect().
		Model(&lfsMetaObjects).
		Where("repository_id=?", repoID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return lfsMetaObjects, nil
}

func (s *lfsMetaObjectStoreImpl) Create(ctx context.Context, lfsObj LfsMetaObject) (*LfsMetaObject, error) {
	initializeXnetDispatchState(&lfsObj)
	err := s.db.Operator.Core.NewInsert().
		Model(&lfsObj).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &lfsObj, nil
}

func (s *lfsMetaObjectStoreImpl) RemoveByOid(ctx context.Context, oid string, repoID int64) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model(&LfsMetaObject{}).
		Where("oid = ? and repository_id= ?", oid, repoID).
		Exec(ctx)

	return err
}

func (s *lfsMetaObjectStoreImpl) UpdateOrCreate(ctx context.Context, input LfsMetaObject) (*LfsMetaObject, error) {
	input.UpdatedAt = time.Now()
	err := s.db.Core.NewUpdate().
		Model((*LfsMetaObject)(nil)).
		Set("size = ?", input.Size).
		Set("existing = ?", input.Existing).
		Set("updated_at = ?", input.UpdatedAt).
		Where("oid = ? and repository_id = ?", input.Oid, input.RepositoryID).
		Returning("*").
		Scan(ctx, &input)
	if err == nil {
		return &input, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("update lfs meta object in tx failed,error:%w", err)
	}

	initializeXnetDispatchState(&input)
	res, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err := assertAffectedOneRow(res, err); err != nil {
		return nil, fmt.Errorf("create lfs meta object in tx failed,error:%w", err)
	}

	return &input, nil
}

// BulkUpdateOrCreate replaces repository LFS metadata and updates its total size atomically.
func (s *lfsMetaObjectStoreImpl) BulkUpdateOrCreate(ctx context.Context, repoID int64, input []LfsMetaObject) error {
	var totalSize int64
	for i := range input {
		totalSize += input[i].Size
	}

	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var current []LfsMetaObject
		query := tx.NewSelect().
			Model(&current).
			Column("oid", "xnet_used", "xnet_dispatch_status", "xnet_dispatched_at", "xnet_next_attempt_at", "xnet_dispatch_attempts").
			Where("repository_id = ?", repoID)
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE")
		}
		if err := query.Scan(ctx); err != nil {
			return err
		}
		currentByOID := make(map[string]LfsMetaObject, len(current))
		for i := range current {
			currentByOID[current[i].Oid] = current[i]
		}
		for i := range input {
			if existing, ok := currentByOID[input[i].Oid]; ok {
				copyXnetDispatchState(&input[i], &existing)
			} else {
				initializeXnetDispatchState(&input[i])
			}
		}

		// Delete all existing records
		_, err := tx.NewDelete().
			Model(&LfsMetaObject{}).
			Where("repository_id = ?", repoID).
			Exec(ctx)
		if err != nil {
			return err
		}

		if len(input) > 0 {
			// Insert new records
			if _, err = tx.NewInsert().
				Model(&input).
				Exec(ctx); err != nil {
				return err
			}
		}

		_, err = tx.NewUpdate().
			Model((*Repository)(nil)).
			Set("lfs_objects_size = ?", totalSize).
			Where("id = ?", repoID).
			Exec(ctx)
		return err
	})
}

func copyXnetDispatchState(destination, source *LfsMetaObject) {
	destination.XnetUsed = source.XnetUsed
	destination.XnetDispatchStatus = source.XnetDispatchStatus
	destination.XnetDispatchedAt = source.XnetDispatchedAt
	destination.XnetNextAttemptAt = source.XnetNextAttemptAt
	destination.XnetDispatchAttempts = source.XnetDispatchAttempts
}

func initializeXnetDispatchState(object *LfsMetaObject) {
	if object.XnetDispatchStatus != "" {
		return
	}
	if object.XnetUsed {
		object.XnetDispatchStatus = XnetDispatchCompleted
		return
	}
	object.XnetDispatchStatus = XnetDispatchPending
}

// BulkUpdateExistingByOIDs updates checked LFS existence states in one transaction.
func (s *lfsMetaObjectStoreImpl) BulkUpdateExistingByOIDs(ctx context.Context, repoID int64, existingOIDs, missingOIDs []string) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if len(existingOIDs) > 0 {
			_, err := tx.NewUpdate().
				Model((*LfsMetaObject)(nil)).
				Set("existing = ?", true).
				Set("updated_at = ?", time.Now()).
				Where("repository_id = ?", repoID).
				Where("oid IN (?)", bun.In(existingOIDs)).
				Exec(ctx)
			if err != nil {
				return err
			}
		}

		if len(missingOIDs) > 0 {
			_, err := tx.NewUpdate().
				Model((*LfsMetaObject)(nil)).
				Set("existing = ?", false).
				Set("updated_at = ?", time.Now()).
				Where("repository_id = ?", repoID).
				Where("oid IN (?)", bun.In(missingOIDs)).
				Exec(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *lfsMetaObjectStoreImpl) UpdateXnetUsed(ctx context.Context, repoID int64, oid string, xnetUsed bool) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model(&LfsMetaObject{}).
		Set("xnet_used = ?", xnetUsed).
		Set("updated_at = ?", time.Now()).
		Where("repository_id = ? AND oid = ?", repoID, oid).
		Exec(ctx)
	return err
}

func (s *lfsMetaObjectStoreImpl) ClaimXnetDispatchObjects(ctx context.Context, claim XnetDispatchClaim) ([]LfsMetaObject, error) {
	if claim.Limit <= 0 || claim.MaxAttempts <= 0 || claim.LeaseDuration <= 0 {
		return nil, fmt.Errorf("invalid xnet dispatch claim: limit, max attempts, and lease duration must be positive")
	}
	var claimed []LfsMetaObject
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockXnetDispatch(ctx, tx, claim.RepositoryID, false); err != nil {
			return err
		}
		limit := claim.Limit
		if claim.MaxInFlight > 0 {
			if err := lockXnetDispatchClaimScheduler(ctx, tx); err != nil {
				return err
			}
		}
		now := claim.Now
		if now.IsZero() {
			now = time.Now()
		}
		if tx.Dialect().Name() == dialect.PG {
			if err := tx.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
				return fmt.Errorf("get database clock for xnet dispatch claim: %w", err)
			}
		}
		if claim.MaxInFlight > 0 {
			active, err := tx.NewSelect().
				Model((*LfsMetaObject)(nil)).
				Where("xnet_used = false").
				Where("xnet_dispatch_status = ?", XnetDispatchDispatched).
				Where("xnet_dispatched_at >= ?", now.Add(-claim.LeaseDuration)).
				Count(ctx)
			if err != nil {
				return err
			}
			available := claim.MaxInFlight - active
			if available <= 0 {
				return nil
			}
			if limit > available {
				limit = available
			}
		}
		query := tx.NewSelect().
			Model(&claimed).
			Join("JOIN repositories AS repository ON repository.id = lfs_meta_object.repository_id").
			Where("repository.deleted_at IS NULL").
			Where("repository.xnet_migration_intent = ?", XnetMigrationActive).
			Where("xnet_used = false").
			Where("xnet_dispatch_attempts < ?", claim.MaxAttempts).
			Where("xnet_dispatch_status = ? OR (xnet_dispatch_status = ? AND (xnet_next_attempt_at IS NULL OR xnet_next_attempt_at <= ?)) OR (xnet_dispatch_status = ? AND xnet_dispatched_at < ?)",
				XnetDispatchPending,
				XnetDispatchFailed,
				now,
				XnetDispatchDispatched,
				now.Add(-claim.LeaseDuration)).
			Order("lfs_meta_object.id ASC").
			Limit(limit)
		if claim.RepositoryID != nil {
			query = query.Where("repository_id = ?", *claim.RepositoryID)
		}
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE OF lfs_meta_object SKIP LOCKED")
		}
		if err := query.Scan(ctx); err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}

		ids := make([]int64, 0, len(claimed))
		for i := range claimed {
			ids = append(ids, claimed[i].ID)
		}
		if _, err := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatch_status = ?", XnetDispatchDispatched).
			Set("xnet_dispatched_at = ?", now).
			Set("xnet_next_attempt_at = NULL").
			Set("xnet_dispatch_attempts = xnet_dispatch_attempts + 1").
			Where("id IN (?)", bun.In(ids)).
			Exec(ctx); err != nil {
			return err
		}
		claimed = nil
		if err := tx.NewSelect().
			Model(&claimed).
			Relation("Repository").
			Relation("Repository.CurrentXnetMigrationTask").
			Where("lfs_meta_object.id IN (?)", bun.In(ids)).
			Order("lfs_meta_object.id ASC").
			Scan(ctx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("claim xnet dispatch objects: %w", err)
	}
	return claimed, nil
}

func (s *lfsMetaObjectStoreImpl) ExpireExhaustedXnetDispatches(
	ctx context.Context, expiry XnetDispatchExpiry,
) ([]int64, error) {
	if expiry.LeaseDuration <= 0 || expiry.MaxAttempts <= 0 || expiry.Limit <= 0 {
		return nil, fmt.Errorf("lease duration, max attempts, and limit must be positive")
	}
	repositories := make(map[int64]struct{})
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockXnetDispatch(ctx, tx, nil, false); err != nil {
			return err
		}
		now := expiry.Now
		if now.IsZero() {
			now = time.Now()
		}
		if tx.Dialect().Name() == dialect.PG {
			if err := tx.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
				return fmt.Errorf("get database clock for xnet dispatch expiry: %w", err)
			}
		}
		var expired []LfsMetaObject
		query := tx.NewSelect().
			Model(&expired).
			Column("id", "repository_id").
			Where("xnet_used = false").
			Where("xnet_dispatch_status = ?", XnetDispatchDispatched).
			Where("xnet_dispatch_attempts >= ?", expiry.MaxAttempts).
			Where("xnet_dispatched_at < ?", now.Add(-expiry.LeaseDuration)).
			Order("id ASC").
			Limit(expiry.Limit)
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE SKIP LOCKED")
		}
		if err := query.Scan(ctx); err != nil {
			return err
		}
		if len(expired) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(expired))
		for _, object := range expired {
			ids = append(ids, object.ID)
			repositories[object.RepositoryID] = struct{}{}
		}
		_, err := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatch_status = ?", XnetDispatchFailed).
			Set("xnet_dispatched_at = NULL").
			Set("xnet_next_attempt_at = NULL").
			Set("updated_at = ?", now).
			Where("id IN (?)", bun.In(ids)).
			Exec(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("expire exhausted xnet dispatches: %w", err)
	}
	repositoryIDs := make([]int64, 0, len(repositories))
	for repositoryID := range repositories {
		repositoryIDs = append(repositoryIDs, repositoryID)
	}
	slices.Sort(repositoryIDs)
	return repositoryIDs, nil
}

func isXnetDispatchClaimActive(current, claimed LfsMetaObject) bool {
	return current.ID == claimed.ID &&
		!current.XnetUsed &&
		current.XnetDispatchStatus == XnetDispatchDispatched &&
		current.XnetDispatchedAt != nil &&
		claimed.XnetDispatchedAt != nil &&
		current.XnetDispatchedAt.Equal(*claimed.XnetDispatchedAt) &&
		current.XnetDispatchAttempts == claimed.XnetDispatchAttempts
}

func (l *xnetPublishRateLimiterImpl) AcquireXnetPublishSlot(
	ctx context.Context, interval time.Duration,
) (wait time.Duration, acquired bool, err error) {
	if interval <= 0 {
		return 0, false, fmt.Errorf("xnet dispatch publish interval must be positive")
	}

	err = l.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`
			INSERT INTO xnet_dispatch_rate_limits (name, next_allowed_at)
			VALUES ('publish', clock_timestamp())
			ON CONFLICT (name) DO NOTHING
		`).Exec(ctx); err != nil {
			return err
		}
		var state struct {
			WaitMicroseconds int64 `bun:"wait_microseconds"`
			Acquired         bool  `bun:"acquired"`
		}
		if err := tx.NewRaw(`
			WITH db_clock AS MATERIALIZED (SELECT clock_timestamp() AS now)
			SELECT
				next_allowed_at <= db_clock.now AS acquired,
				GREATEST(
					CEIL(EXTRACT(EPOCH FROM (next_allowed_at - db_clock.now)) * 1000000),
					0
				)::BIGINT AS wait_microseconds
			FROM xnet_dispatch_rate_limits CROSS JOIN db_clock
			WHERE name = 'publish'
			FOR UPDATE OF xnet_dispatch_rate_limits
		`).Scan(ctx, &state); err != nil {
			return err
		}
		if !state.Acquired {
			wait = time.Duration(state.WaitMicroseconds) * time.Microsecond
			return nil
		}
		if _, err := tx.NewRaw(`
			UPDATE xnet_dispatch_rate_limits
			SET next_allowed_at = clock_timestamp() + CAST(? AS INTERVAL)
			WHERE name = 'publish'
		`, interval.String()).Exec(ctx); err != nil {
			return err
		}
		acquired = true
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("acquire xnet dispatch publish slot: %w", err)
	}
	return wait, acquired, nil
}

func (f *xnetDispatchFenceImpl) WithXnetDispatchPublishFence(
	ctx context.Context, claimed LfsMetaObject, publish func() error,
) (published bool, err error) {
	err = f.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockXnetDispatch(ctx, tx, &claimed.RepositoryID, false); err != nil {
			return err
		}
		var current LfsMetaObject
		query := tx.NewSelect().Model(&current).Where("id = ?", claimed.ID)
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE")
		}
		if err := query.Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if !isXnetDispatchClaimActive(current, claimed) {
			return nil
		}
		if err := publish(); err != nil {
			return err
		}
		now := time.Now()
		if tx.Dialect().Name() == dialect.PG {
			if err := tx.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
				return fmt.Errorf("get database clock after xnet publish: %w", err)
			}
		}
		if _, err := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatched_at = ?", now).
			Set("updated_at = ?", now).
			Where("id = ?", current.ID).
			Exec(ctx); err != nil {
			return err
		}
		published = true
		return nil
	})
	return published, err
}

func (f *xnetDispatchFenceImpl) WithXnetDispatchStopFence(
	ctx context.Context, repoID *int64, stop func() error,
) error {
	return f.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockXnetDispatch(ctx, tx, repoID, true); err != nil {
			return err
		}
		if err := setXnetMigrationStopIntent(ctx, tx, repoID); err != nil {
			return err
		}
		status := XnetDispatchGlobalPause
		if repoID != nil {
			status = XnetDispatchPaused
		}
		query := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatch_status = ?", status).
			Set("xnet_dispatched_at = NULL").
			Set("xnet_next_attempt_at = NULL").
			Set("updated_at = ?", time.Now()).
			Where("xnet_used = false")
		if repoID != nil {
			query = query.
				Where("repository_id = ?", *repoID).
				Where("xnet_dispatch_status NOT IN (?)", bun.In(xnetDispatchStatusesPreservedByPause(status)))
		} else {
			query = query.Where("xnet_dispatch_status = ?", XnetDispatchDispatched)
		}
		if _, err := query.Exec(ctx); err != nil {
			return err
		}
		return stop()
	})
}

func lockXnetDispatch(ctx context.Context, tx bun.Tx, repoID *int64, exclusive bool) error {
	if tx.Dialect().Name() != dialect.PG {
		return nil
	}
	lock := "pg_advisory_xact_lock_shared"
	if exclusive && repoID == nil {
		lock = "pg_advisory_xact_lock"
	}
	if _, err := tx.ExecContext(ctx, "SELECT "+lock+"(hashtextextended(?, 0))", "xnet_dispatch:global"); err != nil {
		return err
	}
	if repoID == nil {
		return nil
	}
	repoLock := "pg_advisory_xact_lock_shared"
	if exclusive {
		repoLock = "pg_advisory_xact_lock"
	}
	_, err := tx.ExecContext(ctx, "SELECT "+repoLock+"(hashtextextended(?, 0))", fmt.Sprintf("xnet_dispatch:repo:%d", *repoID))
	return err
}

func lockXnetDispatchClaimScheduler(ctx context.Context, tx bun.Tx) error {
	if tx.Dialect().Name() != dialect.PG {
		return nil
	}
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "xnet_dispatch:claim_scheduler")
	return err
}

func (s *lfsMetaObjectStoreImpl) CompleteXnetDispatch(ctx context.Context, repoID int64, oid string) error {
	_, err := s.db.Operator.Core.NewUpdate().
		Model((*LfsMetaObject)(nil)).
		Set("xnet_used = true").
		Set("xnet_dispatch_status = ?", XnetDispatchCompleted).
		Set("xnet_dispatched_at = NULL").
		Set("xnet_next_attempt_at = NULL").
		Set("updated_at = ?", time.Now()).
		Where("repository_id = ? AND oid = ?", repoID, oid).
		Exec(ctx)
	return err
}

func (s *lfsMetaObjectStoreImpl) FailXnetDispatch(ctx context.Context, claimed LfsMetaObject) (bool, error) {
	applied := false
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var object LfsMetaObject
		query := tx.NewSelect().
			Model(&object).
			Where("id = ?", claimed.ID)
		if tx.Dialect().Name() == dialect.PG {
			query = query.For("UPDATE")
		}
		if err := query.Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if !isXnetDispatchClaimActive(object, claimed) {
			return nil
		}

		now := time.Now()
		if tx.Dialect().Name() == dialect.PG {
			if err := tx.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
				return fmt.Errorf("get database clock for xnet dispatch failure: %w", err)
			}
		}
		_, err := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatch_status = ?", XnetDispatchFailed).
			Set("xnet_dispatched_at = NULL").
			Set("xnet_next_attempt_at = ?", now.Add(xnetDispatchRetryDelay(object.XnetDispatchAttempts))).
			Set("updated_at = ?", now).
			Where("id = ?", claimed.ID).
			Exec(ctx)
		applied = err == nil
		return err
	})
	return applied, err
}

func (s *lfsMetaObjectStoreImpl) ResumeXnetDispatch(ctx context.Context, resume XnetDispatchResume) error {
	return s.resumeXnetDispatch(ctx, &resume.RepositoryID, resume, append(xnetDispatchPausedStatuses(), XnetDispatchFailed))
}

func (s *lfsMetaObjectStoreImpl) ResumeAllXnetDispatch(ctx context.Context, resume XnetDispatchResume) error {
	return s.resumeXnetDispatch(ctx, nil, resume, []XnetDispatchStatus{XnetDispatchGlobalPause, XnetDispatchFailed})
}

func setXnetMigrationStopIntent(ctx context.Context, tx bun.Tx, repoID *int64) error {
	query := tx.NewUpdate().Model((*Repository)(nil))
	if repoID != nil {
		query = query.Set("xnet_migration_intent = ?", XnetMigrationPaused).Where("id = ?", *repoID)
	} else {
		query = query.Set("xnet_migration_intent = ?", XnetMigrationInactive).
			Where("xnet_migration_intent = ?", XnetMigrationActive)
	}
	_, err := query.Exec(ctx)
	return err
}

func (s *lfsMetaObjectStoreImpl) resumeXnetDispatch(
	ctx context.Context, repoID *int64, resume XnetDispatchResume, statuses []XnetDispatchStatus,
) error {
	if resume.LeaseDuration <= 0 || resume.MaxAttempts <= 0 {
		return fmt.Errorf("lease duration and max attempts must be positive")
	}
	return s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockXnetDispatch(ctx, tx, repoID, true); err != nil {
			return err
		}
		now := resume.Now
		if now.IsZero() {
			now = time.Now()
		}
		if tx.Dialect().Name() == dialect.PG {
			if err := tx.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
				return fmt.Errorf("get database clock for xnet dispatch resume: %w", err)
			}
		}
		intentQuery := tx.NewUpdate().Model((*Repository)(nil)).Set("xnet_migration_intent = ?", XnetMigrationActive)
		if repoID != nil {
			intentQuery = intentQuery.Where("id = ?", *repoID)
		} else {
			intentQuery = intentQuery.Where("xnet_migration_intent != ?", XnetMigrationPaused)
		}
		if _, err := intentQuery.Exec(ctx); err != nil {
			return err
		}

		query := tx.NewUpdate().
			Model((*LfsMetaObject)(nil)).
			Set("xnet_dispatch_status = ?", XnetDispatchPending).
			Set("xnet_dispatched_at = NULL").
			Set("xnet_next_attempt_at = NULL").
			Set("xnet_dispatch_attempts = CASE WHEN xnet_dispatch_status = ? OR xnet_dispatch_attempts >= ? THEN 0 ELSE xnet_dispatch_attempts END",
				XnetDispatchFailed, resume.MaxAttempts).
			Set("updated_at = ?", now).
			Where("xnet_used = false").
			Where("xnet_dispatch_status IN (?) OR (xnet_dispatch_status = ? AND xnet_dispatch_attempts >= ? AND xnet_dispatched_at < ?)",
				bun.In(statuses), XnetDispatchDispatched, resume.MaxAttempts, now.Add(-resume.LeaseDuration))
		if repoID != nil {
			query = query.Where("repository_id = ?", *repoID)
		} else {
			query = query.Where("repository_id IN (?)", tx.NewSelect().Model((*Repository)(nil)).Column("id").Where("xnet_migration_intent = ?", XnetMigrationActive))
		}
		_, err := query.Exec(ctx)
		return err
	})
}

func xnetDispatchPausedStatuses() []XnetDispatchStatus {
	return []XnetDispatchStatus{XnetDispatchPaused, XnetDispatchGlobalPause}
}

func xnetDispatchStatusesPreservedByPause(status XnetDispatchStatus) []XnetDispatchStatus {
	if status == XnetDispatchPaused {
		return []XnetDispatchStatus{XnetDispatchPaused}
	}
	return xnetDispatchPausedStatuses()
}

func xnetDispatchRetryDelay(attempts int) time.Duration {
	const (
		minimumDelay = time.Minute
		maximumDelay = 30 * time.Minute
	)
	if attempts < 1 {
		attempts = 1
	}
	if attempts >= 6 {
		return maximumDelay
	}
	delay := minimumDelay << (attempts - 1)
	if delay > maximumDelay {
		return maximumDelay
	}
	return delay
}

func (s *lfsMetaObjectStoreImpl) GetXnetDispatchStats(
	ctx context.Context, query XnetDispatchStatsQuery,
) (XnetDispatchStats, error) {
	if query.LeaseDuration <= 0 || query.MaxAttempts <= 0 {
		return XnetDispatchStats{}, fmt.Errorf("lease duration and max attempts must be positive")
	}
	now := query.Now
	if now.IsZero() {
		now = time.Now()
	}
	if s.db.Operator.Core.Dialect().Name() == dialect.PG {
		if err := s.db.Operator.Core.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
			return XnetDispatchStats{}, fmt.Errorf("get database clock for xnet dispatch stats: %w", err)
		}
	}
	cutoff := now.Add(-query.LeaseDuration)
	count := func(query *bun.SelectQuery) (int, error) { return query.Count(ctx) }
	base := func() *bun.SelectQuery {
		baseQuery := s.db.Operator.Core.NewSelect().Model((*LfsMetaObject)(nil))
		if query.RepositoryID != nil {
			baseQuery = baseQuery.Where("repository_id = ?", *query.RepositoryID)
		}
		return baseQuery
	}

	active, err := count(base().
		Where("xnet_used = false").
		Where("xnet_dispatch_status = ?", XnetDispatchDispatched).
		Where("xnet_dispatched_at >= ?", cutoff))
	if err != nil {
		return XnetDispatchStats{}, fmt.Errorf("count active xnet dispatches: %w", err)
	}
	retryable, err := count(base().
		Where("xnet_used = false").
		Where("xnet_dispatch_attempts < ?", query.MaxAttempts).
		Where("xnet_dispatch_status IN (?) OR (xnet_dispatch_status = ? AND xnet_dispatched_at < ?)",
			bun.In([]XnetDispatchStatus{XnetDispatchPending, XnetDispatchFailed}), XnetDispatchDispatched, cutoff))
	if err != nil {
		return XnetDispatchStats{}, fmt.Errorf("count retryable xnet dispatches: %w", err)
	}
	exhaustedQuery := func() *bun.SelectQuery {
		return base().
			Where("xnet_used = false").
			Where("xnet_dispatch_attempts >= ?", query.MaxAttempts).
			Where("xnet_dispatch_status NOT IN (?)", bun.In(xnetDispatchPausedStatuses())).
			Where("xnet_dispatch_status != ? OR xnet_dispatched_at < ?", XnetDispatchDispatched, cutoff)
	}
	var exhausted int
	var exhaustedByRepository map[int64]int
	if query.RepositoryID != nil {
		exhausted, err = count(exhaustedQuery())
	} else {
		var groups []struct {
			RepositoryID int64 `bun:"repository_id"`
			Count        int   `bun:"count"`
		}
		err = exhaustedQuery().
			Column("repository_id").
			ColumnExpr("COUNT(*) AS count").
			Group("repository_id").
			Scan(ctx, &groups)
		if err == nil && len(groups) > 0 {
			exhaustedByRepository = make(map[int64]int, len(groups))
			for _, group := range groups {
				exhausted += group.Count
				exhaustedByRepository[group.RepositoryID] = group.Count
			}
		}
	}
	if err != nil {
		return XnetDispatchStats{}, fmt.Errorf("count exhausted xnet dispatches: %w", err)
	}
	completed, err := count(base().Where("xnet_used = true"))
	if err != nil {
		return XnetDispatchStats{}, fmt.Errorf("count completed xnet dispatches: %w", err)
	}
	return XnetDispatchStats{
		Active: active, Retryable: retryable, Exhausted: exhausted, Completed: completed,
		ExhaustedByRepository: exhaustedByRepository,
	}, nil
}

// GetXnetDispatchState uses short-circuit existence checks for per-result task
// updates; it never aggregates the repository's completed-object history.
func (s *lfsMetaObjectStoreImpl) GetXnetDispatchState(ctx context.Context, query types.XnetDispatchStateQuery) (types.XnetDispatchState, error) {
	state := types.XnetDispatchState{}
	if query.LeaseDuration <= 0 || query.MaxAttempts <= 0 {
		return state, fmt.Errorf("lease duration and max attempts must be positive")
	}
	now := time.Now()
	if s.db.Operator.Core.Dialect().Name() == dialect.PG {
		if err := s.db.Operator.Core.NewRaw("SELECT clock_timestamp()").Scan(ctx, &now); err != nil {
			return state, fmt.Errorf("get database clock for xnet dispatch state: %w", err)
		}
	}
	cutoff := now.Add(-query.LeaseDuration)
	base := func() *bun.SelectQuery {
		return s.db.Operator.Core.NewSelect().Model((*LfsMetaObject)(nil)).
			Where("repository_id = ?", query.RepositoryID).Where("xnet_used = false")
	}
	var err error
	state.Active, err = base().Where("xnet_dispatch_status = ?", XnetDispatchDispatched).
		Where("xnet_dispatched_at >= ?", cutoff).Exists(ctx)
	if err != nil {
		return state, fmt.Errorf("check active xnet dispatches: %w", err)
	}
	if state.Active {
		return state, nil
	}
	state.Retryable, err = base().Where("xnet_dispatch_attempts < ?", query.MaxAttempts).
		Where("xnet_dispatch_status IN (?) OR (xnet_dispatch_status = ? AND xnet_dispatched_at < ?)",
			bun.In([]XnetDispatchStatus{XnetDispatchPending, XnetDispatchFailed}), XnetDispatchDispatched, cutoff).Exists(ctx)
	if err != nil {
		return state, fmt.Errorf("check retryable xnet dispatches: %w", err)
	}
	if state.Retryable {
		return state, nil
	}
	state.Exhausted, err = base().Where("xnet_dispatch_attempts >= ?", query.MaxAttempts).
		Where("xnet_dispatch_status NOT IN (?)", bun.In(xnetDispatchPausedStatuses())).
		Where("xnet_dispatch_status != ? OR xnet_dispatched_at < ?", XnetDispatchDispatched, cutoff).Exists(ctx)
	if err != nil {
		return state, fmt.Errorf("check exhausted xnet dispatches: %w", err)
	}
	return state, nil
}

func (s *lfsMetaObjectStoreImpl) CheckIfAllMigratedToXnet(ctx context.Context, repoID int64) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().
		Model((*LfsMetaObject)(nil)).
		Where("repository_id = ? AND xnet_used = false", repoID).
		Exists(ctx)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

func (s *lfsMetaObjectStoreImpl) ExistsByOidExclRepo(ctx context.Context, oid string, repoID int64) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().
		Model((*LfsMetaObject)(nil)).
		Where("oid = ? AND repository_id != ?", oid, repoID).
		Exists(ctx)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ExistsByOidInActiveRepoExclRepo reports whether an object is still referenced
// by a repository that remains visible. Other repositories already awaiting
// deletion must not keep a shared object alive forever.
func (s *lfsMetaObjectStoreImpl) ExistsByOidInActiveRepoExclRepo(ctx context.Context, oid string, repoID int64) (bool, error) {
	exists, err := s.db.Operator.Core.NewSelect().
		Model((*LfsMetaObject)(nil)).
		Join("JOIN repositories AS repository ON repository.id = lfs_meta_object.repository_id").
		Where("lfs_meta_object.oid = ?", oid).
		Where("lfs_meta_object.repository_id != ?", repoID).
		Where("repository.deleted_at IS NULL").
		Exists(ctx)
	if err != nil {
		return false, err
	}
	return exists, nil
}
