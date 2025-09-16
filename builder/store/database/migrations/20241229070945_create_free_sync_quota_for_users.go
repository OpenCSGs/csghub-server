//go:build saas

package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		var users []User
		err := db.NewSelect().
			Model(&User{}).
			Join(`left join "account_sync_quota" on "account_sync_quota"."user_id" = "user"."id"`).
			Where("account_sync_quota.user_id is null").
			Scan(ctx, &users)
		if err != nil {
			return err
		}
		var accountSyncQuotas []AccountSyncQuota
		for _, user := range users {
			accountSyncQuotas = append(accountSyncQuotas, AccountSyncQuota{
				UserID:         user.ID,
				RepoCountLimit: 15,
				RepoCountUsed:  0,
			})
		}
		if len(accountSyncQuotas) > 0 {
			_, err = db.NewInsert().Model(&accountSyncQuotas).Exec(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] ")
		return nil
	})
}
