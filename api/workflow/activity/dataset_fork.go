//go:build saas

package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

func CreateDatasetFork(ctx context.Context, req types.CreateForkReq, config *config.Config) error {
	logger := activity.GetLogger(ctx)
	logger.Info("create dataset fork start", "source", req.SourceNamespace+"/"+req.SourceName, "target", req.TargetNamespace+"/"+req.TargetName)

	datasetComponent, err := component.NewDatasetComponent(config)
	if err != nil {
		return fmt.Errorf("failed to create dataset component, error: %w", err)
	}

	_, err = datasetComponent.CreateFork(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create dataset fork, error: %w", err)
	}

	return nil
}
