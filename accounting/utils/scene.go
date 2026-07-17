package utils

import "opencsg.com/csghub-server/common/types"

func IsUseVoucher(scene types.SceneType) bool {
	switch scene {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation:
		return true
	default:
		return false
	}
}

func IsNeedCalculateBill(scene types.SceneType) bool {
	switch scene {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless,
		types.SceneMultiModalServerless,
		// types.SceneStarship, deprecated
		// types.SceneGuiAgent, deprecated
		types.ScenePortalCharge,
		types.SceneCashCharge:
		return true
	default:
		return false
	}
}

func IsNeedExtractHardware(scene types.SceneType) bool {
	switch scene {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation:
		return true
	default:
		return false
	}
}

func GetSkuUnitTypeByScene(scene types.SceneType) types.SkuUnitType {
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
	case types.SceneModelServerless:
		return types.UnitToken
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
	case types.SceneModelServerless:
		return types.SKUCSGHub
	case types.SceneMultiModalServerless:
		return types.SKUCSGHub
	case types.SceneStarship:
		return types.SKUStarship
	case types.SceneGuiAgent:
		return types.SKUStarship
	}
	return types.SKUReserve
}

func IsNeedCheckMeteringInMinute(scene types.SceneType, valueType types.ChargeValueType) bool {
	switch scene {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless:
		switch valueType {
		case types.TokenNumberType:
			return false
		default:
			return true
		}
	default:
		return false
	}
}

func IsNeedCheckUserSubscription(scene types.SceneType) bool {
	switch types.SceneType(scene) {
	case types.SceneModelInference,
		types.SceneSpace,
		types.SceneModelFinetune,
		types.SceneEvaluation,
		types.SceneModelServerless,
		types.SceneStarship:
		return true
	default:
		return false
	}
}

func IsGetTokenID(scene types.SceneType) bool {
	switch scene {
	case types.SceneModelServerless,
		types.SceneMultiModalServerless:
		return true
	}
	return false
}

// IsChargeScene reports whether the scene is a recharge (balance-increasing)
// event rather than a consumption event. Recharge scenes have positive value
// (money added to the balance); consumption scenes have negative value. The
// daily summary rollup excludes recharge scenes so the consumption report only
// reflects actual usage.
func IsChargeScene(scene types.SceneType) bool {
	switch scene {
	case types.ScenePortalCharge, types.SceneCashCharge:
		return true
	default:
		return false
	}
}

// ChargeSceneValues returns the scene values that are recharges, for use in
// SQL "scene NOT IN (...)" filters during rollup.
func ChargeSceneValues() []int {
	return []int{int(types.ScenePortalCharge), int(types.SceneCashCharge)}
}
