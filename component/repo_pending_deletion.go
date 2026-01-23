package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
)

func (r *repoComponentImpl) DeletePendingDeletion(ctx context.Context) error {
	var batch int
	batchSize := 1000
	for {
		pds, err := r.pendingDeletion.FindByTableNameWithBatch(ctx, database.PendingDeletionTableNameRepository, batchSize, batch)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to find pending deletion", slog.Any("error", err))
			return fmt.Errorf("failed to find pending deletion: %w", err)
		}

		for _, pd := range pds {
			err := r.git.DeleteRepo(ctx, pd.Value)
			if err != nil {
				slog.Error("failed to delete repo", slog.Any("error", err), slog.String("gitRepoPath", pd.Value))
			} else {
				slog.Info("deleted repo", slog.String("gitRepoPath", pd.Value))
			}
			err = r.pendingDeletion.Delete(ctx, pd)
			if err != nil {
				slog.Error("failed to delete pending deletion", slog.Any("error", err), slog.String("gitRepoPath", pd.Value))
			}
			time.Sleep(2 * time.Second)
		}
		if len(pds) < batchSize {
			break
		}
		batch++
	}

	return nil
}
