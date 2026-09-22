package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type accountAccessTokenRateStoreImpl struct {
	db *DB
}

type AccountAccessTokenRateStore interface {
	Create(ctx context.Context, rate *AccountAccessTokenRate) error
	GetStatByTokenID(ctx context.Context, rate *AccountAccessTokenRate) (*AccountAccessTokenRateStat, error)
	DeleteOldByTokenID(ctx context.Context, rate *AccountAccessTokenRate) error
}

// AccountAccessTokenRateStat holds the aggregate result of count(*) and
// sum(token) for a token_id over a time window.
type AccountAccessTokenRateStat struct {
	Count    int   `bun:"count" json:"count"`
	TokenSum int64 `bun:"token_sum" json:"token_sum"`
}

func NewAccountAccessTokenRateStore() AccountAccessTokenRateStore {
	return &accountAccessTokenRateStoreImpl{
		db: defaultDB,
	}
}

func NewAccountAccessTokenRateStoreWithDB(db *DB) AccountAccessTokenRateStore {
	return &accountAccessTokenRateStoreImpl{
		db: db,
	}
}

type AccountAccessTokenRate struct {
	ID         int64 `bun:"id,pk,autoincrement"`
	TokenID    int64 `bun:"token_id,notnull"`
	AccessTime int64 `bun:"access_time,notnull"`
	Token      int64 `bun:"token,nullzero"`
}

func (s *accountAccessTokenRateStoreImpl) Create(ctx context.Context, rate *AccountAccessTokenRate) error {
	err := s.db.Operator.Core.NewInsert().Model(rate).Scan(ctx)
	return errorx.HandleDBError(err, nil)
}

// GetStatByTokenID returns count(*) and sum(token) for the given token_id
// with access_time >= since (unix seconds) in a single aggregation query.
func (s *accountAccessTokenRateStoreImpl) GetStatByTokenID(ctx context.Context, req *AccountAccessTokenRate) (*AccountAccessTokenRateStat, error) {
	var stat AccountAccessTokenRateStat
	err := s.db.Operator.Core.NewSelect().Model((*AccountAccessTokenRate)(nil)).
		ColumnExpr("count(*) as count").
		ColumnExpr("coalesce(sum(token), 0) as token_sum").
		Where("token_id = ?", req.TokenID).
		Where("access_time >= ?", req.AccessTime).
		Scan(ctx, &stat)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &stat, nil
}

// DeleteOldByTokenID deletes rate records for the given token_id with
// access_time <= rate.AccessTime (unix seconds), used to clean up stale
// historical data.
func (s *accountAccessTokenRateStoreImpl) DeleteOldByTokenID(ctx context.Context, rate *AccountAccessTokenRate) error {
	_, err := s.db.Operator.Core.NewDelete().
		Model((*AccountAccessTokenRate)(nil)).
		Where("token_id = ?", rate.TokenID).
		Where("access_time <= ?", rate.AccessTime).
		Exec(ctx)
	return errorx.HandleDBError(err, nil)
}
