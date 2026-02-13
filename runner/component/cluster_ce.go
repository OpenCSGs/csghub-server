//go:build !saas && !ee

package component

import (
	"context"
	"errors"

	"opencsg.com/csghub-server/common/types"
)

func (c *clusterComponentImpl) LabelNode(ctx context.Context, req *types.NodeLabel) error {
	return errors.New("LabelNode is not supported in CE version")
}
