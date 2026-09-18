package openfga

import (
	"fmt"
	"math"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	fga "github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage"
	"github.com/openfga/openfga/pkg/storage/postgres"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var (
	fgaServer   *fga.Server
	fgaServerMu sync.Mutex
)

// borrowedDatastore delegates all datastore operations to OpenFGA's
// PostgreSQL datastore but does not close the application-owned pool.
type borrowedDatastore struct {
	storage.OpenFGADatastore
}

// Close intentionally does nothing because the application owns the pool.
func (d borrowedDatastore) Close() {}

// clearCachedServer removes the default server from the cache when that server is closed.
// Custom providers use standalone servers and must not invalidate the default server cache.
func clearCachedServer(server openFGAServer) {
	cachedServer, ok := server.(*fga.Server)
	if !ok {
		return
	}
	fgaServerMu.Lock()
	defer fgaServerMu.Unlock()
	if fgaServer == cachedServer {
		fgaServer = nil
	}
}

// getServer initializes an OpenFGA server with the pgx pool owned by the application database.DB.
func getServer(cfg *config.Config) (*fga.Server, error) {
	fgaServerMu.Lock()
	defer fgaServerMu.Unlock()
	if fgaServer != nil {
		return fgaServer, nil
	}

	server, err := newServer(cfg)
	if err != nil {
		return nil, err
	}
	fgaServer = server
	return server, nil
}

// newServer initializes OpenFGA with the application database and configuration.
func newServer(cfg *config.Config) (*fga.Server, error) {
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	pgx, ok := db.GetPGXPool()
	if !ok {
		return nil, fmt.Errorf("unable to connect to postgres database, pgxpool not found")
	}
	return newServerWithPGXPool(pgx, cfg)
}

// newServerWithPGXPool initializes an OpenFGA server using an application-owned pool.
func newServerWithPGXPool(pgxpool *pgxpool.Pool, cfg *config.Config) (*fga.Server, error) {
	if pgxpool == nil {
		return nil, fmt.Errorf("pgxpool is nil")
	}
	datastore, err := postgres.NewWithDB(pgxpool, nil, sqlcommon.NewConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to postgres database: %w", err)
	}
	return newServerWithDatastore(borrowedDatastore{datastore}, cfg)
}

// newServerWithDatastore applies application settings to an OpenFGA datastore.
func newServerWithDatastore(datastore storage.OpenFGADatastore, cfg *config.Config) (*fga.Server, error) {
	if cfg != nil && int64(cfg.Rebac.OpenFGAListObjectMaxResult) > math.MaxUint32 {
		return nil, fmt.Errorf("rebac.openfga_list_object_max_result exceeds OpenFGA uint32 range")
	}
	options := []fga.OpenFGAServiceV1Option{
		fga.WithDatastore(datastore),
		fga.WithContextPropagationToDatastore(true),
		fga.WithContinuationTokenSerializer(
			sqlcommon.NewSQLContinuationTokenSerializer(),
		),

		// Limit the pressure OpenFGA can place on its connection pool.
		fga.WithMaxConcurrentReadsForCheck(8),
		fga.WithMaxConcurrentReadsForListObjects(4),
		fga.WithMaxConcurrentReadsForListUsers(4),
	}
	if cfg != nil && cfg.Rebac.OpenFGAListObjectMaxResult > 0 {
		options = append(options, fga.WithListObjectsMaxResults(uint32(cfg.Rebac.OpenFGAListObjectMaxResult)))
	}
	server, err := fga.NewServerWithOpts(options...)
	if err != nil {
		return nil, fmt.Errorf("initialize OpenFGA server: %w", err)
	}
	return server, nil
}
