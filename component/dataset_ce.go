//go:build !saas && !ee

package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (c *datasetComponentImpl) addOpWeightToDataset(ctx context.Context, repoIDs []int64, resDatasets []*types.Dataset) {
}
