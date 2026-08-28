package database

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var defaultHistoryArchiveTables = []HistoryArchiveTable{
	{SourceTable: "account_statements", PartitionedTable: "account_statements_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
	{SourceTable: "audit_logs", PartitionedTable: "audit_logs_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
	{SourceTable: "account_events", PartitionedTable: "account_events_partitioned", PrimaryKey: "event_uuid", TimeColumn: "created_at", ConflictColumns: []string{"event_uuid", "created_at"}},
	{SourceTable: "events", PartitionedTable: "events_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
	{SourceTable: "account_meterings", PartitionedTable: "account_meterings_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
}

type HistoryArchiveTable struct {
	SourceTable      string
	PartitionedTable string
	PrimaryKey       string
	TimeColumn       string
	ConflictColumns  []string
}

type HistoryArchiveResult struct {
	Table            string
	PartitionedTable string
	// Rows is the number of rows actually inserted (after ON CONFLICT dedup).
	Rows int64
	// Scanned is the number of candidate rows found in the source table for
	// this batch. Completion must be judged on Scanned == 0 (no more rows to
	// copy), NOT on Rows == 0 — a batch can scan rows that all conflict (Rows=0)
	// while later batches still have un-migrated history.
	Scanned     int64
	LastPK      string
	CompletedAt time.Time
}

type HistoryArchiveStore interface {
	Backfill(ctx context.Context, batchSize int, tables []HistoryArchiveTable) ([]HistoryArchiveResult, error)
	// EnsureSyncTriggers attaches (when enable is true) or detaches (when false)
	// the AFTER INSERT dual-write triggers on the source hot tables, so the
	// trigger set tracks the HistoryArchive.Enable flag at startup — a real
	// on/off switch, not just a backfill switch. Idempotent: attach uses
	// DROP TRIGGER IF EXISTS + CREATE TRIGGER; detach uses DROP TRIGGER IF
	// EXISTS. The trigger functions themselves are created by the saas
	// migration; this only wires them onto (or off of) the source tables.
	EnsureSyncTriggers(ctx context.Context, enable bool) error
}

type historyArchiveStoreImpl struct {
	db              *DB
	checkpointStore HistoryArchiveCheckpointStore
	// statementTimeout is applied as SET LOCAL statement_timeout inside each
	// backfill transaction so a tight global statement_timeout (e.g. 5s, common
	// on managed PG) does not cancel the legitimate long batch INSERT into a
	// partitioned table with several indexes. SET LOCAL scopes it to the
	// transaction only; business queries are unaffected.
	statementTimeout string
}

// defaultHistoryArchiveStatementTimeout is the per-transaction statement timeout
// for backfill. A 50k-row INSERT into a partitioned table with ~7 indexes takes
// ~13s on the test box; 10m leaves wide headroom for larger batches/tables.
const defaultHistoryArchiveStatementTimeout = "10min"

func DefaultHistoryArchiveTables() []HistoryArchiveTable {
	return slices.Clone(defaultHistoryArchiveTables)
}

func NewHistoryArchiveStore() HistoryArchiveStore {
	return NewHistoryArchiveStoreWithDB(defaultDB)
}

func NewHistoryArchiveStoreWithDB(db *DB) HistoryArchiveStore {
	return &historyArchiveStoreImpl{
		db:               db,
		checkpointStore:  NewHistoryArchiveCheckpointStoreWithDB(db),
		statementTimeout: defaultHistoryArchiveStatementTimeout,
	}
}

// NewHistoryArchiveStoreWithDSN builds a store backed by a dedicated connection
// with a long read/write timeout. The history-archive backfill runs large
// batched INSERTs into partitioned tables that carry several indexes; a single
// batch can take longer than pgdriver's default 10s read / 5s write deadline,
// which kills the activity with "i/o timeout". Using a separate connection keeps
// the generous timeout scoped to the backfill path instead of every business
// query (which should still fail fast on the default connection).
func NewHistoryArchiveStoreWithDSN(dsn string) (HistoryArchiveStore, error) {
	db, err := NewDB(context.Background(), DBConfig{
		Dialect: DialectPostgres,
		DSN:     withLongQueryTimeout(dsn),
	})
	if err != nil {
		return nil, err
	}
	return &historyArchiveStoreImpl{
		db:               db,
		checkpointStore:  NewHistoryArchiveCheckpointStoreWithDB(db),
		statementTimeout: defaultHistoryArchiveStatementTimeout,
	}, nil
}

// withLongQueryTimeout appends a generous read/write timeout to the DSN unless
// it already specifies one, so long backfill statements are not killed by
// pgdriver's 10s/5s defaults.
func withLongQueryTimeout(dsn string) string {
	if strings.Contains(dsn, "read_timeout=") && strings.Contains(dsn, "write_timeout=") {
		return dsn
	}
	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}
	prefix := dsn
	if !strings.Contains(dsn, "read_timeout=") {
		prefix += sep + "read_timeout=30m"
		sep = "&"
	}
	if !strings.Contains(dsn, "write_timeout=") {
		prefix += sep + "write_timeout=30m"
	}
	return prefix
}

func (s *historyArchiveStoreImpl) Backfill(ctx context.Context, batchSize int, tables []HistoryArchiveTable) ([]HistoryArchiveResult, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be positive")
	}
	if len(tables) == 0 {
		tables = DefaultHistoryArchiveTables()
	}

	completedAt := time.Now()
	results := make([]HistoryArchiveResult, 0, len(tables))
	for _, table := range tables {
		if err := validateHistoryArchiveTable(table); err != nil {
			return nil, err
		}

		lastPK, err := s.checkpointStore.GetLastPK(ctx, table.SourceTable)
		if err != nil {
			return nil, fmt.Errorf("load checkpoint for %s: %w", table.SourceTable, err)
		}
		inserted, scanned, newLastPK, err := s.backfillTable(ctx, table, batchSize, lastPK)
		if err != nil {
			return nil, fmt.Errorf("backfill %s: %w", table.SourceTable, err)
		}
		// The checkpoint was already advanced inside backfillTable's transaction
		// (scan + copy + checkpoint are atomic), so a crash cannot leave the
		// cursor behind the copied rows. If we crashed before commit, the whole
		// batch is rolled back and the next run re-scans the same range (all
		// conflict, Rows=0 but Scanned>0) and correctly keeps going.
		results = append(results, HistoryArchiveResult{
			Table:            table.SourceTable,
			PartitionedTable: table.PartitionedTable,
			Rows:             inserted,
			Scanned:          scanned,
			LastPK:           newLastPK,
			CompletedAt:      completedAt,
		})
	}
	return results, nil
}

// backfillTable copies one batch of rows from the source table to the
// partitioned table using keyset pagination on the primary key. It reads the
// last processed pk (lastPK, "" on first run) and returns the new high-water
// mark plus the number of rows actually inserted.
//
// The candidate scan (ensure_quarter_partition for the batch's quarters) and
// the INSERT run inside a single transaction so the candidate set is stable
// and the partition for every inserted row is guaranteed to exist.
//
// This replaces the previous NOT EXISTS anti-join, which scanned the full
// source table every batch (O(N) per batch). With the pk cursor the scan is a
// cheap range scan (O(batchSize)) and ON CONFLICT DO NOTHING dedupes rows that
// were already migrated (e.g. by the live trigger or a prior run).
//
// Safety of the one-way WHERE pk > last_pk cursor: backfill only ever copies
// committed history that already existed when the cursor was set. Live rows
// inserted after the cursor are the dual-write trigger's responsibility, and
// that trigger is fail-loud (a sync failure aborts the source insert), so a
// row is either in both tables or in neither — backfill never has to "catch up"
// a missed live row. Because the cursor scans only immutable, pre-existing
// rows, a one-way > comparison cannot skip one, even for the random-UUID
// primary key on account_events (UUIDs sort lexicographically and are immutable
// once inserted). The previous "swallow trigger errors + rely on backfill"
// design was unsafe precisely because it broke this invariant.
func (s *historyArchiveStoreImpl) backfillTable(ctx context.Context, table HistoryArchiveTable, batchSize int, lastPK string) (inserted, scanned int64, newLastPK string, err error) {
	conflictColumns := table.ConflictColumns
	if len(conflictColumns) == 0 {
		conflictColumns = []string{table.PrimaryKey, table.TimeColumn}
	}

	txErr := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Scope a generous statement timeout to this transaction so a tight
		// global statement_timeout (e.g. 5s) does not cancel the batch INSERT.
		// SET LOCAL reverts at transaction end; other sessions/queries are
		// unaffected.
		if s.statementTimeout != "" {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", s.statementTimeout)); err != nil {
				return fmt.Errorf("set statement_timeout: %w", err)
			}
		}
		// Select the next batch of primary keys strictly after the cursor.
		// ORDER BY pk + LIMIT makes this a bounded index range scan.
		pkRows := make([]string, 0, batchSize)
		scanQ, scanArgs := buildKeysetScanQuery(table, lastPK, batchSize)
		if err := tx.NewRaw(scanQ, scanArgs...).Scan(ctx, &pkRows); err != nil {
			return err
		}
		scanned = int64(len(pkRows))
		if len(pkRows) == 0 {
			newLastPK = lastPK
			return nil
		}
		newLastPK = pkRows[len(pkRows)-1]

		// Ensure the quarterly partitions for these rows' created_at values exist
		// before the INSERT (the AFTER INSERT trigger on the source table does
		// not fire for direct inserts into the partitioned table).
		if err := ensurePartitionsForPKs(ctx, tx, table, pkRows); err != nil {
			return err
		}

		// Copy the rows in pk order. ON CONFLICT DO NOTHING skips rows already
		// present in the partitioned table (idempotent across runs and against
		// the live trigger).
		insertQ, args := buildKeysetInsertQuery(table, lastPK, newLastPK, conflictColumns)
		res, err := tx.NewRaw(insertQ, args...).Exec(ctx)
		if err != nil {
			return err
		}
		inserted, err = res.RowsAffected()
		if err != nil {
			return err
		}
		// Advance the checkpoint inside the same transaction so scan+copy+
		// checkpoint are atomic. A crash before commit rolls the whole batch
		// back (cursor unchanged); the next run re-scans the same range, sees
		// all-conflict rows (Scanned>0, Rows=0) and correctly keeps going
		// instead of falsely reporting completion.
		if newLastPK != lastPK {
			if _, err := tx.NewInsert().Model(&HistoryArchiveCheckpoint{
				TableName: table.SourceTable,
				LastPK:    newLastPK,
			}).On("CONFLICT (table_name) DO UPDATE").
				Set("last_pk = EXCLUDED.last_pk").
				Set("updated_at = current_timestamp").
				Exec(ctx); err != nil {
				return fmt.Errorf("save checkpoint for %s: %w", table.SourceTable, err)
			}
		}
		return nil
	})
	return inserted, scanned, newLastPK, txErr
}

// buildKeysetScanQuery returns the SQL and args to fetch the next batchSize
// primary keys strictly after lastPK. When lastPK is empty (first run) it starts
// from the beginning. The pk is cast to text so bigint ids and uuid strings are
// returned uniformly; comparison/ordering uses the real typed column.
func buildKeysetScanQuery(table HistoryArchiveTable, lastPK string, batchSize int) (string, []any) {
	if lastPK == "" {
		return fmt.Sprintf(`SELECT src.%s::text FROM %s AS src ORDER BY src.%s LIMIT ?`,
			table.PrimaryKey, table.SourceTable, table.PrimaryKey), []any{batchSize}
	}
	return fmt.Sprintf(`SELECT src.%s::text FROM %s AS src WHERE src.%s > ? ORDER BY src.%s LIMIT ?`,
			table.PrimaryKey, table.SourceTable, table.PrimaryKey, table.PrimaryKey),
		[]any{lastPK, batchSize}
}

// buildKeysetInsertQuery returns the SQL and args to copy rows with pk in
// (lastPK, newLastPK] from the source table into the partitioned table. Order is
// irrelevant: ON CONFLICT DO NOTHING makes the insert idempotent regardless of
// row order, so ORDER BY is omitted (it would require wrapping the SELECT and
// complicates the INSERT ... ON CONFLICT syntax).
func buildKeysetInsertQuery(table HistoryArchiveTable, lastPK, newLastPK string, conflictColumns []string) (string, []any) {
	conflict := strings.Join(conflictColumns, ", ")
	if lastPK == "" {
		return fmt.Sprintf(`INSERT INTO %s SELECT src.* FROM %s AS src WHERE src.%s <= ? ON CONFLICT (%s) DO NOTHING`,
				table.PartitionedTable, table.SourceTable, table.PrimaryKey, conflict),
			[]any{newLastPK}
	}
	return fmt.Sprintf(`INSERT INTO %s SELECT src.* FROM %s AS src WHERE src.%s > ? AND src.%s <= ? ON CONFLICT (%s) DO NOTHING`,
			table.PartitionedTable, table.SourceTable, table.PrimaryKey, table.PrimaryKey, conflict),
		[]any{lastPK, newLastPK}
}

// ensurePartitionsForPKs creates the quarterly partitions for the created_at
// values of the given primary keys before they are inserted into the
// partitioned table.
func ensurePartitionsForPKs(ctx context.Context, tx bun.Tx, table HistoryArchiveTable, pks []string) error {
	// Cast pks back to the column type via a parameterized IN list so the
	// created_at values can be read and their quarters ensured.
	q := fmt.Sprintf(`SELECT ensure_quarter_partition(?, src.%s) FROM %s AS src WHERE src.%s IN (?)`,
		table.TimeColumn, table.SourceTable, table.PrimaryKey)
	_, err := tx.NewRaw(q, table.PartitionedTable, bun.In(pks)).Exec(ctx)
	return err
}

// EnsureSyncTriggers attaches or detaches the AFTER INSERT dual-write triggers
// on every source hot table. The trigger + function names are derived from the
// source table name (trg_sync_<source>_partitioned / sync_<source>_partitioned),
// matching the functions created by the saas migration. Identifiers are
// validated with isSafeSQLIdent before formatting, so a mistyped table name in
// defaultHistoryArchiveTables can never reach the SQL string.
//
// Attach is idempotent (DROP IF EXISTS then CREATE); detach is idempotent
// (DROP IF EXISTS). All statements for all tables run in a single ExecContext
// call — pgdriver's simple-query protocol executes every semicolon-separated
// statement in one round-trip, the same way bun runs multi-statement .sql
// migrations (see bun/migrate/migration.go Exec).
func (s *historyArchiveStoreImpl) EnsureSyncTriggers(ctx context.Context, enable bool) error {
	// No DB connection (e.g. the mock-based workflow test harness, which never
	// calls InitDB): nothing to attach or detach, so this is a safe no-op
	// rather than a nil-pointer crash on s.db.BunDB.
	if s.db == nil || s.db.BunDB == nil {
		return nil
	}
	var b strings.Builder
	for _, t := range defaultHistoryArchiveTables {
		for _, ident := range []string{t.SourceTable} {
			if !isSafeSQLIdent(ident) {
				return fmt.Errorf("invalid source table %q", ident)
			}
		}
		triggerName := "trg_sync_" + t.SourceTable + "_partitioned"
		funcName := "sync_" + t.SourceTable + "_partitioned"
		// DROP IF EXISTS first so attach is idempotent and so a stale trigger
		// from a prior schema (e.g. old function signature) is replaced, not
		// left dangling. Detach is just the DROP.
		fmt.Fprintf(&b, "DROP TRIGGER IF EXISTS %s ON %s; ", triggerName, t.SourceTable)
		if enable {
			fmt.Fprintf(&b, "CREATE TRIGGER %s AFTER INSERT ON %s FOR EACH ROW EXECUTE FUNCTION %s(); ",
				triggerName, t.SourceTable, funcName)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	if _, err := s.db.BunDB.ExecContext(ctx, b.String()); err != nil {
		return fmt.Errorf("ensure history archive sync triggers (enable=%v): %w", enable, err)
	}
	return nil
}

func validateHistoryArchiveTable(table HistoryArchiveTable) error {
	for name, value := range map[string]string{
		"sourceTable":      table.SourceTable,
		"partitionedTable": table.PartitionedTable,
		"primaryKey":       table.PrimaryKey,
		"timeColumn":       table.TimeColumn,
	} {
		if !isSafeSQLIdent(value) {
			return fmt.Errorf("invalid %s %q", name, value)
		}
	}
	for _, column := range table.ConflictColumns {
		if !isSafeSQLIdent(column) {
			return fmt.Errorf("invalid conflictColumn %q", column)
		}
	}
	return nil
}

func isSafeSQLIdent(value string) bool {
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, ".") {
		if part == "" {
			return false
		}
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				continue
			}
			return false
		}
	}
	return true
}
