package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
)

// QuotaRateComponent provides per-minute request/token rate
// accounting for API keys backed by the account_access_token_rate table.
type QuotaRateComponent interface {
	// CheckRateLimit reports whether the given API key exceeded its
	// per-minute rate window. RequestQuota limits requests per window and
	// TokenQuota limits total tokens per window; a non-positive quota means
	// the dimension is not limited. The returned RetryAfterSeconds of the
	// error is the remaining window time.
	ReadRateLimit(ctx context.Context, tokenID int64) (*database.AccountAccessTokenRateStat, error)
	// RecordUsage records one API access with its token consumption at the
	// given timestamp (unix seconds) and deletes rate records older than the
	// retention horizon.
	RecordUsage(ctx context.Context, tokenID int64, tokenUsed int64) error
}
