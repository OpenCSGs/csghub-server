package migrations

import (
	"context"
	"fmt"
	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		// Up migration: Change 'amount' from float64 to int64

		// Adjust table names if necessary
		paymentTableName := "payment_payment"
		accountRechargeTableName := "account_recharges" // Use "account_recharge" if that's the correct name

		// Alter 'amount' column in 'payment_payment' table
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
            ALTER TABLE %s
            ALTER COLUMN amount TYPE bigint USING ROUND(amount)::bigint;
        `, paymentTableName))
		if err != nil {
			return fmt.Errorf("failed to alter 'amount' column in 'payment_payment' table: %w", err)
		}

		// Alter 'amount' column in 'account_recharge' table
		_, err = db.ExecContext(ctx, fmt.Sprintf(`
            ALTER TABLE %s
            ALTER COLUMN amount TYPE bigint USING ROUND(amount)::bigint;
        `, accountRechargeTableName))
		if err != nil {
			return fmt.Errorf("failed to alter 'amount' column in 'account_recharge' table: %w", err)
		}

		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		// Down migration: Revert 'amount' back to float64

		paymentTableName := "payment_payment"
		accountRechargeTableName := "account_recharges" // Use "account_recharge" if that's the correct name

		// Revert 'amount' column in 'payment_payment' table
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
            ALTER TABLE %s
            ALTER COLUMN amount TYPE double precision USING amount::double precision;
        `, paymentTableName))
		if err != nil {
			return fmt.Errorf("failed to revert 'amount' column in 'payment_payment' table: %w", err)
		}

		// Revert 'amount' column in 'account_recharge' table
		_, err = db.ExecContext(ctx, fmt.Sprintf(`
            ALTER TABLE %s
            ALTER COLUMN amount TYPE double precision USING amount::double precision;
        `, accountRechargeTableName))
		if err != nil {
			return fmt.Errorf("failed to revert 'amount' column in 'account_recharge' table: %w", err)
		}

		return nil
	})
}
