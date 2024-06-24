package database

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountSyncQuotaStatementStore struct {
	db *DB
}

func NewAccountSyncQuotaStatementStore() *AccountSyncQuotaStatementStore {
	return &AccountSyncQuotaStatementStore{
		db: defaultDB,
	}
}

type AccountSyncQuotaStatement struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	UserID    int64     `bun:",notnull" json:"user_id"`
	RepoPath  string    `bun:",notnull" json:"repo_path"`
	RepoType  string    `bun:",notnull" json:"repo_type"`
	CreatedAt time.Time `bun:",notnull,default:current_timestamp" json:"created_at"`
}

func (s *AccountSyncQuotaStatementStore) Create(ctx context.Context, acctQuotaStatement AccountSyncQuotaStatement) error {
	err := s.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := assertAffectedOneRow(tx.NewInsert().Model(&acctQuotaStatement).Exec(ctx)); err != nil {
			return err
		}

		runSql := "update account_sync_quota set repo_count_used=repo_count_used+1 where user_id = ? and repo_count_limit>repo_count_used"
		if err := assertAffectedOneRow(tx.Exec(runSql, acctQuotaStatement.UserID)); err != nil {
			return errors.New("repo download reach limit")
		}

		return nil
	})

	return err
}

func (s *AccountSyncQuotaStatementStore) Get(ctx context.Context, userID int64, req types.ACCT_QUOTA_STATEMENT_REQ) (*AccountSyncQuotaStatement, error) {
	quotaStatement := &AccountSyncQuotaStatement{}
	err := s.db.Core.NewSelect().Model(quotaStatement).Where("user_id = ? and repo_path = ? and repo_type = ?", userID, req.RepoPath, req.RepoType).Scan(ctx, quotaStatement)
	return quotaStatement, err
}
