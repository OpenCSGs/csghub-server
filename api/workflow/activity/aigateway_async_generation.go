package activity

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/common/types"
)

const aigatewayAsyncGenerationConcurrency = 20

func (a *Activities) ListPendingAIGatewayAsyncGenerations(ctx context.Context) ([]types.AIGatewayAsyncGenerationTarget, error) {
	return a.asyncGenerationService.ListPendingGenerations(ctx)
}

func (a *Activities) InspectAndMeterAIGatewayAsyncGenerations(ctx context.Context, targets []types.AIGatewayAsyncGenerationTarget) error {
	if len(targets) == 0 {
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(aigatewayAsyncGenerationConcurrency)

	for _, target := range targets {
		target := target

		g.Go(func() error {
			if ctx.Err() != nil {
				return nil
			}

			if err := a.asyncGenerationService.InspectAndMeter(ctx, target); err != nil {
				slog.WarnContext(ctx,
					"failed to inspect and meter aigateway async generation",
					"resource_type", target.ResourceType,
					"resource_id", target.ResourceID,
					"provider_resource_id", target.ProviderResourceID,
					"error", err,
				)
			}
			return nil
		})
	}

	_ = g.Wait()
	return nil
}
