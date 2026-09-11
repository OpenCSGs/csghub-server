package migrations

import (
	"context"

	"github.com/uptrace/bun"
	modelopenfga "opencsg.com/csghub-server/builder/store/database/openfga"
	"opencsg.com/csghub-server/common/types"
)

// init registers the immutable OpenFGA authorization model version 1.1 migration.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return insertAuthorizationModel(
			ctx,
			db,
			types.OpenFgaAuthorizationModelIDVer1_1,
			modelopenfga.AuthorizationModelVer1_1,
		)
	}, func(ctx context.Context, db *bun.DB) error {
		return deleteAuthorizationModel(ctx, db, types.OpenFgaAuthorizationModelIDVer1_1)
	})
}
