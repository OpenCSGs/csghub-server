package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/common/types"
)

type memberStoreImpl struct {
	// rebacTupleJobClient records permission changes on this Store's business transaction.
	rebacTupleJobClient RebacTupleJobClient
	db                  *DB
}

type MemberStore interface {
	// ReconcileTuples snapshots the current role, including an absent membership.
	ReconcileTuples(ctx context.Context, orgID, userID int64) error
	Find(ctx context.Context, orgID, userID int64) (*Member, error)
	Add(ctx context.Context, orgID, userID int64, role string) error
	Update(ctx context.Context, orgID, userID int64, role string) error
	// Delete removes the membership identified by the organization and user IDs.
	Delete(ctx context.Context, orgID, userID int64) error
	UserMembers(ctx context.Context, userID int64) ([]Member, error)
	OrganizationMembers(ctx context.Context, orgID int64, role string, pageSize, page int) ([]Member, int, error)
	UserUUIDsByOrganizationID(ctx context.Context, orgID int64) ([]string, error)
}

// NewMemberStore creates a Store for operations that do not require permission events.
// Use the explicit job-client constructor for tuple-changing operations.
func NewMemberStore() MemberStore {
	return NewMemberStoreWithRebacTupleJobClient(defaultDB, nil)
}

// NewMemberStoreWithDB creates a Store for operations that do not require permission events.
// Use the explicit job-client constructor for tuple-changing operations.
func NewMemberStoreWithDB(db *DB) MemberStore {
	return NewMemberStoreWithRebacTupleJobClient(db, nil)
}

// NewMemberStoreWithRebacTupleJobClient injects the transactional permission job producer.
func NewMemberStoreWithRebacTupleJobClient(db *DB, tupleJobs RebacTupleJobClient) MemberStore {
	return &memberStoreImpl{db: db, rebacTupleJobClient: tupleJobs}
}

// Member is the relationship between a user and an organization.
type Member struct {
	ID             int64         `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64         `bun:",notnull" json:"organization_id"`
	UserID         int64         `bun:",notnull" json:"user_id"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization"`
	User           *User         `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Role           string        `bun:",notnull" json:"role"`
	DeletedAt      time.Time     `bun:",soft_delete,nullzero"`
	times
}

func (s *memberStoreImpl) Find(ctx context.Context, orgID, userID int64) (*Member, error) {
	var member Member
	err := s.db.Core.NewSelect().Model(&member).Where("organization_id = ? AND user_id = ?", orgID, userID).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// Add inserts membership and its permission event atomically.
func (s *memberStoreImpl) Add(ctx context.Context, orgID, userID int64, role string) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserRebacTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := lockTupleOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		member := &Member{OrganizationID: orgID, UserID: userID, Role: role}
		if err := assertAffectedOneRow(tx.NewInsert().Model(member).Exec(ctx)); err != nil {
			return err
		}
		return enqueueMemberTx(ctx, tx, s.rebacTupleJobClient, orgID, userID, types.UserRole(role))
	})
}

// Update changes membership and its role projection in one transaction.
func (s *memberStoreImpl) Update(ctx context.Context, orgID, userID int64, role string) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserRebacTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := lockTupleOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		if err := assertAffectedOneRow(tx.NewUpdate().Model((*Member)(nil)).Where("organization_id=? and user_id=?", orgID, userID).Set("role=?", role).Exec(ctx)); err != nil {
			return err
		}
		return enqueueMemberTx(ctx, tx, s.rebacTupleJobClient, orgID, userID, types.UserRole(role))
	})
}

// Delete removes the membership identified by the organization and user IDs.
func (s *memberStoreImpl) Delete(ctx context.Context, orgID, userID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserRebacTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := lockTupleOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*Member)(nil)).Where("organization_id=? and user_id=?", orgID, userID).ForceDelete().Exec(ctx); err != nil {
			return err
		}
		return enqueueMemberTx(ctx, tx, s.rebacTupleJobClient, orgID, userID, "")
	})
}

// ReconcileTuples repairs an existing or absent membership using the current DB role.
func (s *memberStoreImpl) ReconcileTuples(ctx context.Context, orgID, userID int64) error {
	return s.db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := lockUserRebacTx(ctx, tx, userID); err != nil {
			return err
		}
		if err := lockTupleOrganization(ctx, tx, orgID); err != nil {
			return err
		}
		var role types.UserRole
		err := tx.NewSelect().Model((*Member)(nil)).Column("role").Where("organization_id = ? AND user_id = ?", orgID, userID).Scan(ctx, &role)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return enqueueMemberTx(ctx, tx, s.rebacTupleJobClient, orgID, userID, role)
	})
}

func (s *memberStoreImpl) UserMembers(ctx context.Context, userID int64) ([]Member, error) {
	var members []Member
	err := s.db.Core.NewSelect().Model((*Member)(nil)).Where("user_id=?", userID).Scan(ctx, &members)
	return members, err
}

func (s *memberStoreImpl) OrganizationMembers(ctx context.Context, orgID int64, role string, pageSize, page int) ([]Member, int, error) {
	var members []Member
	var total int
	q := s.db.Core.NewSelect().Model((*Member)(nil)).
		Relation("User").
		Join("JOIN users AS u ON u.id = member.user_id").
		Where("organization_id=?", orgID)
	if role != "" {
		q = q.Where("role = ?", role)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count org members,caused by:%w", err)
	}

	q = q.Limit(pageSize).
		Offset((page - 1) * pageSize)
	err = q.Scan(ctx, &members)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find org members,caused by:%w", err)
	}

	return members, total, nil
}

func (s *memberStoreImpl) UserUUIDsByOrganizationID(ctx context.Context, orgID int64) ([]string, error) {
	var uuids []string
	err := s.db.Core.NewSelect().
		Model((*Member)(nil)).
		Column("u.uuid").
		Join("JOIN users AS u ON u.id = member.user_id").
		Where("member.organization_id = ?", orgID).
		Scan(ctx, &uuids)
	if err != nil {
		return nil, fmt.Errorf("failed to get user UUIDs by organization ID, orgID: %d: caused by: %w", orgID, err)
	}
	return uuids, nil
}

// lockTupleOrganization serializes ordinary membership snapshots with each other.
func lockTupleOrganization(ctx context.Context, tx bun.Tx, orgID int64) error {
	q := tx.NewSelect().Model((*Organization)(nil)).Column("id").Where("id = ?", orgID)
	if tx.Dialect().Name() == dialect.PG {
		q.For("UPDATE")
	}
	var id int64
	return q.Scan(ctx, &id)
}

// memberTupleOperations deterministically removes obsolete roles and writes the target.
// An empty role means removing the direct membership entirely.
func memberTupleOperations(orgUUID, userUUID string, role types.UserRole) ([]types.TupleOperation, error) {
	var desired rebac.Relation
	if role != "" {
		var ok bool
		desired, ok = role.ReBACRelation()
		if !ok {
			return nil, fmt.Errorf("unsupported organization role %q", role)
		}
	}
	var deletes, writes []rebac.Relationship
	for _, relation := range []rebac.Relation{rebac.RelationReader, rebac.RelationWriter, rebac.RelationAdmin} {
		r := rebac.Relationship{Subject: rebac.UserSubject(userUUID), Relation: relation, Object: rebac.OrganizationObject(orgUUID)}
		if relation == desired {
			writes = append(writes, r)
		} else {
			deletes = append(deletes, r)
		}
	}
	ops := []types.TupleOperation{{Delete: true, Relationships: deletes}}
	if len(writes) > 0 {
		ops = append(ops, types.TupleOperation{Relationships: writes})
	}
	return ops, nil
}

// enqueueMemberTx requires the caller to hold the user ReBAC lock until commit.
// Only revocations may resolve a soft-deleted user; writes require an active subject.
func enqueueMemberTx(ctx context.Context, tx bun.Tx, tupleJobs RebacTupleJobClient, orgID, userID int64, role types.UserRole) error {
	event, err := memberTupleTx(ctx, tx, orgID, userID, role)
	if err != nil {
		return err
	}
	return enqueueRebacTuple(ctx, tx, tupleJobs, event)
}

// memberTupleTx snapshots member identities before the final batch enqueue.
func memberTupleTx(ctx context.Context, tx bun.Tx, orgID, userID int64, role types.UserRole) (types.RebacTuple, error) {
	var orgUUID, userUUID string
	if err := tx.NewSelect().Model((*Organization)(nil)).Column("uuid").Where("id = ?", orgID).Scan(ctx, &orgUUID); err != nil {
		return types.RebacTuple{}, err
	}
	userQuery := tx.NewSelect().Model((*User)(nil)).Column("uuid").Where("id = ?", userID)
	if role == "" {
		userQuery.WhereAllWithDeleted()
	}
	if err := userQuery.Scan(ctx, &userUUID); err != nil {
		return types.RebacTuple{}, err
	}
	ops, err := memberTupleOperations(orgUUID, userUUID, role)
	if err != nil {
		return types.RebacTuple{}, err
	}
	return types.NewRebacTuple("organization.member", ops...), nil
}
