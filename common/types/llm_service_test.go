package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapacityPolicy_ApplyDefaults(t *testing.T) {
	defaults := CapacityPolicy{
		MaxConcurrency:   32,
		MaxQueueDepth:    16,
		MaxTPM:           10000000,
		MaxRPM:           100,
		QueueWaitSeconds: 60,
	}

	t.Run("nil policy is a no-op", func(t *testing.T) {
		var p *CapacityPolicy
		p.ApplyDefaults(defaults)
		require.Nil(t, p)
	})

	t.Run("disabled policy is left untouched", func(t *testing.T) {
		p := &CapacityPolicy{Enabled: false, MaxConcurrency: -1}
		p.ApplyDefaults(defaults)
		require.Equal(t, &CapacityPolicy{Enabled: false, MaxConcurrency: -1}, p)
	})

	t.Run("enabled policy with all limits unset is fully populated", func(t *testing.T) {
		p := &CapacityPolicy{Enabled: true}
		p.ApplyDefaults(defaults)
		require.Equal(t, &CapacityPolicy{
			Enabled:          true,
			MaxConcurrency:   32,
			MaxQueueDepth:    16,
			MaxTPM:           10000000,
			MaxRPM:           100,
			QueueWaitSeconds: 60,
		}, p)
	})

	t.Run("any limit set keeps the whole policy untouched", func(t *testing.T) {
		p := &CapacityPolicy{Enabled: true, MaxConcurrency: 8}
		p.ApplyDefaults(defaults)
		// Unset fields stay zero (= unlimited for MaxTPM, unset for others);
		// defaults must not leak into partially configured policies.
		require.Equal(t, &CapacityPolicy{Enabled: true, MaxConcurrency: 8}, p)
	})

	t.Run("explicit MaxTPM=0 unlimited survives when other limits are set", func(t *testing.T) {
		p := &CapacityPolicy{Enabled: true, MaxConcurrency: 4, MaxQueueDepth: 4, MaxRPM: 4, QueueWaitSeconds: 30}
		p.ApplyDefaults(defaults)
		require.Equal(t, &CapacityPolicy{
			Enabled:          true,
			MaxConcurrency:   4,
			MaxQueueDepth:    4,
			MaxTPM:           0,
			MaxRPM:           4,
			QueueWaitSeconds: 30,
		}, p)
	})

	t.Run("non-positive default values are ignored", func(t *testing.T) {
		p := &CapacityPolicy{Enabled: true}
		p.ApplyDefaults(CapacityPolicy{MaxConcurrency: 0, MaxQueueDepth: -1, MaxTPM: 0, MaxRPM: -1, QueueWaitSeconds: 0})
		require.Equal(t, &CapacityPolicy{Enabled: true}, p)
	})
}

func TestCapacityPolicy_AllLimitsUnset(t *testing.T) {
	require.True(t, (&CapacityPolicy{}).AllLimitsUnset())
	require.True(t, (&CapacityPolicy{Enabled: true}).AllLimitsUnset())
	require.False(t, (&CapacityPolicy{MaxConcurrency: 1}).AllLimitsUnset())
	require.False(t, (&CapacityPolicy{MaxTPM: -1}).AllLimitsUnset())
	require.False(t, (&CapacityPolicy{QueueWaitSeconds: -1}).AllLimitsUnset())
}

func TestUpstreamCostResourceID_RoundTrip(t *testing.T) {
	for _, id := range []int64{1, 42, 1001} {
		resourceID := UpstreamCostResourceID(id)
		gotID, err := ParseUpstreamCostResourceID(resourceID)
		require.NoError(t, err)
		require.Equal(t, id, gotID)
	}
}

func TestParseUpstreamCostResourceID_Invalid(t *testing.T) {
	tests := []string{
		"",
		"thirdparty://42",
		"upstream://",
		"upstream://abc",
		"upstream://1.5",
		"upstream://0",
		"upstream://-3",
		"upstream://deepseek-r1/42",
	}
	for _, resourceID := range tests {
		t.Run(resourceID, func(t *testing.T) {
			_, err := ParseUpstreamCostResourceID(resourceID)
			require.Error(t, err)
		})
	}
}
