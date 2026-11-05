package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// init adds provider-neutral identity metadata without retaining plaintext ID numbers.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `ALTER TABLE user_verifies
                ADD COLUMN provider VARCHAR NOT NULL DEFAULT 'manual',
                ADD COLUMN verification_id VARCHAR,
                ADD COLUMN id_number_encrypted TEXT,
                ADD COLUMN id_number_hash VARCHAR(64),
                ALTER COLUMN id_card_front DROP NOT NULL,
                ALTER COLUMN id_card_back DROP NOT NULL`)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `CREATE INDEX idx_user_verifies_id_number_hash ON user_verifies (id_number_hash)`)
			return err
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Legacy readers require non-null image columns, including for external rows.
			_, err := tx.ExecContext(ctx, `UPDATE user_verifies SET id_card_front = COALESCE(id_card_front, ''), id_card_back = COALESCE(id_card_back, '')`)
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `ALTER TABLE user_verifies
                DROP COLUMN id_number_hash, DROP COLUMN id_number_encrypted,
                DROP COLUMN verification_id, DROP COLUMN provider,
                ALTER COLUMN id_card_front SET NOT NULL,
                ALTER COLUMN id_card_back SET NOT NULL`)
			return err
		})
	})
}
