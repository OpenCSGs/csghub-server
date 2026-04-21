//go:build !saas

package workflow

import (
	"context"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func DatasetForkWorkflow(ctx context.Context, req types.CreateForkReq, config *config.Config) error {
	// Empty implementation for CE version
	return nil
}
