//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// deleteDirectRepositoryAuthorizations is a no-op outside the EE and SaaS editions.
func deleteDirectRepositoryAuthorizations(context.Context, database.RepositoryAuthorizationStore, rebac.Authorizer, types.RepoAuthSubjectType, int64) error {
	return nil
}
