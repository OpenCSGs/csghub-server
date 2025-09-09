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
	Index(ctx context.Context, req *types.SpaceResourceIndexReq) ([]types.SpaceResource, int, error)
	Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error)
	Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error)
	Delete(ctx context.Context, id int64) error
}

func (c *spaceResourceComponentImpl) Index(ctx context.Context, req *types.SpaceResourceIndexReq) ([]types.SpaceResource, int, error) {
	var clusterIDs []string
	if req.ClusterID == "" {
		clusters, err := c.deployer.ListCluster(ctx)
		if err != nil {
			return nil, 0, err
		}

		for _, cluster := range clusters {
			var timeout bool
			timeout, err = c.deployer.CheckHeartbeatTimeout(ctx, cluster.ClusterID)
			if err != nil {
				slog.Error("failed to check heartbeat timeout", slog.String("cluster", cluster.ClusterID), slog.Any("error", err))
				continue
			}
			if !timeout {
				clusterIDs = append(clusterIDs, cluster.ClusterID)
			}
		}
		if len(clusterIDs) == 0 {
			return nil, 0, fmt.Errorf("can not find any running clusters")
		}
	} else {
		clusterIDs = append(clusterIDs, req.ClusterID)
	}

	var result []types.SpaceResource
	var total int
	for _, clusterID := range clusterIDs {
		var singleClusterResult []types.SpaceResource
		databaseSpaceResources, currentTotal, err := c.spaceResourceStore.Index(ctx, clusterID, req.PageSize, req.Page)
		if err != nil {
			slog.Error("failed to index space resource", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
		}

		clusterResources, err := c.deployer.GetClusterById(ctx, clusterID)
		if err != nil {
			slog.Error("failed to get cluster by id", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
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
			if !c.deployAvailable(req.DeployType, hardware) {
				continue
			}
			if req.DeployType != types.InferenceType && hardware.Replicas != 0 {
				// only inference can have multi-node resources
				continue
			}
			resourceType := common.ResourceType(hardware)
			singleClusterResult = append(singleClusterResult, types.SpaceResource{
				ID:          r.ID,
				ClusterID:   r.ClusterID,
				Name:        r.Name,
				Resources:   r.Resources,
				IsAvailable: isAvailable,
				Type:        resourceType,
			})
		}

		tmpReq := *req
		tmpReq.ClusterID = clusterID
		err = c.updatePriceInfo(&tmpReq, singleClusterResult)
		if err != nil {
			slog.Error("failed to update price info", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
		}

		singleClusterResult, err = c.appendUserResources(ctx, req.CurrentUser, clusterID, singleClusterResult)
		if err != nil {
			slog.Error("failed to append user resources", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
		}
		result = append(result, singleClusterResult...)
		total += currentTotal
	}

	return result, total, nil
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
