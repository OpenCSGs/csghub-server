package activity

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) RuntimeFrameworkScan(ctx context.Context, req types.RuntimeFrameworkModels) error {
	return a.rftScanner.ScanArchitecture(ctx, req.ID, req.ScanType, req.Task, req.Models)
}
