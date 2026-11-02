//go:build !ee && !saas

package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/aigateway/types"
)

// autoModel is unavailable in CE, even if Semantic Router is configured.
func (m *openaiComponentImpl) autoModel(context.Context) (types.Model, bool) {
	return types.Model{}, false
}

func (m *openaiComponentImpl) AutoModelID() string { return "" }

func (m *openaiComponentImpl) ResolveAutoModel(context.Context, types.AutoRouteRequest) (*types.AutoRouteDecision, error) {
	return nil, fmt.Errorf("automatic model routing is not available in CE")
}
