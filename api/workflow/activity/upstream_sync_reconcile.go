package activity

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/aigateway/component/upstream"
)

// UpstreamSyncReconcile is the Temporal activity that performs full reconciliation
// of deploy→upstream sync. It constructs the sync component and calls
// ReconcileUpstreams, which syncs all running deploys and disables ghost upstreams.
func (a *Activities) UpstreamSyncReconcile(ctx context.Context) error {
	syncComp := upstream.NewAIGatewayUpstreamSyncComponentDefault(a.config)

	if err := syncComp.ReconcileUpstreams(ctx); err != nil {
		slog.ErrorContext(ctx, "upstream sync reconcile failed", slog.Any("error", err))
		return fmt.Errorf("upstream sync reconcile: %w", err)
	}

	slog.InfoContext(ctx, "upstream sync reconcile completed")
	return nil
}
