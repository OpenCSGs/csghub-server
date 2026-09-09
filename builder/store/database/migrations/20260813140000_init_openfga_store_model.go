package migrations

import (
	"context"
	"fmt"

	parser "github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// init registers the OpenFGA store and authorization model migration.
func init() {
	const authorizationModelDSL = `
model
  schema 1.1

type user
  relations
    define owner: [user]

    define can_read: owner
    define can_write: owner

type organization
  relations
    define parent: [organization]
    define child: [organization]

    define admin: [user]
    define writer: [user]
    define reader: [user]

    define can_admin: admin or can_admin from parent
    define can_write: can_admin or writer or can_write from parent
    define can_read: can_write or reader or can_read from parent

    define member: admin or writer or reader

type namespace
  relations
    define organization: [organization]

    define owner: [user]
    define admin: [user]
    define writer: [user]
    define reader: [user]

    define can_admin: owner or admin or can_admin from organization
    define can_write: can_admin or writer or can_write from organization
    define can_read: can_write or reader or can_read from organization

type repository
  relations
    define organization: [organization]
    define organization_direct: [organization]

    define owner: [user]
    define admin: [user, organization#member]
    define writer: [user, organization#member]
    define reader: [user, organization#member]

    define can_admin: owner or admin or can_admin from organization or admin from organization_direct
    define can_write: can_admin or writer or can_write from organization or writer from organization_direct
    define can_read: can_write or reader or can_read from organization or reader from organization_direct
`

	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		model, err := parser.TransformDSLToProto(authorizationModelDSL)
		if err != nil {
			return fmt.Errorf("parser.TransformDSLToProto: %w", err)
		}

		schemaVersion := model.GetSchemaVersion()

		model.Id = types.OpenFgaAuthorizationModelID
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
			`, types.OpenFgaStoreID, types.OpenFgaAuthorizationModelID, schemaVersion, pbdata); err != nil {
				return fmt.Errorf("upsert OpenFGA authorization model: %w", err)
			}

			return nil
		})
	}, func(ctx context.Context, db *bun.DB) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `
				DELETE FROM authorization_model
				WHERE store = ? AND authorization_model_id = ?
			`, types.OpenFgaStoreID, types.OpenFgaAuthorizationModelID); err != nil {
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
