package types

import "time"

type RepositoryLite struct {
	ID          int64     `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Nickname    string    `json:"nickname"`
	Description string    `json:"description"`
	Tags        []RepoTag `json:"tags,omitempty"`
}

// UpstreamConfig describes one upstream endpoint for a logical LLM model.
// Weight is reserved for future weighted routing and defaults to 1 when omitted.
type UpstreamConfig struct {
	ID                    int64  `json:"id,omitempty"`
	URL                   string `json:"url"`
	Weight                int    `json:"weight,omitempty"`
	Enabled               bool   `json:"enabled"`
	HealthCheckEnabled    bool   `json:"health_check_enabled"`
	CircuitBreakerEnabled bool   `json:"circuit_breaker_enabled"`
	// HealthState is populated for admin views from the health state table.
	HealthState string `json:"health_state,omitempty"`
	// CircuitState is populated for admin views from the circuit state table.
	CircuitState string `json:"circuit_state,omitempty"`
	// IsAvailable is the computed overall availability for this upstream.
	IsAvailable bool `json:"is_available"`
	// AvailabilityStatus is a readable aggregate status: available/degraded/unavailable/disabled.
	AvailabilityStatus string `json:"availability_status,omitempty"`
	// ModelName overrides the upstream request model ID when this upstream uses
	ModelName string `json:"model_name"`
	// AuthHeader is endpoint-specific auth header value.
	// It supports either a plain "Bearer xxx" string or JSON object string like {"Authorization":"Bearer xxx"}.
	AuthHeader string `json:"auth_header"`
	// Provider identifies upstream provider for this specific endpoint.
	Provider string `json:"provider"`
	// LimitPolicy controls usage-based quota for this specific endpoint.
	LimitPolicy *UsageLimitPolicy `json:"limit_policy,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Metadata    map[string]any    `json:"metadata,omitempty"`
}

// RoutingPolicy controls how a request selects one upstream from Upstreams.
type RoutingPolicy struct {
	Strategy      string `json:"strategy"`
	SessionHeader string `json:"session_header,omitempty"`
	HashReplicas  int    `json:"hash_replicas,omitempty"`
}

// UsageLimitPolicy controls usage-based quota within one fixed time window.
// WindowSeconds should be greater than 0, typical values are 60 or 3600.
type UsageLimitPolicy struct {
	Enabled              bool    `json:"enabled"`
	WindowSeconds        int64   `json:"window_seconds,omitempty"`
	MaxTotalTokens       int64   `json:"max_total_tokens,omitempty"`
	MaxPromptTokens      int64   `json:"max_prompt_tokens,omitempty"`
	MaxCompletionTokens  int64   `json:"max_completion_tokens,omitempty"`
	CachedTokenCostRatio float64 `json:"cached_token_cost_ratio,omitempty"`
	CacheCreateCostRatio float64 `json:"cache_create_cost_ratio,omitempty"`
}

type LLMConfig struct {
	ID                 int64            `json:"id"`
	ModelName          string           `json:"model_name"`
	OfficialName       string           `json:"-"`    // deprecated: derived from upstream
	ApiEndpoint        string           `json:"-"`    // deprecated: derived from upstream
	AuthHeader         string           `json:"-"`    // deprecated: moved to upstream
	Type               int              `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled            bool             `json:"enabled"`
	Provider           string           `json:"-"` // deprecated: moved to upstream
	Upstreams          []UpstreamConfig `json:"upstreams"`
	RoutingPolicy      RoutingPolicy    `json:"routing_policy"`
	Metadata           map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID             int64            `json:"repo_id"`
	Repo               *RepositoryLite  `json:"repo"`
	NeedSensitiveCheck bool             `json:"need_sensitive_check"`
	ModelSizeB         float64          `json:"model_size_b,omitempty"`
	IsAvailable        bool             `json:"is_available"`
	AvailabilityReason string           `json:"availability_reason,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type PromptPrefix struct {
	ID   int64  `json:"id"`
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}

type SearchLLMConfig struct {
	Keyword   string `json:"keyword"`    // Search keyword
	Type      *int   `json:"type"`       // Type of search
	Enabled   *bool  `json:"enabled"`    // Enabled filter
	SortBy    string `json:"sort_by"`    // Sortable field: model_size_b | updated_at
	SortOrder string `json:"sort_order"` // ASC | DESC
}

type SearchPromptPrefix struct {
	Keyword string `json:"keyword"` // Search keyword
	Kind    string `json:"kind"`    // Kind of prompt prefix
}

type UpdateLLMConfigReq struct {
	ID                 int64             `json:"id"`
	ModelName          *string           `json:"model_name"`
	Upstreams          *[]UpstreamConfig `json:"upstreams"`
	Type               *int              `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled            *bool             `json:"enabled"`
	RoutingPolicy      *RoutingPolicy    `json:"routing_policy"`
	Metadata           *map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	NeedSensitiveCheck *bool             `json:"need_sensitive_check"`
	RepoID             *int64            `json:"repo_id"`
	ModelSizeB         *float64          `json:"model_size_b"`
}

type UpdatePromptPrefixReq struct {
	ID   int64   `json:"id"`
	ZH   *string `json:"zh"`
	EN   *string `json:"en"`
	Kind *string `json:"kind"`
}

type CreateLLMConfigReq struct {
	ModelName          string           `json:"model_name" binding:"required"`
	Upstreams          []UpstreamConfig `json:"upstreams,omitempty"`
	Type               int              `json:"type" binding:"required,oneof=1 2 4 8 16"` // 1: optimization, 2: comparison, 4: summary readme, 8: mcp scan, 16: for aigateway call external llm
	Enabled            bool             `json:"enabled"`
	RoutingPolicy      RoutingPolicy    `json:"routing_policy"`
	Metadata           map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID             *int64           `json:"repo_id"`
	NeedSensitiveCheck bool             `json:"need_sensitive_check"`
	ModelSizeB         float64          `json:"model_size_b,omitempty"`
}

type CreatePromptPrefixReq struct {
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}

// CreateUpstreamReq is the request to add a new upstream to an existing LLM config.
type CreateUpstreamReq struct {
	LLMConfigID           int64             `json:"llm_config_id" binding:"required"`
	URL                   string            `json:"url" binding:"required"`
	Weight                int               `json:"weight,omitempty"`
	Enabled               bool              `json:"enabled"`
	ModelName             string            `json:"model_name,omitempty"`
	AuthHeader            string            `json:"auth_header,omitempty"`
	Provider              string            `json:"provider,omitempty"`
	HealthCheckEnabled    bool              `json:"health_check_enabled"`
	CircuitBreakerEnabled bool              `json:"circuit_breaker_enabled"`
	LimitPolicy           *UsageLimitPolicy `json:"limit_policy,omitempty"`
	Tags                  map[string]string `json:"tags,omitempty"`
	Metadata              map[string]any    `json:"metadata,omitempty"`
}

// UpdateUpstreamReq is the request to update an existing upstream.
// Only non-nil fields will be updated.
type UpdateUpstreamReq struct {
	ID                    int64              `json:"id"`
	URL                   *string            `json:"url"`
	Weight                *int               `json:"weight"`
	Enabled               *bool              `json:"enabled"`
	ModelName             *string            `json:"model_name"`
	AuthHeader            *string            `json:"auth_header"`
	Provider              *string            `json:"provider"`
	HealthCheckEnabled    *bool              `json:"health_check_enabled"`
	CircuitBreakerEnabled *bool              `json:"circuit_breaker_enabled"`
	LimitPolicy           **UsageLimitPolicy `json:"limit_policy"`
	Tags                  *map[string]string `json:"tags"`
	Metadata              *map[string]any    `json:"metadata"`
}
