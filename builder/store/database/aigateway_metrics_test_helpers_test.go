package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
)

// ensureAIGatewayMetricsTables drops and recreates both aigateway metrics
// tables from the CURRENT Go models, so the store tests are self-contained
// and do not depend on migration history (the request_model schema change of
// this branch is applied out-of-band in production environments).  The old
// migrations create the tables without the new columns, and a bare
// IfNotExists create would silently keep the stale shape.
func ensureAIGatewayMetricsTables(t *testing.T, db *database.DB) {
	t.Helper()
	ctx := context.TODO()
	err := db.RunInTx(ctx, func(ctx context.Context, tx database.Operator) error {
		// Views built by the schema migrations depend on the events table;
		// drop them together (they are recreated by migrations/DBA scripts,
		// never by the store tests).
		if _, err := tx.Core.NewDropTable().Model((*database.AIGatewayMetricEvent)(nil)).IfExists().Cascade().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.Core.NewDropTable().Model((*database.AIGatewayMetricMinute)(nil)).IfExists().Cascade().Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.Core.NewCreateTable().Model((*database.AIGatewayMetricEvent)(nil)).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.Core.NewCreateTable().Model((*database.AIGatewayMetricMinute)(nil)).Exec(ctx)
		return err
	})
	require.NoError(t, err, "failed to ensure aigateway metrics tables match the models")
}
