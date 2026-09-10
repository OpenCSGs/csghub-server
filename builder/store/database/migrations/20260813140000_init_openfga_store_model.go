package migrations

import (
	"context"
	"fmt"

	parser "github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"github.com/uptrace/bun"
	modelopenfga "opencsg.com/csghub-server/builder/store/database/openfga"
	"opencsg.com/csghub-server/common/types"
)

// init registers the OpenFGA store and authorization model migration.
func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		model, err := parser.TransformDSLToProto(modelopenfga.AuthorizationModelVer1_0)
		if err != nil {
			return fmt.Errorf("parser.TransformDSLToProto: %w", err)
		}

		schemaVersion := model.GetSchemaVersion()

		model.Id = types.OpenFgaAuthorizationModelIDVer1_0
		pbdata, err := sqlcommon.DeterministicMarshalOpts.Marshal(model)
		if err != nil {
			return fmt.Errorf("marshal OpenFGA authorization model: %w", err)
		}

		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO store (id, name, created_at, updated_at, deleted_at)
				VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, NULL)
				ON CONFLICT (id) DO UPDATE
				SET name = EXCLUDED.name,
					updated_at = CURRENT_TIMESTAMP,
					deleted_at = NULL
			`, types.OpenFgaStoreID, types.OpenFgaStoreName); err != nil {
				return fmt.Errorf("upsert OpenFGA store: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				INSERT INTO authorization_model (
					store,
					authorization_model_id,
					schema_version,
					type,
					type_definition,
					serialized_protobuf
				)
				VALUES (?, ?, ?, '', NULL, ?)
				ON CONFLICT (authorization_model_id) DO UPDATE
				SET store = EXCLUDED.store,
					schema_version = EXCLUDED.schema_version,
					type = EXCLUDED.type,
					type_definition = EXCLUDED.type_definition,
					serialized_protobuf = EXCLUDED.serialized_protobuf
			`, types.OpenFgaStoreID, types.OpenFgaAuthorizationModelIDVer1_0, schemaVersion, pbdata); err != nil {
				return fmt.Errorf("upsert OpenFGA authorization model: %w", err)
			}

			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM authorization_model
				WHERE store = ? AND authorization_model_id = ?
			`, types.OpenFgaStoreID, types.OpenFgaAuthorizationModelIDVer1_0); err != nil {
				return fmt.Errorf("delete OpenFGA authorization model: %w", err)
			}

			if _, err := tx.ExecContext(ctx, `
				DELETE FROM store
				WHERE id = ?
			`, types.OpenFgaStoreID); err != nil {
				return fmt.Errorf("delete OpenFGA store: %w", err)
			}

			return nil
		})
	})
}
