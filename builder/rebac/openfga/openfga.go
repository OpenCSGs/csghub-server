package openfga

import (
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	fga "github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage"
	"github.com/openfga/openfga/pkg/storage/postgres"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"opencsg.com/csghub-server/builder/store/database"
)

// Config identifies the OpenFGA store and authorization model used by the Provider.
type Config struct {
	// StoreID identifies the OpenFGA store containing authorization data.
	StoreID string `json:"store_id"`
	// AuthorizationModelID identifies the authorization model used to evaluate requests.
	AuthorizationModelID string `json:"authorization_model_id"`
}

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
func (d borrowedDatastore) Close() {
	fgaServerMu.Lock()
	defer fgaServerMu.Unlock()
	fgaServer = nil
}

// getServer initializes an OpenFGA server with the pgx pool owned by the application database.DB.
func getServer() (*fga.Server, error) {
	fgaServerMu.Lock()
	defer fgaServerMu.Unlock()
	if fgaServer != nil {
		return fgaServer, nil
	}

	server, err := newServer()
	if err != nil {
		return nil, err
	}
	fgaServer = server
	return server, nil
}

func newServer() (*fga.Server, error) {
	db := database.GetDB()
	if db == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	pgx, ok := db.GetPGXPool()
	if !ok {
		return nil, fmt.Errorf("unable to connect to postgres database, pgxpool not found")
	}
	return newServerWithPGXPool(pgx)
}

// newServerWithPGXPool initializes an OpenFGA server using an application-owned pool.
func newServerWithPGXPool(pgxpool *pgxpool.Pool) (*fga.Server, error) {
	if pgxpool == nil {
		return nil, fmt.Errorf("pgxpool is nil")
	}
	datastore, err := postgres.NewWithDB(pgxpool, nil, sqlcommon.NewConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to connect to postgres database: %w", err)
	}
	server, err := fga.NewServerWithOpts(
		fga.WithDatastore(borrowedDatastore{datastore}),
		fga.WithContextPropagationToDatastore(true),
		fga.WithContinuationTokenSerializer(
			sqlcommon.NewSQLContinuationTokenSerializer(),
		),

		// Limit the pressure OpenFGA can place on its connection pool.
		fga.WithMaxConcurrentReadsForCheck(8),
		fga.WithMaxConcurrentReadsForListObjects(4),
		fga.WithMaxConcurrentReadsForListUsers(4),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize OpenFGA server: %w", err)
	}
	return server, nil
}
