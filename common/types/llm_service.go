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
	URL     string `json:"url"`
	Weight  int    `json:"weight,omitempty"`
	Enabled bool   `json:"enabled"`
	// ModelName overrides the upstream request model ID when this endpoint uses
	// a provider-specific model identifier. It falls back to the logical model ID when omitted.
	ModelName string `json:"model_name,omitempty"`
	// AuthHeader is endpoint-specific auth header value.
	// It supports either a plain "Bearer xxx" string or JSON object string like {"Authorization":"Bearer xxx"}.
	AuthHeader string `json:"auth_header,omitempty"`
	// Provider identifies upstream provider for this specific endpoint.
	Provider string `json:"provider,omitempty"`
	// LimitPolicy controls usage-based quota for this specific endpoint.
	LimitPolicy *UsageLimitPolicy `json:"limit_policy,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// RoutingPolicy controls how a request selects one upstream from Upstreams.
type RoutingPolicy struct {
	Strategy        string `json:"strategy"`
	SessionHeader   string `json:"session_header,omitempty"`
	HashReplicas    int    `json:"hash_replicas,omitempty"`
	RoutingGroup    string `json:"routing_group,omitempty"`
	FallbackToFirst bool   `json:"fallback_to_first,omitempty"`
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
	ID            int64            `json:"id"`
	ModelName     string           `json:"model_name"`
	OfficialName  string           `json:"official_name"`
	ApiEndpoint   string           `json:"api_endpoint"`
	AuthHeader    string           `json:"auth_header"`
	Type          int              `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled       bool             `json:"enabled"`
	Provider      string           `json:"provider"`
	Upstreams     []UpstreamConfig `json:"upstreams"`
	RoutingPolicy RoutingPolicy    `json:"routing_policy"`
	Metadata      map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID        int64            `json:"repo_id"`
	Repo          *RepositoryLite  `json:"repo"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type PromptPrefix struct {
	ID   int64  `json:"id"`
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}

type SearchLLMConfig struct {
	Keyword string `json:"keyword"` // Search keyword
	Type    *int   `json:"type"`    // Type of search
	Enabled *bool  `json:"enabled"` // Enabled filter
}

type SearchPromptPrefix struct {
	Keyword string `json:"keyword"` // Search keyword
	Kind    string `json:"kind"`    // Kind of prompt prefix
}

type UpdateLLMConfigReq struct {
	ID            int64             `json:"id"`
	ModelName     *string           `json:"model_name"`
	OfficialName  *string           `json:"official_name"`
	ApiEndpoint   *string           `json:"api_endpoint"`
	Upstreams     *[]UpstreamConfig `json:"upstreams"`
	AuthHeader    *string           `json:"auth_header"`
	Type          *int              `json:"type"` // 1: optimization, 2: comparison, 4: summary readme
	Enabled       *bool             `json:"enabled"`
	Provider      *string           `json:"provider"`
	RoutingPolicy *RoutingPolicy    `json:"routing_policy"`
	Metadata      *map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID        *int64            `json:"repo_id"`
}

type UpdatePromptPrefixReq struct {
	ID   int64   `json:"id"`
	ZH   *string `json:"zh"`
	EN   *string `json:"en"`
	Kind *string `json:"kind"`
}

type CreateLLMConfigReq struct {
	ModelName     string           `json:"model_name" binding:"required"`
	OfficialName  string           `json:"official_name"`
	ApiEndpoint   string           `json:"api_endpoint"`
	Upstreams     []UpstreamConfig `json:"upstreams,omitempty"`
	AuthHeader    string           `json:"auth_header"`
	Type          int              `json:"type" binding:"required,oneof=1 2 4 8 16"` // 1: optimization, 2: comparison, 4: summary readme, 8: mcp scan, 16: for aigateway call external llm
	Provider      string           `json:"provider" binding:"required"`
	Enabled       bool             `json:"enabled"`
	RoutingPolicy RoutingPolicy    `json:"routing_policy"`
	Metadata      map[string]any   `json:"metadata"` // tasks stored as: {"tasks": ["text-generation", "text-to-image"]}
	RepoID        *int64           `json:"repo_id"`
}

type CreatePromptPrefixReq struct {
	ZH   string `json:"zh"`
	EN   string `json:"en"`
	Kind string `json:"kind"`
}
