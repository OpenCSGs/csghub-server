package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// AccountActivity is the activity catalog behind the present activities that
// were hardcoded in common/types/activity.go. VoucherValue is the per-voucher
// face amount of the activity in cents.
type AccountActivity struct {
	ID            int64                `bun:",pk,autoincrement" json:"id"`
	ActID         int64                `bun:",notnull,unique" json:"act_id"`
	ActCode       string               `bun:",notnull,unique" json:"act_code"`
	ActName       string               `bun:",notnull" json:"act_name"`
	Status        types.ActivityStatus `bun:",notnull,default:'active'" json:"status"` // active, expired
	BeginAt       time.Time            `bun:",nullzero" json:"begin_at"`
	EndAt         time.Time            `bun:",nullzero" json:"end_at"`
	Description   string               `bun:",nullzero" json:"description"`
	VoucherValue  int64                `bun:",notnull,default:0" json:"voucher_value"`
	ValidDuration int64                `bun:",nullzero" json:"valid_duration"` // valid duration in seconds
}

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] ")
		if err := createTables(ctx, db, AccountActivity{}); err != nil {
			return fmt.Errorf("failed to create table account_activity, error: %w", err)
		}

		// Seed the catalog with the activities hardcoded in
		// common/types/activity.go, keeping their well-known IDs so existing
		// account_presents.activity_id references stay valid. autohub_new_user
		// is seeded expired: its present is no longer granted.
		activities := []AccountActivity{
			{ActID: 1001, ActCode: "starship_new_user", ActName: "StarShip New User", Status: "expired", Description: "create starship access token for first time", VoucherValue: 0},
			{ActID: 1002, ActCode: "autohub_new_user", ActName: "AutoHub New User", Status: "expired", Description: "use autohub for first time", VoucherValue: 0},
			{ActID: 1003, ActCode: "invitee_credit", ActName: "Invitee Credit", Status: "active", Description: "present credit to invitee", VoucherValue: 9900, ValidDuration: 90 * 24 * 60 * 60 * 1}, // 90 days
			{ActID: 1004, ActCode: "inviter_credit", ActName: "Inviter Credit", Status: "active", Description: "present credit to inviter", VoucherValue: 9900, ValidDuration: 90 * 24 * 60 * 60 * 1}, // 90 days
			{ActID: 1005, ActCode: "csghub_new_user", ActName: "CSGHub New User", Status: "active", Description: "register csghub new user and fill in phone and email", VoucherValue: 1000},
		}
		if _, err := db.NewInsert().Model(&activities).Exec(ctx); err != nil {
			return fmt.Errorf("failed to seed account_activity, error: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		if err := dropTables(ctx, db, AccountActivity{}); err != nil {
			return fmt.Errorf("failed to drop table account_activity, error: %w", err)
		}
		return nil
	})
}
