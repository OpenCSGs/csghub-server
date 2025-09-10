//go:build !ee && !saas

package component

import (
	"context"
	"opencsg.com/csghub-server/common/types"
)

func (sc *statComponentImpl) GetStatSnap(ctx context.Context, req types.StatSnapshotReq) (*types.StatSnapshotResp, error) {
	return nil, nil
}

func (sc *statComponentImpl) MakeStatSnap(ctx context.Context) error {
	return nil
}

func (sc *statComponentImpl) StatRunningDeploys(ctx context.Context) (map[int]*types.StatRunningDeploy, error) {
	return nil, nil
}
