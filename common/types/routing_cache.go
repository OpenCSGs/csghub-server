package types

import "time"

// RoutingLease is authority metadata, never a user-provided routing decision.
type RoutingLease struct {
	SnapshotID   string    `json:"snapshot_id"`
	Generation   string    `json:"generation"`
	Revision     int64     `json:"revision"`
	ExpiresAt    time.Time `json:"expires_at"`
	MaxExpiresAt time.Time `json:"max_expires_at"`
	TTLMS        int64     `json:"ttl_ms"`
	Renewable    *bool     `json:"renewable,omitempty"`
}

// RoutingCacheIdentity binds an immutable ranking; configuration is not scope.
type RoutingCacheIdentity struct {
	Scope          string
	SessionID      string
	TurnID         string
	ConfigIdentity string
	InputIdentity  string
}

// RoutingCacheRecord keeps its existing format for additive optional metadata.
// Records written before exploration have no decision and remain valid until
// expiry; renewal preserves the original ranking and does not invent a decision.
// Exploration attribution becomes available on newly ranked turns after rollout.
type RoutingCacheRecord struct {
	RoutingDecision *RouterDecision `json:"routing_decision,omitempty"`
	GenerationUsed  bool            `json:"generation_used,omitempty"`
	ConfigIdentity  string          `json:"config_identity,omitempty"`
	InputIdentity   string          `json:"input_identity,omitempty"`
	Format          int             `json:"cache_format_version"`
	Scope           string          `json:"scope_digest"`
	SessionID       string          `json:"session_id"`
	TurnID          string          `json:"turn_id"`
	ModelList       []string        `json:"model_list"`
	Lease           RoutingLease    `json:"lease"`
}

type RoutingRenewRequest struct {
	ConfigIdentity   string `json:"config_identity,omitempty"`
	Scope            string `json:"-"`
	SessionID        string `json:"-"`
	TurnID           string `json:"-"`
	SnapshotID       string `json:"snapshot_id"`
	Generation       string `json:"generation"`
	ExpectedRevision int64  `json:"expected_revision"`
}
