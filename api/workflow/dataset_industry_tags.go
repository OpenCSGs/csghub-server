package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"opencsg.com/csghub-server/common/types"
)

func ScanRepoIndustryTagsWorkflow(ctx workflow.Context, req types.ScanRepoIndustryTagsReq) error {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Hour,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}

	actCtx := workflow.WithActivityOptions(ctx, options)
	return workflow.ExecuteActivity(actCtx, activities.ScanRepoIndustryTags, req).Get(ctx, nil)
}
