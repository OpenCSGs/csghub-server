//go:build !ee && !saas

package handler

import "opencsg.com/csghub-server/aigateway/types"

func modelSKUPriceStatus(model *types.Model) (requiresSKUPrice bool, hasConfiguredSKUPrice bool) {
	return false, false
}

func checkModalRequestAllowed(model *types.Model, size string) *modelTargetError {
	return nil
}
