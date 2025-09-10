//go:build !ee && !saas

package workflow

import (
	"go.temporal.io/sdk/workflow"

	"opencsg.com/csghub-server/common/types"
)

func ScanModelTreeWorkflow(ctx workflow.Context, req types.ScanModels) error {
	return nil
}
