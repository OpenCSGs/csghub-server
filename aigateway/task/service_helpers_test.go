package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestStatusRefreshIntervalNilConfigUsesDefault(t *testing.T) {
	require.Equal(t, defaultStatusRefreshInterval, statusRefreshInterval(nil))
}

func TestStatusRefreshIntervalPositiveValueUsed(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.AsyncGenerationStatusRefreshInterval = 30
	require.Equal(t, 30*time.Second, statusRefreshInterval(cfg))
}

func TestStatusRefreshIntervalZeroOrNegativeUsesDefault(t *testing.T) {
	for _, v := range []int{0, -1, -1000} {
		t.Run("value", func(t *testing.T) {
			cfg := &config.Config{}
			cfg.AIGateway.AsyncGenerationStatusRefreshInterval = v
			require.Equal(t, defaultStatusRefreshInterval, statusRefreshInterval(cfg))
		})
	}
}

func TestMeteringBatchSizeNilConfigUsesDefault(t *testing.T) {
	require.Equal(t, defaultMeteringBatchSize, meteringBatchSize(nil))
}

func TestMeteringBatchSizePositiveValueUsed(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.AsyncGenerationMeteringBatchSize = 42
	require.Equal(t, 42, meteringBatchSize(cfg))
}

func TestMeteringBatchSizeZeroOrNegativeUsesDefault(t *testing.T) {
	for _, v := range []int{0, -1, -1000} {
		t.Run("value", func(t *testing.T) {
			cfg := &config.Config{}
			cfg.AIGateway.AsyncGenerationMeteringBatchSize = v
			require.Equal(t, defaultMeteringBatchSize, meteringBatchSize(cfg))
		})
	}
}

func TestAsyncGenerationMaxAgeFromConfigNilConfigUsesDefault(t *testing.T) {
	require.Equal(t, defaultAsyncGenerationMaxAge, asyncGenerationMaxAgeFromConfig(nil))
}

func TestAsyncGenerationMaxAgeFromConfigPositiveValueUsed(t *testing.T) {
	cfg := &config.Config{}
	cfg.AIGateway.AsyncGenerationMaxAge = 3600
	require.Equal(t, 3600*time.Second, asyncGenerationMaxAgeFromConfig(cfg))
}

func TestAsyncGenerationMaxAgeFromConfigZeroOrNegativeUsesDefault(t *testing.T) {
	for _, v := range []int{0, -1, -1000} {
		t.Run("value", func(t *testing.T) {
			cfg := &config.Config{}
			cfg.AIGateway.AsyncGenerationMaxAge = v
			require.Equal(t, defaultAsyncGenerationMaxAge, asyncGenerationMaxAgeFromConfig(cfg))
		})
	}
}

func TestIsTerminalStatusReturnsTrueForCompletedFailedCancelled(t *testing.T) {
	for _, s := range []string{
		"completed",
		"failed",
		"cancelled",
	} {
		t.Run(s, func(t *testing.T) {
			require.True(t, isTerminalStatus(s))
		})
	}
}

func TestIsTerminalStatusCaseAndWhitespaceInsensitive(t *testing.T) {
	cases := []string{
		"COMPLETED",
		"Completed",
		"  failed  ",
		"\tcancelled\n",
		"FaIlEd",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			require.True(t, isTerminalStatus(s))
		})
	}
}

func TestIsTerminalStatusReturnsFalseForNonTerminal(t *testing.T) {
	for _, s := range []string{
		"queued",
		"in_progress",
		"in-progress",
		"running",
		"pending",
		"",
		"unknown",
	} {
		t.Run(s, func(t *testing.T) {
			require.False(t, isTerminalStatus(s))
		})
	}
}

func TestIsCompletedStatusOnlyTrueForCompleted(t *testing.T) {
	for _, s := range []string{
		"completed",
		"COMPLETED",
		"  Completed  ",
	} {
		t.Run(s, func(t *testing.T) {
			require.True(t, isCompletedStatus(s))
		})
	}
}

func TestIsCompletedStatusFalseForOtherTerminals(t *testing.T) {
	// Failed and cancelled are terminal but NOT completed.
	for _, s := range []string{
		"failed",
		"cancelled",
		"queued",
		"in_progress",
		"",
		"completed_extra",
	} {
		t.Run(s, func(t *testing.T) {
			require.False(t, isCompletedStatus(s))
		})
	}
}

func TestNormalizeStatusLowercasesAndTrims(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"completed", "completed"},
		{"COMPLETED", "completed"},
		{"Completed", "completed"},
		{"  in_progress  ", "in_progress"},
		{"\tFailed\n", "failed"},
		{"CANCELLED", "cancelled"},
		{"  ", ""},
		{" In_Progress ", "in_progress"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			require.Equal(t, tc.want, normalizeStatus(tc.in))
		})
	}
}

func TestIsTerminalStatusMatchesCommontypesConstants(t *testing.T) {
	// Sanity check: the implementation must agree with the public status
	// constants in common/types, otherwise orchestrator and presentation
	// layers could disagree about which statuses are terminal.
	require.True(t, isTerminalStatus(string(commontypes.AIGatewayAsyncGenerationStatusCompleted)))
	require.True(t, isTerminalStatus(string(commontypes.AIGatewayAsyncGenerationStatusFailed)))
	require.True(t, isTerminalStatus(string(commontypes.AIGatewayAsyncGenerationStatusCancelled)))
	require.False(t, isTerminalStatus(string(commontypes.AIGatewayAsyncGenerationStatusQueued)))
	require.False(t, isTerminalStatus(string(commontypes.AIGatewayAsyncGenerationStatusInProgress)))
}
