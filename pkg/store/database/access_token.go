package database

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
)

type AccessTokenStore struct {
	db *model.DB
}

func NewAccessTokenStore(db *model.DB) *AccessTokenStore {
	return &AccessTokenStore{
		db: db,
	}
}

type AccessToken struct {
	ID     int    `bun:",pk,autoincrement" json:"id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"`
	UserID int    `bun:",notnull" json:"user_id"`
	User   User   `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	times
}

func (s *AccessTokenStore) Create(ctx context.Context, token *AccessToken) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(token).Scan(ctx)
	return
}

func (s *AccessTokenStore) FindByID(ctx context.Context, id int) (token *AccessToken, err error) {
	var tokens []AccessToken
	err = s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Relation("User").
		Where("access_token.id = ?", id).
		Scan(ctx)
	token = &tokens[0]
	return
}

func (s *AccessTokenStore) Delete(ctx context.Context, username, tkName string) (err error) {
	var token AccessToken
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&token).
		TableExpr("users AS u").
		Where("access_token.user_id = u.id").
		Where("u.username = ?", username).
		Where("access_token.name = ?", tkName).
		Exec(ctx)
	return
}

func (s *AccessTokenStore) IsExist(ctx context.Context, username, tkName string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.name = ?", tkName).
		Exists(ctx)
	return
}
