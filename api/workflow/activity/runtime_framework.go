package activity

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (a *Activities) RuntimeFrameworkScan(ctx context.Context, req types.RuntimeFrameworkModels) error {
	return a.rftScanner.ScanAllModels(ctx, req.ScanType)
}
