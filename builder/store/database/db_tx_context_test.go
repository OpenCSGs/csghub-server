package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestRunInTxPropagatesTxToContext(t *testing.T) {
	ctx := context.Background()
	db, err := NewDB(ctx, DBConfig{Dialect: DialectSQLite, DSN: "file::memory:?cache=shared"})
	require.NoError(t, err)
	defer db.Close()

	t.Run("core RunInTx carries tx in context", func(t *testing.T) {
		err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			got, ok := ctx.Value(txCtxKey{}).(txContextValue)
			require.True(t, ok, "RunInTx closure ctx should carry the bun.Tx")
			require.Equal(t, tx, got.tx)
			require.Same(t, db.BunDB, got.owner)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("DB RunInTx carries tx and keeps raw tx operator", func(t *testing.T) {
		err := db.RunInTx(ctx, func(ctx context.Context, op Operator) error {
			got, ok := ctx.Value(txCtxKey{}).(txContextValue)
			require.True(t, ok, "DB.RunInTx closure ctx should carry the bun.Tx")
			// repository.go relies on the closure operator holding the raw bun.Tx.
			rawTx, ok := op.Core.(bun.Tx)
			require.True(t, ok, "closure Operator.Core must remain a raw bun.Tx")
			require.Equal(t, rawTx, got.tx)
			return nil
		})
		require.NoError(t, err)
	})

	t.Run("nested RunInTx keeps the outer tx in context", func(t *testing.T) {
		err := db.Core.RunInTx(ctx, nil, func(ctx context.Context, outer bun.Tx) error {
			return outer.RunInTx(ctx, nil, func(ctx context.Context, _ bun.Tx) error {
				got, ok := ctx.Value(txCtxKey{}).(txContextValue)
				require.True(t, ok)
				require.Equal(t, outer, got.tx, "nested tx shares the outer connection")
				require.Same(t, db.BunDB, got.owner, "nested tx keeps the root owner")
				return nil
			})
		})
		require.NoError(t, err)
	})
}
