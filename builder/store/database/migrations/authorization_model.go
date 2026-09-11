package migrations

import (
	"context"
	"fmt"

	parser "github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/types"
)

// insertAuthorizationModel inserts an immutable OpenFGA authorization model version.
func insertAuthorizationModel(ctx context.Context, db *bun.DB, modelID, dsl string) error {
	schemaVersion, serialized, err := marshalAuthorizationModel(modelID, dsl)
	if err != nil {
		return err
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return insertAuthorizationModelTx(ctx, tx, modelID, schemaVersion, serialized)
	})
}

// insertAuthorizationModelTx inserts an authorization model using a caller-owned transaction.
func insertAuthorizationModelTx(ctx context.Context, tx bun.Tx, modelID, schemaVersion string, serialized []byte) error {
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
	`, types.OpenFgaStoreID, modelID, schemaVersion, serialized); err != nil {
		return fmt.Errorf("insert OpenFGA authorization model: %w", err)
	}
	return nil
}

// marshalAuthorizationModel parses an OpenFGA DSL and serializes its protobuf representation.
func marshalAuthorizationModel(modelID, dsl string) (schemaVersion string, serialized []byte, err error) {
	model, err := parser.TransformDSLToProto(dsl)
	if err != nil {
		return "", nil, fmt.Errorf("parse OpenFGA authorization model: %w", err)
	}
	model.Id = modelID

	serialized, err = sqlcommon.DeterministicMarshalOpts.Marshal(model)
	if err != nil {
		return "", nil, fmt.Errorf("marshal OpenFGA authorization model: %w", err)
	}
	return model.GetSchemaVersion(), serialized, nil
}

// deleteAuthorizationModel deletes one immutable OpenFGA authorization model version.
func deleteAuthorizationModel(ctx context.Context, db *bun.DB, modelID string) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM authorization_model
			WHERE store = ? AND authorization_model_id = ?
		`, types.OpenFgaStoreID, modelID); err != nil {
			return fmt.Errorf("delete OpenFGA authorization model: %w", err)
		}
		return nil
	})
}
