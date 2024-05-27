package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewClusterComponent(config *config.Config) (*ClusterComponent, error) {
	c := &ClusterComponent{}
	c.cls = database.NewClusterInfoStore()

	return c, nil
}

type ClusterComponent struct {
	cls *database.ClusterInfoStore
}

func (c *ClusterComponent) Index(ctx context.Context) ([]types.Cluster, error) {
	var result []types.Cluster
	clusters, err := c.cls.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		result = append(result, types.Cluster{
			ClusterID:     c.ClusterID,
			ClusterConfig: c.ClusterConfig,
			Region:        c.Region,
		})
	}
	return result, nil
}
