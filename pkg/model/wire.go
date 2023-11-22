package model

import (
	"context"

	"git-devops.opencsg.com/product/community/starhub-server/config"
	"github.com/google/wire"
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
