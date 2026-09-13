package migrations

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	openfgaprovider "opencsg.com/csghub-server/builder/rebac/openfga"
	"opencsg.com/csghub-server/common/types"
)

// init registers the repair of legacy namespace links and their missing authorization tuples.
func init() {
	Migrations.MustRegister(repairOrganizationNamespacesAndTuples, func(context.Context, *bun.DB) error {
		// Data repairs are intentionally retained: reverting them would break valid ownership,
		// and repaired tuples cannot be distinguished from subsequent live authorization writes.
		return nil
	})
}

// repairOrganizationNamespacesAndTuples repairs active legacy organizations and fills their missing tuples.
// The namespace_id update, tuple repair, and conflicting owner cleanup run in one transaction so
// a failure rolls back the database work and the same rows can be retried.
func repairOrganizationNamespacesAndTuples(ctx context.Context, db *bun.DB) error {
	appDB, ok := DatabaseFromContext(ctx)
	if !ok {
		return fmt.Errorf("application database is missing from migration context")
	}
	pool, ok := appDB.GetPGXPool()
	if !ok {
		return fmt.Errorf("organization namespace repair requires a PostgreSQL pgx pool")
	}
	provider, err := openfgaprovider.NewCustomProvider(
		openfgaprovider.WithPGXPool(pool),
		openfgaprovider.WithAuthorizationModelID(types.OpenFgaAuthorizationModelIDVer1_2),
	)
	if err != nil {
		return fmt.Errorf("initialize OpenFGA provider for organization namespace repair: %w", err)
	}
	defer provider.Close()
	authorizer, err := rebac.NewAuthorizer(provider)
	if err != nil {
		return fmt.Errorf("initialize organization namespace repair authorizer: %w", err)
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var repairedNamespaceIDs []int64
		if err := tx.NewRaw(`
			SELECT n.id
			FROM organizations AS o
			JOIN namespaces AS n ON n.path = o.path
			WHERE o.namespace_id = 0 AND o.deleted_at IS NULL
			  AND n.namespace_type = 'organization' AND n.deleted_at IS NULL
			ORDER BY n.id`).Scan(ctx, &repairedNamespaceIDs); err != nil {
			return fmt.Errorf("find organization namespace IDs to repair: %w", err)
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE organizations AS o
			SET namespace_id = n.id
			FROM namespaces AS n
			WHERE o.namespace_id = 0 AND o.deleted_at IS NULL
			  AND o.path = n.path
			  AND n.namespace_type = 'organization' AND n.deleted_at IS NULL`)
		if err != nil {
			return fmt.Errorf("repair organization namespace IDs: %w", err)
		}
		repaired, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("count repaired organization namespace IDs: %w", err)
		}
		var unresolved int
		if err := tx.NewRaw(`SELECT COUNT(*) FROM organizations WHERE namespace_id = 0 AND deleted_at IS NULL`).Scan(ctx, &unresolved); err != nil {
			return fmt.Errorf("count unresolved organization namespace IDs: %w", err)
		}
		slog.InfoContext(ctx, "Organization namespace IDs repaired", "repaired", repaired, "unresolved", unresolved)
		if unresolved > 0 {
			slog.WarnContext(ctx, "Active organizations have no matching organization namespace", "count", unresolved)
		}

		// Resolve repository paths deterministically. Organization namespaces win a
		// case-insensitive collision only when the same path does not have an exact
		// case match, matching NamespaceStore.FindByPath.
		if len(repairedNamespaceIDs) == 0 {
			slog.InfoContext(ctx, "No organization namespace IDs require tuple repair")
			return nil
		}

		phases := []struct {
			objectType rebac.ObjectType
			query      string
		}{
			{rebac.ObjectTypeNamespace, `
			SELECT n.id, COALESCE(o.uuid::text, ''), COALESCE(n.uuid, ''), 'organization'
			FROM namespaces AS n
			JOIN organizations AS o ON o.namespace_id = n.id AND o.deleted_at IS NULL
			WHERE n.namespace_type = 'organization' AND n.deleted_at IS NULL
			  AND n.id IN (?) AND n.id > ?
			ORDER BY n.id LIMIT ?`},
			{rebac.ObjectTypeRepository, repositoryOrganizationTupleBackfillQuery()},
		}
		for _, phase := range phases {
			stats := newTupleBackfillStats(string(phase.objectType))
			if err := backfillLinkedOrganizationTuples(ctx, tx, authorizer, phase.objectType, phase.query, repairedNamespaceIDs, stats); err != nil {
				return fmt.Errorf("repair organization %s tuples: %w", phase.objectType, err)
			}
			slog.InfoContext(ctx, "Organization ownership tuple repair completed",
				"phase", stats.phase, "candidates", stats.candidates,
				"submitted", stats.submitted, "skipped", stats.skipped)
			if stats.skipped > 0 {
				slog.WarnContext(ctx, "Organization ownership tuple repair skipped invalid rows", "phase", stats.phase, "reasons", stats.skipReasons)
			}
		}
		if err := deleteConflictingUserOwnerTuples(ctx, tx, authorizer, repairedNamespaceIDs); err != nil {
			return fmt.Errorf("remove conflicting user repository owners: %w", err)
		}
		return nil
	})
}

// RepositoryNamespaceLookupSubquery resolves a repository namespace with the
// same precedence as NamespaceStore.FindByPath.
func RepositoryNamespaceLookupSubquery() string {
	return `
				SELECT n.id, n.namespace_type
				FROM namespaces AS n
				WHERE LOWER(n.path) = LOWER(SPLIT_PART(r.path, '/', 1)) AND n.deleted_at IS NULL
				ORDER BY (n.path = SPLIT_PART(r.path, '/', 1)) DESC,
				         n.id ASC LIMIT 1`
}

// repositoryOrganizationTupleBackfillQuery repairs organization ownership tuples
// for repositories whose namespace resolves to an organization.
func repositoryOrganizationTupleBackfillQuery() string {
	return `
			SELECT r.id, COALESCE(o.uuid::text, ''), r.id::text,
				CASE WHEN r.org_inherit_blocked THEN 'organization_direct' ELSE 'organization' END
			FROM repositories AS r
			JOIN LATERAL (
` + RepositoryNamespaceLookupSubquery() + `
			) AS n ON n.namespace_type = 'organization'
			JOIN organizations AS o ON o.namespace_id = n.id AND o.deleted_at IS NULL
			WHERE r.deleted_at IS NULL AND n.id IN (?) AND r.id > ?
			ORDER BY r.id LIMIT ?`
}

// deleteConflictingUserOwnerTuples removes user owner tuples for repositories
// that resolve to repaired organization namespaces while still colliding with
// a user namespace by case-insensitive path.
func deleteConflictingUserOwnerTuples(ctx context.Context, db bun.IDB, authorizer rebac.Authorizer, repairedNamespaceIDs []int64) error {
	if len(repairedNamespaceIDs) == 0 {
		return nil
	}
	var lastID int64
	for {
		rows, err := db.QueryContext(ctx, `
			SELECT r.id, u.uuid
			FROM repositories AS r
			JOIN LATERAL (
`+RepositoryNamespaceLookupSubquery()+`
			) AS selected ON selected.namespace_type = 'organization'
			JOIN namespaces AS user_ns
				ON user_ns.namespace_type = 'user'
				AND LOWER(user_ns.path) = LOWER(SPLIT_PART(r.path, '/', 1))
				AND user_ns.deleted_at IS NULL
			JOIN users AS u ON u.id = user_ns.user_id AND u.deleted_at IS NULL
			WHERE r.deleted_at IS NULL AND selected.id IN (?) AND r.id > ?
			ORDER BY r.id LIMIT ?`, bun.In(repairedNamespaceIDs), lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query conflicting user repository owners: %w", err)
		}
		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var repositoryID int64
			var userUUID string
			if err := rows.Scan(&repositoryID, &userUUID); err != nil {
				rows.Close()
				return fmt.Errorf("scan conflicting user repository owner: %w", err)
			}
			lastID = repositoryID
			rowCount++
			if !validTupleID(userUUID) {
				continue
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.UserSubject(userUUID),
				Relation: rebac.RelationOwner,
				Object:   rebac.RepositoryObject(repositoryID),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read conflicting user repository owners: %w", err)
		}
		rows.Close()
		for start := 0; start < len(relationships); start += tupleBackfillWriteBatchSize {
			end := min(start+tupleBackfillWriteBatchSize, len(relationships))
			if err := authorizer.Delete(ctx, relationships[start:end]); err != nil {
				return fmt.Errorf("delete conflicting user repository owners: %w", err)
			}
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

// backfillLinkedOrganizationTuples submits ownership tuples in bounded query and write batches.
// Existing tuples are ignored by the OpenFGA Provider without a pre-write lookup.
// Each query returns the pagination ID, organization UUID, object ID, and current direct relation.
func backfillLinkedOrganizationTuples(ctx context.Context, db bun.IDB, authorizer rebac.Authorizer, objectType rebac.ObjectType, query string, namespaceIDs []int64, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := db.QueryContext(ctx, query, bun.In(namespaceIDs), lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query organization ownership: %w", err)
		}
		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var organizationUUID, objectID string
			var relation rebac.Relation
			if err := rows.Scan(&lastID, &organizationUUID, &objectID, &relation); err != nil {
				rows.Close()
				return fmt.Errorf("scan organization ownership: %w", err)
			}
			rowCount++
			if !validTupleID(organizationUUID) || !validTupleID(objectID) {
				stats.skip("invalid_organization_or_object_id")
				continue
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, organizationUUID),
				Relation: relation,
				Object:   rebac.NewObject(objectType, objectID),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read organization ownership: %w", err)
		}
		rows.Close()
		if err := writeBackfillRelationships(ctx, authorizer, relationships, stats); err != nil {
			return err
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}
