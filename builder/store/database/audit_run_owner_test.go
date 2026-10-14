//go:build ee || saas

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRunAuditReusesTxOnlyForOwnerHandle(t *testing.T) {
	ctx := context.Background()
	dbA, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: "file:txownera?mode=memory&cache=shared"})
	require.NoError(t, err)
	defer dbA.Close()
	dbB, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: "file:txownerb?mode=memory&cache=shared"})
	require.NoError(t, err)
	defer dbB.Close()

	err = dbA.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Same handle: audit work reuses the transaction connection.
		err := runAudit(ctx, dbA.BunDB, func(_ context.Context, db bun.IDB) error {
			require.Equal(t, tx, db)
			return nil
		})
		require.NoError(t, err)

		// Different handle: fall back to that handle's pool instead of running
		// audit statements on the wrong instance's transaction.
		err = runAudit(ctx, dbB.BunDB, func(_ context.Context, db bun.IDB) error {
			require.Same(t, dbB.BunDB, db)
			return nil
		})
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)
}
