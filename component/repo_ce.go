//go:build !saas

package component

import (
	"context"
	"encoding/json"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *repoComponentImpl) CheckAccountAndResource(ctx context.Context, userName string, clusterID string, orderDetailID int64, resource *database.SpaceResource) error {
	var hardware types.HardWare
	err := json.Unmarshal([]byte(resource.Resources), &hardware)
	if err != nil {
		return fmt.Errorf("invalid hardware setting, %w", err)
	}
	_, err = c.deployer.CheckResourceAvailable(ctx, clusterID, 0, &hardware)
	if err != nil {
		return fmt.Errorf("fail to check resource, %w", err)
	}
	return nil
}

func (c *repoComponentImpl) allowPublic(repo *database.Repository) (allow bool, reason string) {
	//always allow public repo in on-premises deployment
	return true, ""
}
