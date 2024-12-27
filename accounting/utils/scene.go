package utils

import "opencsg.com/csghub-server/common/types"

func IsNeedCalculateBill(scene types.SceneType) bool {
	switch scene {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneStarship,
		types.SceneGuiAgent:
		return true
	default:
		return false
	}
}

func GetSkuUnitTypeByScene(scene types.SceneType) string {
	switch scene {
	case types.SceneModelInference:
		return types.UnitMinute
	case types.SceneSpace:
		return types.UnitMinute
	case types.SceneModelFinetune:
		return types.UnitMinute
	case types.SceneMultiSync:
		return types.UnitRepo
	case types.SceneEvaluation:
		return types.UnitMinute
	case types.SceneStarship:
		return types.UnitToken
	case types.SceneGuiAgent:
		return types.UnitToken
	default:
		return types.UnitMinute
	}
}

func GetSKUTypeByScene(scene types.SceneType) types.SKUType {
	switch scene {
	case types.SceneModelInference:
		return types.SKUCSGHub
	case types.SceneSpace:
		return types.SKUCSGHub
	case types.SceneModelFinetune:
		return types.SKUCSGHub
	case types.SceneMultiSync:
		return types.SKUCSGHub
	case types.SceneEvaluation:
		return types.SKUCSGHub
	case types.SceneStarship:
		return types.SKUStarship
	case types.SceneGuiAgent:
		return types.SKUStarship
	}
	return types.SKUReserve
}
