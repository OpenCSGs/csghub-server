//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

// checkAndBuildPDConfig is a no-op for CE builds that do not support PD disaggregation.
func (c *modelComponentImpl) checkAndBuildPDConfig(ctx context.Context, req types.ModelRunReq, hardware types.HardWare, repoID int64) (*types.PDConfig, error) {
	return nil, nil
}
