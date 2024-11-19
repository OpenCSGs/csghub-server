package tests

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/DATA-DOG/go-txdb"
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

// Init a test db, must call `defer db.Close()` in the test
func InitTestDB() *database.DB {
	ctx := context.TODO()
	// reuse the container, so we don't need to recreate the db for each test
	// https://github.com/testcontainers/testcontainers-go/issues/2726
	reuse := testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			req.Reuse = true
			req.Name = "csghub_test"
			return nil
		},
	)

	pc, err := postgres.Run(ctx, "docker.io/postgres:14-alpine", reuse, postgres.WithDatabase("csghub_test"),
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

	// switch to project root, so migrations can work correctly
	os.Chdir("../../../")
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
