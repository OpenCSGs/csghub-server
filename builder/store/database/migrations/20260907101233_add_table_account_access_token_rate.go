package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// AccountAccessTokenRate is the migration-time schema for
// account_access_token_rate.
type AccountAccessTokenRate struct {
	ID         int64 `bun:"id,pk,autoincrement"`
	TokenID    int64 `bun:"token_id,notnull"`
	AccessTime int64 `bun:"access_time,notnull"`
	Token      int64 `bun:"token,nullzero"`
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// 1. Create the account_access_token_rate table.
		_, err := db.NewCreateTable().
			Model((*AccountAccessTokenRate)(nil)).
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create account_access_token_rate table: %w", err)
		}

		// 2. Create query indexes.
		_, err = db.NewCreateIndex().
			Model((*AccountAccessTokenRate)(nil)).
			Index("idx_account_access_token_rate_token_id").
			Column("token_id", "access_time").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("create idx_account_access_token_rate_token_id: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.NewDropTable().
			Model((*AccountAccessTokenRate)(nil)).
			IfExists().
			Cascade().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("drop account_access_token_rate table: %w", err)
		}
		return nil
	})
}
