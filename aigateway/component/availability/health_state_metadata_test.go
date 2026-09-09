package availability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestReduceMultimodalHealthUpdatesOnlyEventDimension(t *testing.T) {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	initial := multimodalHealthState{
		Inference: probeHealthState{
			ConsecutiveFailures: 1,
			LastCheckedAt:       start,
			LastError:           "inference failed",
		},
	}

	next, applied := reduceMultimodalHealth(initial, multimodalProbeEvent{
		Kind:      types.HealthCheckTypeL7API,
		Healthy:   false,
		CheckedAt: start.Add(time.Minute),
		LatencyMs: 100,
		Error:     "models failed",
	})
	require.True(t, applied)
	require.Equal(t, initial.Inference, next.Inference)
	require.Equal(t, 1, next.L7.ConsecutiveFailures)
	require.Equal(t, "models failed", next.L7.LastError)

	next, applied = reduceMultimodalHealth(next, multimodalProbeEvent{
		Kind:      types.HealthCheckTypeL7API,
		Healthy:   true,
		CheckedAt: start.Add(2 * time.Minute),
		LatencyMs: 80,
	})
	require.True(t, applied)
	require.Zero(t, next.L7.ConsecutiveFailures)
	require.Empty(t, next.L7.LastError)
	require.Equal(t, start.Add(2*time.Minute), next.L7.LastSucceededAt)
	require.Equal(t, initial.Inference, next.Inference)
}

func TestReduceMultimodalHealthMixedFailureSequence(t *testing.T) {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	health := multimodalHealthState{}
	events := []multimodalProbeEvent{
		{Kind: types.HealthCheckTypeInference, CheckedAt: start, Error: "inference 1"},
		{Kind: types.HealthCheckTypeL7API, CheckedAt: start.Add(time.Minute), Error: "l7"},
		{Kind: types.HealthCheckTypeL7API, Healthy: true, CheckedAt: start.Add(2 * time.Minute), LatencyMs: 10},
		{Kind: types.HealthCheckTypeInference, CheckedAt: start.Add(3 * time.Minute), Error: "inference 2"},
	}
	for _, event := range events {
		var applied bool
		health, applied = reduceMultimodalHealth(health, event)
		require.True(t, applied)
	}

	projection := projectMultimodalHealth(health, multimodalHealthPolicy{failureThreshold: 3})
	require.Zero(t, health.L7.ConsecutiveFailures)
	require.Equal(t, 2, health.Inference.ConsecutiveFailures)
	require.Equal(t, 2, projection.consecutiveFailures)
	require.Equal(t, types.HealthStateHealthy, projection.healthState)
	require.Equal(t, "inference 2", projection.lastError)
}

func TestReduceMultimodalHealthRejectsStaleEventsPerDimension(t *testing.T) {
	checkedAt := time.Date(2026, 8, 24, 10, 2, 0, 0, time.UTC)
	initial := multimodalHealthState{L7: probeHealthState{
		LastCheckedAt:       checkedAt,
		LastSucceededAt:     checkedAt,
		ConsecutiveFailures: 0,
		LatencyMs:           50,
	}}

	for _, staleCheckedAt := range []time.Time{checkedAt.Add(-time.Minute), checkedAt} {
		next, applied := reduceMultimodalHealth(initial, multimodalProbeEvent{
			Kind: types.HealthCheckTypeL7API, CheckedAt: staleCheckedAt, Error: "stale failure",
		})
		require.False(t, applied)
		require.Equal(t, initial, next)
	}

	next, applied := reduceMultimodalHealth(initial, multimodalProbeEvent{
		Kind: types.HealthCheckTypeInference, CheckedAt: checkedAt.Add(-time.Minute), Healthy: true,
	})
	require.True(t, applied, "an older timestamp in another dimension remains independent")
	require.Equal(t, initial.L7, next.L7)
	require.Equal(t, checkedAt.Add(-time.Minute), next.Inference.LastCheckedAt)
}

func TestProjectMultimodalHealthState(t *testing.T) {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	policy := multimodalHealthPolicy{
		failureThreshold:          3,
		l7LatencyThreshold:        time.Second,
		inferenceLatencyThreshold: 2 * time.Minute,
	}

	for _, test := range []struct {
		name     string
		health   multimodalHealthState
		expected types.HealthState
	}{
		{name: "unknown", expected: types.HealthStateUnknown},
		{name: "L7 success before first inference stays unknown", health: multimodalHealthState{L7: probeHealthState{LastCheckedAt: start}}, expected: types.HealthStateUnknown},
		{name: "healthy", health: multimodalHealthState{L7: probeHealthState{LastCheckedAt: start}, Inference: probeHealthState{LastCheckedAt: start}}, expected: types.HealthStateHealthy},
		{name: "L7 unhealthy", health: multimodalHealthState{L7: probeHealthState{ConsecutiveFailures: 3, LastCheckedAt: start}}, expected: types.HealthStateUnhealthy},
		{name: "inference unhealthy", health: multimodalHealthState{Inference: probeHealthState{ConsecutiveFailures: 3, LastCheckedAt: start}}, expected: types.HealthStateUnhealthy},
		{name: "slow L7", health: multimodalHealthState{L7: probeHealthState{LastCheckedAt: start, LastSucceededAt: start, LastSucceededLatencyMs: 1001}, Inference: probeHealthState{LastCheckedAt: start}}, expected: types.HealthStateDegraded},
		{name: "slow inference", health: multimodalHealthState{Inference: probeHealthState{LastCheckedAt: start, LastSucceededAt: start, LastSucceededLatencyMs: 120001}}, expected: types.HealthStateDegraded},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, projectMultimodalHealth(test.health, policy).healthState)
		})
	}
}

func TestProjectMultimodalHealthCompatibilityFields(t *testing.T) {
	start := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	health := multimodalHealthState{
		L7: probeHealthState{
			ConsecutiveFailures: 2,
			LastCheckedAt:       start,
			LastError:           "l7 failed",
			LatencyMs:           50,
		},
		Inference: probeHealthState{
			ConsecutiveFailures: 1,
			LastCheckedAt:       start.Add(time.Minute),
			LastError:           "inference failed",
			LatencyMs:           300,
		},
	}

	projection := projectMultimodalHealth(health, multimodalHealthPolicy{failureThreshold: 3})
	require.Equal(t, 2, projection.consecutiveFailures)
	require.Equal(t, start.Add(time.Minute), projection.lastCheckAt)
	require.Equal(t, "inference failed", projection.lastError)
	require.Equal(t, int64(300), projection.latencyMs)
}

func TestMultimodalHealthStateMetadataRoundTrip(t *testing.T) {
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	state := &database.AIGatewayUpstreamHealthState{Metadata: map[string]any{"existing": "value"}}
	health := multimodalHealthState{Inference: probeHealthState{
		ConsecutiveFailures: 1,
		LastCheckedAt:       now,
		LastError:           "failed",
	}}
	writeMultimodalHealthState(state, health)

	persisted, found, err := readMultimodalHealthState(map[string]any{
		"existing": "value",
		multimodalHealthMetadataKey: map[string]any{
			"l7": map[string]any{},
			"inference": map[string]any{
				"consecutive_failures": float64(1),
				"last_checked_at":      now.Format(time.RFC3339Nano),
				"last_error":           "failed",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, health, persisted)
	require.Equal(t, "value", state.Metadata["existing"])
}

func TestReadMultimodalHealthStateRejectsMalformedState(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
	}{
		{name: "null", value: nil},
		{name: "wrong type", value: "invalid"},
		{name: "missing L7", value: map[string]any{"inference": map[string]any{}}},
		{name: "missing inference", value: map[string]any{"l7": map[string]any{}}},
		{name: "negative failures", value: map[string]any{"l7": map[string]any{"consecutive_failures": -1}, "inference": map[string]any{}}},
		{name: "negative latency", value: map[string]any{"l7": map[string]any{"latency_ms": -1}, "inference": map[string]any{}}},
		{name: "negative successful latency", value: map[string]any{"l7": map[string]any{}, "inference": map[string]any{"last_succeeded_latency_ms": -1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, found, err := readMultimodalHealthState(map[string]any{multimodalHealthMetadataKey: test.value})
			require.True(t, found)
			require.ErrorIs(t, err, errInvalidMultimodalHealthState)
		})
	}
}

func TestReadMultimodalHealthStateAllowsMissingAndUnknownFields(t *testing.T) {
	_, found, err := readMultimodalHealthState(nil)
	require.NoError(t, err)
	require.False(t, found)

	health, found, err := readMultimodalHealthState(map[string]any{
		multimodalHealthMetadataKey: map[string]any{
			"l7":        map[string]any{"future_field": true},
			"inference": map[string]any{},
			"future":    "ignored",
		},
	})
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, multimodalHealthState{}, health)
}
