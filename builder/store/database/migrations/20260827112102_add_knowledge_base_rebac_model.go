package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	parser "github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/storage/sqlcommon"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	openfgaprovider "opencsg.com/csghub-server/builder/rebac/openfga"
	"opencsg.com/csghub-server/common/types"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add_knowledge_base_rebac_model")
		if err := installKnowledgeBaseAuthorizationModel(ctx, db); err != nil {
			return err
		}
		return backfillKnowledgeBaseRelationships(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] add_knowledge_base_rebac_model")
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `DELETE FROM tuple WHERE store = ? AND object_type = ?`, types.OpenFgaStoreID, rebac.ObjectTypeKnowledgeBase); err != nil {
				return fmt.Errorf("delete knowledge base OpenFGA tuples: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM changelog WHERE store = ? AND object_type = ?`, types.OpenFgaStoreID, rebac.ObjectTypeKnowledgeBase); err != nil {
				return fmt.Errorf("delete knowledge base OpenFGA changelog: %w", err)
			}
			return nil
		})
	})
}

const knowledgeBaseAuthorizationModelDSL = `
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

type knowledge_base
  relations
    define namespace: [namespace]
    define public: [user:*]

    define can_admin: can_admin from namespace
    define can_write: can_write from namespace
    define can_read: public or can_read from namespace
`

func installKnowledgeBaseAuthorizationModel(ctx context.Context, db *bun.DB) error {
	model, err := parser.TransformDSLToProto(knowledgeBaseAuthorizationModelDSL)
	if err != nil {
		return fmt.Errorf("parse knowledge base OpenFGA authorization model: %w", err)
	}
	model.Id = types.OpenFgaAuthorizationModelID
	pbdata, err := sqlcommon.DeterministicMarshalOpts.Marshal(model)
	if err != nil {
		return fmt.Errorf("marshal knowledge base OpenFGA authorization model: %w", err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO authorization_model (
			store, authorization_model_id, schema_version, type, type_definition, serialized_protobuf
		) VALUES (?, ?, ?, '', NULL, ?)
		ON CONFLICT (authorization_model_id) DO UPDATE
		SET store = EXCLUDED.store,
			schema_version = EXCLUDED.schema_version,
			type = EXCLUDED.type,
			type_definition = EXCLUDED.type_definition,
			serialized_protobuf = EXCLUDED.serialized_protobuf
	`, types.OpenFgaStoreID, types.OpenFgaAuthorizationModelID, model.GetSchemaVersion(), pbdata)
	if err != nil {
		return fmt.Errorf("upsert knowledge base OpenFGA authorization model: %w", err)
	}
	return nil
}

func backfillKnowledgeBaseRelationships(ctx context.Context) error {
	appDB, ok := DatabaseFromContext(ctx)
	if !ok {
		return fmt.Errorf("application database is missing from migration context")
	}
	pool, ok := appDB.GetPGXPool()
	if !ok {
		return fmt.Errorf("knowledge base OpenFGA backfill requires a PostgreSQL pgx pool")
	}
	provider, err := openfgaprovider.NewProviderWithPGXPool(pool)
	if err != nil {
		return fmt.Errorf("initialize OpenFGA provider for knowledge base backfill: %w", err)
	}
	defer provider.Close()
	authorizer, err := rebac.NewAuthorizer(provider)
	if err != nil {
		return fmt.Errorf("initialize OpenFGA authorizer for knowledge base backfill: %w", err)
	}
	stats := newTupleBackfillStats("knowledge_bases")
	if err := backfillKnowledgeBases(ctx, pool, authorizer, stats); err != nil {
		return err
	}
	slog.InfoContext(ctx, "OpenFGA knowledge base tuple backfill completed",
		"candidates", stats.candidates,
		"existing", stats.existing,
		"written", stats.written,
		"skipped", stats.skipped,
	)
	return nil
}

func backfillKnowledgeBases(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT id, COALESCE(ns_uuid, ''), public
			FROM agent_knowledge_bases
			WHERE id > $1
			ORDER BY id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query knowledge bases for OpenFGA backfill: %w", err)
		}
		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize*2)
		rowCount := 0
		for rows.Next() {
			var id int64
			var namespaceUUID string
			var public bool
			if err := rows.Scan(&id, &namespaceUUID, &public); err != nil {
				rows.Close()
				return fmt.Errorf("scan knowledge base for OpenFGA backfill: %w", err)
			}
			rowCount++
			lastID = id
			if !validTupleID(namespaceUUID) {
				stats.skip("knowledge_base_missing_namespace_uuid")
				continue
			}
			object := rebac.KnowledgeBaseObject(id)
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.NewSubject(rebac.ObjectTypeNamespace, namespaceUUID),
				Relation: rebac.RelationNamespace,
				Object:   object,
			})
			if public {
				relationships = append(relationships, rebac.Relationship{
					Subject:  rebac.PublicUserWildcard(),
					Relation: rebac.RelationPublic,
					Object:   object,
				})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read knowledge bases for OpenFGA backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write knowledge base OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}
