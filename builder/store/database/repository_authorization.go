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
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// RepositoryAccessMode identifies the source of a repository visibility scope.
type RepositoryAccessMode string

const (
	// RepositoryAccessPublic exposes public repositories only.
	RepositoryAccessPublic RepositoryAccessMode = "public"
	// RepositoryAccessReadable exposes public repositories and the listed private repositories.
	RepositoryAccessReadable RepositoryAccessMode = "readable"
	// RepositoryAccessAdmin exposes all repositories to an administrator.
	RepositoryAccessAdmin RepositoryAccessMode = "admin"
)

// RepositoryAccessScope carries the repository IDs returned by ReBAC for list queries.
// Public repositories remain visible even when ReadableRepositoryIDs is empty.
type RepositoryAccessScope struct {
	Mode                    RepositoryAccessMode
	ReadableRepositoryIDs   []int64
	ReadableRepositoryIDSet map[int64]struct{}
}

// NewRepositoryAccessScope constructs a normalized readable scope from repository IDs.
func NewRepositoryAccessScope(mode RepositoryAccessMode, ids []int64) RepositoryAccessScope {
	if mode != RepositoryAccessReadable {
		return RepositoryAccessScope{Mode: mode}
	}

	unique := make(map[int64]struct{}, len(ids))
	normalized := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		normalized = append(normalized, id)
	}
	slices.Sort(normalized)
	return RepositoryAccessScope{
		Mode:                    mode,
		ReadableRepositoryIDs:   normalized,
		ReadableRepositoryIDSet: unique,
	}
}

// RepositoryAuthorization stores a direct repository grant for a user or organization.
type RepositoryAuthorization struct {
	ID           int64                     `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64                     `bun:",notnull" json:"repository_id"`
	SubjectType  types.RepoAuthSubjectType `bun:",notnull" json:"subject_type"`
	SubjectID    int64                     `bun:",notnull" json:"subject_id"`
	SubjectUUID  string                    `bun:",notnull" json:"subject_uuid"`
	Role         types.UserRole            `bun:",notnull" json:"role"`
	CreateUserID int64                     `bun:",notnull" json:"create_user_id"`
	CreatedAt    time.Time                 `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time                 `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// RepositoryAuthorizationStore persists direct repository grants.
type RepositoryAuthorizationStore interface {
	// Create adds a direct repository grant or updates the existing grant for the same subject.
	Create(ctx context.Context, authorization *RepositoryAuthorization) error
	// Find retrieves a direct repository grant by repository and subject.
	Find(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) (*RepositoryAuthorization, error)
	// List returns direct grants for a repository with pagination and the total count.
	List(ctx context.Context, repositoryID int64, per, page int) ([]RepositoryAuthorization, int, error)
	// ListByRepository returns all direct grants for a repository, including grants whose subjects were soft-deleted.
	ListByRepository(ctx context.Context, repositoryID int64) ([]RepositoryAuthorization, error)
	// UpdateRole changes the role of an existing direct repository grant.
	UpdateRole(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType, role types.UserRole) error
	// Delete removes a direct repository grant.
	Delete(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) error
	// DeleteBySubject removes all direct repository grants for a subject.
	DeleteBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) error
	// DeleteByRepository removes all direct grants for a repository.
	DeleteByRepository(ctx context.Context, repositoryID int64) error
	// ListBySubject returns all direct repository grants for a subject.
	ListBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) ([]RepositoryAuthorization, error)
}

// RebacTupleJobClient persists permission changes in the business transaction.
// Implementations must not execute OpenFGA writes before this transaction commits.
type RebacTupleJobClient interface {
	InsertRebacTupleJobsTx(context.Context, *sql.Tx, []types.RebacTuple) error
}

type repositoryAuthorizationStoreImpl struct {
	// rebacTupleJobClient records permission changes on this Store's business transaction.
	rebacTupleJobClient RebacTupleJobClient
	db                  *DB
}

// NewRepositoryAuthorizationStore creates a direct repository authorization store.
func NewRepositoryAuthorizationStore() RepositoryAuthorizationStore {
	return NewRepositoryAuthorizationStoreWithRebacTupleJobClient(defaultDB, nil)
}

// NewRepositoryAuthorizationStoreWithDB creates a direct repository authorization store with the provided database.
func NewRepositoryAuthorizationStoreWithDB(db *DB) RepositoryAuthorizationStore {
	return NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db, nil)
}

// NewRepositoryAuthorizationStoreWithRebacTupleJobClient injects the transactional permission job producer.
func NewRepositoryAuthorizationStoreWithRebacTupleJobClient(db *DB, tupleJobs RebacTupleJobClient) RepositoryAuthorizationStore {
	return &repositoryAuthorizationStoreImpl{db: db, rebacTupleJobClient: tupleJobs}
}

// roleValid reports whether a repository authorization role is supported.
func (s *repositoryAuthorizationStoreImpl) roleValid(role types.UserRole) bool {
	return role == types.UserRead || role == types.UserWrite
}

// subjectTypeValid reports whether a repository authorization subject type is supported.
func subjectTypeValid(subjectType types.RepoAuthSubjectType) bool {
	return subjectType.IsValid()
}

// Create adds a direct repository grant or updates the existing grant for the same subject.
func (s *repositoryAuthorizationStoreImpl) Create(ctx context.Context, authorization *RepositoryAuthorization) error {
	if !subjectTypeValid(authorization.SubjectType) || !s.roleValid(authorization.Role) {
		return errorx.ErrReqParamInvalid
	}
	return s.mutateGrant(ctx, *authorization, func(ctx context.Context, tx bun.Tx, old *RepositoryAuthorization) (*RepositoryAuthorization, error) {
		_, err := tx.NewInsert().Model(authorization).
			On("CONFLICT (repository_id, subject_type, subject_id) DO UPDATE").
			Set("subject_uuid = EXCLUDED.subject_uuid").Set("role = EXCLUDED.role").Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx)
		return authorization, errorx.HandleDBError(err, nil)
	})
}

// Find retrieves a direct repository grant by repository and subject.
func (s *repositoryAuthorizationStoreImpl) Find(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) (*RepositoryAuthorization, error) {
	if !subjectTypeValid(subjectType) {
		return nil, errorx.ErrReqParamInvalid
	}
	result := new(RepositoryAuthorization)
	err := s.db.Operator.Core.NewSelect().Model(result).
		Where("repository_id = ? AND subject_id = ? AND subject_type = ?", repositoryID, subjectID, subjectType).
		Limit(1).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("repository_id", repositoryID))
	}
	return result, nil
}

// List returns active direct grants for a repository with pagination and the total count.
func (s *repositoryAuthorizationStoreImpl) List(ctx context.Context, repositoryID int64, per, page int) ([]RepositoryAuthorization, int, error) {
	if per <= 0 {
		per = 50
	}
	if page <= 0 {
		page = 1
	}
	items := make([]RepositoryAuthorization, 0)
	// Filter stale grants in SQL so pagination and total count only include active subjects.
	query := s.db.Operator.Core.NewSelect().Model(&items).
		Where(`repository_id = ? AND (
			(subject_type = ? AND EXISTS (
				SELECT 1 FROM users WHERE users.id = subject_id AND users.deleted_at IS NULL
			)) OR
			(subject_type = ? AND EXISTS (
				SELECT 1 FROM organizations WHERE organizations.id = subject_id AND organizations.deleted_at IS NULL
			))
		)`, repositoryID, types.RepoAuthSubjectUser, types.RepoAuthSubjectOrganization)
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	err = query.Order("id DESC").Limit(per).Offset((page - 1) * per).Scan(ctx)
	return items, count, errorx.HandleDBError(err, nil)
}

// ListByRepository returns all direct grants for repository cleanup.
func (s *repositoryAuthorizationStoreImpl) ListByRepository(ctx context.Context, repositoryID int64) (items []RepositoryAuthorization, err error) {
	items = make([]RepositoryAuthorization, 0)
	err = s.db.Operator.Core.NewSelect().Model(&items).
		Where("repository_id = ?", repositoryID).
		Order("id DESC").Scan(ctx)
	return items, errorx.HandleDBError(err, nil)
}

// UpdateRole changes the role of an existing direct repository grant.
// Repeating the locked role leaves the record and permission queue unchanged.
func (s *repositoryAuthorizationStoreImpl) UpdateRole(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType, role types.UserRole) error {
	if !subjectTypeValid(subjectType) || !s.roleValid(role) {
		return errorx.ErrReqParamInvalid
	}
	key := RepositoryAuthorization{RepositoryID: repositoryID, SubjectID: subjectID, SubjectType: subjectType}
	return s.mutateGrant(ctx, key, func(ctx context.Context, tx bun.Tx, old *RepositoryAuthorization) (*RepositoryAuthorization, error) {
		if old == nil {
			return nil, errorx.ErrRepoAuthorizationNotFound
		}
		if old.Role == role {
			return old, nil
		}
		next := *old
		next.Role = role
		err := assertAffectedOneRow(tx.NewUpdate().Model(&next).Set("role = ?", role).Set("updated_at = CURRENT_TIMESTAMP").WherePK().Exec(ctx))
		return &next, errorx.HandleDBError(err, nil)
	})
}

// Delete removes a direct repository grant for a repository subject.
func (s *repositoryAuthorizationStoreImpl) Delete(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) error {
	if !subjectTypeValid(subjectType) {
		return errorx.ErrReqParamInvalid
	}
	key := RepositoryAuthorization{RepositoryID: repositoryID, SubjectID: subjectID, SubjectType: subjectType}
	return s.mutateGrant(ctx, key, func(ctx context.Context, tx bun.Tx, old *RepositoryAuthorization) (*RepositoryAuthorization, error) {
		if old == nil {
			return nil, nil
		}
		_, err := tx.NewDelete().Model(old).WherePK().Exec(ctx)
		return nil, errorx.HandleDBError(err, nil)
	})
}

// DeleteBySubject removes all direct repository grants for a subject.
func (s *repositoryAuthorizationStoreImpl) DeleteBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) error {
	if !subjectTypeValid(subjectType) {
		return errorx.ErrReqParamInvalid
	}
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryAuthorization)(nil)).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// DeleteByRepository removes all direct grants for a repository.
func (s *repositoryAuthorizationStoreImpl) DeleteByRepository(ctx context.Context, repositoryID int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryAuthorization)(nil)).
		Where("repository_id = ?", repositoryID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// ListBySubject returns all direct repository grants for a subject.
func (s *repositoryAuthorizationStoreImpl) ListBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) ([]RepositoryAuthorization, error) {
	if !subjectTypeValid(subjectType) {
		return nil, errorx.ErrReqParamInvalid
	}
	items := make([]RepositoryAuthorization, 0)
	err := s.db.Operator.Core.NewSelect().Model(&items).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Order("repository_id ASC").Scan(ctx)
	return items, errorx.HandleDBError(err, nil)
}

var _ RepositoryAuthorizationStore = (*repositoryAuthorizationStoreImpl)(nil)

// authorizationTuple preserves the grant's stored subject UUID and userset.
func authorizationTuple(grant RepositoryAuthorization) (rebac.Relationship, error) {
	relation, ok := grant.Role.ReBACRelation()
	if !ok {
		return rebac.Relationship{}, fmt.Errorf("invalid repository grant role %q", grant.Role)
	}
	subject := rebac.UserSubject(grant.SubjectUUID)
	switch grant.SubjectType {
	case types.RepoAuthSubjectUser:
	case types.RepoAuthSubjectOrganization:
		subject = rebac.OrganizationMembers(grant.SubjectUUID)
	default:
		return rebac.Relationship{}, fmt.Errorf("invalid repository grant subject %q", grant.SubjectType)
	}
	return rebac.Relationship{Subject: subject, Relation: relation, Object: rebac.RepositoryObject(grant.RepositoryID)}, nil
}

// lockUserRebacTx serializes user grants and memberships with account deletion.
// Acquire all subject locks in ascending user ID order before taking any row locks.
// Keep the existing advisory key so repository grants use the same lock domain.
func lockUserRebacTx(ctx context.Context, tx bun.Tx, userID int64) error {
	if tx.Dialect().Name() != dialect.PG {
		return nil
	}
	key := fmt.Sprintf("rebac:user_repository_grants:%d", userID)
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Exec(ctx)
	return err
}

// lockOrganizationGrantSubjectTx locks the hierarchy root, then the subject,
// before any repository rows. Hierarchy deletion locks the root before deleting
// descendant repositories, so locking only a child would introduce a lock cycle.
// Retained or missing subjects return inactive so revocations remain possible.
func lockOrganizationGrantSubjectTx(ctx context.Context, tx bun.Tx, organizationID int64) (bool, error) {
	var initial Organization
	if err := tx.NewSelect().Model(&initial).WhereAllWithDeleted().Where("id = ?", organizationID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	rootID := organizationID
	if initial.IsHierarchical && !initial.IsRoot {
		if err := tx.NewSelect().Model((*OrganizationUnit)(nil)).Column("root_organization_id").WhereAllWithDeleted().
			Where("organization_id = ?", organizationID).Scan(ctx, &rootID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
	}
	var root Organization
	if err := tx.NewSelect().Model(&root).WhereAllWithDeleted().Where("id = ?", rootID).For("UPDATE").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	subject := root
	if rootID != organizationID {
		if err := tx.NewSelect().Model(&subject).WhereAllWithDeleted().Where("id = ?", organizationID).For("UPDATE").Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
	}
	if subject.IsHierarchical != initial.IsHierarchical || subject.IsRoot != initial.IsRoot {
		return false, fmt.Errorf("organization hierarchy changed concurrently during grant mutation")
	}
	active := root.DeletedAt.IsZero() && subject.DeletedAt.IsZero()
	if rootID != organizationID {
		active = active && root.IsHierarchical && root.IsRoot
	}
	return active, nil
}

// mutateGrant atomically enqueues permission changes while coordinating subject deletion.
// User grants take the subject advisory lock; organization grants lock root and subject
// rows before the repository. The repository lock also serializes concurrent upserts.
// A callback returning the original pointer reports a no-op and must not modify it.
func (s *repositoryAuthorizationStoreImpl) mutateGrant(ctx context.Context, key RepositoryAuthorization, change func(context.Context, bun.Tx, *RepositoryAuthorization) (*RepositoryAuthorization, error)) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		organizationActive := false
		if key.SubjectType == types.RepoAuthSubjectUser {
			if err := lockUserRebacTx(ctx, tx, key.SubjectID); err != nil {
				return err
			}
		} else if key.SubjectType == types.RepoAuthSubjectOrganization {
			var err error
			organizationActive, err = lockOrganizationGrantSubjectTx(ctx, tx, key.SubjectID)
			if err != nil {
				return err
			}
		}
		var repoID int64
		q := tx.NewSelect().Model((*Repository)(nil)).Column("id").Where("id = ?", key.RepositoryID)
		if tx.Dialect().Name() == dialect.PG {
			q.For("UPDATE")
		}
		if err := q.Scan(ctx, &repoID); err != nil {
			return err
		}
		var previous RepositoryAuthorization
		err := tx.NewSelect().Model(&previous).Where("repository_id = ? AND subject_type = ? AND subject_id = ?", key.RepositoryID, key.SubjectType, key.SubjectID).Scan(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var old *RepositoryAuthorization
		if err == nil {
			old = &previous
		}
		next, err := change(ctx, tx, old)
		if err != nil {
			return err
		}
		if next == old {
			return nil
		}
		if next != nil && key.SubjectType == types.RepoAuthSubjectOrganization && !organizationActive {
			return errorx.ErrDatabaseNoRows
		}
		if next != nil && key.SubjectType == types.RepoAuthSubjectUser {
			// A mutation waiting on deletion must not recreate a deleted user's grant.
			// Keep deletion idempotent even when its subject no longer exists.
			var userID int64
			if err := tx.NewSelect().Model((*User)(nil)).Column("id").Where("id = ?", key.SubjectID).Scan(ctx, &userID); err != nil {
				return errorx.HandleDBError(err, nil)
			}
		}
		var operations []types.TupleOperation
		if old != nil && (next == nil || old.Role != next.Role || old.SubjectUUID != next.SubjectUUID) {
			r, err := authorizationTuple(*old)
			if err != nil {
				return err
			}
			operations = append(operations, types.TupleOperation{Delete: true, Relationships: []rebac.Relationship{r}})
		}
		if next != nil {
			r, err := authorizationTuple(*next)
			if err != nil {
				return err
			}
			operations = append(operations, types.TupleOperation{Relationships: []rebac.Relationship{r}})
		}
		return enqueueRebacTuple(ctx, tx, s.rebacTupleJobClient, types.NewRebacTuple("repository.authorization", operations...))
	})
}

// enqueueRebacTuple persists a permission delta using the Store's injected producer.
// Missing wiring fails closed instead of committing an untracked permission change.
func enqueueRebacTuple(ctx context.Context, tx bun.Tx, tupleJobs RebacTupleJobClient, inputs ...types.RebacTuple) error {
	var events []types.RebacTuple
	for _, input := range inputs {
		if len(input.Operations) > 0 {
			events = append(events, input)
		}
	}
	if len(events) == 0 {
		return nil
	}
	if tupleJobs == nil {
		return fmt.Errorf("transactional tuple job client is required")
	}
	return tupleJobs.InsertRebacTupleJobsTx(ctx, tx.Tx, events)
}
