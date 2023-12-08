package model

import (
	"context"

	"github.com/google/wire"
	"opencsg.com/starhub-server/config"
)

// WireSet provides a wire set for this package.
var WireSet = wire.NewSet(
	ProvideDBConfig,
	ProvideDatabse,
)

func ProvideDBConfig(config *config.Config) DBConfig {
	return DBConfig{
		Dialect: DatabaseDialect(config.Database.Driver),
		DSN:     config.Database.DSN,
	}
}

func ProvideDatabse(ctx context.Context, config DBConfig) (db *DB, err error) {
	return NewDB(ctx, config)
}
