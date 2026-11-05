package component

import (
	"context"

	"opencsg.com/csghub-server/aigateway/types"
)

// AutoModelRouter backs the virtual "auto" model in EE and SaaS.
// The shared component keeps this contract so all editions satisfy the
// OpenAIComponent interface; CE never constructs a router.
type AutoModelRouter interface {
	ModelID() string
	Available(ctx context.Context) bool
	Select(ctx context.Context, input types.AutoRouteInput, candidates []types.Model) (*types.AutoRouteDecision, error)
}
