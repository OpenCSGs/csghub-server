//go:build !ee && !saas

package component

import (
	"context"
	"opencsg.com/csghub-server/aigateway/types"
)

func ObserveRouteCall(ctx context.Context, target types.RouteCallTarget, body []byte) *RouteCallObservation {
	return &RouteCallObservation{}
}
