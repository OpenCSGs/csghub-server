package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type accessTokenStoreImpl struct {
	db *DB
}

type AccessTokenStore interface {
	Create(ctx context.Context, token *AccessToken, quota []AccountAccessTokenQuota) error
	// Refresh will disable existing access token, and then generate new one
	Refresh(ctx context.Context, token *AccessToken, newTokenValue string, newExpiredAt time.Time) (*AccessToken, error)
	FindByID(ctx context.Context, id int64) (token *AccessToken, err error)
	Delete(ctx context.Context, username, tkName, app string) (err error)
	IsExist(ctx context.Context, username, tkName, app string) (exists bool, err error)
	FindByUID(ctx context.Context, uid int64) (token *AccessToken, err error)
	GetUserGitToken(ctx context.Context, username string) (*AccessToken, error)
	FindByToken(ctx context.Context, tokenValue, app string) (*AccessToken, error)
	FindByTokenName(ctx context.Context, username, tokenName, app string) (*AccessToken, error)
	FindByUser(ctx context.Context, username, app string) ([]AccessToken, error)
	GetByID(ctx context.Context, id int64) (*AccessToken, error)
	IsExistByUUID(ctx context.Context, uuid string, tkName, app string) (exists bool, err error)
	// UpdateAPIKey updates a gateway API key by id, only works for app=apikey
	UpdateTokenAndQuota(ctx context.Context, key *AccessToken, quota *AccountAccessTokenQuota) (*AccessToken, error)
	// DeleteByID deletes a  API key by id
	DeleteByID(ctx context.Context, id int64) error
	// FindByNsUUID finds gateway API keys by namespace uuid
	FindByNsUUID(ctx context.Context, nsUUID string, app string) ([]AccessToken, error)
}

func NewAccessTokenStoreWithDB(db *DB) AccessTokenStore {
	return &accessTokenStoreImpl{db: db}
}

func NewAccessTokenStore() AccessTokenStore {
	return &accessTokenStoreImpl{
		db: defaultDB,
	}
}

type AccessToken struct {
	ID     int64  `bun:",pk,autoincrement" json:"id"`
	GitID  int64  `bun:",notnull" json:"git_id"`
	Name   string `bun:",notnull" json:"name"`
	Token  string `bun:",notnull" json:"token"` // access token value or api key value
	UserID int64  `bun:",notnull" json:"user_id"`
	User   *User  `bun:"rel:belongs-to,join:user_id=id" json:"user"`
	//example: csghub, starship
	Application types.AccessTokenApp `bun:"column:app," json:"application"`
	Permission  string               `bun:"," json:"permission"`
	IsActive    bool                 `bun:",default:true" json:"is_active"`
	ExpiredAt   time.Time            `bun:",nullzero" json:"expired_at"`
	DeletedAt   time.Time            `bun:",soft_delete,nullzero"`
	// namespace uuid for gateway api key
	NsUUID string `bun:",nullzero" json:"ns_uuid"` // ns uuid
	times
}

func (s *accessTokenStoreImpl) Create(ctx context.Context, token *AccessToken, quotas []AccountAccessTokenQuota) error {
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Insert access token
		err := tx.NewInsert().Model(token).Scan(ctx)
		if err != nil {
			return fmt.Errorf("failed to create access token, err:%w", err)
		}

		// Insert quota records
		if len(quotas) > 0 {
			err = tx.NewInsert().Model(&quotas).Scan(ctx)
			if err != nil {
				return fmt.Errorf("failed to create access token quota, err:%w", err)
			}
		}

		return nil
	})
	return errorx.HandleDBError(err, nil)
}

// Refresh will disable existing access token, and then generate new one
func (s *accessTokenStoreImpl) Refresh(ctx context.Context, token *AccessToken, newTokenValue string, newExpiredAt time.Time) (*AccessToken, error) {
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
	return newToken, errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) FindByID(ctx context.Context, id int64) (*AccessToken, error) {
	var token AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("access_token.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

func (s *accessTokenStoreImpl) Delete(ctx context.Context, username, tkName, app string) (err error) {
	var token AccessToken
	_, err = s.db.Operator.Core.
		NewDelete().
		Model(&token).
		TableExpr("users AS u").
		Where("access_token.user_id = u.id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		ForceDelete().
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) IsExist(ctx context.Context, username, tkName, app string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Join("JOIN users AS u ON u.id = access_token.user_id").
		Where("u.username = ?", username).
		Where("access_token.name = ? and app = ?", tkName, app).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) FindByUID(ctx context.Context, uid int64) (token *AccessToken, err error) {
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
		return nil, errorx.HandleDBError(err, nil)
	}
	if len(tokens) == 0 {
		return nil, errorx.HandleDBError(sql.ErrNoRows, nil)
	}
	token = &tokens[0]
	return
}

func (s *accessTokenStoreImpl) GetUserGitToken(ctx context.Context, username string) (*AccessToken, error) {
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

func (s *accessTokenStoreImpl) FindByToken(ctx context.Context, tokenValue, app string) (*AccessToken, error) {
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
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

func (s *accessTokenStoreImpl) FindByTokenName(ctx context.Context, username, tokenName, app string) (*AccessToken, error) {
	var token AccessToken
	q := s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Relation("User").
		Where("access_token.name = ? and app = ? and is_active = true and username = ?", tokenName, app, username)
	err := q.Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &token, nil
}

func (s *accessTokenStoreImpl) FindByUser(ctx context.Context, username, app string) ([]AccessToken, error) {
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
		return nil, errorx.HandleDBError(err, nil)
	}
	return tokens, nil
}

func (s *accessTokenStoreImpl) GetByID(ctx context.Context, id int64) (*AccessToken, error) {
	token := &AccessToken{}
	err := s.db.Operator.Core.NewSelect().Model(token).WhereAllWithDeleted().Where("id = ?", id).Scan(ctx, token)
	return token, errorx.HandleDBError(err, nil)
}

func (s *accessTokenStoreImpl) IsExistByUUID(ctx context.Context, uuid string, tkName, app string) (exists bool, err error) {
	var token AccessToken
	exists, err = s.db.Operator.Core.
		NewSelect().
		Model(&token).
		Where("ns_uuid = ?", uuid).
		Where("name = ? and app = ?", tkName, app).
		Exists(ctx)
	return exists, errorx.HandleDBError(err, nil)
}

// UpdateTokenAndQuota updates a access token by id, only works for app=apikey
func (s *accessTokenStoreImpl) UpdateTokenAndQuota(ctx context.Context, key *AccessToken, quota *AccountAccessTokenQuota) (*AccessToken, error) {
	err := s.db.Core.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Update access token
		_, err := tx.NewUpdate().Model(key).WherePK().Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to update api key, err:%w", err)
		}

		// Update quota record
		if quota != nil {
			_, err = tx.NewUpdate().Model(quota).WherePK().Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to update api key quota, err:%w", err)
			}
		}

		return nil
	})

	return key, errorx.HandleDBError(err, nil)
}

// DeleteByID soft deletes a access token by id
func (s *accessTokenStoreImpl) DeleteByID(ctx context.Context, id int64) error {
	_, err := s.db.Operator.Core.
		NewUpdate().
		Model(&AccessToken{}).
		Set("is_active = false").
		Set("deleted_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// FindAPIKeyByNsUUID finds gateway API keys by namespace uuid
func (s *accessTokenStoreImpl) FindByNsUUID(ctx context.Context, nsUUID string, app string) ([]AccessToken, error) {
	var tokens []AccessToken
	err := s.db.Operator.Core.
		NewSelect().
		Model(&tokens).
		Where("app = ? and ns_uuid = ? and is_active = true", app, nsUUID).
		Order("id DESC").
		Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return tokens, nil
}
