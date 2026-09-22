package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UpstreamSource identifies the origin of an upstream.
type UpstreamSource string

const (
	UpstreamSourceExternal     UpstreamSource = "external"
	UpstreamSourceCSGHubDeploy UpstreamSource = "csghub"
	UpstreamSourceMixed        UpstreamSource = "mixed"
)

// Resource ID format strings for external LLM (model ID) and CSGHub internal (path segment, repo path).
const (
	ExternalLLMResourceFmt = "thirdparty://%s"
	CSGHubResourceFmt      = "csghub://%s/%s"
	// UpstreamCostResourceFmt scopes an upstream procurement-cost SKU
	// (SKUUpstreamCost) to one upstream by its immutable ai_gateway_upstreams.id,
	// so an LLM model rename never orphans cost rows.
	UpstreamCostResourceFmt = "upstream://%d"
)

const upstreamCostResourcePrefix = "upstream://"

// UpstreamCostResourceID builds the account_price resource_id for an upstream
// cost SKU.
func UpstreamCostResourceID(upstreamID int64) string {
	return fmt.Sprintf(UpstreamCostResourceFmt, upstreamID)
}

// ParseUpstreamCostResourceID extracts the upstream id from a resource_id built
// by UpstreamCostResourceID.
func ParseUpstreamCostResourceID(resourceID string) (upstreamID int64, err error) {
	if !strings.HasPrefix(resourceID, upstreamCostResourcePrefix) {
		return 0, fmt.Errorf("invalid upstream cost resource id %q: missing %q prefix", resourceID, upstreamCostResourcePrefix)
	}
	upstreamID, err = strconv.ParseInt(strings.TrimPrefix(resourceID, upstreamCostResourcePrefix), 10, 64)
	if err != nil || upstreamID <= 0 {
		return 0, fmt.Errorf("invalid upstream cost resource id %q: expected 'upstream://{upstream_id}'", resourceID)
	}
	return upstreamID, nil
}

// Provider type values for Metadata[MetaKeyLLMType].
const (
	ProviderTypeServerless  = "serverless"
	ProviderTypeInference   = "inference"
	ProviderTypeExternalLLM = "external_llm"
)

// InternalModelInfo carries deploy-derived fields for an internal (csghub) upstream.
// It is stored as part of UpstreamMetadata in the upstream's metadata JSONB column.
type InternalModelInfo struct {
	CSGHubModelID    string `json:"csghub_model_id"`   // deploy.Repository.Path
	RepoName         string `json:"repo_name"`         // deploy.Repository.Name
	HFPath           string `json:"hf_path"`           // deploy.Repository.HFPath
	LegacyModelID    string `json:"legacy_model_id"`   // BuildModelID result (matches public model ID)
	OwnerUUID        string `json:"owner_uuid"`        // deploy.User.UUID
	OwnerUsername    string `json:"owner_username"`    // deploy.User.Username
	OwnerNamespace   string `json:"owner_namespace"`   // billing/listing namespace (username or org)
	OwnerType        string `json:"owner_type"`        // "user" or "organization"
	ClusterID        string `json:"cluster_id"`        // deploy.ClusterID
	SvcName          string `json:"svc_name"`          // deploy.SvcName
	SvcType          int    `json:"svc_type"`          // deploy.Type
	ImageID          string `json:"image_id"`          // deploy.ImageID
	RuntimeFramework string `json:"runtime_framework"` // deploy.RuntimeFramework
	EngineArgs       string `json:"engine_args"`       // deploy.EngineArgs
	Task             string `json:"task"`              // deploy.Task
	SourceDeployID   int64  `json:"source_deploy_id"`  // deploy.ID
	CreatedAt        int64  `json:"created_at"`        // deploy.CreatedAt.Unix()
	Host             string `json:"host,omitempty"`    // k8s Host header override (hostname from deploy endpoint)
	// 1-public, 2-private, 3-extension in future
	SecureLevel int `json:"secure_level"`
}

// ResponsesChatAdapter controls chat-completions adapter behavior for
// upstreams that need reasoning request transformation.
type ResponsesChatAdapter struct {
	ReasoningRequest *ReasoningRequestConfig `json:"reasoning_request,omitempty"`
}

// ReasoningRequestConfig configures reasoning effort field transformation.
type ReasoningRequestConfig struct {
	Enabled      bool           `json:"enabled"`
	EffortField  string         `json:"effort_field"`
	EnableExtra  map[string]any `json:"enable_extra,omitempty"`
	DisableExtra map[string]any `json:"disable_extra,omitempty"`
}

// UpstreamMetadata is the typed JSONB metadata for an upstream.
// For external upstreams, only ResponsesChatAdapter may be set (by UI config).
// For csghub deploy upstreams, InternalModelInfo (including Host) is written
// by the deploy sync; ResponsesChatAdapter may be set by UI config.
// HostOverride is retained for backward-compatible API output and legacy DB
// row migration — new rows store host inside InternalModelInfo.Host.
type UpstreamMetadata struct {
	InternalModelInfo    *InternalModelInfo    `json:"internal_model_info,omitempty"`
	HostOverride         string                `json:"host_override,omitempty"`
	ResponsesChatAdapter *ResponsesChatAdapter `json:"responses_chat_adapter,omitempty"`
	// Protocol is an explicit upstream protocol declaration (chat/responses/messages).
	// When set, it overrides URL-based protocol inference.
	Protocol string `json:"protocol,omitempty"`
}

// upstreamMetadataAlias is used for infinite-loop-safe default unmarshaling.
type upstreamMetadataAlias UpstreamMetadata

// oldNestedMetadata represents the legacy nested JSON format:
//
//	{"responses": {"chat_adapter": {"reasoning_request": {...}}}}
type oldNestedMetadata struct {
	Responses struct {
		ChatAdapter *ResponsesChatAdapter `json:"chat_adapter"`
	} `json:"responses"`
}

// UnmarshalJSON accepts both the flat format
// ({"responses_chat_adapter": {...}}) and the legacy nested format
// ({"responses": {"chat_adapter": {...}}}). This ensures backward
// compatibility with existing API clients and DB rows that still use
// the old nested format. The flat format is the canonical in-memory shape;
// note that rows persisted via MarshalJSON are stored in the nested format
// (see MarshalJSON), which this unmarshaler reads back transparently.
func (m *UpstreamMetadata) UnmarshalJSON(data []byte) error {
	// First, try the standard flat-format unmarshal. A failure here indicates
	// genuinely malformed JSON that should not be silently swallowed.
	var flat upstreamMetadataAlias
	if err := json.Unmarshal(data, &flat); err != nil {
		return fmt.Errorf("unmarshal upstream metadata: %w", err)
	}
	*m = UpstreamMetadata(flat)

	// Migrate legacy top-level HostOverride into InternalModelInfo.Host.
	// New rows store host inside InternalModelInfo; old rows may still have
	// it at the top level. Normalize so downstream readers only need to check
	// InternalModelInfo.Host.
	if m.HostOverride != "" && m.InternalModelInfo != nil && m.InternalModelInfo.Host == "" {
		m.InternalModelInfo.Host = m.HostOverride
	}

	// If ResponsesChatAdapter was populated, the new format was used.
	if flat.ResponsesChatAdapter != nil {
		return nil
	}

	// Fall back to the legacy nested format for responses.chat_adapter,
	// used by older API clients and historical DB rows.
	var old oldNestedMetadata
	if err := json.Unmarshal(data, &old); err != nil {
		return fmt.Errorf("unmarshal upstream metadata (legacy nested): %w", err)
	}
	if old.Responses.ChatAdapter != nil && m.ResponsesChatAdapter == nil {
		m.ResponsesChatAdapter = old.Responses.ChatAdapter
	}
	return nil
}

// nestedMetadata is the legacy nested JSON format used for API output:
//
//	{"responses": {"chat_adapter": {"reasoning_request": {...}}}}
//
// This keeps API responses backward-compatible with UI clients that expect
// the nested shape, even though the struct field is named ResponsesChatAdapter.
type nestedMetadata struct {
	InternalModelInfo *InternalModelInfo `json:"internal_model_info,omitempty"`
	HostOverride      string             `json:"host_override,omitempty"`
	Protocol          string             `json:"protocol,omitempty"`
	Responses         *nestedResponses   `json:"responses,omitempty"`
}

type nestedResponses struct {
	ChatAdapter *ResponsesChatAdapter `json:"chat_adapter,omitempty"`
}

// MarshalJSON serializes UpstreamMetadata in the legacy nested format
// so that API responses remain backward-compatible with UI clients.
// HostOverride is populated from InternalModelInfo.Host for backward-
// compatible API output.
func (m UpstreamMetadata) MarshalJSON() ([]byte, error) {
	hostOverride := ""
	if m.InternalModelInfo != nil {
		hostOverride = m.InternalModelInfo.Host
	}
	out := nestedMetadata{
		InternalModelInfo: m.InternalModelInfo,
		HostOverride:      hostOverride,
		Protocol:          m.Protocol,
	}
	if m.ResponsesChatAdapter != nil {
		out.Responses = &nestedResponses{ChatAdapter: m.ResponsesChatAdapter}
	}
	return json.Marshal(out)
}

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
	ID                    int64          `json:"id,omitempty"`
	Source                UpstreamSource `json:"source,omitempty"`
	SourceID              int64          `json:"source_id,omitempty"`
	URL                   string         `json:"url"`
	Weight                int            `json:"weight,omitempty"`
	Enabled               bool           `json:"enabled"`
	HealthCheckEnabled    bool           `json:"health_check_enabled"`
	CircuitBreakerEnabled bool           `json:"circuit_breaker_enabled"`
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
	// CapacityPolicy controls per-upstream capacity limits (concurrency, RPM, TPM, queue).
	CapacityPolicy *CapacityPolicy   `json:"capacity_policy,omitempty"`
	Tags           map[string]string `json:"tags,omitempty"`
	Metadata       *UpstreamMetadata `json:"metadata,omitempty"`
}

// MetadataProtocol returns the explicit protocol override from metadata, if set.
func (uc UpstreamConfig) MetadataProtocol() string {
	if uc.Metadata == nil {
		return ""
	}
	return uc.Metadata.Protocol
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

// CapacityPolicy defines per-upstream capacity limits for admission control
// and capacity-aware routing. When Enabled, the AIGateway will track and
// enforce these limits before forwarding requests to the upstream.
// A zero value means "no limit" for that dimension, except that an enabled
// policy with every limit unset (AllLimitsUnset) is fully populated from the
// configured defaults every time the gateway reads the policy (see
// ApplyDefaults). Disabling a policy never clears the stored limits: admins
// can re-enable later and keep the previous configuration.
type CapacityPolicy struct {
	// Enabled turns on capacity tracking and enforcement for this upstream.
	Enabled bool `json:"enabled"`
	// MaxConcurrency is the maximum number of in-flight requests.
	MaxConcurrency int `json:"max_concurrency,omitempty"`
	// MaxQueueDepth is the maximum number of requests waiting for a slot.
	MaxQueueDepth int `json:"max_queue_depth,omitempty"`
	// MaxTPM is the maximum number of tokens per minute. 0 = unlimited.
	MaxTPM int64 `json:"max_tpm,omitempty"`
	// MaxRPM is the maximum number of requests per minute.
	MaxRPM int `json:"max_rpm,omitempty"`
	// QueueWaitSeconds is how long a request may wait in the upstream queue
	// before being rejected. 0 = unset (runtime falls back to the default).
	QueueWaitSeconds int `json:"queue_wait_seconds,omitempty"`
}

// AllLimitsUnset reports whether every limit field of the policy is zero.
func (p *CapacityPolicy) AllLimitsUnset() bool {
	return p.MaxConcurrency == 0 &&
		p.MaxQueueDepth == 0 &&
		p.MaxTPM == 0 &&
		p.MaxRPM == 0 &&
		p.QueueWaitSeconds == 0
}

// ApplyDefaults populates an enabled policy only when every limit is unset:
// an all-zero enabled policy is fully replaced by the configured defaults,
// while a policy with any limit set is left untouched so explicit values
// (including MaxTPM=0 meaning "unlimited") are never overridden by defaults.
// Default fields that are <= 0 are ignored so operators can keep "no limit"
// for a dimension. It is a no-op when p is nil or disabled.
func (p *CapacityPolicy) ApplyDefaults(defaults CapacityPolicy) {
	if p == nil || !p.Enabled || !p.AllLimitsUnset() {
		return
	}
	if defaults.MaxConcurrency > 0 {
		p.MaxConcurrency = defaults.MaxConcurrency
	}
	if defaults.MaxQueueDepth > 0 {
		p.MaxQueueDepth = defaults.MaxQueueDepth
	}
	if defaults.MaxTPM > 0 {
		p.MaxTPM = defaults.MaxTPM
	}
	if defaults.MaxRPM > 0 {
		p.MaxRPM = defaults.MaxRPM
	}
	if defaults.QueueWaitSeconds > 0 {
		p.QueueWaitSeconds = defaults.QueueWaitSeconds
	}
}

type LLMConfig struct {
	ID                 int64            `json:"id"`
	ModelName          string           `json:"model_name"`
	Source             UpstreamSource   `json:"source,omitempty"`
	OfficialName       string           `json:"-"`     // deprecated: derived from upstream
	ApiEndpoint        string           `json:"-"`     // deprecated: derived from upstream
	AuthHeader         string           `json:"-"`     // deprecated: moved to upstream
	Type               int              `json:"-"`     // internal bitmask, use Types for the API
	Types              []int            `json:"types"` // individual type flags derived from Type
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
	Types     []int  `json:"types"`      // Type flags to match (configs that include any of these flags are returned)
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
	Types              *[]int            `json:"types"` // individual type flags, combined into a bitmask on update
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
	Types              []int            `json:"types" binding:"required,min=1,dive,oneof=1 2 4 8 16"` // individual type flags, combined into a bitmask on create
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
	HealthCheckEnabled    *bool             `json:"health_check_enabled"`
	CircuitBreakerEnabled *bool             `json:"circuit_breaker_enabled"`
	LimitPolicy           *UsageLimitPolicy `json:"limit_policy,omitempty"`
	CapacityPolicy        *CapacityPolicy   `json:"capacity_policy,omitempty"`
	Tags                  map[string]string `json:"tags,omitempty"`
	Metadata              *UpstreamMetadata `json:"metadata,omitempty"`
}

// UpdateUpstreamReq is the request to update an existing upstream.
// Only non-nil fields will be updated.
//
// Double-pointer policy fields (LimitPolicy, CapacityPolicy) follow a
// three-state component contract: nil outer pointer = field omitted (keep the
// stored value), non-nil outer pointer wrapping nil inner pointer = clear the
// policy, non-nil inner pointer = replace the policy. Note that encoding/json
// binds an explicit JSON null to the OMITTED state, so over HTTP the stored
// policy can only be replaced or disabled (Enabled=false), never nulled.
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
	CapacityPolicy        **CapacityPolicy   `json:"capacity_policy"`
	Tags                  *map[string]string `json:"tags"`
	Metadata              *UpstreamMetadata  `json:"metadata"`
}

// TestUpstreamReq is the request to test connectivity to an upstream endpoint.
// The upstream is identified by its database ID; the backend fetches the
// upstream's URL, model name and auth header from the database so that the
// frontend never needs to send credentials or perform cross-origin requests.
type TestUpstreamReq struct {
	ID int64 `json:"id" binding:"required"`
}

// TestUpstreamResult is the result of an upstream connectivity test.
// The request summary is masked so that sensitive header values (such as
// API keys) are never leaked to the frontend.
type TestUpstreamResult struct {
	// Request is the masked request summary (url, method, headers, body).
	Request string `json:"request"`
	// OK indicates whether the upstream returned a 2xx status code.
	OK bool `json:"ok"`
	// Status is the HTTP status code returned by the upstream.
	Status int `json:"status"`
	// StatusText is the HTTP status text returned by the upstream.
	StatusText string `json:"status_text"`
	// Content is the extracted text content from the upstream response.
	Content string `json:"content"`
	// ResponseBody is the raw response body from the upstream.
	ResponseBody string `json:"response_body"`
	// Error is the error message when the test fails (e.g. timeout, network error).
	Error string `json:"error,omitempty"`
}
