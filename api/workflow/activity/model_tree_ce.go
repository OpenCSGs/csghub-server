//go:build !ee && !saas

package activity

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) UpdateModelTree(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	return nil
}

func (a *Activities) ScanModelTree(ctx context.Context, req types.ScanModels) error {
	return nil
}
