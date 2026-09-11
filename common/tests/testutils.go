package tests

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DATA-DOG/go-txdb"
	"github.com/google/uuid"
	redisclient "github.com/redis/go-redis/v9"
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
	pgImage = "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsghq/postgres:15.18"
)

const transactionTestDBTemplate = "csghub_test_tx_template"

const redisTestDatabaseCount = 1024

var (
	transactionTestDBOnce     sync.Once
	transactionTestDBInitErr  error
	transactionTestDBAdminDSN string
	transactionTestDBCloneMu  sync.Mutex

	redisTestContainerOnce    sync.Once
	redisTestContainerInitErr error
	redisTestContainerAddr    string
	redisTestDatabaseIndex    atomic.Int64
)

// Init a test db, must call `defer db.Close()` in the test
func InitTestDB() *database.DB {
	ctx := context.TODO()
	cname := "csghub_test_" + dbSuffix()

	// reuse the container, so we don't need to recreate the db for each test
	// https://github.com/testcontainers/testcontainers-go/issues/2726
	reuse := testcontainers.WithReuseByName(cname)

	pc, err := postgres.Run(ctx,
		pgImage,
		reuse,
		postgres.WithDatabase("csghub_test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(120*time.Second)))
	if err != nil {
		panic(err)
	}

	// testcontainers will create a random dsn eachtime
	dsn, err := pc.ConnectionString(ctx)
	if err != nil {
		panic(err)
	}
	chProjectRoot()
	dbConfig := database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     dsn + "sslmode=disable",
	}
	migrationDB, err := database.NewDB(ctx, dbConfig)
	if err != nil {
		panic(err)
	}
	defer migrationDB.Close()
	migrationDB.BunDB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),

		// BUNDEBUG=1 logs failed queries
		// BUNDEBUG=2 logs all queries
		bundebug.FromEnv("BUNDEBUG"),
	))

	migrator := migrate.NewMigrator(migrationDB.BunDB, migrations.Migrations)
	migrationCtx := migrations.WithDatabase(ctx, migrationDB)
	err = migrator.Init(migrationCtx)
	if err != nil {
		panic(err)
	}
	_, err = migrator.Migrate(migrationCtx)
	if err != nil {
		panic(err)
	}

	// create a new bun connection with txdb(the `true` param), so all sqls run
	// using this connection will be wrapped in a Tx automatically.
	bdb, err := newBun(ctx, database.DBConfig{
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
	transactionTestDBOnce.Do(func() {
		transactionTestDBInitErr = initTransactionTestDBTemplate()
	})
	if transactionTestDBInitErr != nil {
		panic(transactionTestDBInitErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	databaseName := "csghub_test_tx_" + uuid.New().String()
	transactionTestDBCloneMu.Lock()
	err := cloneTransactionTestDB(ctx, databaseName)
	transactionTestDBCloneMu.Unlock()
	if err != nil {
		panic(err)
	}

	bdb, err := newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     transactionTestDatabaseDSN(databaseName),
	}, false)
	if err != nil {
		panic(err)
	}

	return &database.DB{
		Operator: database.Operator{Core: bdb},
		BunDB:    bdb,
	}
}

func initTransactionTestDBTemplate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	containerName := "csghub_test_tx_" + dbSuffix()
	pc, err := postgres.Run(ctx,
		pgImage,
		testcontainers.WithReuseByName(containerName),
		postgres.WithDatabase(transactionTestDBTemplate),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(180*time.Second)))
	if err != nil {
		return fmt.Errorf("start transaction test database container: %w", err)
	}

	templateDSN, err := pc.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return fmt.Errorf("get transaction test database connection string: %w", err)
	}
	transactionTestDBAdminDSN = transactionTestDatabaseDSNWithTimeouts(
		transactionTestDatabaseDSNFrom(templateDSN, "postgres"),
	)

	chProjectRoot()
	dbConfig := database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     templateDSN,
	}
	migrationDB, err := database.NewDB(ctx, dbConfig)
	if err != nil {
		return fmt.Errorf("open transaction test database template: %w", err)
	}
	migrationDB.BunDB.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(false),

		bundebug.FromEnv("BUNDEBUG"),
	))

	migrator := migrate.NewMigrator(migrationDB.BunDB, migrations.Migrations)
	migrationCtx := migrations.WithDatabase(ctx, migrationDB)
	err = migrator.Init(migrationCtx)
	if err != nil {
		migrationDB.Close()
		return fmt.Errorf("initialize transaction test database migrations: %w", err)
	}
	_, err = migrator.Migrate(migrationCtx)
	if err != nil {
		migrationDB.Close()
		return fmt.Errorf("migrate transaction test database template: %w", err)
	}
	if err := migrationDB.Close(); err != nil {
		return fmt.Errorf("close transaction test database template: %w", err)
	}
	return prepareTransactionTestDBTemplate(ctx)
}

func prepareTransactionTestDBTemplate(ctx context.Context) error {
	adminDB, err := newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     transactionTestDBAdminDSN,
	}, false)
	if err != nil {
		return fmt.Errorf("open transaction test database admin connection: %w", err)
	}
	defer adminDB.Close()

	if _, err := adminDB.NewRaw(
		"ALTER DATABASE ? WITH ALLOW_CONNECTIONS false",
		bun.Ident(transactionTestDBTemplate),
	).Exec(ctx); err != nil {
		return fmt.Errorf("disable transaction test database template connections: %w", err)
	}
	if _, err := adminDB.NewRaw(
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = ? AND pid <> pg_backend_pid()",
		transactionTestDBTemplate,
	).Exec(ctx); err != nil {
		return fmt.Errorf("close transaction test database template connections: %w", err)
	}
	return nil
}

func cloneTransactionTestDB(ctx context.Context, databaseName string) error {
	adminDB, err := newBun(ctx, database.DBConfig{
		Dialect: database.DialectPostgres,
		DSN:     transactionTestDBAdminDSN,
	}, false)
	if err != nil {
		return fmt.Errorf("open transaction test database admin connection: %w", err)
	}
	defer adminDB.Close()

	if _, err := adminDB.NewRaw(
		"CREATE DATABASE ? TEMPLATE ?",
		bun.Ident(databaseName),
		bun.Ident(transactionTestDBTemplate),
	).Exec(ctx); err != nil {
		return fmt.Errorf("clone transaction test database %q: %w", databaseName, err)
	}
	return nil
}

func transactionTestDatabaseDSN(databaseName string) string {
	return transactionTestDatabaseDSNFrom(transactionTestDBAdminDSN, databaseName)
}

func transactionTestDatabaseDSNFrom(dsn, databaseName string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		panic(fmt.Errorf("parse transaction test database connection string: %w", err))
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}

func transactionTestDatabaseDSNWithTimeouts(dsn string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		panic(fmt.Errorf("parse transaction test database connection string: %w", err))
	}
	query := parsed.Query()
	query.Set("dial_timeout", "30s")
	query.Set("read_timeout", "30s")
	query.Set("write_timeout", "30s")
	parsed.RawQuery = query.Encode()
	return parsed.String()
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

// InitTestRedis initializes a test Redis instance using testcontainers
// Must call `defer client.Close()` in the test
func InitTestRedis() *redisclient.Client {
	redisTestContainerOnce.Do(func() {
		redisTestContainerInitErr = initTestRedisContainer()
	})
	if redisTestContainerInitErr != nil {
		panic(redisTestContainerInitErr)
	}

	databaseIndex := redisTestDatabaseIndex.Add(1) - 1
	if databaseIndex >= redisTestDatabaseCount {
		panic(fmt.Errorf(
			"allocate Redis test database: index %d exceeds configured count %d",
			databaseIndex,
			redisTestDatabaseCount,
		))
	}

	client := redisclient.NewClient(&redisclient.Options{
		Addr: redisTestContainerAddr,
		DB:   int(databaseIndex),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		panic(fmt.Errorf("pinging Redis test database %d: %w", databaseIndex, err))
	}
	return client
}

func initTestRedisContainer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cname := "csghub_test_redis_" + dbSuffix()
	req := testcontainers.ContainerRequest{
		Image:        "opencsg-registry.cn-beijing.cr.aliyuncs.com/opencsg_public/redis:7.2.5",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("* Ready to accept connections"),
		Name:         cname,
		Cmd:          []string{"redis-server", "--databases", fmt.Sprint(redisTestDatabaseCount)},
	}

	containerRequest := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Reuse:            true, // reuse the container, so we don't need to recreate the redis for each test
	}

	container, err := testcontainers.GenericContainer(ctx, containerRequest)
	if err != nil {
		return fmt.Errorf("start Redis test container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return fmt.Errorf("get Redis test container port: %w", err)
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("get Redis test container host: %w", err)
	}
	redisTestContainerAddr = fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())
	return nil
}
