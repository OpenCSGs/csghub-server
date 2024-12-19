//go:build !ee && !saas

package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *modelComponentImpl) addOpWeightToModel(ctx context.Context, repoIDs []int64, resModels []*types.Model) {

}

func (c *modelComponentImpl) resourceAvailable(ctx context.Context, resource *database.SpaceResource, req types.ModelRunReq, deployReq types.DeployActReq, hardware types.HardWare) error {

	_, err := c.deployer.CheckResourceAvailable(ctx, req.ClusterID, 0, &hardware)
	if err != nil {
		return fmt.Errorf("fail to check resource, %w", err)
	}
	return nil
}

func (c *modelComponentImpl) containerImg(frame *database.RuntimeFramework, hardware types.HardWare) string {
	containerImg := frame.FrameCpuImage
	if hardware.Gpu.Num != "" {
		containerImg = frame.FrameImage
	}
	return containerImg

}
