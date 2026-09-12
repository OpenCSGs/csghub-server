package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	openfgaprovider "opencsg.com/csghub-server/builder/rebac/openfga"
	modelopenfga "opencsg.com/csghub-server/builder/store/database/openfga"
	"opencsg.com/csghub-server/common/types"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add_knowledge_base_rebac_model")
		if err := insertAuthorizationModel(
			ctx,
			db,
			types.OpenFgaAuthorizationModelIDVer1_2,
			modelopenfga.AuthorizationModelVer1_2,
		); err != nil {
			return err
		}
		return backfillKnowledgeBaseRelationships(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] add_knowledge_base_rebac_model")
		if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `DELETE FROM tuple WHERE store = ? AND object_type = ?`, types.OpenFgaStoreID, rebac.ObjectTypeKnowledgeBase); err != nil {
				return fmt.Errorf("delete knowledge base OpenFGA tuples: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM changelog WHERE store = ? AND object_type = ?`, types.OpenFgaStoreID, rebac.ObjectTypeKnowledgeBase); err != nil {
				return fmt.Errorf("delete knowledge base OpenFGA changelog: %w", err)
			}
			return nil
		}); err != nil {
			return err
		}
		return deleteAuthorizationModel(ctx, db, types.OpenFgaAuthorizationModelIDVer1_2)
	})
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
	provider, err := openfgaprovider.NewCustomProvider(
		openfgaprovider.WithPGXPool(pool),
		openfgaprovider.WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_2),
	)
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
