package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestScenarioConstraintStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewScenarioConstraintStoreWithDB(db)

	// The migration seeds the scenario catalog (6 deploy + 4 workflow = 10 rows).
	// Verify a seeded row is readable via both name and code.
	got, err := store.FindByScenario(ctx, "finetune")
	require.Nil(t, err)
	require.Equal(t, types.FinetuneType, got.Code)
	require.Equal(t, int64(types.HardwareMaskGraphic), got.RequiredHardware)
	require.Equal(t, int64(0), got.ExcludeHardware)
	require.Equal(t, 1, got.MaxReplica)

	// sandbox expresses "pure CPU" via exclude_hardware (no accelerator), and
	// required_hardware is 0 (exclude alone enforces it).
	sandbox, err := store.FindByScenario(ctx, "sandbox")
	require.Nil(t, err)
	require.Equal(t, int64(0), sandbox.RequiredHardware)
	require.Equal(t, int64(types.HardwareMaskGraphic), sandbox.ExcludeHardware)

	gotByCode, err := store.FindByCode(ctx, types.FinetuneType)
	require.Nil(t, err)
	require.Equal(t, "finetune", gotByCode.Scenario)
	require.Equal(t, "scenario.finetune", gotByCode.I18nKey)
	require.Equal(t, string(types.ScenarioCategoryDeploy), gotByCode.Category)

	// FindByCode on an unknown code returns (nil, nil) — no row, not an error.
	unknown, err := store.FindByCode(ctx, 999)
	require.Nil(t, err)
	require.Nil(t, unknown)

	// Upsert update on a seeded scenario (same unique key => update).
	_, err = store.Upsert(ctx, database.ScenarioConstraint{
		Scenario:         "finetune",
		Code:             types.FinetuneType,
		Category:         string(types.ScenarioCategoryDeploy),
		I18nKey:          "scenario.finetune",
		RequiredHardware: 2, // now only gpu
		ExcludeHardware:  int64(types.HardwareCPU),
		MaxReplica:       4,
	})
	require.Nil(t, err)
	got2, err := store.FindByScenario(ctx, "finetune")
	require.Nil(t, err)
	require.Equal(t, int64(2), got2.RequiredHardware)
	require.Equal(t, int64(types.HardwareCPU), got2.ExcludeHardware)
	require.Equal(t, 4, got2.MaxReplica)

	// Upsert insert on a brand-new scenario with a code not used by the seed
	// (bit 10 is unassigned). The unique key is scenario, so a new name inserts.
	_, err = store.Upsert(ctx, database.ScenarioConstraint{
		Scenario:         "custom_scenario",
		Code:             10,
		Category:         string(types.ScenarioCategoryDeploy),
		I18nKey:          "scenario.custom",
		RequiredHardware: 0,
		MaxReplica:       0,
	})
	require.Nil(t, err)
	got3, err := store.FindByScenario(ctx, "custom_scenario")
	require.Nil(t, err)
	require.Equal(t, "custom_scenario", got3.Scenario)
	require.Equal(t, 10, got3.Code)

	// FindAllOrdered returns every row ordered by code; the 10 seeded rows
	// (6 deploy + 4 workflow, finetune updated in place) plus the 1 inserted = 11.
	all, err := store.FindAllOrdered(ctx)
	require.Nil(t, err)
	require.Equal(t, 11, len(all))
	// ordered by code ascending: space(0), inference(1), finetune(2), ...
	require.Equal(t, 0, all[0].Code)
	require.Equal(t, 1, all[1].Code)
	require.Equal(t, 2, all[2].Code)

	// Delete the inserted row, then confirm it's gone (no row => nil, nil).
	err = store.Delete(ctx, "custom_scenario")
	require.Nil(t, err)
	gone, err := store.FindByScenario(ctx, "custom_scenario")
	require.Nil(t, err)
	require.Nil(t, gone)
}
