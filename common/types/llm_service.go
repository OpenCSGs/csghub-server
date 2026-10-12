package types

import (
	"encoding/json"
	"fmt"
	"net/url"
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

// Resource ID scheme prefixes, for prefix checks without fmt.
const (
	ExternalLLMResourcePrefix = "thirdparty://"
	CSGHubResourcePrefix      = "csghub://"
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
	// IsFullURL marks the URL as a complete endpoint URL used as-is (no path
	// appended). When false the URL is a base URL and the client protocol
	// path is appended for natively supported API formats.
	IsFullURL bool `json:"is_full_url"`
	// SupportedEndpoints maps a protocol (chat / responses / messages) to an
	// optional per-protocol endpoint URL. An empty value means the protocol
	// is served by the main URL field (full URL as-is, or base URL with the
	// protocol path appended). A missing key means the protocol is not
	// declared; routing falls back to URL inference and legacy adapters.
	SupportedEndpoints map[ApiFormat]string `json:"supported_endpoints,omitempty"`
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

// HasProtocolDeclaration reports whether the upstream carries an explicit or
// inferable API-format declaration (SupportedEndpoints, metadata.protocol, a
// recognizable URL terminal path, or the CSGHub-hosted chat default). When
// false the upstream's protocol is unknown and the legacy routing applies:
// the request is passed through natively with the client's own protocol.
func (uc UpstreamConfig) HasProtocolDeclaration() bool {
	if len(uc.SupportedEndpoints) > 0 || uc.MetadataProtocol() != "" {
		return true
	}
	if _, ok := InferProtocolFromURLPath(uc.URL); ok {
		return true
	}
	return uc.Source == UpstreamSourceCSGHubDeploy
}

// legacyDeclaredEndpoints resolves the declared endpoint map for upstreams
// without an explicit SupportedEndpoints config (legacy rows): the
// metadata.protocol override, the URL terminal path inference, or the
// CSGHub-hosted chat default. Returns nil when nothing is declared.
func (uc UpstreamConfig) legacyDeclaredEndpoints() map[ApiFormat]string {
	if proto := uc.MetadataProtocol(); proto != "" {
		return map[ApiFormat]string{ApiFormat(proto).Normalized(): ""}
	}
	if inferred, ok := InferProtocolFromURLPath(uc.URL); ok {
		return map[ApiFormat]string{ApiFormat(inferred): ""}
	}
	if uc.Source == UpstreamSourceCSGHubDeploy {
		return map[ApiFormat]string{ApiFormatChat: ""}
	}
	return nil
}

// SupportsProtocol reports whether the upstream natively supports the given
// client protocol ("chat" / "responses" / "messages", matching the Protocol
// constants of aigateway/types). The explicit SupportedEndpoints map wins;
// when it is empty the declaration is inferred from the metadata.protocol
// declaration or the URL terminal path (legacy rows), and an undeclared
// CSGHub-hosted upstream defaults to chat.
func (uc UpstreamConfig) SupportsProtocol(p ApiFormat) bool {
	declared := uc.declaredApiFormats()
	if len(declared) == 0 {
		declared = uc.legacyDeclaredEndpoints()
		if declared == nil {
			return false
		}
	}
	_, ok := declared[p.Normalized()]
	return ok
}

// declaredApiFormats returns the EXPLICIT declaration set: the
// SupportedEndpoints keys unioned with the legacy metadata.protocol override
// (an override whose format is already a map key is folded in; its endpoint
// value stays the map's). This mirrors SupportedProtocolsOf in the protocol
// router so candidate selection and routing agree on what an upstream
// explicitly declares. URL inference and the CSGHub chat default are NOT
// part of this set — those are legacy fallbacks applied by the callers.
func (uc UpstreamConfig) declaredApiFormats() map[ApiFormat]string {
	declared := make(map[ApiFormat]string, len(uc.SupportedEndpoints)+1)
	for key, value := range uc.SupportedEndpoints {
		declared[key.Normalized()] = strings.TrimSpace(value)
	}
	if proto := uc.MetadataProtocol(); proto != "" {
		if f := ApiFormat(proto).Normalized(); f != "" {
			if _, exists := declared[f]; !exists {
				declared[f] = ""
			}
		}
	}
	return declared
}

// DeclaredProtocolEndpoint returns the per-protocol endpoint URL for the
// given protocol: the explicit SupportedEndpoints value when declared (may be
// empty, meaning the main URL serves this protocol), or "" when the protocol
// is not declared.
func (uc UpstreamConfig) DeclaredProtocolEndpoint(p ApiFormat) string {
	declared := uc.SupportedEndpoints
	if len(declared) == 0 {
		declared = uc.legacyDeclaredEndpoints()
		if declared == nil {
			return ""
		}
	}
	return strings.TrimSpace(declared[p.Normalized()])
}

// ResolveSampleEndpoint resolves the concrete endpoint a protocol sample
// (health check / connection test) should probe. A per-protocol endpoint
// override is used as-is when set; otherwise full URLs and URLs that already
// carry a recognizable protocol path are used as-is, and a base URL with a
// declared API format (or a CSGHub-hosted deploy, which defaults to chat)
// gets the declared protocol's standard path appended — chat first when
// several formats are declared.
func ResolveSampleEndpoint(uc UpstreamConfig) string {
	// A per-protocol endpoint override is always a complete URL.
	for _, order := range RoutableApiFormats {
		for key, endpoint := range uc.SupportedEndpoints {
			if key.Normalized() == order && strings.TrimSpace(endpoint) != "" {
				return strings.TrimSpace(endpoint)
			}
		}
	}
	raw := strings.TrimSpace(uc.URL)
	if raw == "" || uc.IsFullURL {
		return raw
	}
	if _, ok := InferProtocolFromURLPath(raw); ok {
		return raw
	}
	declared := uc.SupportedEndpoints
	if len(declared) == 0 && uc.Source == UpstreamSourceCSGHubDeploy && csghubDeployServesChat(uc.deployTask()) {
		// CSGHub LLM deploys (vLLM/SGLang) expose a bare service root and
		// serve chat completions at /v1/chat/completions — probe there.
		// Multimodal deploys (image/audio/video tasks) do not serve chat;
		// keeping the URL raw makes the sample registry skip them (legacy
		// behavior) instead of probing a 404 and marking them unhealthy.
		declared = map[ApiFormat]string{ApiFormatChat: ""}
	}
	for _, order := range RoutableApiFormats {
		if _, declaredMatch := declared[order]; !declaredMatch {
			continue
		}
		segments := order.StandardPathSegments()
		if len(segments) == 0 {
			return raw
		}
		// Deduplicate a trailing /v1 on the base path, mirroring the native
		// routing path construction.
		trimmed := raw
		if !strings.Contains(trimmed, "://") {
			trimmed = "http://" + trimmed
		}
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return raw
		}
		baseSegments := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
		if len(baseSegments) > 0 && strings.EqualFold(baseSegments[len(baseSegments)-1], PathSegV1) {
			segments = segments[1:]
		}
		// Rebuild from the parsed URL so the base query (e.g. Azure's
		// ?api-version=...) survives the append.
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.Join(segments, "/")
		parsed.RawPath = ""
		return parsed.String()
	}
	return raw
}

// csghubDeployServesChat reports whether a CSGHub deploy with the given
// InternalModelInfo.Task serves chat completions. An unknown task ("") keeps
// the chat probe (LLM deploys are the norm); known multimodal tasks opt out.
func csghubDeployServesChat(task string) bool {
	t := strings.ToLower(strings.TrimSpace(task))
	return t == "" || t == "text-generation"
}

// EffectiveSampleEndpoint resolves the health-check / connection-test probe
// endpoint for this upstream (see ResolveSampleEndpoint).
func (uc UpstreamConfig) EffectiveSampleEndpoint() string {
	return ResolveSampleEndpoint(uc)
}

// deployTask extracts the CSGHub deploy task from metadata, if any.
func (uc UpstreamConfig) deployTask() string {
	if uc.Metadata != nil && uc.Metadata.InternalModelInfo != nil {
		return uc.Metadata.InternalModelInfo.Task
	}
	return ""
}

// InferProtocolFromURLPath guesses the API format from a URL's terminal path
// segments: .../chat/completions → ApiFormatChat, .../responses →
// ApiFormatResponses, .../messages or .../anthropic → ApiFormatMessages,
// .../systemone → ApiFormatSystemOne. It mirrors the URL inference used by
// the AIGateway protocol router so admin CRUD, backfill and routing all
// agree on what a URL means.
func InferProtocolFromURLPath(rawURL string) (ApiFormat, bool) {
	if strings.TrimSpace(rawURL) == "" {
		return "", false
	}
	// url.Parse interprets a bare host:port/path as scheme=host, so prepend a
	// scheme when absent to ensure the path is parsed correctly.
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	switch last := strings.ToLower(parts[len(parts)-1]); last {
	case PathSegCompletions:
		if len(parts) >= 2 && strings.EqualFold(parts[len(parts)-2], PathSegChat) {
			return ApiFormatChat, true
		}
	case PathSegResponses:
		return ApiFormatResponses, true
	case PathSegMessages, PathSegAnthropic:
		return ApiFormatMessages, true
	case PathSegSystemOne:
		return ApiFormatSystemOne, true
	}
	return "", false
}

// RoutingPolicy controls how a request selects one upstream from Upstreams.
type RoutingPolicy struct {
	Strategy      string `json:"strategy"`
	SessionHeader string `json:"session_header,omitempty"`
	HashReplicas  int    `json:"hash_replicas,omitempty"`
}

// CapacityPolicy defines per-upstream capacity limits for admission control
// and capacity-aware routing. When Enabled, the AIGateway will track and
// enforce these limits before forwarding requests to the upstream.
//
// An enabled policy is in exactly one of two mutually exclusive modes:
//
//   - Rate mode: QueueEnabled() is false. Requests beyond MaxConcurrency are
//     rejected immediately. MaxTPM/MaxRPM are enforced.
//   - Queue mode: QueueEnabled() is true (MaxQueueDepth > 0 AND
//     QueueWaitSeconds > 0). Requests beyond MaxConcurrency enter a bounded
//     queue and wait up to QueueWaitSeconds. MaxTPM/MaxRPM are NOT enforced
//     in queue mode, so a queue-mode policy must have them unset (0).
//
// A policy with every limit unset (AllLimitsUnset) is fully populated from
// the configured defaults as a queue-mode policy every time the gateway reads
// it (see ApplyDefaults). Disabling a policy never clears the stored limits:
// admins can re-enable later and keep the previous configuration.
type CapacityPolicy struct {
	// Enabled turns on capacity tracking and enforcement for this upstream.
	Enabled bool `json:"enabled"`
	// MaxConcurrency is the maximum number of in-flight requests.
	MaxConcurrency int `json:"max_concurrency,omitempty"`
	// MaxQueueDepth is the maximum number of requests waiting for a slot.
	MaxQueueDepth int `json:"max_queue_depth,omitempty"`
	// MaxTPM is the maximum number of tokens per minute. 0 = unlimited.
	// Ignored (and required to be 0) in queue mode.
	MaxTPM int64 `json:"max_tpm,omitempty"`
	// MaxRPM is the maximum number of requests per minute. 0 = unlimited.
	// Ignored (and required to be 0) in queue mode.
	MaxRPM int `json:"max_rpm,omitempty"`
	// QueueWaitSeconds is how long a request may wait in the upstream queue
	// before being rejected.
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

// QueueEnabled reports whether the bounded queue is active: it requires both
// MaxQueueDepth and QueueWaitSeconds to be positive. With either at zero the
// upstream is in rate mode and excess requests are rejected immediately.
func (p *CapacityPolicy) QueueEnabled() bool {
	return p != nil && p.MaxQueueDepth > 0 && p.QueueWaitSeconds > 0
}

// ApplyDefaults populates an enabled policy only when every limit is unset:
// an all-zero enabled policy is replaced by the configured defaults as a
// queue-mode policy (concurrency + queue depth + queue wait; TPM/RPM stay
// unset because they are not enforceable in queue mode), while a policy with
// any limit set is left untouched so explicit values (including MaxTPM=0
// meaning "unlimited") are never overridden by defaults. Default fields that
// are <= 0 are ignored so operators can keep "no limit" for a dimension.
// It is a no-op when p is nil or disabled.
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

// AIGatewayMetricEventQuery is the filter/pagination parameters for listing
// raw AI Gateway metric events (model request logs). StartTime/EndTime are
// mandatory and filter on the minute-truncated bucket_time hypertable
// dimension so queries stay chunk-pruned.
type AIGatewayMetricEventQuery struct {
	StartTime    time.Time `json:"start_time"`     // required, bucket_time >= StartTime
	EndTime      time.Time `json:"end_time"`       // required, bucket_time <= EndTime
	Model        string    `json:"model"`          // optional exact match
	StatusCode   int       `json:"status_code"`    // optional exact match, 0 means all
	Username     string    `json:"username"`       // optional exact match
	APIKeyMasked string    `json:"api_key_masked"` // optional exact match
	RequestID    string    `json:"request_id"`     // optional exact match
	// LlmConfigID optionally restricts events to upstreams belonging to this
	// llm config (resolved via upstream_id).
	LlmConfigID int64 `json:"llm_config_id"`
	// UpstreamIDs is filled by the component layer when LlmConfigID is set.
	UpstreamIDs []int64 `json:"-"`
	Per         int     `json:"-"`
	Page        int     `json:"-"`
}

// AIGatewayMetricEventLog is the API representation of one AI Gateway model
// request log row. LlmConfigID is resolved from UpstreamID so the frontend
// can link to the llm config detail page.
type AIGatewayMetricEventLog struct {
	RequestID string `json:"request_id"`
	// StartTime is the full-precision request start time, pre-formatted in
	// the global product timezone ("2006-01-02 15:04:05"); empty for rows
	// written before the diagnostics columns existed, so the frontend can
	// fall back to CreatedAt.
	StartTime           string `json:"start_time,omitempty"`
	CreatedAt           string `json:"created_at"`
	Model               string `json:"model"`
	Provider            string `json:"provider"`
	APIKeyMasked        string `json:"api_key_masked"`
	Username            string `json:"username"`
	StatusCode          int    `json:"status_code"`
	IsStream            bool   `json:"is_stream"`
	ErrorType           string `json:"error_type"`
	ErrorMessage        string `json:"error_message"`
	LatencyMs           int64  `json:"latency_ms"`
	TTFTMs              int64  `json:"ttft_ms"`
	QueueWaitMs         int64  `json:"queue_wait_ms"`
	PromptTokens        int64  `json:"prompt_tokens"`
	CompletionTokens    int64  `json:"completion_tokens"`
	TotalTokens         int64  `json:"total_tokens"`
	CachedTokens        int64  `json:"cached_tokens"`
	CacheCreationTokens int64  `json:"cache_creation_tokens"`
	UpstreamID          int64  `json:"upstream_id"`
	LlmConfigID         int64  `json:"llm_config_id"`
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
	LLMConfigID           int64  `json:"llm_config_id" binding:"required"`
	URL                   string `json:"url" binding:"required"`
	Weight                int    `json:"weight,omitempty"`
	Enabled               bool   `json:"enabled"`
	ModelName             string `json:"model_name,omitempty"`
	AuthHeader            string `json:"auth_header,omitempty"`
	Provider              string `json:"provider,omitempty"`
	HealthCheckEnabled    *bool  `json:"health_check_enabled"`
	CircuitBreakerEnabled *bool  `json:"circuit_breaker_enabled"`
	// IsFullURL marks the URL as a complete endpoint URL used as-is; when false
	// the URL is a base URL and the client protocol path is appended for the
	// supported API formats.
	IsFullURL bool `json:"is_full_url"`
	// SupportedEndpoints maps a declared protocol to an optional per-protocol
	// endpoint URL (empty value = the main URL serves that protocol).
	SupportedEndpoints map[ApiFormat]string `json:"supported_endpoints,omitempty"`
	CapacityPolicy     *CapacityPolicy      `json:"capacity_policy,omitempty"`
	Tags               map[string]string    `json:"tags,omitempty"`
	Metadata           *UpstreamMetadata    `json:"metadata,omitempty"`
}

// UpdateUpstreamReq is the request to update an existing upstream.
// Only non-nil fields will be updated.
//
// Double-pointer policy fields (CapacityPolicy) follow a
// three-state component contract: nil outer pointer = field omitted (keep the
// stored value), non-nil outer pointer wrapping nil inner pointer = clear the
// policy, non-nil inner pointer = replace the policy. Note that encoding/json
// binds an explicit JSON null to the OMITTED state, so over HTTP the stored
// policy can only be replaced or disabled (Enabled=false), never nulled.
type UpdateUpstreamReq struct {
	ID                    int64   `json:"id"`
	URL                   *string `json:"url"`
	Weight                *int    `json:"weight"`
	Enabled               *bool   `json:"enabled"`
	ModelName             *string `json:"model_name"`
	AuthHeader            *string `json:"auth_header"`
	Provider              *string `json:"provider"`
	HealthCheckEnabled    *bool   `json:"health_check_enabled"`
	CircuitBreakerEnabled *bool   `json:"circuit_breaker_enabled"`
	// IsFullURL/SupportedEndpoints follow the pointer contract above: nil =
	// omitted (keep stored value), non-nil = replace.
	IsFullURL          *bool                 `json:"is_full_url"`
	SupportedEndpoints *map[ApiFormat]string `json:"supported_endpoints"`
	CapacityPolicy     **CapacityPolicy      `json:"capacity_policy"`
	Tags               *map[string]string    `json:"tags"`
	Metadata           *UpstreamMetadata     `json:"metadata"`
}

// CheckUpstreamReq is the request to test connectivity to an upstream endpoint.
// The upstream is identified by its database ID; the backend fetches the
// upstream's URL, model name and auth header from the database so that the
// frontend never needs to send credentials or perform cross-origin requests.
type CheckUpstreamReq struct {
	ID int64 `json:"id" binding:"required"`
}

// CheckUpstreamResult is the result of an upstream connectivity test.
// The request summary is masked so that sensitive header values (such as
// API keys) are never leaked to the frontend.
type CheckUpstreamResult struct {
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
