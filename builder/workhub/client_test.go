package workhub

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
)

// TestStopWorkClientKeepsPoolOpenAfterStopFailure verifies timed-out shutdown remains retryable.
func TestStopWorkClientKeepsPoolOpenAfterStopFailure(t *testing.T) {
	poolClosed := false

	err := stopWorkClient(
		context.Background(),
		func(context.Context) error { return context.DeadlineExceeded },
		func() { poolClosed = true },
	)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.False(t, poolClosed)
}

// TestNewWorkClientRetainsDiscardedJobs verifies worker maintenance never deletes terminal failed jobs.
func TestNewWorkClientRetainsDiscardedJobs(t *testing.T) {
	config := &river.Config{}
	client, err := NewWorkClient(context.Background(), "postgresql://user:password@localhost/database", config)
	require.NoError(t, err)
	t.Cleanup(client.(*workClient).pool.Close)

	require.Equal(t, time.Duration(-1), config.DiscardedJobRetentionPeriod)
}

// TestNewWorkClientRescueAndLeaseConfig preserves opt-in leases with one rescue threshold.
func TestNewWorkClientRescueAndLeaseConfig(t *testing.T) {
	for _, leased := range []bool{false, true} {
		t.Run(fmt.Sprintf("leased=%t", leased), func(t *testing.T) {
			cfg := &river.Config{RescueStuckJobsAfter: 2 * time.Hour}
			if leased {
				cfg.JobLease = &river.JobLeaseConfig{}
			}
			client, err := NewWorkClient(context.Background(), "postgresql://user:password@localhost/database", cfg)
			require.NoError(t, err)
			t.Cleanup(client.(*workClient).pool.Close)
			require.Equal(t, 10*time.Minute, cfg.RescueStuckJobsAfter)
			require.Equal(t, leased, cfg.JobLease != nil)
		})
	}
}

// TestLinearJitterRetryPolicy verifies retries use five-minute linear steps with ten-percent jitter.
func TestLinearJitterRetryPolicy(t *testing.T) {
	policy := &linearJitterRetryPolicy{}
	for retry := 1; retry <= 3; retry++ {
		retry := retry
		t.Run(fmt.Sprintf("retry_%d", retry), func(t *testing.T) {
			job := &rivertype.JobRow{Errors: make([]rivertype.AttemptError, retry-1)}
			baseDelay := time.Duration(retry) * 5 * time.Minute
			before := time.Now().UTC()

			nextRetry := policy.NextRetry(job)

			after := time.Now().UTC()
			require.False(t, nextRetry.Before(before.Add(baseDelay-baseDelay/10)))
			require.False(t, nextRetry.After(after.Add(baseDelay+baseDelay/10)))
		})
	}
}

// TestInsertOptsDefaultsToThreeRetries verifies the initial attempt is followed by three retries.
func TestInsertOptsDefaultsToThreeRetries(t *testing.T) {
	opts := (&InsertOpts{}).riverInsertOpts()
	require.Equal(t, 4, opts.MaxAttempts)
}
