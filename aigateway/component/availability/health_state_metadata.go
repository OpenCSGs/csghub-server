package availability

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/store/database"
)

const multimodalHealthMetadataKey = "multimodal"

var errInvalidMultimodalHealthState = errors.New("invalid multimodal health state")

type probeHealthState struct {
	ConsecutiveFailures    int       `json:"consecutive_failures"`
	LastCheckedAt          time.Time `json:"last_checked_at,omitempty"`
	LastSucceededAt        time.Time `json:"last_succeeded_at,omitempty"`
	LastError              string    `json:"last_error,omitempty"`
	LatencyMs              int64     `json:"latency_ms"`
	LastSucceededLatencyMs int64     `json:"last_succeeded_latency_ms"`
}

type multimodalHealthState struct {
	L7        probeHealthState `json:"l7"`
	Inference probeHealthState `json:"inference"`
}

type multimodalProbeEvent struct {
	Kind      types.HealthCheckType
	Healthy   bool
	CheckedAt time.Time
	LatencyMs int64
	Error     string
}

type multimodalHealthPolicy struct {
	failureThreshold          int
	l7LatencyThreshold        time.Duration
	inferenceLatencyThreshold time.Duration
}

type multimodalHealthProjection struct {
	healthState         types.HealthState
	lastCheckAt         time.Time
	lastError           string
	consecutiveFailures int
	latencyMs           int64
}

func readMultimodalHealthState(metadata map[string]any) (multimodalHealthState, bool, error) {
	if metadata == nil {
		return multimodalHealthState{}, false, nil
	}
	value, ok := metadata[multimodalHealthMetadataKey]
	if !ok {
		return multimodalHealthState{}, false, nil
	}
	if value == nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: null metadata", errInvalidMultimodalHealthState)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: marshal metadata: %v", errInvalidMultimodalHealthState, err)
	}
	var persisted struct {
		L7        *probeHealthState `json:"l7"`
		Inference *probeHealthState `json:"inference"`
	}
	if err := json.Unmarshal(payload, &persisted); err != nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: unmarshal metadata: %v", errInvalidMultimodalHealthState, err)
	}
	if persisted.L7 == nil || persisted.Inference == nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: missing probe dimension", errInvalidMultimodalHealthState)
	}
	result := multimodalHealthState{L7: *persisted.L7, Inference: *persisted.Inference}
	if err := validateProbeHealthState(result.L7); err != nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: L7: %v", errInvalidMultimodalHealthState, err)
	}
	if err := validateProbeHealthState(result.Inference); err != nil {
		return multimodalHealthState{}, true, fmt.Errorf("%w: inference: %v", errInvalidMultimodalHealthState, err)
	}
	return result, true, nil
}

func validateProbeHealthState(state probeHealthState) error {
	if state.ConsecutiveFailures < 0 {
		return errors.New("negative consecutive failures")
	}
	if state.LatencyMs < 0 || state.LastSucceededLatencyMs < 0 {
		return errors.New("negative latency")
	}
	return nil
}

func writeMultimodalHealthState(state *database.AIGatewayUpstreamHealthState, health multimodalHealthState) {
	if state.Metadata == nil {
		state.Metadata = make(map[string]any)
	}
	state.Metadata[multimodalHealthMetadataKey] = health
}

func reduceMultimodalHealth(
	previous multimodalHealthState,
	event multimodalProbeEvent,
) (multimodalHealthState, bool) {
	next := previous
	var target *probeHealthState
	switch event.Kind {
	case types.HealthCheckTypeL7API:
		target = &next.L7
	case types.HealthCheckTypeInference:
		target = &next.Inference
	default:
		return previous, false
	}
	if !target.LastCheckedAt.IsZero() && !event.CheckedAt.After(target.LastCheckedAt) {
		return previous, false
	}

	target.LastCheckedAt = event.CheckedAt
	target.LatencyMs = event.LatencyMs
	if event.Healthy {
		target.ConsecutiveFailures = 0
		target.LastSucceededAt = event.CheckedAt
		target.LastSucceededLatencyMs = event.LatencyMs
		target.LastError = ""
	} else {
		target.ConsecutiveFailures++
		target.LastError = event.Error
	}
	return next, true
}

func multimodalLastCheckedAt(health multimodalHealthState, kind types.HealthCheckType) time.Time {
	switch kind {
	case types.HealthCheckTypeL7API:
		return health.L7.LastCheckedAt
	case types.HealthCheckTypeInference:
		return health.Inference.LastCheckedAt
	default:
		return time.Time{}
	}
}

func projectMultimodalHealth(
	health multimodalHealthState,
	policy multimodalHealthPolicy,
) multimodalHealthProjection {
	projection := multimodalHealthProjection{
		healthState:         deriveMultimodalHealthState(health, policy),
		consecutiveFailures: max(health.L7.ConsecutiveFailures, health.Inference.ConsecutiveFailures),
	}

	latest := health.L7
	if health.Inference.LastCheckedAt.After(latest.LastCheckedAt) {
		latest = health.Inference
	}
	projection.lastCheckAt = latest.LastCheckedAt
	projection.latencyMs = latest.LatencyMs

	latestFailure, found := latestActiveFailure(health)
	if found {
		projection.lastError = latestFailure.LastError
	}
	return projection
}

func deriveMultimodalHealthState(health multimodalHealthState, policy multimodalHealthPolicy) types.HealthState {
	threshold := policy.failureThreshold
	if threshold <= 0 {
		threshold = 3
	}
	if health.L7.ConsecutiveFailures >= threshold || health.Inference.ConsecutiveFailures >= threshold {
		return types.HealthStateUnhealthy
	}
	if health.Inference.LastCheckedAt.IsZero() {
		return types.HealthStateUnknown
	}
	if successfulProbeIsSlow(health.L7, policy.l7LatencyThreshold) ||
		successfulProbeIsSlow(health.Inference, policy.inferenceLatencyThreshold) {
		return types.HealthStateDegraded
	}
	if !health.L7.LastCheckedAt.IsZero() || !health.Inference.LastCheckedAt.IsZero() {
		return types.HealthStateHealthy
	}
	return types.HealthStateUnknown
}

func successfulProbeIsSlow(state probeHealthState, threshold time.Duration) bool {
	return threshold > 0 && !state.LastSucceededAt.IsZero() &&
		state.LastSucceededLatencyMs > threshold.Milliseconds()
}

func latestActiveFailure(health multimodalHealthState) (probeHealthState, bool) {
	var latest probeHealthState
	found := false
	for _, state := range []probeHealthState{health.L7, health.Inference} {
		if state.ConsecutiveFailures > 0 && (!found || state.LastCheckedAt.After(latest.LastCheckedAt)) {
			latest = state
			found = true
		}
	}
	return latest, found
}
