//go:build !ee && !saas

package component

import (
	"context"
	"strings"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func checkTagName(rf *database.RuntimeFramework, tag string) bool {
	return strings.Contains(rf.FrameImage, tag)
}

// updatePDRecommendation is a no-op for CE builds.
// PD disaggregation recommendations are only available in ee/saas builds.
func (c *runtimeArchitectureComponentImpl) updatePDRecommendation(ctx context.Context, repo *database.Repository, modelInfo *types.ModelInfo) {
	// no-op
}

// loadPDRecommendations is a no-op for CE builds.
func (c *runtimeArchitectureComponentImpl) loadPDRecommendations(ctx context.Context) error {
	return nil
}
