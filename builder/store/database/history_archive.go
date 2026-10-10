package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var ErrHistoryPartitionCutoverNotActive = errors.New("history partition cutover is not active")
var ErrHistoryPartitionCutoverActive = errors.New("history partition cutover is active")

const historyPartitionDDLAdvisoryLockKey int64 = 0x4353474855504854

var defaultHistoryArchiveTables = []HistoryArchiveTable{
	{SourceTable: "account_statements", PartitionedTable: "account_statements_partitioned", PrimaryKey: "id", TimeColumn: "recorded_at", ConflictColumns: []string{"id", "recorded_at"}, syncUpdates: true},
	{SourceTable: "audit_logs", PartitionedTable: "audit_logs_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
	{SourceTable: "account_events", PartitionedTable: "account_events_partitioned", PrimaryKey: "event_uuid", TimeColumn: "created_at", ConflictColumns: []string{"event_uuid", "created_at"}},
	{SourceTable: "events", PartitionedTable: "events_partitioned", PrimaryKey: "id", TimeColumn: "created_at", ConflictColumns: []string{"id", "created_at"}},
	{SourceTable: "account_meterings", PartitionedTable: "account_meterings_partitioned", PrimaryKey: "id", TimeColumn: "recorded_at", ConflictColumns: []string{"id", "recorded_at"}},
}

type HistoryArchiveTable struct {
	SourceTable      string
	PartitionedTable string
	PrimaryKey       string
	TimeColumn       string
	ConflictColumns  []string
	// syncUpdates is reserved for append-mostly history tables that still have
	// a supported business update path, such as cancelling an account statement.
	syncUpdates bool
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
	// Cutover atomically replaces the five source history tables with their
	// fully backfilled partitioned candidates, retaining the old tables with a
	// _back suffix for rollback.
	Cutover(ctx context.Context) error
	// Rollback atomically restores the ordinary backup tables as the business
	// tables and retains the partitioned copies with a _failed_partitioned
	// suffix for investigation and controlled dual-write retry.
	Rollback(ctx context.Context) error
	// EnsureFuturePartitions keeps the five post-cutover business tables ready
	// for writes in the current quarter and the following two quarters.
	EnsureFuturePartitions(ctx context.Context) error
	// EnsureSyncTriggers attaches (when enable is true) or detaches (when false)
	// the fail-loud write-sync triggers on the append-mostly source tables, so the
	// trigger set tracks the HistoryArchive.Enable flag at startup — a real
	// on/off switch, not just a backfill switch. Idempotent: attach uses
	// DROP TRIGGER IF EXISTS + CREATE TRIGGER; detach uses DROP TRIGGER IF
	// EXISTS. The trigger functions themselves are created by the saas
	// migration; this only wires them onto (or off of) the source tables.
	EnsureSyncTriggers(ctx context.Context, enable bool) error
}

var historyCutoverIdentityTables = []string{
	"account_statements",
	"audit_logs",
	"events",
	"account_meterings",
}

type historyIdempotencySource struct {
	sourceTable   string
	triggerName   string
	registryTable string
	functionName  string
}

var historyIdempotencyTriggers = []historyIdempotencySource{
	{sourceTable: "account_events", triggerName: "account_events_register_idempotency", registryTable: "account_event_idempotencies", functionName: "register_account_event_idempotency"},
	{sourceTable: "account_meterings", triggerName: "account_meterings_register_idempotency", registryTable: "account_metering_idempotencies", functionName: "register_account_metering_idempotency"},
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

const (
	defaultHistoryCutoverLockTimeout      = "1s"
	defaultHistoryCutoverStatementTimeout = "30s"
)

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

// NewHistoryArchiveStoreWithDSN builds a store backed by a dedicated pgx
// connection. Long-running SQL is bounded with transaction-local
// statement_timeout settings rather than driver-specific DSN parameters.
func NewHistoryArchiveStoreWithDSN(dsn string) (HistoryArchiveStore, error) {
	store, _, err := NewHistoryArchiveStoreWithDSNAndClose(context.Background(), dsn)
	return store, err
}

// NewHistoryArchiveStoreWithDSNAndClose builds the same dedicated long-query
// store as NewHistoryArchiveStoreWithDSN and returns its close function for
// short-lived callers such as migration commands.
func NewHistoryArchiveStoreWithDSNAndClose(ctx context.Context, dsn string) (HistoryArchiveStore, func() error, error) {
	db, err := NewDB(ctx, DBConfig{
		Dialect: DialectPostgres,
		DSN:     dsn,
	})
	if err != nil {
		return nil, nil, err
	}
	store := &historyArchiveStoreImpl{
		db:               db,
		checkpointStore:  NewHistoryArchiveCheckpointStoreWithDB(db),
		statementTimeout: defaultHistoryArchiveStatementTimeout,
	}
	return store, db.Close, nil
}

func (s *historyArchiveStoreImpl) Backfill(ctx context.Context, batchSize int, tables []HistoryArchiveTable) ([]HistoryArchiveResult, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batchSize must be positive")
	}
	if len(tables) == 0 {
		if err := s.validateHistoryBackfillTopology(ctx); err != nil {
			return nil, err
		}
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

func (s *historyArchiveStoreImpl) validateHistoryBackfillTopology(ctx context.Context) error {
	if s.db == nil || s.db.BunDB == nil {
		return fmt.Errorf("history archive database is not initialized")
	}

	var topology string
	err := s.db.BunDB.RunInTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		for _, table := range defaultHistoryArchiveTables {
			relations := []string{
				table.SourceTable,
				table.PartitionedTable,
				table.SourceTable + "_back",
				table.SourceTable + "_failed_partitioned",
			}
			kinds := make([]string, len(relations))
			for i, relation := range relations {
				if err := tx.NewRaw(`SELECT COALESCE((
					SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)
				), '')`, relation).Scan(ctx, &kinds[i]); err != nil {
					return fmt.Errorf("inspect relation %s: %w", relation, err)
				}
			}

			var tableTopology string
			switch {
			case kinds[0] == "r" && kinds[1] == "p" && kinds[2] == "" && kinds[3] == "":
				tableTopology = "pre-cutover"
			case kinds[0] == "p" && kinds[1] == "" && kinds[2] == "r" && kinds[3] == "":
				tableTopology = "post-cutover"
			case kinds[0] == "r" && kinds[1] == "" && kinds[2] == "" && kinds[3] == "p":
				tableTopology = "rollback"
			default:
				return fmt.Errorf("invalid history backfill topology for %s: source=%q candidate=%q backup=%q failed=%q",
					table.SourceTable, kinds[0], kinds[1], kinds[2], kinds[3])
			}

			if topology == "" {
				topology = tableTopology
				continue
			}
			if topology != tableTopology {
				return fmt.Errorf("invalid mixed history backfill topology: %s is %s, previous tables are %s",
					table.SourceTable, tableTopology, topology)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if topology == "post-cutover" || topology == "rollback" {
		return ErrHistoryPartitionCutoverActive
	}
	return nil
}

// Cutover swaps all history tables in one transaction. ACCESS EXCLUSIVE locks
// stop writes between the final row-count validation and the renames. The old
// tables are deliberately retained as <source>_back for an explicit rollback;
// this operation never deletes them.
func (s *historyArchiveStoreImpl) Cutover(ctx context.Context) error {
	if s.db == nil || s.db.BunDB == nil {
		return fmt.Errorf("history archive database is not initialized")
	}

	for _, table := range defaultHistoryArchiveTables {
		if err := validateHistoryArchiveTable(table); err != nil {
			return err
		}
		if !isSafeSQLIdent(table.SourceTable + "_back") {
			return fmt.Errorf("invalid backup table %q", table.SourceTable+"_back")
		}
	}

	identityMaximums, err := s.validateHistoryCutoverPreflight(ctx)
	if err != nil {
		return fmt.Errorf("validate history cutover preflight: %w", err)
	}

	err = s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%s'", defaultHistoryCutoverLockTimeout)); err != nil {
			return fmt.Errorf("set lock_timeout: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", defaultHistoryCutoverStatementTimeout)); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
		if _, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(?)`, historyPartitionDDLAdvisoryLockKey).Exec(ctx); err != nil {
			return fmt.Errorf("lock history partition DDL: %w", err)
		}

		var lockSQL strings.Builder
		lockSQL.WriteString("LOCK TABLE ")
		for i, table := range defaultHistoryArchiveTables {
			if i > 0 {
				lockSQL.WriteString(", ")
			}
			fmt.Fprintf(&lockSQL, "%s, %s", table.SourceTable, table.PartitionedTable)
		}
		lockSQL.WriteString(" IN ACCESS EXCLUSIVE MODE")
		if _, err := tx.ExecContext(ctx, lockSQL.String()); err != nil {
			return fmt.Errorf("lock history tables: %w", err)
		}

		for _, table := range defaultHistoryArchiveTables {
			if err := validateHistoryCutoverLockedState(ctx, tx, table); err != nil {
				return err
			}
		}
		// Coverage was checked during the read-only preflight. Revalidate only
		// the cheap catalog invariant under the source-table locks: every write
		// admitted between preflight and LOCK must have used the expected live
		// registration trigger. Do not rescan the large source tables here.
		for _, idempotency := range historyIdempotencyTriggers {
			if err := validateAccountEventIdempotencySource(ctx, tx, idempotency); err != nil {
				return err
			}
		}

		sequenceNames := make(map[string]string, len(historyCutoverIdentityTables))
		for _, tableName := range historyCutoverIdentityTables {
			// The preflight maximum avoids scanning large tables while writers are
			// stopped, but a caller can still insert an explicit high ID between
			// preflight and this transaction acquiring its locks. Re-read the current
			// maximum under the lock using the source table's ID index so the promoted
			// sequence is greater than every row that actually made the cutover.
			var lockedMaximum sql.NullInt64
			if err := tx.NewRaw(fmt.Sprintf(`SELECT (SELECT id FROM %s ORDER BY id DESC LIMIT 1)`, tableName)).Scan(ctx, &lockedMaximum); err != nil {
				return fmt.Errorf("read locked identity maximum for %s: %w", tableName, err)
			}
			if lockedMaximum.Valid && (!identityMaximums[tableName].Valid || lockedMaximum.Int64 > identityMaximums[tableName].Int64) {
				identityMaximums[tableName] = lockedMaximum
			}

			candidateTable := tableName + "_partitioned"
			defaultSequence, err := resolveIdentityDefaultSequence(ctx, tx, candidateTable)
			if err != nil {
				return err
			}
			if _, err := tx.NewRaw(`ALTER SEQUENCE ? OWNED BY ?.?`,
				bun.Ident(defaultSequence), bun.Ident(candidateTable), bun.Ident("id")).Exec(ctx); err != nil {
				return fmt.Errorf("assign identity sequence to %s.id: %w", candidateTable, err)
			}

			var candidateSequence string
			if err := tx.NewRaw(`SELECT COALESCE(pg_get_serial_sequence(?, 'id'), '')`, candidateTable).Scan(ctx, &candidateSequence); err != nil {
				return fmt.Errorf("resolve identity sequence for %s.id: %w", candidateTable, err)
			}
			if candidateSequence == "" {
				return fmt.Errorf("identity sequence for %s.id is missing", candidateTable)
			}
			sequenceNames[tableName] = candidateSequence
		}

		for _, table := range defaultHistoryArchiveTables {
			triggerName := "trg_sync_" + table.SourceTable + "_partitioned"
			if !isSafeSQLIdent(triggerName) {
				return fmt.Errorf("invalid sync trigger %q", triggerName)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", triggerName, table.SourceTable)); err != nil {
				return fmt.Errorf("drop sync trigger for %s: %w", table.SourceTable, err)
			}
		}
		for _, idempotency := range historyIdempotencyTriggers {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", idempotency.triggerName, idempotency.sourceTable)); err != nil {
				return fmt.Errorf("drop pre-cutover idempotency trigger for %s: %w", idempotency.sourceTable, err)
			}
		}

		for _, table := range defaultHistoryArchiveTables {
			backupTable := table.SourceTable + "_back"
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", table.SourceTable, backupTable)); err != nil {
				return fmt.Errorf("rename %s to %s: %w", table.SourceTable, backupTable, err)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", table.PartitionedTable, table.SourceTable)); err != nil {
				return fmt.Errorf("rename %s to %s: %w", table.PartitionedTable, table.SourceTable, err)
			}
		}
		for _, table := range defaultHistoryArchiveTables {
			if err := createHistoryWriteSync(ctx, tx, table, table.SourceTable+"_back", "back", false); err != nil {
				return fmt.Errorf("create reverse sync for %s: %w", table.SourceTable, err)
			}
		}

		if err := restoreHistoryIdempotencyTriggers(ctx, tx); err != nil {
			return err
		}

		for _, tableName := range historyCutoverIdentityTables {
			maximum := identityMaximums[tableName]
			if !maximum.Valid {
				continue
			}
			query := fmt.Sprintf(`SELECT setval(?::regclass, GREATEST(last_value, ?), true) FROM %s`, sequenceNames[tableName])
			if _, err := tx.ExecContext(ctx, query, sequenceNames[tableName], maximum.Int64); err != nil {
				return fmt.Errorf("align identity sequence for %s: %w", tableName, err)
			}
		}

		for _, table := range defaultHistoryArchiveTables {
			functionName := "sync_" + table.SourceTable + "_partitioned"
			if !isSafeSQLIdent(functionName) {
				return fmt.Errorf("invalid sync function %q", functionName)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", functionName)); err != nil {
				return fmt.Errorf("drop sync function for %s: %w", table.SourceTable, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("cut over history tables: %w", err)
	}
	return nil
}

func resolveIdentityDefaultSequence(ctx context.Context, tx bun.Tx, tableName string) (string, error) {
	var sequenceName string
	if err := tx.NewRaw(`SELECT COALESCE((
		SELECT (regexp_match(
			pg_get_expr(column_default.adbin, column_default.adrelid),
			$pattern$nextval\('([^']+)'::regclass\)$pattern$
		))[1]
		FROM pg_attribute AS id_column
		JOIN pg_attrdef AS column_default
		  ON column_default.adrelid = id_column.attrelid AND column_default.adnum = id_column.attnum
		WHERE id_column.attrelid = to_regclass(?) AND id_column.attname = 'id'
	), '')`, tableName).Scan(ctx, &sequenceName); err != nil {
		return "", fmt.Errorf("inspect identity sequence for %s.id: %w", tableName, err)
	}
	if sequenceName == "" {
		return "", fmt.Errorf("identity sequence for %s.id is missing", tableName)
	}
	return sequenceName, nil
}

// Rollback reverses a completed cutover without dropping either copy. The
// restored ordinary tables become the write authority again, while fail-loud
// write-sync triggers keep the preserved partitioned copies current for a later
// retry.
func (s *historyArchiveStoreImpl) Rollback(ctx context.Context) error {
	if s.db == nil || s.db.BunDB == nil {
		return fmt.Errorf("history archive database is not initialized")
	}
	for _, table := range defaultHistoryArchiveTables {
		if err := validateHistoryArchiveTable(table); err != nil {
			return err
		}
		for _, name := range []string{table.SourceTable + "_back", table.SourceTable + "_failed_partitioned"} {
			if !isSafeSQLIdent(name) {
				return fmt.Errorf("invalid rollback table %q", name)
			}
		}
	}
	if err := s.validateHistoryRollbackPreflight(ctx); err != nil {
		return fmt.Errorf("validate history rollback preflight: %w", err)
	}

	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%s'", defaultHistoryCutoverLockTimeout)); err != nil {
			return fmt.Errorf("set lock_timeout: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", defaultHistoryCutoverStatementTimeout)); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
		if _, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(?)`, historyPartitionDDLAdvisoryLockKey).Exec(ctx); err != nil {
			return fmt.Errorf("lock history partition DDL: %w", err)
		}

		var lockSQL strings.Builder
		lockSQL.WriteString("LOCK TABLE ")
		for i, table := range defaultHistoryArchiveTables {
			if i > 0 {
				lockSQL.WriteString(", ")
			}
			fmt.Fprintf(&lockSQL, "%s, %s_back", table.SourceTable, table.SourceTable)
		}
		lockSQL.WriteString(" IN ACCESS EXCLUSIVE MODE")
		if _, err := tx.ExecContext(ctx, lockSQL.String()); err != nil {
			return fmt.Errorf("lock history rollback tables: %w", err)
		}

		if err := validateHistoryRollbackState(ctx, tx); err != nil {
			return err
		}

		sequenceNames := make(map[string]string, len(historyCutoverIdentityTables))
		maximums := make(map[string]sql.NullInt64, len(historyCutoverIdentityTables))
		for _, tableName := range historyCutoverIdentityTables {
			sequenceName, err := resolveIdentityDefaultSequence(ctx, tx, tableName+"_back")
			if err != nil {
				return err
			}
			sequenceNames[tableName] = sequenceName
			var activeMaximum, backupMaximum sql.NullInt64
			if err := tx.NewRaw(fmt.Sprintf(`SELECT (SELECT id FROM %s ORDER BY id DESC LIMIT 1)`, tableName)).Scan(ctx, &activeMaximum); err != nil {
				return fmt.Errorf("read active identity maximum for %s: %w", tableName, err)
			}
			if err := tx.NewRaw(fmt.Sprintf(`SELECT (SELECT id FROM %s_back ORDER BY id DESC LIMIT 1)`, tableName)).Scan(ctx, &backupMaximum); err != nil {
				return fmt.Errorf("read backup identity maximum for %s: %w", tableName, err)
			}
			switch {
			case activeMaximum.Valid && backupMaximum.Valid:
				maximums[tableName] = sql.NullInt64{Int64: max(activeMaximum.Int64, backupMaximum.Int64), Valid: true}
			case activeMaximum.Valid:
				maximums[tableName] = activeMaximum
			default:
				maximums[tableName] = backupMaximum
			}
		}

		for _, table := range defaultHistoryArchiveTables {
			reverseTrigger := "trg_sync_" + table.SourceTable + "_back"
			reverseFunction := "sync_" + table.SourceTable + "_back"
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", reverseTrigger, table.SourceTable)); err != nil {
				return fmt.Errorf("drop reverse sync trigger for %s: %w", table.SourceTable, err)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", reverseFunction)); err != nil {
				return fmt.Errorf("drop reverse sync function for %s: %w", table.SourceTable, err)
			}
		}
		for _, idempotency := range historyIdempotencyTriggers {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", idempotency.triggerName, idempotency.sourceTable)); err != nil {
				return fmt.Errorf("drop active idempotency trigger for %s: %w", idempotency.sourceTable, err)
			}
		}

		for _, table := range defaultHistoryArchiveTables {
			failed := table.SourceTable + "_failed_partitioned"
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", table.SourceTable, failed)); err != nil {
				return fmt.Errorf("preserve failed partition table %s: %w", table.SourceTable, err)
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s_back RENAME TO %s", table.SourceTable, table.SourceTable)); err != nil {
				return fmt.Errorf("restore backup table %s: %w", table.SourceTable, err)
			}
		}

		for _, tableName := range historyCutoverIdentityTables {
			sequence := sequenceNames[tableName]
			if _, err := tx.NewRaw(`ALTER SEQUENCE ? OWNED BY ?.?`, bun.Ident(sequence), bun.Ident(tableName), bun.Ident("id")).Exec(ctx); err != nil {
				return fmt.Errorf("restore identity sequence ownership for %s.id: %w", tableName, err)
			}
			if maximums[tableName].Valid {
				if _, err := tx.ExecContext(ctx, `SELECT setval(?::regclass, GREATEST(last_value, ?), true) FROM `+sequence, sequence, maximums[tableName].Int64); err != nil {
					return fmt.Errorf("align restored identity sequence for %s: %w", tableName, err)
				}
			}
		}

		for _, table := range defaultHistoryArchiveTables {
			if err := createHistoryWriteSync(ctx, tx, table, table.SourceTable+"_failed_partitioned", "partitioned", true); err != nil {
				return fmt.Errorf("restore sync for %s: %w", table.SourceTable, err)
			}
		}

		if err := restoreHistoryIdempotencyTriggers(ctx, tx); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("roll back history table cutover: %w", err)
	}
	return nil
}

func createHistoryWriteSync(ctx context.Context, tx bun.Tx, table HistoryArchiveTable, targetTable, suffix string, ensurePartition bool) error {
	functionName := "sync_" + table.SourceTable + "_" + suffix
	triggerName := "trg_sync_" + table.SourceTable + "_" + suffix
	for _, ident := range []string{targetTable, functionName, triggerName} {
		if !isSafeSQLIdent(ident) {
			return fmt.Errorf("invalid history sync identifier %q", ident)
		}
	}
	var ensureSQL string
	if ensurePartition {
		ensureSQL = fmt.Sprintf(`
        IF NOT EXISTS (
            SELECT 1
            FROM pg_partition_tree(to_regclass('%s')) AS tree
            JOIN pg_class AS child ON child.oid = tree.relid
            WHERE tree.isleaf
              AND NEW.%s >= (regexp_match(pg_get_expr(child.relpartbound, child.oid), $re$FROM \('([^']+)'\)$re$))[1]::timestamptz
              AND NEW.%s < (regexp_match(pg_get_expr(child.relpartbound, child.oid), $re$TO \('([^']+)'\)$re$))[1]::timestamptz
        ) THEN
            PERFORM ensure_quarter_partition('%s', NEW.%s);
        END IF;`, targetTable, table.TimeColumn, table.TimeColumn, targetTable, table.TimeColumn)
	}
	var replaceSQL string
	triggerEvents := "INSERT"
	if table.syncUpdates {
		replaceSQL = fmt.Sprintf(`
    IF TG_OP = 'UPDATE' THEN
        DELETE FROM %s WHERE %s = OLD.%s AND %s = OLD.%s;
    END IF;`, targetTable, table.PrimaryKey, table.PrimaryKey, table.TimeColumn, table.TimeColumn)
		triggerEvents = "INSERT OR UPDATE"
	}
	functionSQL := fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger AS $$
BEGIN%s%s
    INSERT INTO %s SELECT NEW.*;
    RETURN NEW;
END; $$ LANGUAGE plpgsql`, functionName, replaceSQL, ensureSQL, targetTable)
	if _, err := tx.ExecContext(ctx, functionSQL); err != nil {
		return fmt.Errorf("create function %s: %w", functionName, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s; CREATE TRIGGER %s AFTER %s ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
		triggerName, table.SourceTable, triggerName, triggerEvents, table.SourceTable, functionName)); err != nil {
		return fmt.Errorf("create trigger %s: %w", triggerName, err)
	}
	return nil
}

func restoreHistoryIdempotencyTriggers(ctx context.Context, tx bun.Tx) error {
	for _, idempotency := range historyIdempotencyTriggers {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s; CREATE TRIGGER %s BEFORE INSERT OR UPDATE OF event_uuid ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			idempotency.triggerName, idempotency.sourceTable, idempotency.triggerName, idempotency.sourceTable, idempotency.functionName)); err != nil {
			return fmt.Errorf("restore idempotency trigger for %s: %w", idempotency.sourceTable, err)
		}
	}
	return nil
}

func validateHistoryRollbackState(ctx context.Context, db bun.IDB) error {
	for _, table := range defaultHistoryArchiveTables {
		for _, relation := range []struct{ name, kind string }{
			{table.SourceTable, "p"},
			{table.SourceTable + "_back", "r"},
		} {
			var kind string
			if err := db.NewRaw(`SELECT COALESCE((SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)), '')`, relation.name).Scan(ctx, &kind); err != nil {
				return fmt.Errorf("inspect relation %s: %w", relation.name, err)
			}
			if kind != relation.kind {
				return fmt.Errorf("relation %s has kind %q, want %q", relation.name, kind, relation.kind)
			}
		}
		failed := table.SourceTable + "_failed_partitioned"
		var failedExists bool
		if err := db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, failed).Scan(ctx, &failedExists); err != nil {
			return fmt.Errorf("inspect failed relation %s: %w", failed, err)
		}
		if failedExists {
			return fmt.Errorf("failed partition relation %s already exists", failed)
		}
		triggerName := "trg_sync_" + table.SourceTable + "_back"
		functionName := "sync_" + table.SourceTable + "_back"
		var triggerReady bool
		expectedTriggerType := int16(5)
		if table.syncUpdates {
			expectedTriggerType = 21
		}
		if err := db.NewRaw(`SELECT EXISTS (
			SELECT 1
			FROM pg_trigger AS trigger
			WHERE trigger.tgname = ?
			  AND trigger.tgrelid = to_regclass(?)
			  AND NOT trigger.tgisinternal
			  AND trigger.tgenabled IN ('O', 'A')
			  AND trigger.tgtype = ?
			  AND trigger.tgfoid = to_regprocedure(?)
		)`, triggerName, table.SourceTable, expectedTriggerType, functionName+"()").Scan(ctx, &triggerReady); err != nil {
			return fmt.Errorf("inspect reverse sync trigger for %s: %w", table.SourceTable, err)
		}
		if !triggerReady {
			return fmt.Errorf("enabled reverse sync trigger %s with function %s is required on %s", triggerName, functionName, table.SourceTable)
		}
	}
	return nil
}

func (s *historyArchiveStoreImpl) validateHistoryRollbackPreflight(ctx context.Context) error {
	return s.db.BunDB.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if s.statementTimeout != "" {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", s.statementTimeout)); err != nil {
				return fmt.Errorf("set rollback preflight statement_timeout: %w", err)
			}
		}
		if err := validateHistoryRollbackState(ctx, tx); err != nil {
			return err
		}
		for _, table := range defaultHistoryArchiveTables {
			if err := validateHistoryRollbackEquality(ctx, tx, table); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateHistoryRollbackEquality(ctx context.Context, tx bun.Tx, table HistoryArchiveTable) error {
	backup := table.SourceTable + "_back"
	var activeCount, backupCount int64
	if err := tx.NewRaw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table.SourceTable)).Scan(ctx, &activeCount); err != nil {
		return fmt.Errorf("count rows in %s: %w", table.SourceTable, err)
	}
	if err := tx.NewRaw(fmt.Sprintf("SELECT COUNT(*) FROM %s", backup)).Scan(ctx, &backupCount); err != nil {
		return fmt.Errorf("count rows in %s: %w", backup, err)
	}
	if activeCount != backupCount {
		return fmt.Errorf("row count mismatch for %s: active=%d backup=%d", table.SourceTable, activeCount, backupCount)
	}

	var keysDiffer bool
	keyQuery := fmt.Sprintf(`SELECT EXISTS (
		SELECT 1 FROM (
			(SELECT %s FROM %s EXCEPT SELECT %s FROM %s)
			UNION ALL
			(SELECT %s FROM %s EXCEPT SELECT %s FROM %s)
		) AS key_difference
	)`, table.PrimaryKey, table.SourceTable, table.PrimaryKey, backup,
		table.PrimaryKey, backup, table.PrimaryKey, table.SourceTable)
	if err := tx.NewRaw(keyQuery).Scan(ctx, &keysDiffer); err != nil {
		return fmt.Errorf("compare key sets for %s: %w", table.SourceTable, err)
	}
	if keysDiffer {
		return fmt.Errorf("key set mismatch for %s on %s", table.SourceTable, table.PrimaryKey)
	}

	var contentsDiffer bool
	contentQuery := fmt.Sprintf(`SELECT EXISTS (
		SELECT 1 FROM (
			(SELECT to_jsonb(active_row) FROM %s AS active_row EXCEPT SELECT to_jsonb(backup_row) FROM %s AS backup_row)
			UNION ALL
			(SELECT to_jsonb(backup_row) FROM %s AS backup_row EXCEPT SELECT to_jsonb(active_row) FROM %s AS active_row)
		) AS content_difference
	)`, table.SourceTable, backup, backup, table.SourceTable)
	if err := tx.NewRaw(contentQuery).Scan(ctx, &contentsDiffer); err != nil {
		return fmt.Errorf("compare row contents for %s: %w", table.SourceTable, err)
	}
	if contentsDiffer {
		return fmt.Errorf("row content mismatch for %s", table.SourceTable)
	}
	return nil
}

func (s *historyArchiveStoreImpl) validateHistoryCutoverPreflight(ctx context.Context) (map[string]sql.NullInt64, error) {
	identityMaximums := make(map[string]sql.NullInt64, len(historyCutoverIdentityTables))
	err := s.db.BunDB.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		// Exact COUNT/EXCEPT/content validation legitimately scans the complete
		// history tables. Override a tight managed-Postgres session default with
		// the same migration-scoped long timeout used by backfill, before issuing
		// any validation query. The separate locked transaction retains its much
		// shorter statement timeout.
		if s.statementTimeout != "" {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", s.statementTimeout)); err != nil {
				return fmt.Errorf("set preflight statement_timeout: %w", err)
			}
		}
		for _, table := range defaultHistoryArchiveTables {
			if err := validateHistoryCutoverState(ctx, tx, table); err != nil {
				return err
			}
		}
		for _, idempotency := range historyIdempotencyTriggers {
			if err := validateAccountEventIdempotencySource(ctx, tx, idempotency); err != nil {
				return err
			}
			var missing bool
			if err := tx.NewRaw(fmt.Sprintf(`SELECT EXISTS (
				SELECT 1 FROM %s AS source
				WHERE NOT EXISTS (
					SELECT 1 FROM %s AS registry
					WHERE registry.event_uuid = source.event_uuid
				)
			)`, idempotency.sourceTable, idempotency.registryTable)).Scan(ctx, &missing); err != nil {
				return fmt.Errorf("validate idempotency coverage for %s: %w", idempotency.sourceTable, err)
			}
			if missing {
				return fmt.Errorf("idempotency coverage is incomplete for %s in %s", idempotency.sourceTable, idempotency.registryTable)
			}
		}
		for _, tableName := range historyCutoverIdentityTables {
			var maximum sql.NullInt64
			if err := tx.NewRaw(fmt.Sprintf("SELECT MAX(id) FROM %s", tableName)).Scan(ctx, &maximum); err != nil {
				return fmt.Errorf("read identity maximum for %s: %w", tableName, err)
			}
			identityMaximums[tableName] = maximum
		}
		return nil
	})
	return identityMaximums, err
}

// validateHistoryCutoverLockedState deliberately limits itself to catalog
// lookups. Exact row validation already ran in a repeatable-read preflight;
// the fail-loud dual-write trigger preserves equality until these locks stop
// writers, so repeating full table scans while locked would only extend the
// outage window.
func validateHistoryCutoverLockedState(ctx context.Context, tx bun.Tx, table HistoryArchiveTable) error {
	backupTable := table.SourceTable + "_back"
	for _, relation := range []struct {
		name string
		kind string
	}{{table.SourceTable, "r"}, {table.PartitionedTable, "p"}} {
		var kind string
		if err := tx.NewRaw(`SELECT COALESCE((SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)), '')`, relation.name).Scan(ctx, &kind); err != nil {
			return fmt.Errorf("inspect relation %s: %w", relation.name, err)
		}
		if kind != relation.kind {
			return fmt.Errorf("relation %s has kind %q, want %q", relation.name, kind, relation.kind)
		}
	}
	var backupExists bool
	if err := tx.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, backupTable).Scan(ctx, &backupExists); err != nil {
		return fmt.Errorf("inspect backup relation %s: %w", backupTable, err)
	}
	if backupExists {
		return fmt.Errorf("backup relation %s already exists", backupTable)
	}
	if err := validateHistorySyncTrigger(ctx, tx, table); err != nil {
		return err
	}
	return nil
}

func validateAccountEventIdempotencySource(ctx context.Context, tx bun.Tx, source historyIdempotencySource) error {
	var registryKind string
	if err := tx.NewRaw(`SELECT COALESCE((SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)), '')`, source.registryTable).Scan(ctx, &registryKind); err != nil {
		return fmt.Errorf("inspect %s: %w", source.registryTable, err)
	}
	if registryKind != "r" {
		return fmt.Errorf("%s must be an ordinary table", source.registryTable)
	}
	var valid bool
	if err := tx.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_trigger
		WHERE tgname = ? AND tgrelid = to_regclass(?)
		  AND NOT tgisinternal AND tgenabled IN ('O', 'A') AND tgtype = 23
		  AND tgfoid = to_regprocedure(?)
		  AND tgnargs = 0 AND octet_length(tgargs) = 0
	)`, source.triggerName, source.sourceTable, source.functionName+"()").Scan(ctx, &valid); err != nil {
		return fmt.Errorf("inspect idempotency trigger %s: %w", source.triggerName, err)
	}
	if !valid {
		return fmt.Errorf("origin-enabled BEFORE INSERT OR UPDATE row trigger %s on %s bound to %s() with no arguments is required", source.triggerName, source.sourceTable, source.functionName)
	}
	return nil
}

func validateHistorySyncTrigger(ctx context.Context, tx bun.Tx, table HistoryArchiveTable) error {
	triggerName := "trg_sync_" + table.SourceTable + "_partitioned"
	functionName := "sync_" + table.SourceTable + "_partitioned"
	// tgtype is 5 for AFTER INSERT or 21 for AFTER INSERT|UPDATE row triggers.
	// tgfoid pins the
	// trigger to the expected sync function so a same-name trigger bound to a
	// no-op or wrong function cannot pass preflight and let writes slip past the
	// dual-write that the cutover safety model assumes.
	var enabled bool
	expectedTriggerType := int16(5)
	if table.syncUpdates {
		expectedTriggerType = 21
	}
	if err := tx.NewRaw(`SELECT EXISTS (
		SELECT 1 FROM pg_trigger
		WHERE tgname = ? AND tgrelid = to_regclass(?)
		  AND NOT tgisinternal AND tgenabled IN ('O', 'A')
		  AND tgtype = ?
		  AND tgfoid = to_regprocedure(?)
	)`, triggerName, table.SourceTable, expectedTriggerType, functionName+"()").Scan(ctx, &enabled); err != nil {
		return fmt.Errorf("inspect sync trigger for %s: %w", table.SourceTable, err)
	}
	if !enabled {
		return fmt.Errorf("origin-enabled sync trigger %s on %s bound to %s is required", triggerName, table.SourceTable, functionName)
	}
	return nil
}

// EnsureFuturePartitions validates that cutover has completed and creates any
// missing quarterly partitions for the current quarter and the following two.
// Partition coverage is checked by bounds rather than child-table name because
// partitions created before cutover retain their *_partitioned_YYYYqN names
// after the parent is renamed.
func (s *historyArchiveStoreImpl) EnsureFuturePartitions(ctx context.Context) error {
	if s.db == nil || s.db.BunDB == nil {
		return fmt.Errorf("history archive database is not initialized")
	}

	err := s.db.BunDB.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		const partitionCoverageSQL = `WITH target AS (
			SELECT date_trunc('quarter', current_timestamp) + (? * interval '3 months') AS at
		)
		SELECT EXISTS (
			SELECT 1
			FROM pg_partition_tree(to_regclass(?)) AS tree
			JOIN pg_class AS child ON child.oid = tree.relid
			CROSS JOIN target
			WHERE tree.isleaf
			  AND target.at >= (regexp_match(pg_get_expr(child.relpartbound, child.oid), $re$FROM \('([^']+)'\)$re$))[1]::timestamptz
			  AND target.at < (regexp_match(pg_get_expr(child.relpartbound, child.oid), $re$TO \('([^']+)'\)$re$))[1]::timestamptz
		)`

		maintenanceTopology := ""
		for _, table := range defaultHistoryArchiveTables {
			relations := []string{table.SourceTable, table.PartitionedTable, table.SourceTable + "_back", table.SourceTable + "_failed_partitioned"}
			kinds := make([]string, len(relations))
			for i, relation := range relations {
				if !isSafeSQLIdent(relation) {
					return fmt.Errorf("invalid history table %q", relation)
				}
				if err := tx.NewRaw(`SELECT COALESCE((
					SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)
				), '')`, relation).Scan(ctx, &kinds[i]); err != nil {
					return fmt.Errorf("inspect relation %s: %w", relation, err)
				}
			}

			tableTopology := ""
			switch {
			case kinds[0] == "r" && kinds[1] == "p" && kinds[2] == "" && kinds[3] == "":
				tableTopology = "pre-cutover"
			case kinds[0] == "p" && kinds[1] == "" && kinds[2] == "r" && kinds[3] == "":
				tableTopology = "post-cutover"
			case kinds[0] == "r" && kinds[1] == "" && kinds[2] == "" && kinds[3] == "p":
				tableTopology = "rollback"
			default:
				return fmt.Errorf("invalid history partition maintenance topology for %s: source=%q candidate=%q backup=%q failed=%q",
					table.SourceTable, kinds[0], kinds[1], kinds[2], kinds[3])
			}
			if maintenanceTopology == "" {
				maintenanceTopology = tableTopology
			} else if maintenanceTopology != tableTopology {
				return fmt.Errorf("invalid mixed history partition maintenance topology: %s is %s, previous tables are %s",
					table.SourceTable, tableTopology, maintenanceTopology)
			}
		}
		if maintenanceTopology != "post-cutover" {
			return ErrHistoryPartitionCutoverNotActive
		}

		for _, table := range defaultHistoryArchiveTables {
			for quarter := 0; quarter < 3; quarter++ {
				var covered bool
				if err := tx.NewRaw(partitionCoverageSQL, quarter, table.SourceTable).Scan(ctx, &covered); err != nil {
					return fmt.Errorf("inspect quarter %d partition for %s: %w", quarter, table.SourceTable, err)
				}
				if covered {
					continue
				}
				if _, err := tx.NewRaw(`SELECT ensure_quarter_partition(
					?, date_trunc('quarter', current_timestamp) + (? * interval '3 months')
				)`, table.SourceTable, quarter).Exec(ctx); err != nil {
					return fmt.Errorf("ensure quarter %d partition for %s: %w", quarter, table.SourceTable, err)
				}
				if err := tx.NewRaw(partitionCoverageSQL, quarter, table.SourceTable).Scan(ctx, &covered); err != nil {
					return fmt.Errorf("verify quarter %d partition for %s: %w", quarter, table.SourceTable, err)
				}
				if !covered {
					return fmt.Errorf("quarter %d partition remains uncovered for %s after ensure_quarter_partition", quarter, table.SourceTable)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure future history partitions: %w", err)
	}
	return nil
}

func validateHistoryCutoverState(ctx context.Context, tx bun.Tx, table HistoryArchiveTable) error {
	backupTable := table.SourceTable + "_back"
	relations := []struct {
		name string
		kind string
	}{
		{name: table.SourceTable, kind: "r"},
		{name: table.PartitionedTable, kind: "p"},
	}
	for _, relation := range relations {
		var kind string
		if err := tx.NewRaw(`SELECT COALESCE((
			SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)
		), '')`, relation.name).Scan(ctx, &kind); err != nil {
			return fmt.Errorf("inspect relation %s: %w", relation.name, err)
		}
		if kind != relation.kind {
			return fmt.Errorf("relation %s has kind %q, want %q", relation.name, kind, relation.kind)
		}
	}

	var backupExists bool
	if err := tx.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, backupTable).Scan(ctx, &backupExists); err != nil {
		return fmt.Errorf("inspect backup relation %s: %w", backupTable, err)
	}
	if backupExists {
		return fmt.Errorf("backup relation %s already exists", backupTable)
	}
	if err := validateHistorySyncTrigger(ctx, tx, table); err != nil {
		return err
	}

	var sourceCount, candidateCount int64
	if err := tx.NewRaw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table.SourceTable)).Scan(ctx, &sourceCount); err != nil {
		return fmt.Errorf("count rows in %s: %w", table.SourceTable, err)
	}
	if err := tx.NewRaw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table.PartitionedTable)).Scan(ctx, &candidateCount); err != nil {
		return fmt.Errorf("count rows in %s: %w", table.PartitionedTable, err)
	}
	if sourceCount != candidateCount {
		return fmt.Errorf("row count mismatch for %s: source=%d candidate=%d", table.SourceTable, sourceCount, candidateCount)
	}

	keyColumn := table.PrimaryKey
	var keysDiffer bool
	keySetQuery := fmt.Sprintf(`SELECT EXISTS (
		SELECT 1 FROM (
			(SELECT %s FROM %s EXCEPT SELECT %s FROM %s)
			UNION ALL
			(SELECT %s FROM %s EXCEPT SELECT %s FROM %s)
		) AS key_difference
	)`,
		keyColumn, table.SourceTable, keyColumn, table.PartitionedTable,
		keyColumn, table.PartitionedTable, keyColumn, table.SourceTable)
	if err := tx.NewRaw(keySetQuery).Scan(ctx, &keysDiffer); err != nil {
		return fmt.Errorf("compare key sets for %s: %w", table.SourceTable, err)
	}
	if keysDiffer {
		return fmt.Errorf("key set mismatch for %s on %s", table.SourceTable, keyColumn)
	}

	var contentsDiffer bool
	contentQuery := fmt.Sprintf(`SELECT EXISTS (
		SELECT 1
		FROM %s AS source_row
		JOIN %s AS candidate_row USING (%s)
		WHERE to_jsonb(source_row) IS DISTINCT FROM to_jsonb(candidate_row)
	)`, table.SourceTable, table.PartitionedTable, keyColumn)
	if err := tx.NewRaw(contentQuery).Scan(ctx, &contentsDiffer); err != nil {
		return fmt.Errorf("compare row contents for %s: %w", table.SourceTable, err)
	}
	if contentsDiffer {
		return fmt.Errorf("row content mismatch for %s", table.SourceTable)
	}
	return nil
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

		// Ensure the quarterly partitions for these rows' time-column values
		// (created_at or recorded_at, depending on the table) exist before the
		// INSERT (the AFTER INSERT trigger on the source table does not fire
		// for direct inserts into the partitioned table).
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

// ensurePartitionsForPKs creates the quarterly partitions for the time-column
// values (created_at or recorded_at, depending on the table) of the given
// primary keys before they are inserted into the partitioned table.
func ensurePartitionsForPKs(ctx context.Context, tx bun.Tx, table HistoryArchiveTable, pks []string) error {
	// Cast pks back to the column type via a parameterized IN list so the
	// time-column values can be read and their quarters ensured.
	q := fmt.Sprintf(`SELECT ensure_quarter_partition(?, src.%s) FROM %s AS src WHERE src.%s IN (?)`,
		table.TimeColumn, table.SourceTable, table.PrimaryKey)
	_, err := tx.NewRaw(q, table.PartitionedTable, bun.In(pks)).Exec(ctx)
	return err
}

// EnsureSyncTriggers attaches or detaches fail-loud write-sync triggers
// on every source hot table. The trigger + function names are derived from the
// source table name (trg_sync_<source>_partitioned / sync_<source>_partitioned),
// matching the functions created by the saas migration. Identifiers are
// validated with isSafeSQLIdent before formatting, so a mistyped table name in
// defaultHistoryArchiveTables can never reach the SQL string.
//
// Attach is idempotent (DROP IF EXISTS then CREATE); detach is idempotent
// (DROP IF EXISTS). Before cutover, the complete ordinary-source/partitioned-
// candidate topology is required. After cutover, both operations are a no-op so
// startup cannot recreate removed phase-1 functions or detach reverse-sync
// triggers from the active partitioned tables. The completed-rollback topology
// is also a no-op so its fail-loud sync into *_failed_partitioned remains in
// place. Mixed and missing topology is rejected before any DDL runs.
func (s *historyArchiveStoreImpl) EnsureSyncTriggers(ctx context.Context, enable bool) error {
	// No DB connection (e.g. the mock-based workflow test harness, which never
	// calls InitDB): nothing to attach or detach, so this is a safe no-op
	// rather than a nil-pointer crash on s.db.BunDB.
	if s.db == nil || s.db.BunDB == nil {
		return nil
	}
	err := s.db.BunDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%s'", defaultHistoryCutoverLockTimeout)); err != nil {
			return fmt.Errorf("set lock_timeout: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL statement_timeout = '%s'", defaultHistoryCutoverStatementTimeout)); err != nil {
			return fmt.Errorf("set statement_timeout: %w", err)
		}
		if _, err := tx.NewRaw(`SELECT pg_advisory_xact_lock(?)`, historyPartitionDDLAdvisoryLockKey).Exec(ctx); err != nil {
			return fmt.Errorf("lock history partition DDL: %w", err)
		}
		preCutover := true
		postCutover := true
		postRollback := true
		relationKinds := make(map[string]string, len(defaultHistoryArchiveTables)*4)
		for _, table := range defaultHistoryArchiveTables {
			failedPartitioned := table.SourceTable + "_failed_partitioned"
			for _, ident := range []string{table.SourceTable, table.PartitionedTable, table.SourceTable + "_back", failedPartitioned} {
				if !isSafeSQLIdent(ident) {
					return fmt.Errorf("invalid history archive table %q", ident)
				}
				var kind string
				if err := tx.NewRaw(`SELECT COALESCE((SELECT relkind::text FROM pg_class WHERE oid = to_regclass(?)), '')`, ident).Scan(ctx, &kind); err != nil {
					return fmt.Errorf("inspect relation %s: %w", ident, err)
				}
				relationKinds[ident] = kind
			}
			preCutover = preCutover && relationKinds[table.SourceTable] == "r" && relationKinds[table.PartitionedTable] == "p" && relationKinds[table.SourceTable+"_back"] == "" && relationKinds[failedPartitioned] == ""
			postCutover = postCutover && relationKinds[table.SourceTable] == "p" && relationKinds[table.PartitionedTable] == "" && relationKinds[table.SourceTable+"_back"] == "r" && relationKinds[failedPartitioned] == ""
			postRollback = postRollback && relationKinds[table.SourceTable] == "r" && relationKinds[table.PartitionedTable] == "" && relationKinds[table.SourceTable+"_back"] == "" && relationKinds[failedPartitioned] == "p"
		}
		if postCutover || postRollback {
			return nil
		}
		if !preCutover {
			return fmt.Errorf("invalid history archive sync topology: relation kinds %v", relationKinds)
		}

		var b strings.Builder
		for _, table := range defaultHistoryArchiveTables {
			triggerName := "trg_sync_" + table.SourceTable + "_partitioned"
			funcName := "sync_" + table.SourceTable + "_partitioned"
			fmt.Fprintf(&b, "DROP TRIGGER IF EXISTS %s ON %s; ", triggerName, table.SourceTable)
			if enable {
				triggerEvents := "INSERT"
				if table.syncUpdates {
					triggerEvents = "INSERT OR UPDATE"
				}
				fmt.Fprintf(&b, "CREATE TRIGGER %s AFTER %s ON %s FOR EACH ROW EXECUTE FUNCTION %s(); ",
					triggerName, triggerEvents, table.SourceTable, funcName)
			}
		}
		_, err := tx.ExecContext(ctx, b.String())
		return err
	})
	if err != nil {
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
