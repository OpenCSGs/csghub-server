package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

type AccountVoucher struct {
	ID         int64                `bun:",pk,autoincrement" json:"id"`
	VoucherNo  string               `bun:",notnull,unique" json:"voucher_no"`
	TargetType string               `bun:",notnull" json:"target_type"` // user or org
	TargetUUID string               `bun:",notnull" json:"target_uuid"`
	TargetName string               `bun:",notnull" json:"target_name"`
	Total      float64              `bun:",notnull,default:0" json:"total"`
	Used       float64              `bun:",notnull,default:0" json:"used"`
	BeginDate  time.Time            `bun:",notnull" json:"begin_date"`
	EndDate    time.Time            `bun:",notnull" json:"end_date"`
	Status     types.VoucherStatus  `bun:",notnull" json:"status"`
	Rules      []types.VoucherRules `bun:",type:jsonb,nullzero" json:"rules"`
	Notes      string               `bun:",nullzero" json:"notes"`
	IssueUUID  string               `bun:",notnull" json:"issue_uuid"`
	IssueName  string               `bun:",notnull" json:"issue_name"`
	times
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if err := createTables(ctx, db, AccountVoucher{}); err != nil {
			return fmt.Errorf("failed to create table account_voucher, error: %w", err)
		}

		_, err := db.NewCreateIndex().
			Model(&AccountVoucher{}).
			Index("idx_account_voucher_targettype_status_begin_date").
			Column("target_type", "status", "begin_date").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_voucher on target_type/status/begin_date, error: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model(&AccountVoucher{}).
			Index("idx_account_voucher_targetuuid_status_begin_date").
			Column("target_uuid", "status", "begin_date").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_voucher on target_uuid/status/begin_date, error: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model(&AccountVoucher{}).
			Index("idx_account_voucher_status_begin_date").
			Column("status", "begin_date").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_voucher on status/begin_date, error: %w", err)
		}

		_, err = db.NewCreateIndex().
			Model(&AccountVoucher{}).
			Index("idx_account_voucher_status_end_date").
			Column("status", "end_date").
			IfNotExists().
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create index for account_voucher on status/end_date, error: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		return dropTables(ctx, db, AccountVoucher{})
	})
}
