//go:build !ee && !saas

package component

import "context"

// rewriteURLViaGateway is a no-op in CE (community edition) where
// StorageGatewayComponent is not available. The original URL is returned unchanged.
func (c *evaluationComponentImpl) rewriteURLViaGateway(ctx context.Context, originalURL string) string {
	return originalURL
}

// rewriteURLViaGateway is a no-op in CE for userComponentImpl as well.
func (c *userComponentImpl) rewriteURLViaGateway(ctx context.Context, originalURL string) string {
	return originalURL
}
