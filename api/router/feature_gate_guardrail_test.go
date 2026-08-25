package router

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// The /api/v1/admin/audit_logs route must stay gated behind the license
// FeatureGate so a route rename cannot silently un-gate the feature
// (fail-open drift). gin's Routes() does not expose the middleware chain, so
// this source-scan encodes the invariant directly.
func TestAuditLogsRouteStaysFeatureGated(t *testing.T) {
	const want = `adminGroup.GET("/audit_logs", middlewareCollection.Auth.NeedAdmin, middlewareCollection.License.FeatureGate(types.FeatureAuditLog), auditLogHandler.ListAuditLogs)`
	for _, file := range []string{"api_ee.go", "api_saas.go"} {
		src, err := os.ReadFile(file)
		require.NoError(t, err)
		require.Contains(t, string(src), want, "%s must keep /audit_logs gated behind FeatureGate(types.FeatureAuditLog)", file)
	}
}
