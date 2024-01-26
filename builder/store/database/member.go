package database

import "context"

type MemberStore struct {
	db *DB
}

func NewMemberStore() *MemberStore {
	return &MemberStore{
		db: defaultDB,
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

func (s *MemberStore) Find(ctx context.Context, orgID, userID int64) (*Member, error) {
	var member Member
	err := s.db.Core.NewSelect().Model(&member).Where("organization_id = ? AND user_id = ?", orgID, userID).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (s *MemberStore) Add(ctx context.Context, orgID, userID int64, role string) error {
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

func (s *MemberStore) Delete(ctx context.Context, orgID, userID int64, role string) error {
	var member Member
	_, err := s.db.Core.NewDelete().Model(&member).Where("organization_id=? and user_id=? and role=?", orgID, userID, role).Exec(ctx)
	return err
}

func (s *MemberStore) UserMembers(ctx context.Context, userID int64) ([]Member, error) {
	var members []Member
	err := s.db.Core.NewSelect().Model((*Member)(nil)).Where("user_id=?", userID).Scan(ctx, &members)
	return members, err
}
