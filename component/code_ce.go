//go:build !saas && !ee

package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (c *codeComponentImpl) addOpWeightToCodes(ctx context.Context, repoIDs []int64, resCodes []*types.Code) {
}
