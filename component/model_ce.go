//go:build !ee && !saas

package component

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (c *modelComponentImpl) addOpWeightToModel(ctx context.Context, repoIDs []int64, resModels []*types.Model) {

}

func modelRunUpdateDeployRepo(dp types.DeployRepo, req types.ModelRunReq) types.DeployRepo {
	return dp
}
