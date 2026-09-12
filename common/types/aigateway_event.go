package types

// ProviderTypeFromDeployType maps a deploy type integer to the LLM provider
// type string. This is the single source of truth for deploy-type → provider
// mapping, used by both the trigger side (builder/event) and the consumer
// side (aigateway).
func ProviderTypeFromDeployType(t int) string {
	switch t {
	case ServerlessType:
		return ProviderTypeServerless
	case InferenceType:
		return ProviderTypeInference
	default:
		return ProviderTypeInference
	}
}

// DeployUpstreamInfo carries only the deploy fields that the upstream sync
// consumer needs. It is built on the trigger side (API server) from a
// deploy loaded with Repository and User relations, so the consumer
// (AIGateway) does not need to query the server's deploy tables.
type DeployUpstreamInfo struct {
	DeployID         int64  `json:"deploy_id"`
	RepoPath         string `json:"repo_path"`          // Repository.Path
	RepoName         string `json:"repo_name"`          // Repository.Name
	HFPath           string `json:"hf_path"`            // Repository.HFPath
	DeployType       int    `json:"deploy_type"`        // deploy.Type
	Provider         string `json:"provider"`           // pre-computed from DeployType
	Endpoint         string `json:"endpoint"`
	ClusterID        string `json:"cluster_id"`
	SvcName          string `json:"svc_name"`
	ImageID          string `json:"image_id"`
	RuntimeFramework string `json:"runtime_framework"`
	EngineArgs       string `json:"engine_args"`
	Task             string `json:"task"`
	UserUUID         string `json:"user_uuid"`
	OwnerUsername    string `json:"owner_username"`
	// OwnerNamespace is the billing/listing namespace (username or org name).
	OwnerNamespace string `json:"owner_namespace"`
	// OwnerType is "user" or "organization", resolved from the namespace table
	// on the trigger side so the consumer can efficiently check membership
	// without querying the server's namespace tables.
	OwnerType     string `json:"owner_type"`
	CreatedAt     int64  `json:"created_at"`         // unix timestamp
	LegacyModelID string `json:"legacy_model_id"`    // pre-computed
}

// DeployUpstreamSyncEvent is the MQ message payload for deploy→upstream sync.
// It is published by deploy lifecycle handlers when a deploy transitions to
// running, is stopped, or is deleted. The AIGateway upstream sync consumer
// processes these events to keep the ai_gateway_upstreams table in sync.
//
// For running events, Deploy carries the pre-built upstream info so the
// consumer does not need to query the server's deploy tables. For stop and
// delete events, Deploy is nil — the consumer only needs DeployID to find
// and disable/delete the upstream by source + source_id.
type DeployUpstreamSyncEvent struct {
	// DeployID is the database ID of the deploy record.
	DeployID int64 `json:"deploy_id"`
	// EventTime is a monotonically increasing timestamp (Unix nanoseconds)
	// set by the publisher when the event is created. The consumer uses it to
	// reject stale events: if a running event arrives after a later stop or
	// delete event has already been processed for the same deploy, the stale
	// running event is discarded to prevent resurrecting a stopped or deleted
	// upstream.
	EventTime int64 `json:"event_time"`
	// Deploy is the pre-built upstream info for running events.
	// nil for stop/delete events.
	Deploy *DeployUpstreamInfo `json:"deploy,omitempty"`
}
