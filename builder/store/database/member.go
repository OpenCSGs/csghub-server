package database

import (
	"context"
	"fmt"
)

type memberStoreImpl struct {
	db *DB
}

type MemberStore interface {
	Find(ctx context.Context, orgID, userID int64) (*Member, error)
	Add(ctx context.Context, orgID, userID int64, role string) error
	Delete(ctx context.Context, orgID, userID int64, role string) error
	UserMembers(ctx context.Context, userID int64) ([]Member, error)
	OrganizationMembers(ctx context.Context, orgID int64, pageSize, page int) ([]Member, int, error)
	UserUUIDsByOrganizationID(ctx context.Context, orgID int64) ([]string, error)
}

func NewMemberStore() MemberStore {
	return &memberStoreImpl{
		db: defaultDB,
	}
}

func NewMemberStoreWithDB(db *DB) MemberStore {
	return &memberStoreImpl{
		db: db,
	}
}

// Member is the relationship between a user and an organization.
type Member struct {
	ID             int64         `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64         `bun:",pk" json:"organization_id"`
	UserID         int64         `bun:",pk" json:"user_id"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization"`
	User           *User         `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Role           string        `bun:",notnull" json:"role"`
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

func (s *memberStoreImpl) Add(ctx context.Context, orgID, userID int64, role string) error {
	member := &Member{
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
	}
	result, err := s.db.Core.NewInsert().Model(member).Exec(ctx)
	if err != nil {
		return err
	}
	return assertAffectedOneRow(result, err)
}

func (s *memberStoreImpl) Delete(ctx context.Context, orgID, userID int64, role string) error {
	var member Member
	_, err := s.db.Core.NewDelete().Model(&member).Where("organization_id=? and user_id=? and role=?", orgID, userID, role).Exec(ctx)
	return err
}

func (s *memberStoreImpl) UserMembers(ctx context.Context, userID int64) ([]Member, error) {
	var members []Member
	err := s.db.Core.NewSelect().Model((*Member)(nil)).Where("user_id=?", userID).Scan(ctx, &members)
	return members, err
}

func (s *memberStoreImpl) OrganizationMembers(ctx context.Context, orgID int64, pageSize, page int) ([]Member, int, error) {
	var members []Member
	var total int
	q := s.db.Core.NewSelect().Model((*Member)(nil)).
		Relation("User").
		Where("organization_id=?", orgID).
		Limit(pageSize).
		Offset((page - 1) * pageSize)
	err := q.Scan(ctx, &members)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find org members,caused by:%w", err)
	}
	total, err = q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count org members,caused by:%w", err)
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
