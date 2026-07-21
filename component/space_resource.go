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
	// ListScenarios returns every known deploy/workflow scenario with its code
	// (bit position, passed as deploy_type to Index), name and display name, read
	// from the space_resource_scenario_constraints table. Lets callers discover
	// scenario codes dynamically instead of hardcoding them.
	ListScenarios(ctx context.Context) ([]types.ScenarioInfo, error)
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
	// req.DeployType is a scenario code (bit position): 0-7 deploy scenarios,
	// 32-38 workflow scenarios (see space_resource_scenario_constraints table).
	// scenarioMask is used to filter resources by their scenarios bitmask (a
	// resource is included only if it supports the requested scenario). A zero
	// scenarioMask means the caller did not pass a known scenario code, so no
	// scenario filtering is applied.
	// FindByCode resolves the code into the scenario row in one query: the row
	// carries both the scenario name (for the bitmask) and the constraint
	// (required_hardware / max_replica) that drives the filtering below. A nil
	// row means the code is not a known scenario => no scenario filtering and no
	// constraint (replaces the previously hardcoded deployAvailable / replica
	// checks).
	scenarioRow, err := c.scenarioConstraintStore.FindByCode(ctx, req.DeployType)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario by code",
			slog.Int("code", req.DeployType), slog.Any("error", err))
		return nil, 0, fmt.Errorf("load scenario by code failed: %w", err)
	}
	var scenarioMask int64
	var constraint *database.ScenarioConstraint
	if scenarioRow != nil {
		scenarioMask = int64(1) << uint(scenarioRow.Code)
		constraint = scenarioRow
	}
	// Load the full scenario catalog once so the per-resource mask<->name
	// conversion below stays in memory instead of hitting the DB per resource.
	catalog, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
		return nil, 0, fmt.Errorf("load scenario catalog failed: %w", err)
	}
	for _, clusterID := range req.ClusterIDs {
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
			// Filter by the resource's scenarios bitmask: skip resources that do
			// not support the requested scenario. scenarioMask == 0 (no known
			// scenario code passed) disables this filter.
			if scenarioMask != 0 && r.Scenarios&scenarioMask == 0 {
				continue
			}
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
			if !c.hardwareSatisfiesConstraint(constraint, hardware) {
				// resource hardware does not meet the scenario's required_hardware bitmask
				continue
			}
			if !c.replicaSatisfiesConstraint(constraint, hardware.Replicas) {
				// resource replicas exceed the scenario's max_replica (0 = unlimited)
				continue
			}
			resourceType := common.ResourceType(hardware)
			scenarios := maskToScenarioNames(catalog, r.Scenarios)
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
		}

		err = c.updatePriceInfo(req, singleClusterResult)
		if err != nil {
			slog.ErrorContext(ctx, "failed to update space resource price info", slog.String("clusterID", clusterID), slog.Any("error", err))
			return nil, 0, err
		}

		// Filter by IsAvailable AFTER updatePriceInfo, because updatePriceInfo
		// may change IsAvailable (e.g. marking resources without price as
		// unavailable). Filtering before updatePriceInfo would pass resources
		// that later become unavailable, or miss resources that are
		// unavailable due to price but pass the cluster resource check.
		for i := range singleClusterResult {
			if req.IsAvailable != nil && *req.IsAvailable != singleClusterResult[i].IsAvailable {
				continue
			}
			result = append(result, singleClusterResult[i])
		}
	}
	total = len(result)
	return result, total, nil
}

// hardwareSatisfiesConstraint reports whether the resource hardware meets the
// scenario's hardware constraints. Delegates to the shared
// types.HardwareSatisfiesConstraint so the Index query and the sandbox
// auto-allocator apply identical rules. A nil constraint means no rule
// configured => always satisfied.
func (c *spaceResourceComponentImpl) hardwareSatisfiesConstraint(constraint *database.ScenarioConstraint, hardware types.HardWare) bool {
	if constraint == nil {
		return true
	}
	return types.HardwareSatisfiesConstraint(constraint.RequiredHardware, constraint.ExcludeHardware, hardware)
}

// replicaSatisfiesConstraint reports whether the resource replica count is within
// the scenario's max_replica. Delegates to the shared types.ReplicaSatisfiesConstraint
// so the Index query and the sandbox auto-allocator apply identical rules. A nil
// constraint or a zero max_replica means "unlimited" (always satisfied).
func (c *spaceResourceComponentImpl) replicaSatisfiesConstraint(constraint *database.ScenarioConstraint, replicas int) bool {
	if constraint == nil {
		return true
	}
	return types.ReplicaSatisfiesConstraint(constraint.MaxReplica, replicas)
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
		catalog, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
			return nil, fmt.Errorf("load scenario catalog failed: %w", err)
		}
		mask, err := scenarioNamesToMask(catalog, sanitizeScenarios(req.Scenarios))
		if err != nil {
			return nil, err
		}
		sr.Scenarios = mask
	}

	sr, err = c.spaceResourceStore.Update(ctx, *sr)
	if err != nil {
		slog.Error("error updating space resource", slog.Any("error", err))
		return nil, err
	}

	catalog, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
		return nil, fmt.Errorf("load scenario catalog failed: %w", err)
	}
	scenarios := maskToScenarioNames(catalog, sr.Scenarios)
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
	catalog, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
		return nil, fmt.Errorf("load scenario catalog failed: %w", err)
	}
	sr := database.SpaceResource{
		Name:      req.Name,
		Resources: req.Resources,
		ClusterID: req.ClusterID,
	}
	mask, err := scenarioNamesToMask(catalog, sanitizeScenarios(req.Scenarios))
	if err != nil {
		return nil, err
	}
	sr.Scenarios = mask
	res, err := c.spaceResourceStore.Create(ctx, sr)
	if err != nil {
		slog.Error("error creating space resource", slog.Any("error", err))
		return nil, err
	}

	scenariosResult := maskToScenarioNames(catalog, res.Scenarios)
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

	catalog, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
		return nil, fmt.Errorf("load scenario catalog failed: %w", err)
	}

	result := make([]types.SpaceResource, 0, len(dbResources))
	for _, r := range dbResources {
		scenarios := maskToScenarioNames(catalog, r.Scenarios)
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

// ListScenarios returns every known scenario (deploy + workflow) with its code
// (bit position), name and display name, read from the
// space_resource_scenario_constraints table ordered by code.
func (c *spaceResourceComponentImpl) ListScenarios(ctx context.Context) ([]types.ScenarioInfo, error) {
	rows, err := c.scenarioConstraintStore.FindAllOrdered(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to load scenario catalog", slog.Any("error", err))
		return nil, fmt.Errorf("load scenario catalog failed: %w", err)
	}
	result := make([]types.ScenarioInfo, 0, len(rows))
	for _, r := range rows {
		result = append(result, types.ScenarioInfo{
			Code:     r.Code,
			Name:     r.Scenario,
			I18nKey:  r.I18nKey,
			Category: types.ScenarioCategory(r.Category),
		})
	}
	return result, nil
}

// maskToScenarioNames converts a scenarios bitmask into a list of scenario names
// using the in-memory catalog (loaded once via FindAllOrdered). A mask of -1
// (all bits set) returns every known scenario name. Unknown set bits are
// skipped. A zero mask returns an empty (non-nil) slice.
func maskToScenarioNames(catalog []database.ScenarioConstraint, mask int64) []string {
	if mask == int64(types.ScenarioAll) {
		result := make([]string, 0, len(catalog))
		for _, s := range catalog {
			result = append(result, s.Scenario)
		}
		return result
	}
	result := make([]string, 0, len(catalog))
	for _, s := range catalog {
		if mask&(int64(1)<<uint(s.Code)) != 0 {
			result = append(result, s.Scenario)
		}
	}
	return result
}

// scenarioNamesToMask converts a list of scenario names into a bitmask using the
// in-memory catalog (loaded once via FindAllOrdered). Every requested name must
// exist in the catalog; any unknown name (stale caller, typo, or a scenario not
// yet present in the DB) returns a BadRequest error so the request is rejected
// instead of silently storing 0 (which would make the resource vanish from all
// scenario-filtered queries). An empty list yields 0 (supports nothing) and is
// allowed — it means the caller explicitly bound no scenarios.
func scenarioNamesToMask(catalog []database.ScenarioConstraint, scenarios []string) (int64, error) {
	known := make(map[string]int, len(catalog))
	for _, s := range catalog {
		known[s.Scenario] = s.Code
	}
	var mask int64
	for _, name := range scenarios {
		code, ok := known[name]
		if !ok {
			return 0, errorx.BadRequest(fmt.Errorf("unknown scenario name %q", name), errorx.Ctx().Set("field", "scenarios").Set("scenario", name))
		}
		mask |= int64(1) << uint(code)
	}
	return mask, nil
}
