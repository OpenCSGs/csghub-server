package component

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

func maskNodeName(nodeName string) string {
	if len(nodeName) <= 5 {
		return "*****"
	}
	return nodeName[:1] + "***" + nodeName[len(nodeName)-3:]
}

type SpaceResourceComponent interface {
	Index(ctx context.Context, req *types.SpaceResourceIndexReq) ([]types.SpaceResource, int, error)
	Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error)
	Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error)
	Delete(ctx context.Context, id int64) error
	ListHardwareTypes(ctx context.Context, clusterId string) ([]string, error)
	ListAll(ctx context.Context) ([]types.SpaceResource, error)
}

// validateResources checks that the Resources string is non-empty and valid
// JSON that can be unmarshalled into a HardWare struct.
func validateResources(resources string) error {
	if strings.TrimSpace(resources) == "" {
		return errorx.BadRequest(errors.New("resources is empty"), errorx.Ctx().Set("field", "resources"))
	}
	var hw types.HardWare
	if err := json.Unmarshal([]byte(resources), &hw); err != nil {
		return errorx.BadRequest(err, errorx.Ctx().Set("field", "resources"))
	}
	if len(strings.TrimSpace(hw.Cpu.Type)) < 1 ||
		len(strings.TrimSpace(hw.Cpu.Num)) < 1 ||
		len(strings.TrimSpace(hw.Memory)) < 1 {
		return errorx.BadRequest(errors.New("cpu resource is required"), errorx.Ctx().Set("field", "resources"))
	}
	return nil
}

func sanitizeScenarios(scenarios []string) []string {
	if scenarios == nil {
		return []string{}
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(scenarios))
	for _, s := range scenarios {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		result = append(result, s)
	}
	return result
}

func (c *spaceResourceComponentImpl) Index(ctx context.Context, req *types.SpaceResourceIndexReq) ([]types.SpaceResource, int, error) {
	var result []types.SpaceResource
	var total int
	for _, clusterID := range req.ClusterIDs {
		resourceCount := 0
		var singleClusterResult []types.SpaceResource
		dbReq := types.SpaceResourceFilter{
			ClusterID:    clusterID,
			HardwareType: req.HardwareType,
			ResourceType: req.ResourceType,
		}
		databaseSpaceResources, _, err := c.spaceResourceStore.Index(ctx, dbReq, math.MaxInt, 1)
		if err != nil {
			slog.ErrorContext(ctx, "failed to index space resource", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
		}

		clusterResources, err := c.deployer.GetClusterById(ctx, clusterID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to get cluster by id", slog.String("clusterID", clusterID), slog.Any("error", err))
			continue
		}
		slog.InfoContext(ctx, "get space cluster resource", slog.Any("clusterResources", clusterResources))
		for _, r := range databaseSpaceResources {
			var isAvailable bool
			var hardware types.HardWare
			var availableStatusList []types.ResourceAvailableStatus
			err := json.Unmarshal([]byte(r.Resources), &hardware)
			if err != nil {
				slog.ErrorContext(ctx, "invalid hardware setting", slog.Any("error", err), slog.String("hardware", r.Resources))
			} else {
				isAvailable, availableStatusList = deploy.CheckResource(clusterResources, &hardware, c.config)
				for i := range availableStatusList {
					availableStatusList[i].NodeName = maskNodeName(availableStatusList[i].NodeName)
				}
			}
			if !c.deployAvailable(req.DeployType, hardware) {
				// must have gpu for finetune
				continue
			}
			if req.DeployType != types.InferenceType && hardware.Replicas > 1 {
				// only inference can have multi-node resources
				continue
			}
			if req.IsAvailable != nil {
				// filter by request status
				if *req.IsAvailable != isAvailable {
					continue
				}
			}
			resourceType := common.ResourceType(hardware)
			scenarios := r.Scenarios
			if scenarios == nil {
				scenarios = []string{}
			}
			singleClusterResult = append(singleClusterResult, types.SpaceResource{
				ID:                  r.ID,
				ClusterID:           r.ClusterID,
				ClusterRegion:       clusterResources.Region,
				Name:                r.Name,
				Resources:           r.Resources,
				IsAvailable:         isAvailable,
				Type:                resourceType,
				Scenarios:           scenarios,
				AvailableStatusList: availableStatusList,
			})
			resourceCount++
		}

		err = c.updatePriceInfo(req, singleClusterResult)
		if err != nil {
			slog.ErrorContext(ctx, "failed to update space resource price info", slog.String("clusterID", clusterID), slog.Any("error", err))
			return nil, 0, err
		}

		// comment user reserved resource will update later
		// singleClusterResult, err = c.appendUserResources(ctx, req.CurrentUser, clusterID, singleClusterResult)
		// if err != nil {
		// 	slog.Error("failed to append user resources", slog.String("clusterID", clusterID), slog.Any("error", err))
		// 	continue
		// }
		result = append(result, singleClusterResult...)
		total += resourceCount
	}

	return result, total, nil
}

func (c *spaceResourceComponentImpl) Update(ctx context.Context, req *types.UpdateSpaceResourceReq) (*types.SpaceResource, error) {
	if err := validateResources(req.Resources); err != nil {
		return nil, err
	}
	sr, err := c.spaceResourceStore.FindByID(ctx, req.ID)
	if err != nil {
		slog.Error("error getting space resource", slog.Any("error", err))
		return nil, err
	}
	sr.Name = req.Name
	sr.Resources = req.Resources

	if req.Scenarios != nil {
		sr.Scenarios = sanitizeScenarios(req.Scenarios)
	}

	sr, err = c.spaceResourceStore.Update(ctx, *sr)
	if err != nil {
		slog.Error("error updating space resource", slog.Any("error", err))
		return nil, err
	}

	scenarios := sr.Scenarios
	if scenarios == nil {
		scenarios = []string{}
	}
	result := &types.SpaceResource{
		ID:        sr.ID,
		Name:      sr.Name,
		Resources: sr.Resources,
		Scenarios: scenarios,
	}

	return result, nil
}

func (c *spaceResourceComponentImpl) Create(ctx context.Context, req *types.CreateSpaceResourceReq) (*types.SpaceResource, error) {
	if err := validateResources(req.Resources); err != nil {
		return nil, err
	}
	sr := database.SpaceResource{
		Name:      req.Name,
		Resources: req.Resources,
		ClusterID: req.ClusterID,
		Scenarios: sanitizeScenarios(req.Scenarios),
	}
	res, err := c.spaceResourceStore.Create(ctx, sr)
	if err != nil {
		slog.Error("error creating space resource", slog.Any("error", err))
		return nil, err
	}

	scenariosResult := res.Scenarios
	if scenariosResult == nil {
		scenariosResult = []string{}
	}
	result := &types.SpaceResource{
		ID:        res.ID,
		Name:      res.Name,
		Resources: res.Resources,
		Scenarios: scenariosResult,
	}

	return result, nil
}

func (c *spaceResourceComponentImpl) Delete(ctx context.Context, id int64) error {
	sr, err := c.spaceResourceStore.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("error finding space resource %d failed: %w", id, err)
	}

	err = c.spaceResourceStore.Delete(ctx, *sr)
	if err != nil {
		return fmt.Errorf("error deleting space resource %d failed: %w", id, err)
	}

	// Off-line the corresponding price info
	delReq := types.AcctPriceOffLineReq{
		SkuType:    types.SKUCSGHub,
		ResourceID: strconv.FormatInt(id, 10),
	}
	_, err = c.accountComponent.OffLinePrice(ctx, delReq)
	if err != nil {
		slog.WarnContext(ctx, "off-line price failed for delete space resource",
			slog.Any("err", err), slog.Any("spaceResource", sr), slog.Any("delReq", delReq))
	}

	return nil
}

func (c *spaceResourceComponentImpl) ListHardwareTypes(ctx context.Context, clusterId string) ([]string, error) {
	types, err := c.spaceResourceStore.FindAllResourceTypes(ctx, clusterId)
	if err != nil {
		slog.Error("error listing hardware types", slog.Any("error", err))
		return nil, err
	}
	return types, nil
}

func (c *spaceResourceComponentImpl) ListAll(ctx context.Context) ([]types.SpaceResource, error) {
	dbResources, err := c.spaceResourceStore.FindAll(ctx)
	if err != nil {
		slog.Error("error listing all space resources", slog.Any("error", err))
		return nil, err
	}

	result := make([]types.SpaceResource, 0, len(dbResources))
	for _, r := range dbResources {
		scenarios := r.Scenarios
		if scenarios == nil {
			scenarios = []string{}
		}
		result = append(result, types.SpaceResource{
			ID:        r.ID,
			Name:      r.Name,
			ClusterID: r.ClusterID,
			Resources: r.Resources,
			Scenarios: scenarios,
		})
	}

	return result, nil
}
