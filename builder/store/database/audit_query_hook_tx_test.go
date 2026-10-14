//go:build ee || saas

package database_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
)

// auditHookWidget is a throwaway model so these tests don't depend on larger models.
type auditHookWidget struct {
	bun.BaseModel `bun:"table:audit_hook_widgets"`
	ID            int64  `bun:",pk,autoincrement"`
	Name          string `bun:",notnull"`
}

func setupAuditHookTestDB(t *testing.T) *database.DB {
	t.Helper()
	// The audit hook gates on the process-global config; enable it before the
	// hook is constructed in NewDB.
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	cfg.AuditLog.Enabled = true
	db := tests.InitTransactionTestDBWithHooks()
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.BunDB.NewRaw(
		"CREATE TABLE audit_hook_widgets (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)",
	).Exec(context.Background())
	require.NoError(t, err)
	return db
}

func widgetAuditLogs(t *testing.T, db *database.DB) []database.AuditLog {
	t.Helper()
	var logs []database.AuditLog
	err := db.BunDB.NewSelect().Model(&logs).
		Where("table_name = ?", "audit_hook_widgets").
		Scan(context.Background())
	require.NoError(t, err)
	return logs
}

func updateWidgetInTx(db *database.DB, ctx context.Context, widget *auditHookWidget, after func() error) error {
	return db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		if _, err := tx.Core.NewUpdate().Model(widget).Where("id = ?", widget.ID).Exec(ctx); err != nil {
			return err
		}
		if after != nil {
			return after()
		}
		return nil
	})
}

func TestAuditHookTxCommitWritesAuditRow(t *testing.T) {
	db := setupAuditHookTestDB(t)
	ctx := context.Background()

	widget := &auditHookWidget{Name: "before"}
	_, err := db.BunDB.NewInsert().Model(widget).Exec(ctx)
	require.NoError(t, err)

	widget.Name = "after"
	require.NoError(t, updateWidgetInTx(db, ctx, widget, nil))

	logs := widgetAuditLogs(t, db)
	require.Len(t, logs, 1)
	require.Contains(t, string(logs[0].Before), "before")
	require.Contains(t, string(logs[0].After), "after")
}

func TestAuditHookTxSurvivesAuditFailure(t *testing.T) {
	db := setupAuditHookTestDB(t)
	ctx := context.Background()

	_, err := db.BunDB.NewRaw("ALTER TABLE audit_logs RENAME TO audit_logs_bak").Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.BunDB.NewRaw("ALTER TABLE audit_logs_bak RENAME TO audit_logs").Exec(context.Background())
	})

	widget := &auditHookWidget{Name: "before"}
	_, err = db.BunDB.NewInsert().Model(widget).Exec(ctx)
	require.NoError(t, err)

	widget.Name = "after"
	require.NoError(t, updateWidgetInTx(db, ctx, widget, nil),
		"audit insert failure must not abort the business transaction")

	var name string
	require.NoError(t, db.BunDB.NewSelect().Model((*auditHookWidget)(nil)).
		Column("name").Where("id = ?", widget.ID).Scan(ctx, &name))
	require.Equal(t, "after", name)
}

func TestAuditHookTxRollbackDropsAuditRow(t *testing.T) {
	db := setupAuditHookTestDB(t)
	ctx := context.Background()

	widget := &auditHookWidget{Name: "before"}
	_, err := db.BunDB.NewInsert().Model(widget).Exec(ctx)
	require.NoError(t, err)

	sentinel := errors.New("boom")
	widget.Name = "after"
	err = updateWidgetInTx(db, ctx, widget, func() error { return sentinel })
	require.ErrorIs(t, err, sentinel)

	require.Empty(t, widgetAuditLogs(t, db), "audit row must roll back with the business transaction")

	var name string
	require.NoError(t, db.BunDB.NewSelect().Model((*auditHookWidget)(nil)).
		Column("name").Where("id = ?", widget.ID).Scan(ctx, &name))
	require.Equal(t, "before", name)
}

func TestAuditHookTxSavepointCleanupAfterFailure(t *testing.T) {
	db := setupAuditHookTestDB(t)
	ctx := context.Background()

	_, err := db.BunDB.NewRaw("ALTER TABLE audit_logs RENAME TO audit_logs_bak").Exec(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.BunDB.NewRaw("ALTER TABLE audit_logs_bak RENAME TO audit_logs").Exec(context.Background())
	})

	widget := &auditHookWidget{Name: "before"}
	_, err = db.BunDB.NewInsert().Model(widget).Exec(ctx)
	require.NoError(t, err)

	widget.Name = "after"
	err = db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		// The audit insert fails and rolls back to the audit savepoint.
		if _, err := tx.Core.NewUpdate().Model(widget).Where("id = ?", widget.ID).Exec(ctx); err != nil {
			return err
		}
		// A nested transaction after the failed audit must still commit cleanly.
		bunTx, ok := tx.Core.(bun.Tx)
		if !ok {
			return fmt.Errorf("unexpected operator core type %T", tx.Core)
		}
		return bunTx.RunInTx(ctx, nil, func(ctx context.Context, ntx bun.Tx) error {
			widget.Name = "nested"
			_, err := ntx.NewUpdate().Model(widget).Where("id = ?", widget.ID).Exec(ctx)
			return err
		})
	})
	require.NoError(t, err, "a failed audit must not leave savepoint state that breaks later nested transactions")

	var name string
	require.NoError(t, db.BunDB.NewSelect().Model((*auditHookWidget)(nil)).
		Column("name").Where("id = ?", widget.ID).Scan(ctx, &name))
	require.Equal(t, "nested", name)
}

func TestAuditHookTxPoolExhaustionNoDeadlock(t *testing.T) {
	db := setupAuditHookTestDB(t)
	db.BunDB.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	widget := &auditHookWidget{Name: "before"}
	_, err := db.BunDB.NewInsert().Model(widget).Exec(ctx)
	require.NoError(t, err)

	widget.Name = "after"
	require.NoError(t, updateWidgetInTx(db, ctx, widget, nil),
		"audited update must reuse the transaction connection instead of waiting for the pool")
	require.Len(t, widgetAuditLogs(t, db), 1)
}

// auditLogSkipModel maps to the hardcoded skip-list table name "auditlog", so
// this test does not depend on the process-global config skip list.
type auditLogSkipModel struct {
	bun.BaseModel `bun:"table:auditlog"`
	ID            int64  `bun:",pk,autoincrement"`
	Name          string `bun:",notnull"`
}

type selectCountingHook struct {
	table string
	count atomic.Int32
}

func (h *selectCountingHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *selectCountingHook) AfterQuery(_ context.Context, event *bun.QueryEvent) {
	if event == nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(event.Operation()), "SELECT") &&
		strings.Contains(event.Query, h.table) {
		h.count.Add(1)
	}
}

func TestAuditHookSkipsSnapshotForSkipListedTable(t *testing.T) {
	db := setupAuditHookTestDB(t)
	ctx := context.Background()

	_, err := db.BunDB.NewRaw(
		"CREATE TABLE auditlog (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)",
	).Exec(ctx)
	require.NoError(t, err)

	counter := &selectCountingHook{table: "auditlog"}
	db.BunDB.AddQueryHook(counter)

	row := &auditLogSkipModel{Name: "before"}
	_, err = db.BunDB.NewInsert().Model(row).Exec(ctx)
	require.NoError(t, err)

	row.Name = "after"
	err = db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		_, err := tx.Core.NewUpdate().Model(row).Where("id = ?", row.ID).Exec(ctx)
		return err
	})
	require.NoError(t, err)

	require.Zero(t, counter.count.Load(),
		"a skip-listed table must not pay a before-image snapshot read")
	var logs []database.AuditLog
	require.NoError(t, db.BunDB.NewSelect().Model(&logs).
		Where("table_name = ?", "auditlog").Scan(ctx))
	require.Empty(t, logs, "a skip-listed table must not produce audit rows")
}
