//go:build !saas && !ee

package imagerunner

import (
	"context"
	"opencsg.com/csghub-server/common/types"
)

func (h *LocalRunner) LabelNode(ctx context.Context, req *types.NodeLabel) error {
	return nil
}
