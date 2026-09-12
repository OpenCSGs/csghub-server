package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// cleanRepositoryAuthorizations removes direct repository grant tuples and their persisted records.
func cleanRepositoryAuthorizations(
	ctx context.Context,
	store database.RepositoryAuthorizationStore,
	authorizer rebac.Authorizer,
	repositoryID int64,
) error {
	if store == nil {
		return fmt.Errorf("repository authorization store is required")
	}

	authorizations, err := store.ListByRepository(ctx, repositoryID)
	if err != nil {
		return fmt.Errorf("list repository authorizations: %w", err)
	}
	relationships := make([]rebac.Relationship, 0, len(authorizations))
	for _, authorization := range authorizations {
		relation, ok := authorization.Role.ReBACRelation()
		if !ok {
			return fmt.Errorf("invalid repository authorization role %q", authorization.Role)
		}
		var subject rebac.Subject
		switch authorization.SubjectType {
		case types.RepoAuthSubjectUser:
			subject = rebac.UserSubject(authorization.SubjectUUID)
		case types.RepoAuthSubjectOrganization:
			subject = rebac.OrganizationMembers(authorization.SubjectUUID)
		default:
			return fmt.Errorf("invalid repository authorization subject type %q", authorization.SubjectType)
		}
		relationships = append(relationships, rebac.Relationship{
			Subject: subject, Relation: relation, Object: rebac.RepositoryObject(repositoryID),
		})
	}
	for start := 0; start < len(relationships); start += rebac.DefaultMaxBatchSize {
		end := start + rebac.DefaultMaxBatchSize
		if end > len(relationships) {
			end = len(relationships)
		}
		if authorizer == nil {
			return fmt.Errorf("repository ReBAC authorizer is required")
		}
		if err := authorizer.Delete(ctx, relationships[start:end]); err != nil {
			return fmt.Errorf("delete repository authorization tuples: %w", err)
		}
	}

	if err := store.DeleteByRepository(ctx, repositoryID); err != nil {
		return fmt.Errorf("delete repository authorization records: %w", err)
	}
	return nil
}
