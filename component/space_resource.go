package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewSpaceResourceComponent(config *config.Config) (*SpaceResourceComponent, error) {
	c := &SpaceResourceComponent{}
	c.srs = database.NewSpaceResourceStore()
	c.deployer = deploy.NewDeployer()
	return c, nil
}

type SpaceResourceComponent struct {
	srs      *database.SpaceResourceStore
	deployer deploy.Deployer
}

func (c *SpaceResourceComponent) Index(ctx context.Context, clusterId string, deployType int) ([]types.SpaceResource, error) {
	// backward compatibility for old api
	if clusterId == "" {
		clusters, err := c.deployer.ListCluster(ctx)
		if err != nil {
			return nil, err
		}
		if len(clusters) == 0 {
			return nil, fmt.Errorf("can not list clusters")
		}
		clusterId = clusters[0].ClusterID
	}
	var result []types.SpaceResource
	databaseSpaceResources, err := c.srs.Index(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	clusterResources, err := c.deployer.GetClusterById(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	for _, r := range databaseSpaceResources {
		var isAvailable bool
		var hardware types.HardWare
		err := json.Unmarshal([]byte(r.Resources), &hardware)
		if err != nil {
			slog.Error("invalid hardware setting", slog.Any("error", err), slog.String("hardware", r.Resources))
		} else {
			isAvailable = deploy.CheckResource(clusterResources, &hardware)
		}
		if deployType == types.FinetuneType {
			if hardware.Gpu.Num == "" {
				continue
			}
		}
		result = append(result, types.SpaceResource{
			ID:          r.ID,
			Name:        r.Name,
			Resources:   r.Resources,
			IsAvailable: isAvailable,
		})
	}

	return result, nil
}

func (c *SpaceResourceComponent) Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error) {
	sr, err := c.srs.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space resource", slog.Any("error", err))
		return nil, err
	}
	sr.Name = req.Name
	sr.Resources = req.Resources

	sr, err = c.srs.Update(ctx, *sr)
	if err != nil {
		slog.Error("error updating space resource", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceResource{
		ID:        sr.ID,
		Name:      sr.Name,
		Resources: sr.Resources,
	}

	return result, nil
}

func (c *SpaceResourceComponent) Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error) {
	sr := database.SpaceResource{
		Name:      req.Name,
		Resources: req.Resources,
		ClusterID: req.ClusterID,
	}
	res, err := c.srs.Create(ctx, sr)
	if err != nil {
		slog.Error("error creating space resource", slog.Any("error", err))
		return nil, err
	}

	result := &types.SpaceResource{
		ID:        res.ID,
		Name:      res.Name,
		Resources: res.Resources,
	}

	return result, nil
}

func (c *SpaceResourceComponent) Delete(ctx context.Context, id int64) error {
	sr, err := c.srs.FindByID(ctx, id)
	if err != nil {
		slog.Error("error finding space resource", slog.Any("error", err))
		return err
	}

	err = c.srs.Delete(ctx, *sr)
	if err != nil {
		slog.Error("error deleting space resource", slog.Any("error", err))
		return err
	}
	return nil
}
