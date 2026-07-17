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

// buildPDConfig is a no-op for CE builds that do not support PD disaggregation.
func buildPDConfig(pdConfig *types.PDConfig, hardware types.HardWare, minReplica, maxReplica int) error {
	return nil
}

// validateAndFillPDRoleConfigs is a no-op for CE builds that do not support PD disaggregation.
func validateAndFillPDRoleConfigs(pdConfig *types.PDConfig) error {
	return nil
}
