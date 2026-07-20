//go:build !ee && !saas

package component

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestSpaceResourceComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "c1"}, math.MaxInt, 1).Return(
		[]database.SpaceResource{
			{ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`, Scenarios: int64(types.ScenarioFinetune)},
			{ID: 2, Name: "sr2", Resources: `{"memory": "1000"}`},
		}, 0, nil,
	)
	// FindByCode resolves deploy_type (FinetuneType=2) into the scenario row,
	// which carries both the name (for the bitmask) and the constraint.
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindByCode(ctx, types.FinetuneType).Return(
		&database.ScenarioConstraint{Scenario: "finetune", Code: types.FinetuneType, RequiredHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1}, nil)
	// FindAllOrdered returns the catalog used for the per-resource mask<->name
	// conversion (in memory, not per-resource DB hits).
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType}}, nil)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)
	req := &types.SpaceResourceIndexReq{
		ClusterIDs:  []string{"c1"},
		DeployType:  types.FinetuneType,
		CurrentUser: "user",
		Per:         50,
		Page:        1,
	}
	data, _, err := sc.Index(ctx, req)
	require.Nil(t, err)
	require.Equal(t, []types.SpaceResource{
		{
			ID: 1, Name: "sr", Resources: `{"memory": "1000", "gpu": {"num": "5"}}`,
			IsAvailable: false, Type: "gpu", Scenarios: []string{"finetune"},
		},
	}, data)

}

func TestSpaceResourceComponent_Index_No_Cluster(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "", ResourceType: "", HardwareType: ""}, math.MaxInt, 1).
		Return([]database.SpaceResource{}, 0, nil)
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindByCode(ctx, types.FinetuneType).Return(
		&database.ScenarioConstraint{Scenario: "finetune", Code: types.FinetuneType, RequiredHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1}, nil)
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType}}, nil)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "").Return(nil, nil)

	req := &types.SpaceResourceIndexReq{
		ClusterIDs:  []string{""},
		DeployType:  types.FinetuneType,
		CurrentUser: "user",
		Per:         50,
		Page:        1,
	}

	data, total, err := sc.Index(ctx, req)
	require.Nil(t, err)
	require.Equal(t, 0, total)
	require.Nil(t, data)
}

func TestSpaceResourceComponent_Index_With_Status_Filter(t *testing.T) {
	ctx := context.TODO()
	t.Run("found running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "cluster2"}, math.MaxInt, 1).
			Return([]database.SpaceResource{}, 20, nil)
		sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindByCode(ctx, types.FinetuneType).Return(
			&database.ScenarioConstraint{Scenario: "finetune", Code: types.FinetuneType, RequiredHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1}, nil)
		sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
			[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType}}, nil)
		sc.mocks.deployer.EXPECT().GetClusterById(ctx, "cluster2").Return(&types.ClusterRes{}, nil)
		req := &types.SpaceResourceIndexReq{
			ClusterIDs:  []string{"cluster2"},
			DeployType:  types.FinetuneType,
			CurrentUser: "user1",
			Per:         50,
			Page:        1,
		}
		_, _, err := sc.Index(ctx, req)
		require.Nil(t, err)
	})

	t.Run("no running cluster", func(t *testing.T) {
		sc := initializeTestSpaceResourceComponent(ctx, t)
		sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindByCode(ctx, types.FinetuneType).Return(
			&database.ScenarioConstraint{Scenario: "finetune", Code: types.FinetuneType, RequiredHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1}, nil)
		sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
			[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType}}, nil)
		req := &types.SpaceResourceIndexReq{
			DeployType:  types.FinetuneType,
			CurrentUser: "user1",
			Per:         50,
			Page:        1,
		}
		_, total, err := sc.Index(ctx, req)
		require.NoError(t, err)
		require.Equal(t, 0, total)
	})
}

// TestSpaceResourceComponent_Index_Sandbox_ExcludeHardware verifies that the
// sandbox scenario's exclude_hardware (HardwareMaskGraphic) rejects a CPU+GPU
// resource while accepting a pure-CPU resource. This is the "pure CPU" rule:
// required_hardware can only say "has CPU", so exclude is what blocks accelerators.
func TestSpaceResourceComponent_Index_Sandbox_ExcludeHardware(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, types.SpaceResourceFilter{ClusterID: "c1"}, math.MaxInt, 1).Return(
		[]database.SpaceResource{
			// pure CPU resource — should pass sandbox's exclude (no accelerator)
			{ID: 1, Name: "cpu-only", Resources: `{"memory": "1000"}`, Scenarios: int64(types.ScenarioSandbox)},
			// CPU+GPU resource — should be rejected (GPU is in exclude mask)
			{ID: 2, Name: "cpu-gpu", Resources: `{"memory": "1000", "gpu": {"num": "1"}}`, Scenarios: int64(types.ScenarioSandbox)},
		}, 0, nil,
	)
	// sandbox: required=0, exclude=HardwareMaskGraphic (no accelerator allowed)
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindByCode(ctx, types.SandboxType).Return(
		&database.ScenarioConstraint{Scenario: "sandbox", Code: types.SandboxType, RequiredHardware: 0, ExcludeHardware: int64(types.HardwareMaskGraphic), MaxReplica: 1}, nil)
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "sandbox", Code: types.SandboxType}}, nil)
	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)

	req := &types.SpaceResourceIndexReq{
		ClusterIDs:  []string{"c1"},
		DeployType:  types.SandboxType,
		CurrentUser: "user",
		Per:         50,
		Page:        1,
	}
	data, _, err := sc.Index(ctx, req)
	require.Nil(t, err)
	// only the pure-CPU resource survives; the CPU+GPU one is filtered out
	require.Equal(t, []types.SpaceResource{
		{ID: 1, Name: "cpu-only", Resources: `{"memory": "1000"}`, IsAvailable: false, Type: "cpu", Scenarios: []string{"sandbox"}},
	}, data)
}
