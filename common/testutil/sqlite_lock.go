// Package testutil provides database-independent fixtures for tests without importing business stores.
package testutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"sync"

	"github.com/uptrace/bun/driver/sqliteshim"
)

// registerSQLiteLockDriver registers the test-only transaction-lock adapter once per test process.
var registerSQLiteLockDriver sync.Once

// OpenSQLiteWithTransactionLocks emulates PostgreSQL advisory transaction locks using SQLite's writer lock.
// It preserves real SQLite transactions and queries; it does not validate PostgreSQL lock semantics.
func OpenSQLiteWithTransactionLocks(dsn string) (*sql.DB, error) {
	const name = "csghub_sqlite_transaction_lock_test"
	registerSQLiteLockDriver.Do(func() { sql.Register(name, sqliteLockDriver{sqliteshim.Driver()}) })
	return sql.Open(name, dsn)
}

// sqliteLockDriver wraps the project's selected SQLite driver exclusively for test fixtures.
type sqliteLockDriver struct{ driver.Driver }

// Open preserves the underlying connection and adds advisory-lock statement translation.
func (d sqliteLockDriver) Open(dsn string) (driver.Conn, error) {
	connection, err := d.Driver.Open(dsn)
	if err != nil {
		return nil, err
	}
	return sqliteLockConn{connection}, nil
}

// sqliteLockConn forwards ordinary operations to SQLite while emulating advisory locks inside the same transaction.
type sqliteLockConn struct{ driver.Conn }

// BeginTx forwards transaction options to the project's context-aware SQLite driver.
func (c sqliteLockConn) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if connection, ok := c.Conn.(driver.ConnBeginTx); ok {
		return connection.BeginTx(ctx, options)
	}
	return nil, errors.New("SQLite test driver does not support BeginTx")
}

// ExecContext translates only the lock statement; all other writes retain their actual SQL and arguments.
func (c sqliteLockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.HasPrefix(query, "SELECT pg_advisory_xact_lock(") {
		query = "UPDATE roles SET role_code = role_code WHERE is_admin = TRUE"
	}
	if connection, ok := c.Conn.(driver.ExecerContext); ok {
		return connection.ExecContext(ctx, query, args)
	}
	return nil, driver.ErrSkip
}

// Prepare applies the same lock translation when a driver uses database/sql's prepared-statement fallback.
func (c sqliteLockConn) Prepare(query string) (driver.Stmt, error) {
	if strings.HasPrefix(query, "SELECT pg_advisory_xact_lock(") {
		query = "UPDATE roles SET role_code = role_code WHERE is_admin = TRUE"
	}
	return c.Conn.Prepare(query)
}
