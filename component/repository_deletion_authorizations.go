package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
)

// cleanRepositoryAuthorizations removes database grants after their tuples were queued.
func cleanRepositoryAuthorizations(ctx context.Context, store database.RepositoryAuthorizationStore, repositoryID int64) error {
	if err := store.DeleteByRepository(ctx, repositoryID); err != nil {
		return fmt.Errorf("delete repository authorization records: %w", err)
	}
	return nil
}
