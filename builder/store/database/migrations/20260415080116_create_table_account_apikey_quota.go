package migrations

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountAccessTokenQuota struct {
	ID          int64                          `bun:",pk,autoincrement" json:"id"`
	APIKey      string                         `bun:",notnull" json:"api_key"`
	QuotaType   types.AccountingQuotaType      `bun:",notnull" json:"quota_type"`
	ValueType   types.AccountingQuotaValueType `bun:",notnull" json:"value_type"`
	PeriodStart int64                          `bun:",notnull,default:0" json:"period_start"`
	PeriodEnd   int64                          `bun:",notnull,default:0" json:"period_end"`
	Usage       float64                        `bun:",notnull,default:0" json:"usage"`
	Quota       float64                        `bun:",notnull,default:0" json:"quota"`
	LastUsedAt  *time.Time                     `bun:",nullzero" json:"last_used_at"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, AccountAccessTokenQuota{})
		if err != nil {
			return err
		}
		// Create unique index on api_key
		_, err = db.NewCreateIndex().
			Model(&AccountAccessTokenQuota{}).
			Unique().
			Index("idx_account_apikey_quota_api_key_unique").
			Column("api_key", "quota_type", "value_type").
			Exec(ctx)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountAccessTokenQuota{})
	})
}
