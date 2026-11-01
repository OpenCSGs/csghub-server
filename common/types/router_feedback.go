package types

import "encoding/json"

// RouterPreviousCall describes a completed observation, not a predicted cache hit.
type RouterPreviousCall struct {
	InputPrefixHash    string  `json:"-"` // Gateway-only evidence; never transmitted to Router.
	InputMessageCount  int     `json:"-"`
	CacheReusable      *bool   `json:"cache_reusable,omitempty"` // Gateway confirms the sole eligible upstream matches the observation.
	RequestID          string  `json:"request_id"`
	AttemptID          string  `json:"attempt_id"`
	Model              string  `json:"model"`
	UpstreamID         *string `json:"upstream_id"`
	PromptTokens       *int64  `json:"prompt_tokens"`
	CachedPromptTokens *int64  `json:"cached_prompt_tokens"`
	CompletedAt        string  `json:"completed_at"`
}

// RouterCallFeedback is bounded execution metadata. No prompt or response bodies.
type RouterCallFeedback struct {
	ExplorationTargetMatch *bool `json:"exploration_target_match,omitempty"`
	RouterPreviousCall
	SessionID                string          `json:"-"`
	TurnID                   string          `json:"-"`
	ConfigIdentity           string          `json:"-"`
	SnapshotID               string          `json:"-"`
	UpstreamProtocol         string          `json:"upstream_protocol"`
	SubmittedParametersHash  *string         `json:"submitted_parameters_hash,omitempty"`
	GenerationParametersHash *string         `json:"generation_parameters_hash"`
	GenerationConfig         json.RawMessage `json:"generation_config"`
	Status                   string          `json:"status"`
	ErrorCode                *string         `json:"error_code"`
	FinishReason             *string         `json:"finish_reason"`
	CompletionTokens         *int64          `json:"completion_tokens"`
	ReasoningTokens          *int64          `json:"reasoning_tokens"`
	TTFTMS                   *float64        `json:"ttft_ms"`
	DurationMS               float64         `json:"duration_ms"`
}

// RouterTurnFeedback is sent once the next turn starts. Calls remain separate
// so retries and model switches are not mistaken for one model's usage.
type RouterTurnFeedback struct {
	RoutingDecision *RouterDecision       `json:"routing_decision,omitempty"`
	SessionID       string                `json:"session_id"`
	TurnID          string                `json:"turn_id"`
	ConfigIdentity  string                `json:"config_identity"`
	SnapshotID      string                `json:"snapshot_id"`
	Calls           []*RouterCallFeedback `json:"calls"`
	Incomplete      bool                  `json:"incomplete"`
	OmittedAttempts int                   `json:"omitted_attempts"`
}
