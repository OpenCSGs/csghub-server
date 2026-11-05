//go:build !ee && !saas

package plan

import "opencsg.com/csghub-server/aigateway/types"

func routingInputCategory(code string) types.PlanErrorCategory { return types.PlanErrUnknown }
