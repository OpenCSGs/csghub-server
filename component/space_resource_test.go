package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const validResourcesJSON = `{"cpu":{"num":"2","type":"intel"},"memory":"8G"}`

func TestValidateResources(t *testing.T) {
	t.Run("empty resources", func(t *testing.T) {
		err := validateResources("")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("blank resources", func(t *testing.T) {
		err := validateResources("   ")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("invalid json", func(t *testing.T) {
		err := validateResources("not-json")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("valid hardware json", func(t *testing.T) {
		err := validateResources(validResourcesJSON)
		require.Nil(t, err)
	})
	t.Run("empty json object missing required fields", func(t *testing.T) {
		err := validateResources("{}")
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("json missing cpu type", func(t *testing.T) {
		err := validateResources(`{"cpu":{"num":"2"},"memory":"8G"}`)
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("json missing cpu num", func(t *testing.T) {
		err := validateResources(`{"cpu":{"type":"intel"},"memory":"8G"}`)
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
	t.Run("json missing memory", func(t *testing.T) {
		err := validateResources(`{"cpu":{"type":"intel","num":"2"}}`)
		require.True(t, errors.Is(err, errorx.ErrBadRequest))
	})
}

func TestSpaceResourceComponent_Update(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{}, nil,
	)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Update(ctx, database.SpaceResource{
		Name:      "n",
		Resources: validResourcesJSON,
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: validResourcesJSON}, nil)
	// Update loads the scenario catalog once (FindAllOrdered) for the
	// mask<->name conversion when building the response.
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{}, nil)

	data, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
		Scenarios: []string{},
	}, data)
}

func TestSpaceResourceComponent_Update_InvalidResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: "invalid",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

// TestSpaceResourceComponent_Update_ResourcesOnly_HardwareConflict verifies that
// a Resources-only update (req.Scenarios == nil) still validates the EXISTING
// scenarios against the new hardware. A resource tagged "sandbox" (excludes
// accelerators) must be rejected when the update adds a GPU — otherwise the
// resource ends up in an inconsistent GPU+sandbox state.
func TestSpaceResourceComponent_Update_ResourcesOnly_HardwareConflict(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	// existing resource is sandbox-tagged (bit7 = 128)
	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{ID: 1, Scenarios: int64(types.ScenarioSandbox)}, nil)
	// catalog knows sandbox with exclude = HardwareMaskGraphic (no accelerator)
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "sandbox", Code: types.SandboxType, ExcludeHardware: int64(types.HardwareMaskGraphic)}}, nil)

	// update adds a GPU but does NOT pass Scenarios — existing sandbox must be
	// validated against the new hardware and rejected.
	_, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
		ID:        1,
		Name:      "n",
		Resources: `{"cpu":{"num":"2","type":"intel"},"memory":"8G","gpu":{"num":"1","type":"nvidia"}}`,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestSpaceResourceComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	// Create loads the scenario catalog once (FindAllOrdered) for the
	// names<->mask conversion; an empty catalog is fine for an empty scenarios list.
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{}, nil)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Create(ctx, database.SpaceResource{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
		Scenarios: 0,
	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: validResourcesJSON, Scenarios: 0}, nil)

	data, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
	})
	require.Nil(t, err)
	require.Equal(t, &types.SpaceResource{
		ID:        1,
		Name:      "n",
		Resources: validResourcesJSON,
		Scenarios: []string{},
	}, data)
}

func TestSpaceResourceComponent_Create_EmptyResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: "",
		ClusterID: "c",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestSpaceResourceComponent_Create_InvalidResources(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: "not-json",
		ClusterID: "c",
	})
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

// TestSpaceResourceComponent_Create_UnknownScenario verifies that an unknown
// scenario name (not in the catalog) is rejected with BadRequest instead of
// being silently dropped to a 0 mask (which would make the resource vanish from
// scenario-filtered queries).
func TestSpaceResourceComponent_Create_UnknownScenario(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	// catalog only knows "finetune"; "bogus" is unknown
	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType}}, nil)

	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
		Scenarios: []string{"finetune", "bogus"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

// TestSpaceResourceComponent_Create_ScenarioHardwareMismatch verifies that a
// scenario whose required_hardware is not satisfied by the resource hardware
// is rejected with BadRequest (server-side enforcement of the frontend grey-out).
// A pure-CPU resource selecting finetune (requires a graphic accelerator) fails.
func TestSpaceResourceComponent_Create_ScenarioHardwareMismatch(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "finetune", Code: types.FinetuneType, RequiredHardware: int64(types.HardwareMaskGraphic)}}, nil)

	// validResourcesJSON is pure CPU (no accelerator) — finetune requires one.
	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: validResourcesJSON,
		ClusterID: "c",
		Scenarios: []string{"finetune"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

// TestSpaceResourceComponent_Create_ScenarioExcludeHardware verifies that a
// scenario whose exclude_hardware is hit by the resource hardware is rejected.
// A CPU+GPU resource selecting sandbox (excludes graphic accelerators) fails.
func TestSpaceResourceComponent_Create_ScenarioExcludeHardware(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
		[]database.ScenarioConstraint{{Scenario: "sandbox", Code: types.SandboxType, ExcludeHardware: int64(types.HardwareMaskGraphic)}}, nil)

	// CPU+GPU resource — sandbox excludes accelerators.
	_, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
		Name:      "n",
		Resources: `{"cpu":{"num":"2","type":"intel"},"memory":"8G","gpu":{"num":"1","type":"nvidia"}}`,
		ClusterID: "c",
		Scenarios: []string{"sandbox"},
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, errorx.ErrBadRequest))
}

func TestSpaceResourceComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSpaceResourceComponent(ctx, t)

	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
		&database.SpaceResource{}, nil,
	)
	sc.mocks.stores.SpaceResourceMock().EXPECT().Delete(ctx, database.SpaceResource{}).Return(nil)
	sc.mocks.components.accounting.EXPECT().OffLinePrice(ctx, types.AcctPriceOffLineReq{
		SkuType:    types.SKUCSGHub,
		ResourceID: "1",
	}).Return(nil, nil)

	err := sc.Delete(ctx, 1)
	require.Nil(t, err)
}

func TestSpaceResourceComponent_ListHardwareTypes(t *testing.T) {
	t.Run("list hardware types", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)

		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAllResourceTypes(ctx, "c1").Return(
			[]string{"type1", "type2"}, nil,
		)

		types, err := sc.ListHardwareTypes(ctx, "c1")
		require.Nil(t, err)
		require.Equal(t, []string{"type1", "type2"}, types)
	})
	t.Run("error listing hardware types", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)
		assertError := errors.New("database error")
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAllResourceTypes(ctx, "c1").Return(
			nil, assertError,
		)

		types, err := sc.ListHardwareTypes(ctx, "c1")
		require.NotNil(t, err)
		require.Nil(t, types)
	})
}

func TestSpaceResourceComponent_ListAll(t *testing.T) {
	t.Run("list all resources", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)

		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAll(ctx).Return(
			[]database.SpaceResource{
				{ID: 1, Name: "resource1", ClusterID: "c1", Resources: "{}"},
				{ID: 2, Name: "resource2", ClusterID: "c2", Resources: "{}"},
			}, nil,
		)
		// ListAll loads the scenario catalog once (FindAllOrdered) for the
		// per-resource mask<->name conversion.
		sc.mocks.stores.ScenarioConstraintMock().EXPECT().FindAllOrdered(ctx).Return(
			[]database.ScenarioConstraint{}, nil)

		resources, err := sc.ListAll(ctx)
		require.Nil(t, err)
		require.Equal(t, []types.SpaceResource{
			{ID: 1, Name: "resource1", ClusterID: "c1", Resources: "{}", Scenarios: []string{}},
			{ID: 2, Name: "resource2", ClusterID: "c2", Resources: "{}", Scenarios: []string{}},
		}, resources)
	})
	t.Run("error listing all resources", func(t *testing.T) {
		ctx := context.TODO()
		sc := initializeTestSpaceResourceComponent(ctx, t)
		assertError := errors.New("database error")
		sc.mocks.stores.SpaceResourceMock().EXPECT().FindAll(ctx).Return(
			nil, assertError,
		)

		resources, err := sc.ListAll(ctx)
		require.NotNil(t, err)
		require.Nil(t, resources)
	})
}
