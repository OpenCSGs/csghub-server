package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/rebac"
	openfgaprovider "opencsg.com/csghub-server/builder/rebac/openfga"
	"opencsg.com/csghub-server/common/types"
)

const (
	// tupleBackfillQueryBatchSize bounds each business table scan.
	tupleBackfillQueryBatchSize = 1000
	// tupleBackfillWriteBatchSize matches OpenFGA's default maximum tuples per write.
	tupleBackfillWriteBatchSize = 100
)

// tupleBackfillStats records the outcome of one backfill phase.
type tupleBackfillStats struct {
	phase       string
	candidates  int
	existing    int
	written     int
	skipped     int
	skipReasons map[string]int
}

func newTupleBackfillStats(phase string) *tupleBackfillStats {
	return &tupleBackfillStats{
		phase:       phase,
		skipReasons: make(map[string]int),
	}
}

func (s *tupleBackfillStats) skip(reason string) {
	s.skipped++
	s.skipReasons[reason]++
}

// init registers the one-time historical OpenFGA tuple backfill.
func init() {
	Migrations.MustRegister(func(ctx context.Context, _ *bun.DB) error {
		return backfillOpenFGATuples(ctx)
	}, func(ctx context.Context, db *bun.DB) error {
		// Rollback intentionally clears all OpenFGA authorization history, including live tuples.
		// This is destructive and is only appropriate when reverting the authorization data migration.
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `TRUNCATE TABLE tuple, changelog`); err != nil {
				return fmt.Errorf("clear OpenFGA tuple and changelog tables: %w", err)
			}
			return nil
		})
	})
}

// backfillOpenFGATuples synchronizes active historical business records to OpenFGA.
func backfillOpenFGATuples(ctx context.Context) error {
	appDB, ok := DatabaseFromContext(ctx)
	if !ok {
		return fmt.Errorf("application database is missing from migration context")
	}
	pool, ok := appDB.GetPGXPool()
	if !ok {
		return fmt.Errorf("OpenFGA tuple backfill requires a PostgreSQL pgx pool")
	}

	provider, err := openfgaprovider.NewProviderWithPGXPool(pool)
	if err != nil {
		return fmt.Errorf("initialize OpenFGA provider for tuple backfill: %w", err)
	}
	defer provider.Close()

	authorizer, err := rebac.NewAuthorizer(provider)
	if err != nil {
		return fmt.Errorf("initialize OpenFGA authorizer for tuple backfill: %w", err)
	}

	phases := make([]*tupleBackfillStats, 0, 4)
	usersStats := newTupleBackfillStats("users")
	if err := backfillUsers(ctx, pool, authorizer, usersStats); err != nil {
		return err
	}
	phases = append(phases, usersStats)

	namespacesStats := newTupleBackfillStats("namespaces")
	if err := backfillNamespaces(ctx, pool, authorizer, namespacesStats); err != nil {
		return err
	}
	phases = append(phases, namespacesStats)

	organizationsStats := newTupleBackfillStats("organization_members")
	if err := backfillOrganizationMembers(ctx, pool, authorizer, organizationsStats); err != nil {
		return err
	}
	phases = append(phases, organizationsStats)

	organizationCreatorsStats := newTupleBackfillStats("organization_creators")
	if err := backfillOrganizationCreators(ctx, pool, authorizer, organizationCreatorsStats); err != nil {
		return err
	}
	phases = append(phases, organizationCreatorsStats)

	repositoriesStats := newTupleBackfillStats("repositories")
	if err := backfillRepositories(ctx, pool, authorizer, repositoriesStats); err != nil {
		return err
	}
	phases = append(phases, repositoriesStats)

	for _, stats := range phases {
		attrs := []any{
			"phase", stats.phase,
			"candidates", stats.candidates,
			"existing", stats.existing,
			"written", stats.written,
			"skipped", stats.skipped,
		}
		if stats.skipped > 0 {
			slog.WarnContext(ctx, "OpenFGA historical tuple backfill skipped invalid rows", append(attrs, "reasons", stats.skipReasons)...)
			continue
		}
		slog.InfoContext(ctx, "OpenFGA historical tuple backfill phase completed", attrs...)
	}
	return nil
}

type userTupleBackfillRow struct {
	id   int64
	uuid string
}

func backfillUsers(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT id, COALESCE(uuid, '')
			FROM users
			WHERE deleted_at IS NULL AND id > $1
			ORDER BY id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query users for OpenFGA tuple backfill: %w", err)
		}

		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var row userTupleBackfillRow
			if err := rows.Scan(&row.id, &row.uuid); err != nil {
				rows.Close()
				return fmt.Errorf("scan user for OpenFGA tuple backfill: %w", err)
			}
			rowCount++
			lastID = row.id
			if !validTupleID(row.uuid) {
				stats.skip("user_missing_uuid")
				continue
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.UserSubject(row.uuid),
				Relation: rebac.RelationOwner,
				Object:   rebac.UserObject(row.uuid),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read users for OpenFGA tuple backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write user OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

type namespaceTupleBackfillRow struct {
	id               int64
	uuid             string
	namespaceType    string
	userUUID         string
	organizationUUID string
}

func backfillNamespaces(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT
				n.id,
				COALESCE(n.uuid, ''),
				COALESCE(n.namespace_type, ''),
				COALESCE(u.uuid, ''),
				COALESCE(o.uuid::text, '')
			FROM namespaces AS n
			LEFT JOIN users AS u
				ON n.namespace_type = 'user'
				AND u.id = n.user_id
				AND u.deleted_at IS NULL
			LEFT JOIN organizations AS o
				ON n.namespace_type = 'organization'
				AND o.namespace_id = n.id
				AND o.deleted_at IS NULL
			WHERE n.deleted_at IS NULL AND n.id > $1
			ORDER BY n.id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query namespaces for OpenFGA tuple backfill: %w", err)
		}

		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var row namespaceTupleBackfillRow
			if err := rows.Scan(&row.id, &row.uuid, &row.namespaceType, &row.userUUID, &row.organizationUUID); err != nil {
				rows.Close()
				return fmt.Errorf("scan namespace for OpenFGA tuple backfill: %w", err)
			}
			rowCount++
			lastID = row.id
			if !validTupleID(row.uuid) {
				stats.skip("namespace_missing_uuid")
				continue
			}

			var relationship rebac.Relationship
			switch row.namespaceType {
			case databaseUserNamespace:
				if !validTupleID(row.userUUID) {
					stats.skip("user_namespace_missing_user_uuid")
					continue
				}
				relationship = rebac.Relationship{
					Subject:  rebac.UserSubject(row.userUUID),
					Relation: rebac.RelationOwner,
					Object:   rebac.NamespaceObject(row.uuid),
				}
			case databaseOrganizationNamespace:
				if !validTupleID(row.organizationUUID) {
					stats.skip("organization_namespace_missing_organization_uuid")
					continue
				}
				relationship = rebac.Relationship{
					Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, row.organizationUUID),
					Relation: rebac.RelationOrganization,
					Object:   rebac.NamespaceObject(row.uuid),
				}
			default:
				stats.skip("namespace_unknown_type")
				continue
			}
			relationships = append(relationships, relationship)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read namespaces for OpenFGA tuple backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write namespace OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

// databaseUserNamespace and databaseOrganizationNamespace avoid importing database models into this migration.
const (
	databaseUserNamespace         = "user"
	databaseOrganizationNamespace = "organization"
)

type organizationMemberTupleBackfillRow struct {
	id               int64
	organizationUUID string
	userUUID         string
	role             string
}

func backfillOrganizationMembers(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT m.id, o.uuid::text, u.uuid, m.role
			FROM members AS m
			JOIN organizations AS o
				ON o.id = m.organization_id AND o.deleted_at IS NULL
			JOIN users AS u
				ON u.id = m.user_id AND u.deleted_at IS NULL
			WHERE m.deleted_at IS NULL AND m.id > $1
			ORDER BY m.id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query organization members for OpenFGA tuple backfill: %w", err)
		}

		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var row organizationMemberTupleBackfillRow
			if err := rows.Scan(&row.id, &row.organizationUUID, &row.userUUID, &row.role); err != nil {
				rows.Close()
				return fmt.Errorf("scan organization member for OpenFGA tuple backfill: %w", err)
			}
			rowCount++
			lastID = row.id
			if !validTupleID(row.organizationUUID) {
				stats.skip("organization_member_missing_organization_uuid")
				continue
			}
			if !validTupleID(row.userUUID) {
				stats.skip("organization_member_missing_user_uuid")
				continue
			}
			relation, ok := organizationMemberRelation(row.role)
			if !ok {
				stats.skip("organization_member_unknown_role")
				continue
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.UserSubject(row.userUUID),
				Relation: relation,
				Object:   rebac.OrganizationObject(row.organizationUUID),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read organization members for OpenFGA tuple backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write organization member OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

// organizationMemberRelation maps current and legacy member roles to direct ReBAC relations.
func organizationMemberRelation(role string) (rebac.Relation, bool) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case string(types.UserAdmin), "owner":
		return rebac.RelationAdmin, true
	case string(types.UserWrite), "writer":
		return rebac.RelationWriter, true
	case string(types.UserRead), "reader":
		return rebac.RelationReader, true
	default:
		return "", false
	}
}

type organizationCreatorTupleBackfillRow struct {
	id               int64
	organizationUUID string
	userUUID         string
}

// backfillOrganizationCreators repairs legacy organizations that have no active membership for their creator.
func backfillOrganizationCreators(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT o.id, o.uuid::text, u.uuid
			FROM organizations AS o
			JOIN users AS u
				ON u.id = o.user_id AND u.deleted_at IS NULL
			WHERE o.deleted_at IS NULL
			  AND o.user_id <> 0
			  AND o.id > $1
			  AND NOT EXISTS (
				SELECT 1
				FROM members AS m
				WHERE m.organization_id = o.id
				  AND m.user_id = o.user_id
				  AND m.deleted_at IS NULL
			  )
			ORDER BY o.id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query organization creators for OpenFGA tuple backfill: %w", err)
		}

		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var row organizationCreatorTupleBackfillRow
			if err := rows.Scan(&row.id, &row.organizationUUID, &row.userUUID); err != nil {
				rows.Close()
				return fmt.Errorf("scan organization creator for OpenFGA tuple backfill: %w", err)
			}
			rowCount++
			lastID = row.id
			if !validTupleID(row.organizationUUID) {
				stats.skip("organization_creator_missing_organization_uuid")
				continue
			}
			if !validTupleID(row.userUUID) {
				stats.skip("organization_creator_missing_user_uuid")
				continue
			}
			relationships = append(relationships, rebac.Relationship{
				Subject:  rebac.UserSubject(row.userUUID),
				Relation: rebac.RelationAdmin,
				Object:   rebac.OrganizationObject(row.organizationUUID),
			})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read organization creators for OpenFGA tuple backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write organization creator OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

type repositoryTupleBackfillRow struct {
	id               int64
	namespaceType    string
	userUUID         string
	organizationUUID string
}

func backfillRepositories(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, stats *tupleBackfillStats) error {
	var lastID int64
	for {
		rows, err := pool.Query(ctx, `
			SELECT
				r.id,
				COALESCE(n.namespace_type, ''),
				COALESCE(u.uuid, ''),
				COALESCE(o.uuid::text, '')
			FROM repositories AS r
			LEFT JOIN namespaces AS n
				ON LOWER(n.path) = LOWER(SPLIT_PART(r.path, '/', 1))
				AND n.deleted_at IS NULL
			LEFT JOIN users AS u
				ON n.namespace_type = 'user'
				AND u.id = n.user_id
				AND u.deleted_at IS NULL
			LEFT JOIN organizations AS o
				ON n.namespace_type = 'organization'
				AND o.namespace_id = n.id
				AND o.deleted_at IS NULL
			WHERE r.deleted_at IS NULL AND r.id > $1
			ORDER BY r.id
			LIMIT $2`, lastID, tupleBackfillQueryBatchSize)
		if err != nil {
			return fmt.Errorf("query repositories for OpenFGA tuple backfill: %w", err)
		}

		relationships := make([]rebac.Relationship, 0, tupleBackfillQueryBatchSize)
		rowCount := 0
		for rows.Next() {
			var row repositoryTupleBackfillRow
			if err := rows.Scan(&row.id, &row.namespaceType, &row.userUUID, &row.organizationUUID); err != nil {
				rows.Close()
				return fmt.Errorf("scan repository for OpenFGA tuple backfill: %w", err)
			}
			rowCount++
			lastID = row.id

			var relationship rebac.Relationship
			switch row.namespaceType {
			case databaseUserNamespace:
				if !validTupleID(row.userUUID) {
					stats.skip("personal_repository_missing_user_uuid")
					continue
				}
				relationship = rebac.Relationship{
					Subject:  rebac.UserSubject(row.userUUID),
					Relation: rebac.RelationOwner,
					Object:   rebac.RepositoryObject(row.id),
				}
			case databaseOrganizationNamespace:
				if !validTupleID(row.organizationUUID) {
					stats.skip("organization_repository_missing_organization_uuid")
					continue
				}
				relationship = rebac.Relationship{
					Subject:  rebac.NewSubject(rebac.ObjectTypeOrganization, row.organizationUUID),
					Relation: rebac.RelationOrganization,
					Object:   rebac.RepositoryObject(row.id),
				}
			default:
				stats.skip("repository_namespace_not_found_or_unknown")
				continue
			}
			relationships = append(relationships, relationship)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("read repositories for OpenFGA tuple backfill: %w", err)
		}
		rows.Close()
		if err := writeMissingRelationships(ctx, pool, authorizer, relationships, stats); err != nil {
			return fmt.Errorf("write repository OpenFGA tuples: %w", err)
		}
		if rowCount < tupleBackfillQueryBatchSize {
			return nil
		}
	}
}

// writeMissingRelationships writes only tuples absent from the OpenFGA tuple table.
func writeMissingRelationships(ctx context.Context, pool *pgxpool.Pool, authorizer rebac.Authorizer, relationships []rebac.Relationship, stats *tupleBackfillStats) error {
	unique := deduplicateRelationships(relationships)
	stats.candidates += len(unique)
	for start := 0; start < len(unique); start += tupleBackfillWriteBatchSize {
		end := start + tupleBackfillWriteBatchSize
		if end > len(unique) {
			end = len(unique)
		}
		batch := unique[start:end]
		existing, err := existingRelationshipKeys(ctx, pool, batch)
		if err != nil {
			return err
		}
		stats.existing += len(existing)
		missing := make([]rebac.Relationship, 0, len(batch)-len(existing))
		for _, relationship := range batch {
			if _, ok := existing[relationshipKey(relationship)]; !ok {
				missing = append(missing, relationship)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if err := authorizer.Write(ctx, missing); err != nil {
			return fmt.Errorf("write %d missing relationships: %w", len(missing), err)
		}
		stats.written += len(missing)
	}
	return nil
}

func existingRelationshipKeys(ctx context.Context, pool *pgxpool.Pool, relationships []rebac.Relationship) (map[string]struct{}, error) {
	existing := make(map[string]struct{}, len(relationships))
	if len(relationships) == 0 {
		return existing, nil
	}

	args := make([]any, 1, 1+len(relationships)*4)
	args[0] = types.OpenFgaStoreID
	placeholders := make([]string, 0, len(relationships))
	for index, relationship := range relationships {
		base := 2 + index*4
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d)", base, base+1, base+2, base+3))
		args = append(args,
			string(relationship.Object.Type),
			relationship.Object.ID,
			string(relationship.Relation),
			relationship.Subject.String(),
		)
	}

	query := fmt.Sprintf(`
		SELECT object_type, object_id, relation, _user
		FROM tuple
		WHERE store = $1
		  AND (object_type, object_id, relation, _user) IN (%s)`, strings.Join(placeholders, ", "))
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query existing OpenFGA tuples: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var objectType, objectID, relation, subject string
		if err := rows.Scan(&objectType, &objectID, &relation, &subject); err != nil {
			return nil, fmt.Errorf("scan existing OpenFGA tuple: %w", err)
		}
		existing[tupleKey(objectType, objectID, relation, subject)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read existing OpenFGA tuples: %w", err)
	}
	return existing, nil
}

func deduplicateRelationships(relationships []rebac.Relationship) []rebac.Relationship {
	if len(relationships) < 2 {
		return relationships
	}
	unique := make([]rebac.Relationship, 0, len(relationships))
	seen := make(map[string]struct{}, len(relationships))
	for _, relationship := range relationships {
		key := relationshipKey(relationship)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, relationship)
	}
	return unique
}

func relationshipKey(relationship rebac.Relationship) string {
	return tupleKey(
		string(relationship.Object.Type),
		relationship.Object.ID,
		string(relationship.Relation),
		relationship.Subject.String(),
	)
}

func tupleKey(objectType, objectID, relation, subject string) string {
	return objectType + ":" + objectID + "#" + relation + "@" + subject
}

func validTupleID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	return !strings.ContainsAny(value, ":#@ \t\r\n")
}
