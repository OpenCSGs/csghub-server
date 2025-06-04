package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type SpaceResourceComponent interface {
	Index(ctx context.Context, clusterId string, deployType int, currentUser string) ([]types.SpaceResource, error)
	Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error)
	Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error)
	Delete(ctx context.Context, id int64) error
}

func (c *spaceResourceComponentImpl) Index(ctx context.Context, clusterId string, deployType int, currentUser string) ([]types.SpaceResource, error) {
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
	databaseSpaceResources, err := c.spaceResourceStore.Index(ctx, clusterId)
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
		if !c.deployAvailable(deployType, hardware) {
			continue
		}
		if deployType == types.SpaceType && hardware.Replicas != 0 {
			// Space resources should not have multi-node resources
			continue
		}
		resourceType := common.ResourceType(hardware)
		result = append(result, types.SpaceResource{
			ID:          r.ID,
			Name:        r.Name,
			Resources:   r.Resources,
			IsAvailable: isAvailable,
			Type:        resourceType,
		})
	}
	err = c.updatePriceInfo(currentUser, result)
	if err != nil {
		return nil, err
	}

	result, err = c.appendUserResources(ctx, currentUser, clusterId, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *spaceResourceComponentImpl) Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error) {
	sr, err := c.spaceResourceStore.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space resource", slog.Any("error", err))
		return nil, err
	}
	sr.Name = req.Name
	sr.Resources = req.Resources

	sr, err = c.spaceResourceStore.Update(ctx, *sr)
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

func (c *spaceResourceComponentImpl) Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error) {
	sr := database.SpaceResource{
		Name:      req.Name,
		Resources: req.Resources,
		ClusterID: req.ClusterID,
	}
	res, err := c.spaceResourceStore.Create(ctx, sr)
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

func (c *spaceResourceComponentImpl) Delete(ctx context.Context, id int64) error {
	sr, err := c.spaceResourceStore.FindByID(ctx, id)
	if err != nil {
		slog.Error("error finding space resource", slog.Any("error", err))
		return err
	}

	err = c.spaceResourceStore.Delete(ctx, *sr)
	if err != nil {
		slog.Error("error deleting space resource", slog.Any("error", err))
		return err
	}
	return nil
}
