package tests

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"time"

	"github.com/DATA-DOG/go-txdb"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/migrate"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database/migrations"
)

// This is a modified version of db.go NewDB method, used in test only.
func newBun(ctx context.Context, config database.DBConfig, useTxdb bool) (bunDB *bun.DB, err error) {
	switch config.Dialect {
	case database.DialectPostgres:
		var sqlDB *sql.DB
		if useTxdb {
			sqlDB = sql.OpenDB(txdb.New("pg", config.DSN))
		} else {
			sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(config.DSN)))
		}
		bunDB = bun.NewDB(sqlDB, pgdialect.New(), bun.WithDiscardUnknownColumns())
	default:
		err = fmt.Errorf("unknown database dialect %q", config.Dialect)
		return
	}

	err = bunDB.PingContext(ctx)
	if err != nil {
		err = fmt.Errorf("pinging %s database: %w", config.Dialect, err)
		return
	}

	bunDB.RegisterModel((*database.RepositoryTag)(nil))
	bunDB.RegisterModel((*database.CollectionRepository)(nil))
	return
}

var chMu sync.Mutex

func chProjectRoot() {
	chMu.Lock()
	defer chMu.Unlock()
	for {
		_, err := os.Stat("builder/store/database/migrations")
		if err != nil {
			err = os.Chdir("../")
			if err != nil {
				panic(err)
			}
			continue
		}
		return
	}
}

var _dbSuffix = ""
var _suffixMu sync.Mutex

// Get db suffix, different packages will use different random numbers.
// We do this because the migrator can't run parallel, but different packages' tests are running parallel.
// So different packages must use different test databases to avoid migrate error.
func dbSuffix() string {
	_suffixMu.Lock()
	defer _suffixMu.Unlock()

	if _dbSuffix == "" {
		_dbSuffix = cast.ToString(rand.IntN(2 << 16))
	}
	return _dbSuffix
}

const (
	pgImage = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/csghub/postgres:15.10"
)

// Init a test db, must call `defer db.Close()` in the test
func InitTestDB() *database.DB {
	ctx := context.TODO()
	// reuse the container, so we don't need to recreate the db for each test
	// https://github.com/testcontainers/testcontainers-go/issues/2726
	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "csghub_test_" + dbSuffix()
			return nil
		},
	)

	pc, err := postgres.Run(ctx,
		pgImage,
		reuse,
		postgres.WithDatabase("csghub_test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)))
	if err != nil {
		panic(err)
	}

	// testcontainers will create a random dsn eachtime
	dsn, err := pc.ConnectionString(ctx)
	if err != nil {
		panic(err)
	}
	chProjectRoot()
	bdb, err := newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     dsn + "sslmode=disable",
	}, false)
	if err != nil {
		panic(err)
	}
	defer bdb.Close()
	bdb.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),

		// BUNDEBUG=1 logs failed queries
		// BUNDEBUG=2 logs all queries
		bundebug.FromEnv("BUNDEBUG"),
	))

	migrator := migrate.NewMigrator(bdb, migrations.Migrations)
	err = migrator.Init(ctx)
	if err != nil {
		panic(err)
	}
	_, err = migrator.Migrate(ctx)
	if err != nil {
		panic(err)
	}

	// create a new bun connection with txdb(the `true` param), so all sqls run
	// using this connection will be wrapped in a Tx automatically.
	bdb, err = newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     dsn + "sslmode=disable",
	}, true)
	if err != nil {
		panic(err)
	}
	bdb.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),
		bundebug.FromEnv("BUNDEBUG"),
	))

	return &database.DB{
		Operator: database.Operator{Core: bdb},
		BunDB:    bdb,
	}
}

// Create a random test postgres Database without txdb,
// this method is *MUCH SLOWER* than TestDB, use it only when you need to testing concurrent
// transaction execution(e.g., test concurrent select for update locks).
func InitTransactionTestDB() *database.DB {
	ctx := context.TODO()
	cname := "csghub_test_tx_" + uuid.New().String()
	// reuse the container, so we don't need to recreate the db for each test
	// https://github.com/testcontainers/testcontainers-go/issues/2726
	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = cname
			return nil
		},
	)

	pc, err := postgres.Run(ctx,
		pgImage,
		reuse, postgres.WithDatabase(cname),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)))
	if err != nil {
		panic(err)
	}

	// testcontainers will create a random dsn eachtime
	dsn, err := pc.ConnectionString(ctx)
	if err != nil {
		panic(err)
	}

	chProjectRoot()

	bdb, err := newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     dsn + "sslmode=disable",
	}, false)
	if err != nil {
		panic(err)
	}
	defer bdb.Close()
	bdb.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),

		bundebug.FromEnv("BUNDEBUG"),
	))

	migrator := migrate.NewMigrator(bdb, migrations.Migrations)
	err = migrator.Init(ctx)
	if err != nil {
		panic(err)
	}
	_, err = migrator.Migrate(ctx)
	if err != nil {
		panic(err)
	}
	bdb, err = newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     dsn + "sslmode=disable",
	}, false)
	if err != nil {
		panic(err)
	}

	return &database.DB{
		Operator: database.Operator{Core: bdb},
		BunDB:    bdb,
	}
}

func CheckZhparser(ctx context.Context, db *bun.DB, driver string) (bool, error) {
	if driver != "pg" {
		return false, nil
	}
	var count int
	err := db.NewRaw("SELECT COUNT(*) FROM pg_extension WHERE extname = 'zhparser'").
		Scan(ctx, &count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	}
	return false, nil
}
