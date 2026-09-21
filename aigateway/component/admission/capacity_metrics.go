package admission

import (
	"strconv"

	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
)

// capacityStateDimensions are the Redis accounting dimensions reported per
// upstream.
var capacityStateDimensions = []struct {
	dimension string
	current   func(s types.UpstreamCapacityState) int64
}{
	{"concurrency", func(s types.UpstreamCapacityState) int64 { return s.CurrentConcurrency }},
	{"rpm", func(s types.UpstreamCapacityState) int64 { return s.CurrentRPM }},
	{"tpm", func(s types.UpstreamCapacityState) int64 { return s.CurrentTPM }},
}

// ReportCapacityStates publishes the capacity snapshot to Prometheus.
//
// Semantics (must match types.UpstreamCapacityState): values are the
// Redis-side counters as of the observation; the selected upstream of an
// admitted request reports post-acquire values, all others pre-acquire.
// Gauges are observational — freshness follows the check/observation rate
// of the reporting replica.
//
// The reporting helpers tolerate an uninitialized prometheus registry
// (InitMetrics runs at service startup).
func ReportCapacityStates(modelID string, states []types.UpstreamCapacityState) {
	if prom.AIGatewayUpstreamCapacityCurrent == nil {
		return
	}
	for _, s := range states {
		upstreamID := formatUpstreamID(s.UpstreamID)
		for _, dim := range capacityStateDimensions {
			prom.AIGatewayUpstreamCapacityCurrent.
				WithLabelValues(modelID, upstreamID, dim.dimension).
				Set(float64(dim.current(s)))
		}
	}
}

// ReportCapacityBlocked counts blocked dimensions per upstream. A nonzero
// rate on this counter is the primary saturation signal for capacity
// planning.
func ReportCapacityBlocked(modelID string, states []types.UpstreamCapacityState) {
	if prom.AIGatewayUpstreamCapacityBlockedTotal == nil {
		return
	}
	for _, s := range states {
		if s.Feasible {
			continue
		}
		upstreamID := formatUpstreamID(s.UpstreamID)
		for _, dim := range s.BlockedBy {
			prom.AIGatewayUpstreamCapacityBlockedTotal.
				WithLabelValues(modelID, upstreamID, dim).
				Inc()
		}
	}
}

func formatUpstreamID(id int64) string {
	return strconv.FormatInt(id, 10)
}
