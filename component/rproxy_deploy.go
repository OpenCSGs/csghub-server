package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
)

// RProxyDeployComponent resolves the current deploy row for RProxy routing.
type RProxyDeployComponent interface {
	GetDeployBySvcName(ctx context.Context, svcName string) (*database.Deploy, error)
}

type rproxyDeployComponentImpl struct {
	deployStore database.LatestDeployBySvcNameStore
}

func NewRProxyDeployComponent() RProxyDeployComponent {
	return &rproxyDeployComponentImpl{
		deployStore: database.NewLatestDeployBySvcNameStore(),
	}
}

func (c *rproxyDeployComponentImpl) GetDeployBySvcName(ctx context.Context, svcName string) (*database.Deploy, error) {
	return c.deployStore.GetLatestDeployBySvcName(ctx, svcName)
}
