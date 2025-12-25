package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/types"
)

func TestScene_IsNeedCalculateBill(t *testing.T) {

	scenes := []types.SceneType{
		types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless,
		types.SceneStarship,
		types.SceneGuiAgent,
	}

	for _, scene := range scenes {
		res := IsNeedCalculateBill(scene)
		require.True(t, res)
	}

	scenes = []types.SceneType{
		types.SceneReserve,
		types.ScenePortalCharge,
		types.ScenePayOrder,
		types.SceneCashCharge,
		types.SceneMultiSync,
		types.SceneUnknow,
	}

	for _, scene := range scenes {
		res := IsNeedCalculateBill(scene)
		require.False(t, res)
	}

}

func TestScene_GetSkuUnitTypeByScene(t *testing.T) {

	scenes := map[types.SceneType]types.SkuUnitType{
		types.SceneModelInference:  types.UnitMinute,
		types.SceneSpace:           types.UnitMinute,
		types.SceneModelFinetune:   types.UnitMinute,
		types.SceneMultiSync:       types.UnitRepo,
		types.SceneEvaluation:      types.UnitMinute,
		types.SceneModelServerless: types.UnitToken,
		types.SceneStarship:        types.UnitToken,
		types.SceneGuiAgent:        types.UnitToken,
	}

	for scene, unit := range scenes {
		res := GetSkuUnitTypeByScene(scene)
		require.Equal(t, unit, res)
	}
}

func TestScene_GetSKUTypeByScene(t *testing.T) {
	scenes := map[types.SceneType]types.SKUType{
		types.SceneModelInference:  types.SKUCSGHub,
		types.SceneSpace:           types.SKUCSGHub,
		types.SceneModelFinetune:   types.SKUCSGHub,
		types.SceneMultiSync:       types.SKUCSGHub,
		types.SceneEvaluation:      types.SKUCSGHub,
		types.SceneModelServerless: types.SKUCSGHub,
		types.SceneStarship:        types.SKUStarship,
		types.SceneGuiAgent:        types.SKUStarship,
	}

	for scene, skuType := range scenes {
		res := GetSKUTypeByScene(scene)
		require.Equal(t, skuType, res)
	}
}

func TestAccountUsersStore_IsNeedCheckUserSubscription(t *testing.T) {
	scenes := map[types.SceneType]bool{
		types.SceneModelInference:  true,
		types.SceneSpace:           true,
		types.SceneModelFinetune:   true,
		types.SceneEvaluation:      true,
		types.SceneModelServerless: true,
		types.SceneGuiAgent:        false,
		types.SceneStarship:        true,
	}

	for scene, skuType := range scenes {
		res := IsNeedCheckUserSubscription(scene)
		require.Equal(t, skuType, res)
	}
}
