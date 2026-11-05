//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// deleteDirectRepositoryAuthorizations preserves the CE edition's cleanup boundary.
func deleteDirectRepositoryAuthorizations(context.Context, database.RepositoryAuthorizationStore, types.RepoAuthSubjectType, int64) error {
	return nil
}
