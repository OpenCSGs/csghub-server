//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *datasetComponentImpl) getXnetMigrationProgress(ctx context.Context, repo *database.Repository) int {
	return 0
}

func (c *datasetComponentImpl) addOpWeightToDataset(ctx context.Context, repoIDs []int64, resDatasets []*types.Dataset) {
}
