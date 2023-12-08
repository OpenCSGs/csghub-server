package database

import "opencsg.com/starhub-server/pkg/model"

type MemberStore struct {
	db *model.DB
}

func NewMemberStore(db *model.DB) *MemberStore {
	return &MemberStore{
		db: db,
	}
}

type Member struct {
	ID             int64         `bun:",pk,autoincrement" json:"id"`
	OrganizationID int64         `bun:",pk" json:"organization_id"`
	UserID         int64         `bun:",pk" json:"user_id"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization"`
	User           *User         `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	Role           string        `bun:",notnull" json:"role"`
	times
}
