//go:build !saas

package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func createAccountEvent(ctx context.Context, db *DB, input AccountEvent) error {
	res, err := db.Core.NewInsert().Model(&input).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("insert event log failed, error:%w", err)
	}
	return nil
}

func runWithAccountMeteringIdempotency(
	ctx context.Context,
	db *DB,
	insert func(context.Context, bun.Tx) error,
) error {
	return db.Core.RunInTx(ctx, nil, insert)
}
