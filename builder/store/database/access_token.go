package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccessTokenStore struct {
	db *DB
}

func NewAccessTokenStore() *AccessTokenStore {
	return &AccessTokenStore{
		db: defaultDB,
	}
}

type AccessToken struct {
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	GitID  int64  `bun:",notnull" json:"git_id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"`
	UserID int64  `bun:",notnull" json:"user_id"`
	User   *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	//example: csghub, starship
	Application types.AccessTokenApp `bun:"column:app," json:"application"`
	Permission  string               `bun:"," json:"permission"`
	IsActive    bool                 `bun:",default:true" json:"is_active"`
	ExpiredAt   time.Time            `bun:",nullzero" json:"expired_at"`
	times
}

func (s *AccessTokenStore) Create(ctx context.Context, token *AccessToken) (err error) {
	err = s.db.Operator.Core.NewInsert().Model(token).Scan(ctx)
	return
}

// Refresh will disable existing access token, and then generate new one
func (s *AccessTokenStore) Refresh(ctx context.Context, token *AccessToken, newTokenValue string, newExpiredAt time.Time) (*AccessToken, error) {
	var newToken *AccessToken
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().Model(token).
			Set("is_active = false").
			Where("user_id = ? and name = ? and app = ? and (is_active is null or is_active = true) ", token.UserID, token.Name, token.Application).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to disable old token, err:%w", err)
		}

		// make a copy
		newToken = &AccessToken{
			Name:        token.Name,
			Token:       newTokenValue,
			UserID:      token.UserID,
			Application: token.Application,
			Permission:  token.Permission,
			IsActive:    true,
		}
		if newExpiredAt.After(time.Now()) {
			newToken.ExpiredAt = newExpiredAt
		} else {
			//don't change old key's expire time
			newToken.ExpiredAt = token.ExpiredAt
		}
		_, err = tx.NewInsert().Model(newToken).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed insert new token, err:%w", err)
		}

		return nil
	})

	newToken.User = token.User
	return newToken, err
}

func (s *AccessTokenStore) FindByID(ctx context.Context, id int64) (token *AccessToken, err error) {
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

func (s *AccessTokenStore) Delete(ctx context.Context, username, tkName, app string) (err error) {
	var token AccessToken
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&token).
		TableExpr("users AS u").
		Where("access_token.user_id = u.id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		Exec(ctx)
	return
}

func (s *AccessTokenStore) IsExist(ctx context.Context, username, tkName, app string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		Exists(ctx)
	return
}

func (s *AccessTokenStore) FindByUID(ctx context.Context, uid int64) (token *AccessToken, err error) {
	var tokens []AccessToken
	err = s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Relation("User").
		Where("user_id = ?", uid).
		Where("app = ?", "git").
		Where("is_active = true and (expired_at is null or expired_at > ?)", time.Now()).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("access token not found")
	}
	token = &tokens[0]
	return
}

func (s *AccessTokenStore) GetUserGitToken(ctx context.Context, username string) (*AccessToken, error) {
	var token AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.app = ?", "git").
		Where("is_active = true and (access_token.expired_at is null or access_token.expired_at > ?)", time.Now()).
		Order("created_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *AccessTokenStore) FindByToken(ctx context.Context, tokenValue, app string) (*AccessToken, error) {
	var token AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("token = ? and is_active = true", tokenValue)
	if len(app) > 0 {
		q = q.Where("app = ?", app)
	}
	err := q.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *AccessTokenStore) FindByTokenName(ctx context.Context, username, tokenName, app string) (*AccessToken, error) {
	var token AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("access_token.name = ? and app = ? and is_active = true and username = ?", tokenName, app, username)
	err := q.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *AccessTokenStore) FindByUser(ctx context.Context, username, app string) ([]AccessToken, error) {
	var tokens []AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Relation("User").
		Where("is_active = true and username = ?", username).
		Order("created_at DESC")
	if len(app) > 0 {
		q = q.Where("app = ?", app)
	}
	err := q.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}
