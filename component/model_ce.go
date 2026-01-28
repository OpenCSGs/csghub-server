//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *modelComponentImpl) getXnetMigrationProgress(ctx context.Context, repo *database.Repository) int {
	return 0
}

func (c *modelComponentImpl) addOpWeightToModel(ctx context.Context, repoIDs []int64, resModels []*types.Model) {
}

func modelRunUpdateDeployRepo(dp types.DeployRepo, req types.ModelRunReq) types.DeployRepo {
	return dp
}
