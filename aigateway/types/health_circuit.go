package types

import (
	"time"

	commontypes "opencsg.com/csghub-server/common/types"
)

// HealthState represents the health state of a provider/model/endpoint
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateDegraded  HealthState = "degraded"
	HealthStateUnhealthy HealthState = "unhealthy"
)

// CircuitState represents the circuit breaker state
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

// HealthCheckType defines the type of health check
type HealthCheckType string

const (
	HealthCheckTypeL7API    HealthCheckType = "l7_api"    // /v1/models API check
	HealthCheckTypeInference HealthCheckType = "inference" // Light inference check
)

// ProviderHealthStatus represents the health status of a provider endpoint
type ProviderHealthStatus struct {
	UpstreamID          int64        `json:"upstream_id"`
	Provider            string       `json:"provider,omitempty"`
	ModelName           string       `json:"model_name,omitempty"`
	Endpoint            string       `json:"endpoint,omitempty"`
	HealthState         HealthState  `json:"health_state"`
	LastCheckAt         time.Time    `json:"last_check_at"`
	LastError           string       `json:"last_error,omitempty"`
	ConsecutiveFailures int          `json:"consecutive_failures"`
	LatencyMs           int64        `json:"latency_ms"`
}

// ProviderCircuitStatus represents the circuit breaker status of a provider endpoint
type ProviderCircuitStatus struct {
	UpstreamID      int64        `json:"upstream_id"`
	Provider        string       `json:"provider,omitempty"`
	ModelName       string       `json:"model_name,omitempty"`
	Endpoint        string       `json:"endpoint,omitempty"`
	CircuitState    CircuitState `json:"circuit_state"`
	FailureCount    int           `json:"failure_count"`
	SuccessCount    int           `json:"success_count"`
	LastStateChange time.Time     `json:"last_state_change"`
	NextRetryAt     *time.Time    `json:"next_retry_at,omitempty"`
}

// HealthCheckConfig defines the configuration for health checking
type HealthCheckConfig struct {
	// Enable health checking
	Enabled bool `json:"enabled"`
	
	// L7 API check configuration
	L7APICheck L7APICheckConfig `json:"l7_api_check"`
	
	// Inference check configuration
	InferenceCheck InferenceCheckConfig `json:"inference_check"`
	
	// Health determination rules
	HealthRules HealthRulesConfig `json:"health_rules"`
}

// L7APICheckConfig defines L7 API health check configuration
type L7APICheckConfig struct {
	Enabled  bool          `json:"enabled"`
	Interval time.Duration `json:"interval"`  // Check interval, default 5-10s
	Timeout  time.Duration `json:"timeout"`   // Request timeout
}

// InferenceCheckConfig defines inference health check configuration
type InferenceCheckConfig struct {
	Enabled    bool          `json:"enabled"`
	Interval   time.Duration `json:"interval"`   // Check interval, default 30-60s
	Timeout    time.Duration `json:"timeout"`    // Request timeout
	MaxTokens  int           `json:"max_tokens"` // Max tokens for inference check, default 1
	Prompt     string        `json:"prompt"`     // Prompt for inference check
}

// HealthRulesConfig defines rules for determining health state
type HealthRulesConfig struct {
	ConsecutiveFailuresForUnhealthy int           `json:"consecutive_failures_for_unhealthy"` // Default 3
	LatencyThresholdForDegraded     time.Duration `json:"latency_threshold_for_degraded"`     // Latency threshold for degraded state
}

// CircuitBreakerConfig defines the configuration for circuit breaker
type CircuitBreakerConfig struct {
	// Enable circuit breaker
	Enabled bool `json:"enabled"`
	
	// Failure threshold to trip the circuit
	FailureThreshold int `json:"failure_threshold"` // Default 3
	
	// Error rate threshold (0.0 - 1.0)
	ErrorRateThreshold float64 `json:"error_rate_threshold"` // Default 0.5 (50%)
	
	// Sliding window size for error rate calculation
	SlidingWindowSize int `json:"sliding_window_size"` // Default 10
	
	// Duration to keep circuit open before trying half-open
	OpenDuration time.Duration `json:"open_duration"` // Default 30s
	
	// Number of requests to allow in half-open state
	HalfOpenMaxRequests int `json:"half_open_max_requests"` // Default 1
}

// ModelAvailabilityStatus represents the combined availability status for routing
type ModelAvailabilityStatus struct {
	ModelID                string                 `json:"model_id"`
	Provider               string                 `json:"provider"`
	Endpoint               string                 `json:"endpoint"`
	IsAvailable            bool                   `json:"is_available"`
	HealthState            HealthState            `json:"health_state"`
	CircuitState           CircuitState           `json:"circuit_state"`
	Reason                 string                 `json:"reason,omitempty"` // Reason if not available
	UpstreamAvailabilities []UpstreamAvailability `json:"upstream_availabilities,omitempty"`
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	UpstreamID int64           `json:"upstream_id"`
	Provider   string          `json:"provider,omitempty"`
	ModelName  string          `json:"model_name,omitempty"`
	Endpoint   string          `json:"endpoint,omitempty"`
	CheckType  HealthCheckType `json:"check_type"`
	Healthy    bool            `json:"healthy"`
	LatencyMs  int64           `json:"latency_ms"`
	Error      string          `json:"error,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// CircuitBreakerEvent represents an event in circuit breaker state machine
type CircuitBreakerEvent struct {
	UpstreamID   int64        `json:"upstream_id"`
	Provider     string       `json:"provider,omitempty"`
	ModelName    string       `json:"model_name,omitempty"`
	Endpoint     string       `json:"endpoint,omitempty"`
	OldState     CircuitState `json:"old_state"`
	NewState     CircuitState `json:"new_state"`
	Reason       string       `json:"reason"`
	Timestamp    time.Time    `json:"timestamp"`
	FailureCount int          `json:"failure_count"`
}
// IsUpstreamUnhealthy checks the inline health state carried on UpstreamConfig
// (populated from DB at model-fetch time). Returns true when the upstream
// should be excluded from routing due to unhealthy health state.
func IsUpstreamUnhealthy(u commontypes.UpstreamConfig) bool {
	return u.HealthCheckEnabled && u.HealthState == string(HealthStateUnhealthy)
}

// IsUpstreamCircuitOpen checks the inline circuit state carried on UpstreamConfig
// (populated from DB at model-fetch time). Returns true when the circuit
// breaker is open and the upstream should be excluded from routing.
func IsUpstreamCircuitOpen(u commontypes.UpstreamConfig) bool {
	return u.CircuitBreakerEnabled && u.CircuitState == string(CircuitStateOpen)
}

// IsUpstreamUnavailable checks the inline health/circuit state carried on
// UpstreamConfig (populated from DB at model-fetch time). Returns true and
// a reason when the upstream should be excluded from routing.
func IsUpstreamUnavailable(u commontypes.UpstreamConfig) (bool, string) {
	if IsUpstreamCircuitOpen(u) {
		return true, "circuit breaker is open"
	}
	if IsUpstreamUnhealthy(u) {
		return true, "health state is unhealthy"
	}
	return false, ""
}
