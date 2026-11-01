package database_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
	"opencsg.com/csghub-server/common/tests"
)

var xnetDispatchIndexNames = []string{
	"idx_lfs_meta_objects_xnet_dispatch",
	"idx_repositories_xnet_migration_intent",
	"idx_lfs_meta_objects_repository_xnet_dispatch",
}

func xnetDispatchMigration(t *testing.T) migrate.Migration {
	t.Helper()
	for _, migration := range migrations.Migrations.Sorted() {
		if migration.Name == "20260924033917" {
			return migration
		}
	}
	t.Fatal("Xnet dispatch migration not registered")
	return migrate.Migration{}
}

func xnetIndexOID(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	var oid int64
	require.NoError(t, db.NewRaw("SELECT indexrelid::bigint FROM pg_index WHERE indexrelid = to_regclass(?) AND indisvalid", name).Scan(context.Background(), &oid))
	return oid
}

func TestXnetDispatchMigration_ReusesValidIndexesAndRollsBack(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	migration := xnetDispatchMigration(t)
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	oids := make([]int64, len(xnetDispatchIndexNames))
	for i, name := range xnetDispatchIndexNames {
		oids[i] = xnetIndexOID(t, db.BunDB, name)
	}
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	for i, name := range xnetDispatchIndexNames {
		require.Equal(t, oids[i], xnetIndexOID(t, db.BunDB, name), name)
	}
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	for _, name := range append(append([]string{}, xnetDispatchIndexNames...), "xnet_dispatch_rate_limits") {
		var missing bool
		require.NoError(t, db.BunDB.NewRaw("SELECT to_regclass(?) IS NULL", name).Scan(ctx, &missing))
		require.True(t, missing, name)
	}
	var columns int
	require.NoError(t, db.BunDB.NewRaw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND
		((table_name = 'repositories' AND column_name = 'xnet_migration_intent') OR
		(table_name = 'lfs_meta_objects' AND column_name IN ('xnet_dispatch_status', 'xnet_dispatch_attempts', 'xnet_dispatched_at', 'xnet_next_attempt_at')))
	`).Scan(ctx, &columns))
	require.Zero(t, columns)
}

// Cancel at a later index to reproduce an interrupted startup migration without
// relying on machine speed or building a production-sized table.
type cancelXnetIndexBuild struct{ cancel context.CancelFunc }

func (h *cancelXnetIndexBuild) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	if h.cancel != nil && strings.Contains(event.Query, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_repositories_xnet_migration_intent") {
		h.cancel()
		h.cancel = nil
	}
	return ctx
}
func (*cancelXnetIndexBuild) AfterQuery(context.Context, *bun.QueryEvent) {}

func TestXnetDispatchMigration_ResumesAfterCancellation(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	migration := xnetDispatchMigration(t)
	require.NoError(t, migration.Down(ctx, db.BunDB, nil))
	interrupted, cancel := context.WithCancel(ctx)
	defer cancel()
	db.BunDB.AddQueryHook(&cancelXnetIndexBuild{cancel: cancel})
	require.Error(t, migration.Up(interrupted, db.BunDB, nil))
	require.ErrorIs(t, interrupted.Err(), context.Canceled)
	firstOID := xnetIndexOID(t, db.BunDB, xnetDispatchIndexNames[0])
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	require.Equal(t, firstOID, xnetIndexOID(t, db.BunDB, xnetDispatchIndexNames[0]))
	for _, name := range xnetDispatchIndexNames {
		require.NotZero(t, xnetIndexOID(t, db.BunDB, name))
	}
	var rateTableExists bool
	require.NoError(t, db.BunDB.NewRaw("SELECT to_regclass('xnet_dispatch_rate_limits') IS NOT NULL").Scan(ctx, &rateTableExists))
	require.True(t, rateTableExists)
}

func TestXnetDispatchMigration_RebuildsInvalidIndex(t *testing.T) {
	db := tests.InitTransactionTestDB()
	defer db.Close()
	ctx := context.Background()
	migration := xnetDispatchMigration(t)
	firstOID := xnetIndexOID(t, db.BunDB, xnetDispatchIndexNames[0])
	name := xnetDispatchIndexNames[2]
	_, err := db.BunDB.NewRaw("DROP INDEX CONCURRENTLY ?", bun.Ident(name)).Exec(ctx)
	require.NoError(t, err)
	objects := []database.LfsMetaObject{{RepositoryID: 99191, Oid: "invalid-index-a"}, {RepositoryID: 99191, Oid: "invalid-index-b"}}
	_, err = db.BunDB.NewInsert().Model(&objects).Exec(ctx)
	require.NoError(t, err)
	// Duplicate keys make PostgreSQL leave a real INVALID concurrent-build artifact.
	_, err = db.BunDB.NewRaw("CREATE UNIQUE INDEX CONCURRENTLY ? ON lfs_meta_objects (repository_id)", bun.Ident(name)).Exec(ctx)
	require.Error(t, err)
	var valid bool
	require.NoError(t, db.BunDB.NewRaw("SELECT indisvalid FROM pg_index WHERE indexrelid = to_regclass(?)", name).Scan(ctx, &valid))
	require.False(t, valid)
	require.NoError(t, migration.Up(ctx, db.BunDB, nil))
	require.NotZero(t, xnetIndexOID(t, db.BunDB, name))
	require.Equal(t, firstOID, xnetIndexOID(t, db.BunDB, xnetDispatchIndexNames[0]))
}
