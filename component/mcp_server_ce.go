//go:build !saas && !ee

package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (m *mcpServerComponentImpl) addOpWeightToMCPs(ctx context.Context, repoIDs []int64, res []*types.MCPServer) {
}
