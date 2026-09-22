package types

import (
	"errors"
	"fmt"
	"strings"
)

// AdmissionAction is the outcome of a capacity admission decision.
//
// v1 only produces admit and reject. The queue execution model (waiting for
// a slot with backpressure, dispatcher, fairness) is a different lifecycle
// entirely and is reserved for v2 — see CapacityPolicy.MaxQueueDepth.
type AdmissionAction string

const (
	// AdmissionAdmit means the request may proceed to the selected upstream.
	AdmissionAdmit AdmissionAction = "admit"
	// AdmissionReject means the request must not proceed.
	AdmissionReject AdmissionAction = "reject"
	// AdmissionReroute means the waiting request was cancelled because its
	// upstream became unavailable during the queue wait. The planner must
	// re-run model resolution and admission against a fresh candidate set.
	AdmissionReroute AdmissionAction = "reroute"
)

// Admission rejection reasons. The empty reason means "not rejected".
// budget_exceeded / queue_full / runtime_unhealthy / slo_protection are
// reserved for future admission inputs (see the Token Factory admission
// spec); they are not produced by v1.
const (
	AdmissionReasonConcurrencyExceeded = "concurrency_exceeded"
	AdmissionReasonRPMExceeded         = "rpm_exceeded"
	AdmissionReasonTPMExceeded         = "tpm_exceeded"
	// AdmissionReasonCapacityExceeded is used when more than one capacity
	// dimension blocked the request and no single reason applies.
	AdmissionReasonCapacityExceeded = "capacity_exceeded"
	// AdmissionReasonQueueFull means the admission reservation queue of the
	// selected upstream was full (bounded backpressure): the request was
	// rejected immediately without waiting.
	AdmissionReasonQueueFull = "queue_full"
	// AdmissionReasonQueueTimeout means the request waited in the queue
	// longer than CapacityPolicy.QueueWaitSeconds and gave up. Rendered as
	// HTTP 408.
	AdmissionReasonQueueTimeout = "queue_timeout"
	// AdmissionReasonQueueCancelled means the request left the queue without
	// being promoted and without a queue timeout: the client disconnected
	// (context cancelled). The ticket (or an already-granted lease) was
	// cleaned up; there is no meaningful response to render.
	AdmissionReasonQueueCancelled = "queue_cancelled"
)

// AdmissionPriority is the scheduling priority of one request inside the
// admission reservation queue. Lower numeric values are promoted first
// (high before low); within one priority the queue is FIFO by enqueue time.
//
// The queue discipline is "unified": once an upstream's queue is non-empty,
// every new request enqueues and is scheduled by (priority, enqueue time) —
// a fresh high-priority request does not bypass already-waiting requests.
// Sustained high-priority traffic can therefore starve low-priority
// requests; aging (promoting long-waiting tickets) is a deliberate v2
// non-goal.
type AdmissionPriority int

const (
	// AdmissionPriorityHigh is promoted before AdmissionPriorityLow.
	AdmissionPriorityHigh AdmissionPriority = 0
	// AdmissionPriorityLow is the default for requests that declare no
	// priority.
	AdmissionPriorityLow AdmissionPriority = 1
)

// AdmissionPriorityFromSource resolves the queue priority from the API
// key's server-side priority scope, set by the quota middleware from the
// key's quota configuration (the ONLY source: clients cannot influence
// queue scheduling). Unrecognized or missing scope values default to low.
func AdmissionPriorityFromSource(priorityScope string) AdmissionPriority {
	switch strings.ToLower(strings.TrimSpace(priorityScope)) {
	case "high":
		return AdmissionPriorityHigh
	default:
		return AdmissionPriorityLow
	}
}

// AdmissionLease identifies the Redis lease held by one upstream attempt.
//
// The lease is the authority for both the concurrency slot and the TPM
// reservation: the Redis token metadata hash (key suffix ":lease") stores
// "<est>:<window>" per lease token, and every lifecycle operation (release,
// expired-lease cleanup) must take lease ownership (ZREM == 1) before
// touching reservation aggregates.
type AdmissionLease struct {
	// ModelID is the logical model the lease belongs to (part of the Redis
	// hash-tag shard key).
	ModelID string
	// UpstreamID is the upstream the lease was acquired on.
	UpstreamID int64
	// Token is the unique lease token (ZSET member; also the key of the
	// token metadata hash entry).
	Token string
	// ReservedTokens is the TPM reservation recorded at acquire time. It is
	// informational for callers (e.g. re-reserving on fallback); the Redis
	// token metadata hash is the authority.
	ReservedTokens int64
}

// UpstreamCapacityState is the per-upstream capacity observation returned by
// an admission check. It is a snapshot taken at check time (T0).
//
// For the selected upstream of an admitted request, Current* INCLUDES this
// request's reservation acquired atomically by the admission script
// (post-acquire). For non-selected upstreams, Current* reflects the state
// before this request's reservation (pre-acquire) because no reservation was
// acquired for them.
//
// UpstreamCapacityState is observational only. Feasible for a non-selected
// upstream means "feasible at T0" — it does NOT guarantee the upstream still
// has capacity at fallback time, because other replicas may have acquired
// reservations in between. A fallback must therefore always re-acquire
// atomically against Redis; LeaseToken is the sole authority for ownership
// and finalization.
type UpstreamCapacityState struct {
	UpstreamID         int64
	CurrentConcurrency int64
	CurrentRPM         int64
	CurrentTPM         int64
	MaxConcurrency     int
	MaxRPM             int
	MaxTPM             int64
	// Feasible reports whether this upstream could accept the request at
	// check time.
	Feasible bool
	// Selected is true for the upstream whose lease was acquired by an
	// admitted request. Its Current* values are post-acquire.
	Selected bool
	// BlockedBy lists the dimensions that blocked this upstream:
	// "concurrency" / "rpm" / "tpm".
	BlockedBy []string
}

// AdmissionDecision is the result of one admission check.
type AdmissionDecision struct {
	Action AdmissionAction
	// Reason is a rejection reason constant; empty when admitted.
	Reason string
	// RetryAfterSeconds is a retry hint for rejected requests. It is NOT a
	// capacity guarantee: for RPM/TPM it is the seconds until the current
	// window ends; for concurrency it is a configured hint.
	RetryAfterSeconds int64
	// Lease is non-nil when a lease was actually acquired (Action == admit
	// and admission control ran). Fail-open admissions carry no lease.
	Lease *AdmissionLease
	// SelectedUpstreamID is the upstream the request should use. It differs
	// from the router-preferred upstream when a capacity-aware fallback
	// re-selection happened.
	SelectedUpstreamID int64
	// ReSelected is true when admission replaced the preferred upstream.
	ReSelected bool
	// States is the observation snapshot for the evaluated candidates.
	States []UpstreamCapacityState
	// FailOpen is true when Redis was unavailable and the request was
	// admitted without a lease (protection degraded, request allowed).
	FailOpen bool
}

// AdmissionOutcome is the planner-facing result of an admission check. It
// pairs the decision with the rebuilt model target when admission re-selected
// a different upstream (capacity-aware fallback within the router-owned
// candidate set).
type AdmissionOutcome struct {
	Decision *AdmissionDecision
	// ReSelectedTarget is the fully rebuilt model target for the re-selected
	// upstream (target/auth/host/model name/fallback candidates). nil when
	// no re-selection happened.
	ReSelectedTarget *ModelTarget
}

// AdmissionNoTPMEstimate is the EstimatedTokens sentinel for requests that
// do not participate in the TPM admission dimension (multimodal requests,
// where a text-based token estimate would be fiction). Such requests reserve
// no TPM tokens, skip the TPM feasibility gate, and exclude TPM headroom
// from the selection score; their actual usage is still committed to the TPM
// window at finalize, so the window aggregate remains the true total usage.
const AdmissionNoTPMEstimate int64 = 0

// CapacityAdmissionRequest carries the admission inputs for one request.
type CapacityAdmissionRequest struct {
	// NSUUID is the tenant namespace UUID (billing identity), carried for
	// admission logging.
	NSUUID string
	// Model is the resolved model; Model.Upstreams is the router-owned
	// candidate set (already availability-filtered).
	Model *Model
	// PreferredUpstreamID is the upstream selected by the router.
	PreferredUpstreamID int64
	// AllowSelect enables capacity-aware fallback re-selection within the
	// candidate set. It must be false for pinned or session-affinity
	// requests.
	AllowSelect bool
	// EstimatedTokens is the TPM pre-reservation estimate. A value
	// <= AdmissionNoTPMEstimate (0) means the request does not participate
	// in the TPM dimension (multimodal requests).
	EstimatedTokens int64
	// Priority is the queue scheduling priority used when the request must
	// wait in an upstream's admission reservation queue. It has no effect
	// on immediate admission.
	Priority AdmissionPriority
}

// MultimodalContentProvider reports whether a request body carries non-text
// content (images, audio, ...). It mirrors the PromptTextProvider pattern:
// protocol request types implement it optionally, and the admission adapter
// type-asserts it to skip the text-based TPM estimate. Request types without
// multimodal content do not need to implement it.
type MultimodalContentProvider interface {
	HasMultimodalContent() bool
}

// AdmissionUsage carries the actual token usage observed for a lease,
// used to correct the TPM reservation at finalize time. nil usage means
// the attempt produced no usable usage (full reservation reclaim).
type AdmissionUsage struct {
	TotalTokens int64
}

// AdmissionDeniedError is the domain error returned when admission rejects a
// request. The planner categorizes it into PlanErrCapacityExceeded; protocol
// handlers render it as HTTP 429.
type AdmissionDeniedError struct {
	Decision *AdmissionDecision
}

func (e *AdmissionDeniedError) Error() string {
	if e == nil || e.Decision == nil {
		return "request denied by capacity admission"
	}
	if e.Decision.Reason == "" {
		return "request denied by capacity admission"
	}
	return fmt.Sprintf("request denied by capacity admission: %s", e.Decision.Reason)
}

// IsAdmissionDenied reports whether err was produced by capacity admission.
func IsAdmissionDenied(err error) bool {
	var denied *AdmissionDeniedError
	return errors.As(err, &denied)
}
