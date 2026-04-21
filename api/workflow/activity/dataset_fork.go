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

	// Update task status to "in_progress"
	logger.Info("updating dataset purchase task status to in_progress", "related_dataset_id", req.RelatedDatasetID)
	
	// Execute fork operation
	_, err = datasetComponent.CreateFork(ctx, req)
	if err != nil {
		logger.Error("failed to create dataset fork", "error", err)
		// Update task status to "failed"
		return fmt.Errorf("failed to create dataset fork, error: %w", err)
	}

	// Update task status to "completed"
	logger.Info("updating dataset purchase task status to completed", "related_dataset_id", req.RelatedDatasetID)

	return nil
}
