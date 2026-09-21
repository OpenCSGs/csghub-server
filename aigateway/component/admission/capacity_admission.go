package admission

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	prom "opencsg.com/csghub-server/builder/prometheus"
	"opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/common/config"
	commontypes "opencsg.com/csghub-server/common/types"
)

// Capacity admission control — the first backpressure layer between the
// AIGateway and the upstream runtimes.
//
// # Reservation lifecycle invariant
//
// Every reservation (a concurrency slot or a TPM token reservation) is owned
// by a lease: the lease ZSET member is the unique token, and the token
// metadata hash
// stores "<estimatedTokens>:<tpmWindowStart>" per token. Only an operation
// that successfully takes lease ownership (ZREM == 1) may modify the
// reservation aggregates. Acquire, expired-lease cleanup and finalize all
// follow this guard, which makes every operation exactly-once across
// replicas: a finalize that arrives after another replica already reclaimed
// the lease is a no-op.
//
// # Semantics table
//
//	Dimension   | Check           | Acquire        | Commit            | Release   | Expired cleanup
//	Concurrency | cur < max       | ZADD lease     | —                 | ZREM      | ZREM
//	RPM         | cur < max       | INCRBY 1       | —                 | —         | window rollover
//	TPM (soft)  | cur+est <= max  | INCRBY est     | INCRBY actual     | INCRBY    | INCRBY -est
//	            |                 | + record owner | (current window)  | -est      | (original window)
//
// RPM is immutable accounting: it is counted once per upstream attempt at
// admission and never rolled back. TPM is a SOFT limit: the pre-reservation
// is an estimate (prompt heuristic + completion reserve), corrected with the
// real usage after the response; actual usage can exceed MaxTPM by the
// estimation error. The reservation is bound to the window at acquire time;
// finalize/release always correct the ORIGINAL window recorded in the lease
// metadata, while committed usage lands in the window at completion time.
//
// # Multi-replica behavior
//
// All state lives in Redis (shared across gateway replicas). Keys carry the
// "{m:<modelID>}" cluster hash tag so that all keys of one model's admission
// decision share a Redis Cluster slot and stay atomically scriptable; load
// is spread per model. Lease expiry is evaluated with Redis server TIME, so
// replica clock skew is irrelevant. When Redis is unavailable the controller
// fails OPEN (configurable): requests proceed without a lease, which degrades
// to no backpressure instead of blocking all traffic.
//
// # Router vs Admission boundary
//
// The Router owns the candidate set ("where should this request go");
// Admission only evaluates feasibility within that candidate set ("can it be
// accepted"). Admission never queries the database to build its own candidate
// set. Session affinity and pinned upstreams are never re-selected by
// capacity (protecting KV/prefix cache reuse); availability-driven fallback
// is a separate mechanism that may break affinity to protect success rate.
//
// The reject path below is the v2 queue extension point: when a queue
// implementation lands, an infeasible-but-queueable request would be routed
// to a waiting structure instead of being rejected (CapacityPolicy
// .MaxQueueDepth already exists but is intentionally unused in v1).
const (
	capacityKeyPrefix       = "aigateway:capacity"
	capacityWindowSeconds   = int64(60)
	capacityTTLBufferSecond = int64(60)
	// admissionEstimateMinFloor is the minimum TPM reservation per request
	// even when the prompt estimate is empty.
	admissionEstimateMinFloor = int64(64)
)

// capacityAdmissionScript atomically evaluates all candidate upstreams of a
// model and acquires a lease on the selected one.
//
// KEYS: per candidate i: KEYS[2i+1] = "<shard>:conc" (lease ZSET),
// KEYS[2i+2] = "<shard>:lease" (per-token reservation metadata hash). The
// RPM/TPM window
// keys are derived inside the script from the ":conc" key prefix so the
// window start is computed from Redis TIME (single time authority; the
// derived keys share the same cluster hash tag).
//
// ARGV: [1] window seconds, [2] lease TTL seconds, [3] lease token,
// [4] preferred candidate index (-1 when unknown), [5] allow select (0/1),
// [6] estimated tokens, then per candidate: max concurrency, max RPM,
// max TPM (0 = unlimited).
//
// est <= 0 means the request does not participate in the TPM dimension
// (multimodal requests, where a text-based token estimate would be fiction):
// no TPM feasibility gate, no TPM input in the selection score, and no TPM
// reservation is written (no window increment, no lease metadata). Actual
// usage is still committed to the TPM window at finalize, so the window
// aggregate remains the true total usage.
//
// Returns a flat array: [admitted(0/1), chosenIndex, then per candidate:
// feasible, conc, rpm, tpm, blockedConc, blockedRPM, blockedTPM].
const capacityAdmissionScript = `
local window_sec = tonumber(ARGV[1])
local lease_ttl_sec = tonumber(ARGV[2])
local token = ARGV[3]
local preferred = tonumber(ARGV[4])
local allow_select = tonumber(ARGV[5]) == 1
local est = tonumber(ARGV[6])

local t = redis.call('TIME')
local now_sec = tonumber(t[1])
local now_ms = now_sec * 1000 + math.floor(tonumber(t[2]) / 1000)
local cutoff = now_ms - window_sec * 1000
local expire_sec = lease_ttl_sec + 60

-- Sliding-window GC helper: drops RPM/TPM-usage entries older than the
-- window. TPM-usage members encode their token delta as ":<tokens>" so the
-- dropped contributions can be subtracted from the running sum EXACTLY
-- (member-wise ZREM, never ZREMRANGEBYSCORE, so a LIMIT cannot cause drift).
-- Reservations are NOT age-GC'd: a live lease's reservation must survive as
-- long as the lease, so it is removed only by release/cleanup/commit.
local function gc_usage(prefix)
	local stale = redis.call('ZRANGEBYSCORE', prefix .. ':tpm:usage', '-inf', '(' .. cutoff, 'LIMIT', 0, 500)
	local dropped = 0
	for _, m in ipairs(stale) do
		local tokens = string.match(m, ':(%-?%d+)$')
		if tokens then
			dropped = dropped + tonumber(tokens)
			redis.call('ZREM', prefix .. ':tpm:usage', m)
		end
	end
	if dropped > 0 and redis.call('EXISTS', prefix .. ':tpm:sum') == 1 then
		redis.call('DECRBY', prefix .. ':tpm:sum', dropped)
	end
	return dropped
end

-- Sliding-window retry hint: seconds until the oldest entry of a dimension
-- leaves the window (some capacity frees up then). 0 when undeterminable.
local function oldest_exit_sec(zset_key)
	local oldest = redis.call('ZRANGE', zset_key, 0, 0, 'WITHSCORES')
	if oldest[2] then
		local exit_sec = math.ceil(tonumber(oldest[2]) / 1000) + window_sec - now_sec
		if exit_sec < 1 then exit_sec = 1 end
		if exit_sec > window_sec then exit_sec = window_sec end
		return exit_sec
	end
	return 0
end

local n = math.floor(#KEYS / 2)
local results = {}

for i = 0, n - 1 do
	local conc_key = KEYS[i * 2 + 1]
	local lease_key = KEYS[i * 2 + 2]
	local prefix = string.match(conc_key, '^(.*):conc$')
	local rpm_key = prefix .. ':rpm'
	local sum_key = prefix .. ':tpm:sum'
	local usage_key = prefix .. ':tpm:usage'

	-- Reclaim leases left behind by crashed replicas. Ownership guard:
	-- only a successful ZREM may reclaim the TPM reservation the lease owns.
	-- Capped per run: under a crash backlog the rest is reclaimed by the
	-- next script run instead of blocking Redis with one long script.
	local expired = redis.call('ZRANGEBYSCORE', conc_key, '-inf', now_ms, 'LIMIT', 0, 200)
	for _, tok in ipairs(expired) do
		if redis.call('ZREM', conc_key, tok) == 1 then
			local meta = redis.call('HGET', lease_key, tok)
			if meta then
				local m_est = string.match(meta, '^(%-?%d+):')
				if m_est and redis.call('EXISTS', sum_key) == 1 then
					redis.call('DECRBY', sum_key, tonumber(m_est))
				end
				redis.call('HDEL', lease_key, tok)
			end
		end
	end

	redis.call('ZREMRANGEBYSCORE', rpm_key, '-inf', '(' .. cutoff)
	gc_usage(prefix)

	local conc = redis.call('ZCARD', conc_key)
	local rpm = redis.call('ZCARD', rpm_key)
	local tpm = tonumber(redis.call('GET', sum_key) or '0')
	local max_conc = tonumber(ARGV[7 + i * 3])
	local max_rpm = tonumber(ARGV[8 + i * 3])
	local max_tpm = tonumber(ARGV[9 + i * 3])

	local blocked_c, blocked_r, blocked_t = 0, 0, 0
	if max_conc > 0 and conc >= max_conc then blocked_c = 1 end
	if max_rpm > 0 and rpm >= max_rpm then blocked_r = 1 end
	-- est <= 0 skips the TPM dimension entirely: the request reserves no
	-- tokens, so the window sum must not block it.
	if max_tpm > 0 and est > 0 and tpm + est > max_tpm then blocked_t = 1 end

	-- Sliding-window retry hint: when a window dimension blocked, report
	-- when its oldest contribution leaves the window; 0 = undeterminable
	-- (client falls back to the configured hint).
	local hint = 0
	if blocked_r == 1 then hint = oldest_exit_sec(rpm_key) end
	if hint == 0 and (blocked_t == 1 or blocked_r == 1) then hint = oldest_exit_sec(usage_key) end

	-- Bottleneck headroom: the smallest remaining ratio across the enabled
	-- dimensions decides the score; unlimited (max <= 0) dimensions are
	-- excluded; an all-unlimited upstream scores +inf. Requests without a
	-- TPM estimate (est <= 0) do not feed TPM headroom into the score.
	local score = math.huge
	if max_conc > 0 then score = math.min(score, (max_conc - conc) / max_conc) end
	if max_rpm > 0 then score = math.min(score, (max_rpm - rpm) / max_rpm) end
	if max_tpm > 0 and est > 0 then score = math.min(score, (max_tpm - tpm - est) / max_tpm) end

	results[i + 1] = { blocked_c + blocked_r + blocked_t == 0 and 1 or 0, conc, rpm, tpm, blocked_c, blocked_r, blocked_t, score, hint }
end

local chosen = -1
if allow_select then
	local best = nil
	for i = 0, n - 1 do
		local r = results[i + 1]
		if r[1] == 1 and (best == nil or r[8] > best) then
			best = r[8]
			chosen = i
		end
	end
else
	if preferred >= 0 and preferred < n and results[preferred + 1][1] == 1 then
		chosen = preferred
	end
end

if chosen >= 0 then
	local conc_key = KEYS[chosen * 2 + 1]
	local lease_key = KEYS[chosen * 2 + 2]
	local prefix = string.match(conc_key, '^(.*):conc$')
	local rpm_key = prefix .. ':rpm'
	local sum_key = prefix .. ':tpm:sum'
	local usage_key = prefix .. ':tpm:usage'
	redis.call('ZADD', conc_key, now_ms + lease_ttl_sec * 1000, token)
	-- RPM is immutable per-attempt accounting: the entry ages out of the
	-- sliding window via GC, release/cleanup does not decrement it.
	redis.call('ZADD', rpm_key, now_ms, token)
	redis.call('EXPIRE', conc_key, expire_sec)
	redis.call('EXPIRE', lease_key, expire_sec)
	redis.call('EXPIRE', rpm_key, expire_sec)
	redis.call('EXPIRE', sum_key, expire_sec)
	redis.call('EXPIRE', usage_key, expire_sec)
	-- No TPM reservation for est <= 0: no sum increment and no lease
	-- metadata, so release/cleanup reclaims nothing for this lease.
	if est > 0 then
		redis.call('HSET', lease_key, token, tostring(est) .. ':' .. now_sec)
		-- Reservation contribution: lease-bound (removed by release,
		-- cleanup or the finalize correction), never age-GC'd.
		redis.call('INCRBY', sum_key, est)
	end
	-- The selected upstream's reported state includes this request's
	-- reservation (post-acquire); non-selected candidates report the
	-- pre-acquire snapshot.
	local chosen_r = results[chosen + 1]
	chosen_r[2] = chosen_r[2] + 1
	chosen_r[3] = chosen_r[3] + 1
	chosen_r[4] = chosen_r[4] + est
end

local out = { chosen >= 0 and 1 or 0, chosen }
for i = 0, n - 1 do
	local r = results[i + 1]
	out[#out + 1] = r[1]
	out[#out + 1] = r[2]
	out[#out + 1] = r[3]
	out[#out + 1] = r[4]
	out[#out + 1] = r[5]
	out[#out + 1] = r[6]
	out[#out + 1] = r[7]
	out[#out + 1] = r[9]
end
return out
`

// capacityAdmissionReleaseScript releases a lease and reclaims its TPM
// reservation from the running window sum. Ownership guard: the ZREM return
// value decides whether the reservation is touched, so release is idempotent
// and safe across replicas. The lease metadata payload is "<est>:<ws>"; the
// recorded window start is informational only under the sliding window — the
// reservation lives in the running sum until this release removes it.
//
// ARGV[2] (releaseRPM, 0/1) additionally removes the lease's own RPM window
// entry. It is set for CLIENT-CANCELED attempts: an aborted request never
// reaches the upstream's inference path, so it must not rate-lock a
// legitimate client retry for the rest of the sliding window. The entry is
// the lease's own member, so the guard keeps the removal exactly-once.
//
// KEYS: [1] "<shard>:conc", [2] "<shard>:lease", [3] "<shard>:rpm".
// ARGV: [1] token, [2] releaseRPM (0/1).
// Returns 1 when this call released the lease, 0 otherwise.
const capacityAdmissionReleaseScript = `
local token = ARGV[1]
if redis.call('ZREM', KEYS[1], token) == 1 then
	local meta = redis.call('HGET', KEYS[2], token)
	redis.call('HDEL', KEYS[2], token)
	if meta then
		local est = string.match(meta, '^(%-?%d+):')
		if est then
			local prefix = string.match(KEYS[2], '^(.*):lease$')
			local sum_key = prefix .. ':tpm:sum'
			-- The sum cannot expire while a reservation is live (renew
			-- refreshes its TTL beyond lease life); the EXISTS guard only
			-- guards against drift in pathological cases.
			if redis.call('EXISTS', sum_key) == 1 then
				redis.call('DECRBY', sum_key, tonumber(est))
			end
		end
	end
	if ARGV[2] == '1' then
		redis.call('ZREM', KEYS[3], token)
	end
	return 1
end
return 0
`

// capacityAdmissionRenewScript extends a lease only when it still exists.
// It never resurrects a lease that expired cleanup already reclaimed, so a
// long-running request whose lease was reclaimed keeps running unprotected
// instead of double-counting its reservation.
//
// Renew also refreshes the TPM running-sum AND usage-ledger TTLs: a live
// reservation must never be orphaned by the sum key expiring while its lease
// is still alive (long streams renew for hours; the sum would otherwise
// evaporate after 2 minutes of no other writes and the reservation would
// leak out of the window aggregate). Refreshing the usage key keeps its
// entries GC-able by the next request, so committed usage that is still
// inside the sliding window can never get stranded out of reach of the sum.
//
// KEYS: [1] "<shard>:conc", [2] "<shard>:lease", [3] "<shard>:tpm:sum",
// [4] "<shard>:tpm:usage". ARGV: [1] token, [2] lease TTL seconds. Returns 1
// when renewed, 0 when the lease is gone.
const capacityAdmissionRenewScript = `
local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local lease_ttl_sec = tonumber(ARGV[2])
if redis.call('ZSCORE', KEYS[1], ARGV[1]) then
	redis.call('ZADD', KEYS[1], now_ms + lease_ttl_sec * 1000, ARGV[1])
	redis.call('EXPIRE', KEYS[1], lease_ttl_sec + 60)
	redis.call('EXPIRE', KEYS[2], lease_ttl_sec + 60)
	redis.call('EXPIRE', KEYS[3], lease_ttl_sec + 60)
	redis.call('EXPIRE', KEYS[4], lease_ttl_sec + 60)
	return 1
end
return 0
`

// capacityAdmissionCommitScript adds actual token usage to the running TPM
// window sum as an age-GC'd usage contribution (immutable accounting: usage
// is never rolled back). The member encodes its token delta so the sliding
// window GC can subtract dropped contributions exactly; the lease token
// doubles as a unique member (exactly one usage-carrying finalize per lease).
//
// KEYS: [1] "<shard>:conc" (prefix/slot anchor). ARGV: [1] lease token,
// [2] actual tokens, [3] expire seconds.
const capacityAdmissionCommitScript = `
local t = redis.call('TIME')
local now_ms = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local prefix = string.match(KEYS[1], '^(.*):conc$')
local usage_key = prefix .. ':tpm:usage'
local sum_key = prefix .. ':tpm:sum'
local member = ARGV[1] .. ':' .. ARGV[2]
redis.call('ZADD', usage_key, now_ms, member)
redis.call('INCRBY', sum_key, tonumber(ARGV[2]))
redis.call('EXPIRE', usage_key, tonumber(ARGV[3]))
redis.call('EXPIRE', sum_key, tonumber(ARGV[3]))
return 1
`

// capacityAdmissionObserveScript is the read-only half of the admission
// check: it purges expired leases (crash self-healing), ages out the
// sliding-window entries, and returns the post-cleanup capacity state,
// WITHOUT acquiring anything. It backs Observe — the seed of the periodic
// capacity collector.
//
// KEYS: per upstream i: KEYS[2i+1] = "<shard>:conc", KEYS[2i+2] =
// "<shard>:lease". ARGV: [1] window seconds.
// Returns a flat array, 3 values per upstream: conc, rpm, tpm.
const capacityAdmissionObserveScript = `
local window_sec = tonumber(ARGV[1])
local t = redis.call('TIME')
local now_sec = tonumber(t[1])
local now_ms = now_sec * 1000 + math.floor(tonumber(t[2]) / 1000)
local cutoff = now_ms - window_sec * 1000

local n = math.floor(#KEYS / 2)
local out = {}
for i = 0, n - 1 do
	local conc_key = KEYS[i * 2 + 1]
	local lease_key = KEYS[i * 2 + 2]
	local prefix = string.match(conc_key, '^(.*):conc$')
	local rpm_key = prefix .. ':rpm'
	local sum_key = prefix .. ':tpm:sum'
	local usage_key = prefix .. ':tpm:usage'

	local expired = redis.call('ZRANGEBYSCORE', conc_key, '-inf', now_ms, 'LIMIT', 0, 200)
	for _, tok in ipairs(expired) do
		if redis.call('ZREM', conc_key, tok) == 1 then
			local meta = redis.call('HGET', lease_key, tok)
			if meta then
				local m_est = string.match(meta, '^(%-?%d+):')
				if m_est and redis.call('EXISTS', sum_key) == 1 then
					redis.call('DECRBY', sum_key, tonumber(m_est))
				end
				redis.call('HDEL', lease_key, tok)
			end
		end
	end

	redis.call('ZREMRANGEBYSCORE', rpm_key, '-inf', '(' .. cutoff)
	local stale = redis.call('ZRANGEBYSCORE', usage_key, '-inf', '(' .. cutoff, 'LIMIT', 0, 500)
	local dropped = 0
	for _, m in ipairs(stale) do
		local tokens = string.match(m, ':(%-?%d+)$')
		if tokens then
			dropped = dropped + tonumber(tokens)
			redis.call('ZREM', usage_key, m)
		end
	end
	if dropped > 0 and redis.call('EXISTS', sum_key) == 1 then
		redis.call('DECRBY', sum_key, dropped)
	end

	out[#out + 1] = redis.call('ZCARD', conc_key)
	out[#out + 1] = redis.call('ZCARD', rpm_key)
	out[#out + 1] = tonumber(redis.call('GET', sum_key) or '0')
end
return out
`

// CapacityAdmissionController enforces per-upstream CapacityPolicy admission.
//
// Two distinct operations:
//
//   - Check (initial admission) evaluates the router-owned candidate set,
//     selects the best feasible upstream and atomically acquires its lease:
//     "selection + occupation".
//   - Acquire (fallback) pins a specific upstream and atomically
//     check+acquires it, WITHOUT any re-selection: "specified occupation".
//     Fallback must never re-run the selection — it would risk re-selecting
//     the upstream that just failed — and must never trust the T0 snapshot
//     for the acquire decision.
type CapacityAdmissionController interface {
	// Check evaluates admission for the router-owned candidate set and
	// acquires a lease on the selected upstream. It returns nil when
	// admission does not apply (no candidate upstream has an enabled
	// CapacityPolicy). Check never returns an error: Redis failures
	// degrade according to the fail-open option.
	//
	// req.EstimatedTokens <= 0 (types.AdmissionNoTPMEstimate) means the
	// request does not participate in the TPM dimension (multimodal
	// requests): no TPM reservation, no TPM feasibility gate, and TPM
	// headroom is excluded from the selection score.
	Check(ctx context.Context, req types.CapacityAdmissionRequest) *types.AdmissionDecision
	// Acquire pins the given upstream and atomically check+acquires a lease
	// for it (no re-selection, single candidate). Returns nil when the
	// upstream has no enabled CapacityPolicy, and a reject decision when it
	// is over capacity at acquire time. estimatedTokens <= 0 means the
	// request does not participate in the TPM dimension (see Check).
	Acquire(ctx context.Context, model *types.Model, upstreamID int64, estimatedTokens int64) *types.AdmissionDecision
	// Finalize closes the lease lifecycle. usage == nil means the attempt
	// produced no usable usage: the TPM reservation is fully reclaimed and
	// nothing is committed. With usage, the actual total tokens are added
	// to the current TPM window and the reservation is reclaimed. Both
	// steps are idempotent (lease-ownership guarded).
	//
	// For leases acquired without a TPM reservation (ReservedTokens <= 0)
	// there is nothing to reclaim and the actual usage is still committed:
	// the TPM window aggregate stays the true total usage even when a
	// request did not participate in TPM admission.
	Finalize(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage)
	// FinalizeCanceled finalizes a lease whose CLIENT aborted the attempt
	// (HTTP 499 / context canceled). Identical to Finalize, plus the lease's
	// own RPM window entry is removed: an aborted request never reaches the
	// upstream's inference path, so keeping its RPM accounting would
	// rate-lock a legitimate client retry for the rest of the sliding
	// window. Server-side failures keep the immutable RPM accounting.
	FinalizeCanceled(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage)
	// Renew extends the lease if it still exists in Redis. It returns false
	// when the lease is gone (never resurrects it). Redis errors return
	// true so transient outages do not unregister live leases.
	Renew(ctx context.Context, lease *types.AdmissionLease) bool
	// Observe runs the expired-lease cleanup and returns the post-cleanup
	// capacity snapshot (conc/rpm/tpm per upstream) for the given upstreams
	// of one model — the read-only half of Check. It never acquires
	// anything and is the foundation for a periodic capacity collector
	// (freshness independent of request traffic).
	Observe(ctx context.Context, modelID string, upstreamIDs []int64) ([]types.UpstreamCapacityState, error)
	// EstimateAdmissionTokens returns the TPM reservation estimate for a
	// prompt text (cheap heuristic, no tokenizer calls).
	EstimateAdmissionTokens(promptText string) int64
}

// CapacityAdmissionOptions configures the admission controller.
type CapacityAdmissionOptions struct {
	LeaseTTL                time.Duration
	FailOpen                bool
	PromptCharsPerToken     int
	CompletionTokenEstimate int64
	MaxTokenEstimate        int64
	RetryAfterHint          time.Duration
	// RenewInterval is the lease renewal period; <= 0 disables the renewer
	// (tests construct leases with manual renewal).
	RenewInterval time.Duration
}

const (
	defaultAdmissionLeaseTTL    = 60 * time.Second
	defaultAdmissionCharsPerTok = 4
	defaultAdmissionCompletion  = int64(1000)
	defaultAdmissionMaxEstimate = int64(32768)
	defaultAdmissionRetryHint   = 5 * time.Second
)

// CapacityAdmissionOptionsFromConfig builds options from the
// AIGateway.CapacityAdmission config section.
func CapacityAdmissionOptionsFromConfig(cfg *config.Config) CapacityAdmissionOptions {
	opts := CapacityAdmissionOptions{
		LeaseTTL:                defaultAdmissionLeaseTTL,
		FailOpen:                true,
		PromptCharsPerToken:     defaultAdmissionCharsPerTok,
		CompletionTokenEstimate: defaultAdmissionCompletion,
		MaxTokenEstimate:        defaultAdmissionMaxEstimate,
		RetryAfterHint:          defaultAdmissionRetryHint,
	}
	if cfg == nil {
		return opts
	}
	a := cfg.AIGateway.CapacityAdmission
	if a.LeaseTTLSeconds > 0 {
		opts.LeaseTTL = time.Duration(a.LeaseTTLSeconds) * time.Second
	}
	opts.FailOpen = a.FailOpen
	if a.EstimatePromptCharsPerToken > 0 {
		opts.PromptCharsPerToken = a.EstimatePromptCharsPerToken
	}
	if a.EstimateCompletionTokens > 0 {
		opts.CompletionTokenEstimate = a.EstimateCompletionTokens
	}
	if a.EstimateMaxTokens > 0 {
		opts.MaxTokenEstimate = a.EstimateMaxTokens
	}
	if a.RetryAfterHintSeconds > 0 {
		opts.RetryAfterHint = time.Duration(a.RetryAfterHintSeconds) * time.Second
	}
	// Renew at TTL/3 so a lease survives transient renewal failures while a
	// crashed replica's leases are reclaimed within TTL.
	opts.RenewInterval = opts.LeaseTTL / 3
	return opts
}

type capacityAdmissionImpl struct {
	redisClient cache.RedisClient
	opts        CapacityAdmissionOptions
	renewer     *leaseRenewer

	// warnedTPMTooSmall avoids repeating the misconfiguration warning for
	// the same model:upstream.
	warnedTPMTooSmall sync.Map
}

// NewCapacityAdmissionController builds the Redis-backed admission
// controller. A nil redisClient is allowed: every operation degrades to
// fail-open so a missing cache dependency never blocks requests.
func NewCapacityAdmissionController(redisClient cache.RedisClient, opts CapacityAdmissionOptions) CapacityAdmissionController {
	if opts.LeaseTTL <= 0 {
		opts.LeaseTTL = defaultAdmissionLeaseTTL
	}
	if opts.PromptCharsPerToken <= 0 {
		opts.PromptCharsPerToken = defaultAdmissionCharsPerTok
	}
	if opts.CompletionTokenEstimate <= 0 {
		opts.CompletionTokenEstimate = defaultAdmissionCompletion
	}
	if opts.MaxTokenEstimate <= 0 {
		opts.MaxTokenEstimate = defaultAdmissionMaxEstimate
	}
	if opts.RetryAfterHint <= 0 {
		opts.RetryAfterHint = defaultAdmissionRetryHint
	}
	// RenewInterval <= 0 keeps the background renewer disabled (opt-in via
	// CapacityAdmissionOptionsFromConfig; tests construct without renewal).
	impl := &capacityAdmissionImpl{
		redisClient: redisClient,
		opts:        opts,
	}
	impl.renewer = newLeaseRenewer(impl.Renew, opts.RenewInterval)
	impl.preloadScripts()
	return impl
}

// preloadScripts pushes the admission Lua scripts into the Redis script
// cache (SCRIPT LOAD) at controller construction, so every later RunScript
// call hits EVALSHA directly instead of paying the first-call NOSCRIPT
// fallback per script. Preloading is best-effort and panic-guarded: any
// failure only delays the savings (RunScript falls back to EVAL + LOAD
// automatically), never blocks construction, and Redis re-loads scripts
// transparently after a restart.
func (c *capacityAdmissionImpl) preloadScripts() {
	defer func() {
		// Construction must survive an unreachable or mocking Redis client.
		if rec := recover(); rec != nil {
			slog.Warn("capacity admission script preload panicked; RunScript will self-heal",
				slog.Any("panic", rec))
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, script := range []string{
		capacityAdmissionScript,
		capacityAdmissionReleaseScript,
		capacityAdmissionRenewScript,
		capacityAdmissionCommitScript,
		capacityAdmissionObserveScript,
	} {
		if err := c.redisClient.LoadScript(ctx, script); err != nil {
			slog.Warn("capacity admission script preload failed; RunScript will self-heal",
				slog.Any("error", err))
			return
		}
	}
	slog.Info("capacity admission lua scripts preloaded")
}

// admissionCandidate is one policy-enabled upstream in the candidate set.
type admissionCandidate struct {
	upstream commontypes.UpstreamConfig
	policy   commontypes.CapacityPolicy
}

func (c *capacityAdmissionImpl) Check(ctx context.Context, req types.CapacityAdmissionRequest) *types.AdmissionDecision {
	if c.redisClient == nil {
		// Misconfiguration (nil Redis client) degrades exactly like a Redis
		// outage, per the fail-open option.
		return c.degenerateDecision(req)
	}
	candidates := admissionCandidates(req.Model, req.PreferredUpstreamID, req.AllowSelect)
	if len(candidates) == 0 {
		return nil
	}
	started := time.Now()
	decision := c.evaluate(ctx, req, candidates)
	c.recordMetrics(req.Model, decision, time.Since(started))
	return decision
}

// degenerateDecision builds the decision for a nil Redis client
// (misconfiguration): fail-open admits without a lease, fail-close rejects.
func (c *capacityAdmissionImpl) degenerateDecision(req types.CapacityAdmissionRequest) *types.AdmissionDecision {
	if !c.opts.FailOpen {
		return c.rejectDecision(nil, req, types.AdmissionReasonCapacityExceeded)
	}
	return &types.AdmissionDecision{Action: types.AdmissionAdmit, FailOpen: true}
}

// Acquire implements the fallback operation: pin the given upstream and
// atomically check+acquire its lease. Re-selection is impossible by
// construction (single candidate), so a failed upstream can never be picked
// again and the T0 candidate snapshot is never used as acquire authority.
func (c *capacityAdmissionImpl) Acquire(ctx context.Context, model *types.Model, upstreamID int64, estimatedTokens int64) *types.AdmissionDecision {
	return c.Check(ctx, types.CapacityAdmissionRequest{
		Model:               model,
		PreferredUpstreamID: upstreamID,
		AllowSelect:         false,
		EstimatedTokens:     estimatedTokens,
	})
}

// admissionCandidates reduces the router-owned candidate set to the
// policy-enabled upstreams. When allowSelect is false (pinned or
// session-affinity requests) only the preferred upstream is evaluated;
// if it has no enabled policy, admission does not apply at all.
func admissionCandidates(model *types.Model, preferredID int64, allowSelect bool) []admissionCandidate {
	if model == nil {
		return nil
	}
	candidates := make([]admissionCandidate, 0, len(model.Upstreams))
	for _, up := range model.Upstreams {
		if up.CapacityPolicy == nil || !up.CapacityPolicy.Enabled {
			continue
		}
		candidates = append(candidates, admissionCandidate{upstream: up, policy: *up.CapacityPolicy})
	}
	if len(candidates) == 0 {
		return nil
	}
	if !allowSelect {
		for _, cand := range candidates {
			if cand.upstream.ID == preferredID {
				return []admissionCandidate{cand}
			}
		}
		return nil
	}
	return candidates
}

func (c *capacityAdmissionImpl) evaluate(ctx context.Context, req types.CapacityAdmissionRequest, candidates []admissionCandidate) *types.AdmissionDecision {
	c.warnSmallTPM(req.Model, candidates)

	tokenValue := uuid.NewString()
	keys := make([]string, 0, len(candidates)*2)
	args := make([]any, 0, 6+len(candidates)*3)
	preferredIdx := -1
	for i, cand := range candidates {
		keys = append(keys,
			capacityConcurrencyKey(req.Model.ID, cand.upstream.ID),
			capacityLeaseKey(req.Model.ID, cand.upstream.ID),
		)
		if cand.upstream.ID == req.PreferredUpstreamID {
			preferredIdx = i
		}
	}
	args = append(args,
		capacityWindowSeconds,
		int64(c.opts.LeaseTTL/time.Second),
		tokenValue,
		preferredIdx,
		boolToInt(req.AllowSelect),
		req.EstimatedTokens,
	)
	for _, cand := range candidates {
		args = append(args, cand.policy.MaxConcurrency, cand.policy.MaxRPM, cand.policy.MaxTPM)
	}

	startedAt := time.Now()
	result, err := c.redisClient.RunScript(ctx, capacityAdmissionScript, keys, args...)
	runScriptLatencyMS := float64(time.Since(startedAt).Microseconds()) / 1000.0
	if err != nil {
		incrAdmissionRedisError("check")
		if !c.opts.FailOpen {
			slog.ErrorContext(ctx, "capacity admission check failed, failing closed",
				slog.Any("error", err), slog.String("model", req.Model.ID),
				slog.Float64("run_script_latency_ms", runScriptLatencyMS),
				slog.Int("candidates", len(candidates)))
			return c.rejectDecision(candidates, req, "capacity_exceeded")
		}
		slog.WarnContext(ctx, "capacity admission check failed, failing open",
			slog.Any("error", err), slog.String("model", req.Model.ID),
			slog.Float64("run_script_latency_ms", runScriptLatencyMS),
			slog.Int("candidates", len(candidates)))
		return &types.AdmissionDecision{Action: types.AdmissionAdmit, FailOpen: true}
	}

	vals, ok := result.([]any)
	if !ok || len(vals) < 2 || len(vals) < 2+len(candidates)*8 {
		incrAdmissionRedisError("check")
		slog.WarnContext(ctx, "capacity admission script returned unexpected result, failing open",
			slog.Any("result", result), slog.String("model", req.Model.ID),
			slog.Float64("run_script_latency_ms", runScriptLatencyMS),
			slog.Int("candidates", len(candidates)))
		if !c.opts.FailOpen {
			return c.rejectDecision(candidates, req, "capacity_exceeded")
		}
		return &types.AdmissionDecision{Action: types.AdmissionAdmit, FailOpen: true}
	}

	states, hints := parseAdmissionStates(candidates, vals)
	// Piggyback capacity-state reporting on the request path: gauges and
	// blocked counters refresh at admission time, zero extra Redis round
	// trips. A periodic Observe-driven collector covers the traffic-less
	// freshness gaps.
	ReportCapacityStates(req.Model.ID, states)
	ReportCapacityBlocked(req.Model.ID, states)
	admitted, _ := scriptResultToInt64(vals[0])
	chosenIdx, _ := scriptResultToInt64(vals[1])
	if admitted != 1 || chosenIdx < 0 || chosenIdx >= int64(len(candidates)) {
		reason, retryAfter := c.rejectReason(states, hints)
		slog.InfoContext(ctx, "capacity admission rejected",
			slog.String("model", req.Model.ID),
			slog.String("reason", reason),
			slog.Int64("retry_after_hint", retryAfter),
			slog.Float64("run_script_latency_ms", runScriptLatencyMS),
			slog.Int("candidates", len(candidates)))
		return &types.AdmissionDecision{
			Action:            types.AdmissionReject,
			Reason:            reason,
			RetryAfterSeconds: retryAfter,
			States:            states,
		}
	}

	chosen := candidates[chosenIdx]
	// The script reported the selected upstream's state post-acquire; mark
	// it so consumers can tell pre-acquire snapshots apart.
	states[chosenIdx].Selected = true
	lease := &types.AdmissionLease{
		ModelID:        req.Model.ID,
		UpstreamID:     chosen.upstream.ID,
		Token:          tokenValue,
		ReservedTokens: req.EstimatedTokens,
	}
	c.renewer.Register(lease)
	slog.InfoContext(ctx, "capacity admission lease acquired",
		slog.String("model", req.Model.ID),
		slog.Int64("upstream_id", lease.UpstreamID),
		slog.Int64("reserved_tokens", lease.ReservedTokens),
		slog.Bool("reselected", lease.UpstreamID != req.PreferredUpstreamID),
		slog.Float64("run_script_latency_ms", runScriptLatencyMS),
		slog.Int("candidates", len(candidates)),
		leaseTokenAttr(lease))
	return &types.AdmissionDecision{
		Action:             types.AdmissionAdmit,
		Lease:              lease,
		SelectedUpstreamID: chosen.upstream.ID,
		ReSelected:         chosen.upstream.ID != req.PreferredUpstreamID,
		States:             states,
	}
}

// rejectDecision builds a reject decision for fail-close mode where no
// script state is available.
func (c *capacityAdmissionImpl) rejectDecision(candidates []admissionCandidate, req types.CapacityAdmissionRequest, reason string) *types.AdmissionDecision {
	states := make([]types.UpstreamCapacityState, 0, len(candidates))
	for _, cand := range candidates {
		states = append(states, types.UpstreamCapacityState{
			UpstreamID:     cand.upstream.ID,
			MaxConcurrency: cand.policy.MaxConcurrency,
			MaxRPM:         cand.policy.MaxRPM,
			MaxTPM:         cand.policy.MaxTPM,
		})
	}
	return &types.AdmissionDecision{
		Action:            types.AdmissionReject,
		Reason:            reason,
		RetryAfterSeconds: c.retryAfterHintSeconds(),
		States:            states,
	}
}

// rejectReason aggregates the blocked dimensions across candidates: a single
// blocked dimension maps to its specific reason, multiple dimensions map to
// capacity_exceeded. Retry-After is a hint: under the sliding window there is
// no fixed boundary, so window-based reasons report when the oldest blocked
// contribution leaves the window (computed by the script; 0 = undeterminable,
// falling back to the configured hint), concurrency returns the configured
// hint.
func (c *capacityAdmissionImpl) rejectReason(states []types.UpstreamCapacityState, hints []int64) (string, int64) {
	seen := make(map[string]struct{})
	minHint := int64(0)
	for i, s := range states {
		for _, dim := range s.BlockedBy {
			seen[dim] = struct{}{}
		}
		if i < len(hints) && hints[i] > 0 && (minHint == 0 || hints[i] < minHint) {
			minHint = hints[i]
		}
	}
	var reason string
	switch len(seen) {
	case 0:
		reason = types.AdmissionReasonCapacityExceeded
	case 1:
		for dim := range seen {
			reason = admissionReasonForDimension(dim)
		}
	default:
		reason = types.AdmissionReasonCapacityExceeded
	}

	windowBased := false
	for dim := range seen {
		if dim == "rpm" || dim == "tpm" {
			windowBased = true
		}
	}
	if windowBased && minHint > 0 {
		return reason, minHint
	}
	return reason, c.retryAfterHintSeconds()
}

func (c *capacityAdmissionImpl) retryAfterHintSeconds() int64 {
	hint := c.opts.RetryAfterHint
	if hint > c.opts.LeaseTTL {
		hint = c.opts.LeaseTTL
	}
	seconds := int64(hint / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func admissionReasonForDimension(dim string) string {
	switch dim {
	case "concurrency":
		return types.AdmissionReasonConcurrencyExceeded
	case "rpm":
		return types.AdmissionReasonRPMExceeded
	case "tpm":
		return types.AdmissionReasonTPMExceeded
	default:
		return types.AdmissionReasonCapacityExceeded
	}
}

// parseAdmissionStates parses the flat script result into per-candidate
// capacity states; the script returns 8 values per candidate (feasible, conc,
// rpm, tpm, blockedConc, blockedRPM, blockedTPM, retryHint). The retry hint
// is returned separately for rejectReason.
func parseAdmissionStates(candidates []admissionCandidate, vals []any) ([]types.UpstreamCapacityState, []int64) {
	states := make([]types.UpstreamCapacityState, 0, len(candidates))
	hints := make([]int64, 0, len(candidates))
	for i, cand := range candidates {
		base := 2 + i*8
		feasible, _ := scriptResultToInt64(vals[base])
		conc, _ := scriptResultToInt64(vals[base+1])
		rpm, _ := scriptResultToInt64(vals[base+2])
		tpm, _ := scriptResultToInt64(vals[base+3])
		blockedC, _ := scriptResultToInt64(vals[base+4])
		blockedR, _ := scriptResultToInt64(vals[base+5])
		blockedT, _ := scriptResultToInt64(vals[base+6])
		hint, _ := scriptResultToInt64(vals[base+7])
		state := types.UpstreamCapacityState{
			UpstreamID:         cand.upstream.ID,
			CurrentConcurrency: conc,
			CurrentRPM:         rpm,
			CurrentTPM:         tpm,
			MaxConcurrency:     cand.policy.MaxConcurrency,
			MaxRPM:             cand.policy.MaxRPM,
			MaxTPM:             cand.policy.MaxTPM,
			Feasible:           feasible == 1,
		}
		if blockedC == 1 {
			state.BlockedBy = append(state.BlockedBy, "concurrency")
		}
		if blockedR == 1 {
			state.BlockedBy = append(state.BlockedBy, "rpm")
		}
		if blockedT == 1 {
			state.BlockedBy = append(state.BlockedBy, "tpm")
		}
		states = append(states, state)
		hints = append(hints, hint)
	}
	return states, hints
}

func (c *capacityAdmissionImpl) Finalize(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage) {
	if lease == nil || lease.Token == "" || c.redisClient == nil {
		return
	}
	c.finalizeWithFlags(ctx, lease, usage, false)
}

// FinalizeCanceled implements the client-canceled finalize: same as Finalize
// plus the lease's own RPM window entry is removed (see the interface doc).
func (c *capacityAdmissionImpl) FinalizeCanceled(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage) {
	if lease == nil || lease.Token == "" || c.redisClient == nil {
		return
	}
	c.finalizeWithFlags(ctx, lease, usage, true)
}

func (c *capacityAdmissionImpl) finalizeWithFlags(ctx context.Context, lease *types.AdmissionLease, usage *token.Usage, clientCanceled bool) {
	if lease == nil || lease.Token == "" || c.redisClient == nil {
		return
	}
	c.renewer.Unregister(lease.Token)

	concKey := capacityConcurrencyKey(lease.ModelID, lease.UpstreamID)
	if usage != nil && usage.TotalTokens > 0 {
		// Actual usage lands in the running window sum as an age-GC'd
		// contribution (immutable accounting). The reservation is reclaimed
		// separately by the release below.
		expireSec := int64(c.opts.LeaseTTL/time.Second) + capacityTTLBufferSecond
		if _, err := c.redisClient.RunScript(ctx, capacityAdmissionCommitScript, []string{concKey}, lease.Token, usage.TotalTokens, expireSec); err != nil {
			incrAdmissionRedisError("finalize")
			slog.WarnContext(ctx, "capacity admission commit failed", slog.Any("error", err),
				slog.String("model", lease.ModelID), slog.Int64("upstream_id", lease.UpstreamID),
				leaseTokenAttr(lease))
		}
	}
	leaseKey := capacityLeaseKey(lease.ModelID, lease.UpstreamID)
	releaseRPM := int64(0)
	if clientCanceled {
		releaseRPM = 1
	}
	releaseKeys := []string{concKey, leaseKey, capacityRPMKey(lease.ModelID, lease.UpstreamID)}
	if _, err := c.redisClient.RunScript(ctx, capacityAdmissionReleaseScript, releaseKeys, lease.Token, releaseRPM); err != nil {
		incrAdmissionRedisError("finalize")
		slog.WarnContext(ctx, "capacity admission release failed", slog.Any("error", err),
			slog.String("model", lease.ModelID), slog.Int64("upstream_id", lease.UpstreamID),
			leaseTokenAttr(lease))
	}
}

func (c *capacityAdmissionImpl) Renew(ctx context.Context, lease *types.AdmissionLease) bool {
	if lease == nil || lease.Token == "" || c.redisClient == nil {
		return false
	}
	keys := []string{
		capacityConcurrencyKey(lease.ModelID, lease.UpstreamID),
		capacityLeaseKey(lease.ModelID, lease.UpstreamID),
		capacityTPMSumKey(lease.ModelID, lease.UpstreamID),
		capacityTPMUsageKey(lease.ModelID, lease.UpstreamID),
	}
	result, err := c.redisClient.RunScript(ctx, capacityAdmissionRenewScript, keys, lease.Token, int64(c.opts.LeaseTTL/time.Second))
	if err != nil {
		incrAdmissionRedisError("renew")
		// Transient Redis error: keep the lease registered; the next tick
		// retries and the expired-lease cleanup is the safety net.
		return true
	}
	alive, _ := scriptResultToInt64(result)
	return alive == 1
}

// Observe implements the read-only half of Check: expired-lease cleanup plus
// a state snapshot for the given upstreams, never acquiring anything. It is
// the entry point for a periodic capacity collector whose freshness does not
// depend on request traffic.
func (c *capacityAdmissionImpl) Observe(ctx context.Context, modelID string, upstreamIDs []int64) ([]types.UpstreamCapacityState, error) {
	if len(upstreamIDs) == 0 || c.redisClient == nil {
		return nil, nil
	}
	keys := make([]string, 0, len(upstreamIDs)*2)
	for _, id := range upstreamIDs {
		keys = append(keys, capacityConcurrencyKey(modelID, id), capacityLeaseKey(modelID, id))
	}
	result, err := c.redisClient.RunScript(ctx, capacityAdmissionObserveScript, keys, capacityWindowSeconds)
	if err != nil {
		incrAdmissionRedisError("observe")
		return nil, err
	}
	vals, ok := result.([]any)
	if !ok || len(vals) < len(upstreamIDs)*3 {
		return nil, fmt.Errorf("capacity observe script returned unexpected result: %v", result)
	}
	states := make([]types.UpstreamCapacityState, 0, len(upstreamIDs))
	for i, id := range upstreamIDs {
		conc, _ := scriptResultToInt64(vals[i*3])
		rpm, _ := scriptResultToInt64(vals[i*3+1])
		tpm, _ := scriptResultToInt64(vals[i*3+2])
		states = append(states, types.UpstreamCapacityState{
			UpstreamID:         id,
			CurrentConcurrency: conc,
			CurrentRPM:         rpm,
			CurrentTPM:         tpm,
		})
	}
	return states, nil
}

// EstimateAdmissionTokens implements the cheap TPM pre-reservation estimate:
// ceil(prompt runes / charsPerToken) + completion reserve, floored at
// completion reserve + min floor and capped at MaxTokenEstimate so an
// oversized prompt can neither overflow the arithmetic nor monopolize a TPM
// window. Multimodal content is only covered by the floor/completion part —
// a known soft-limit boundary.
func (c *capacityAdmissionImpl) EstimateAdmissionTokens(promptText string) int64 {
	promptEst := int64(0)
	if c.opts.PromptCharsPerToken > 0 {
		// Cap the rune count before arithmetic: a prompt that would exceed
		// the max estimate on its own contributes the cap, nothing more.
		maxPromptRunes := c.opts.MaxTokenEstimate * int64(c.opts.PromptCharsPerToken)
		runes := int64(utf8.RuneCountInString(promptText))
		if runes > maxPromptRunes {
			runes = maxPromptRunes
		}
		promptEst = (runes + int64(c.opts.PromptCharsPerToken) - 1) / int64(c.opts.PromptCharsPerToken)
	}
	est := promptEst + c.opts.CompletionTokenEstimate
	if minEst := c.opts.CompletionTokenEstimate + admissionEstimateMinFloor; est < minEst {
		est = minEst
	}
	if est > c.opts.MaxTokenEstimate {
		est = c.opts.MaxTokenEstimate
	}
	return est
}

// warnSmallTPM logs once per model:upstream when MaxTPM is below the minimum
// per-request reservation, which would reject every request.
func (c *capacityAdmissionImpl) warnSmallTPM(model *types.Model, candidates []admissionCandidate) {
	minEst := c.opts.CompletionTokenEstimate + admissionEstimateMinFloor
	for _, cand := range candidates {
		if cand.policy.MaxTPM <= 0 || cand.policy.MaxTPM >= minEst {
			continue
		}
		key := fmt.Sprintf("%s:%d", model.ID, cand.upstream.ID)
		if _, loaded := c.warnedTPMTooSmall.LoadOrStore(key, struct{}{}); loaded {
			continue
		}
		slog.Warn("capacity policy MaxTPM is smaller than the minimum per-request reservation; requests may always be rejected",
			slog.String("model", model.ID), slog.Int64("upstream_id", cand.upstream.ID),
			slog.Int64("max_tpm", cand.policy.MaxTPM), slog.Int64("min_estimate", minEst))
	}
}

func (c *capacityAdmissionImpl) recordMetrics(model *types.Model, decision *types.AdmissionDecision, latency time.Duration) {
	if decision == nil || model == nil {
		return
	}
	decisionLabel := string(decision.Action)
	if decision.FailOpen {
		decisionLabel = "fail_open"
	}
	provider := model.OwnedBy
	if model.Provider != "" {
		provider = model.Provider
	}
	incrAdmissionDecision(decisionLabel, decision.Reason, model.ID, provider)
	observeAdmissionLatency(model.ID, float64(latency.Milliseconds()))
}

func capacityConcurrencyKey(modelID string, upstreamID int64) string {
	return fmt.Sprintf("%s:{m:%s}:u:%d:conc", capacityKeyPrefix, modelID, upstreamID)
}

func capacityLeaseKey(modelID string, upstreamID int64) string {
	return fmt.Sprintf("%s:{m:%s}:u:%d:lease", capacityKeyPrefix, modelID, upstreamID)
}

func capacityRPMKey(modelID string, upstreamID int64) string {
	return fmt.Sprintf("%s:{m:%s}:u:%d:rpm", capacityKeyPrefix, modelID, upstreamID)
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// leaseRenewer keeps active leases alive: one background goroutine per pod
// renews every registered lease every interval (lease TTL / 3), so a
// long-running request never expires its own lease while a crashed replica
// stops renewing and its leases are reclaimed within TTL. interval <= 0
// disables the background loop (manual renewAll for tests).
type leaseRenewer struct {
	renewFn  func(ctx context.Context, lease *types.AdmissionLease) bool
	interval time.Duration

	mu     sync.Mutex
	leases map[string]types.AdmissionLease
	start  sync.Once
	stop   chan struct{}
}

func newLeaseRenewer(renewFn func(ctx context.Context, lease *types.AdmissionLease) bool, interval time.Duration) *leaseRenewer {
	return &leaseRenewer{
		renewFn:  renewFn,
		interval: interval,
		leases:   make(map[string]types.AdmissionLease),
		stop:     make(chan struct{}),
	}
}

func (r *leaseRenewer) Register(lease *types.AdmissionLease) {
	if lease == nil || lease.Token == "" || r.interval <= 0 {
		return
	}
	r.mu.Lock()
	r.leases[lease.Token] = *lease
	r.mu.Unlock()
	r.start.Do(func() {
		go r.loop()
	})
}

func (r *leaseRenewer) Unregister(token string) {
	r.mu.Lock()
	delete(r.leases, token)
	r.mu.Unlock()
}

func (r *leaseRenewer) loop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.renewAll(context.Background())
		}
	}
}

func (r *leaseRenewer) renewAll(ctx context.Context) {
	r.mu.Lock()
	leases := make([]types.AdmissionLease, 0, len(r.leases))
	for _, l := range r.leases {
		leases = append(leases, l)
	}
	r.mu.Unlock()
	for i := range leases {
		func() {
			defer func() {
				// A renewal panic must never take down the renew loop.
				if rec := recover(); rec != nil {
					slog.Error("panic in capacity admission lease renewal", slog.Any("panic", rec))
				}
			}()
			if !r.renewFn(ctx, &leases[i]) {
				r.Unregister(leases[i].Token)
			}
		}()
	}
}

// Stop terminates the background renewal loop (used by tests).
func (r *leaseRenewer) Stop() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
}

// leaseTokenAttr returns the log attribute carrying the admission lease
// token (nil-safe). Finalize/renew run in detached contexts (the trace may
// be absent), so the token is the correlation key for these lines.
func leaseTokenAttr(lease *types.AdmissionLease) slog.Attr {
	if lease == nil {
		return slog.String("admission_token", "")
	}
	return slog.String("admission_token", lease.Token)
}

// Metric helpers tolerate an uninitialized prometheus registry (InitMetrics
// runs at service startup; unit tests may not register metrics).
func incrAdmissionRedisError(operation string) {
	if prom.AIGatewayAdmissionRedisErrors != nil {
		prom.AIGatewayAdmissionRedisErrors.WithLabelValues(operation).Inc()
	}
}

func incrAdmissionDecision(decision, reason, model, provider string) {
	if prom.AIGatewayAdmissionTotal != nil {
		prom.AIGatewayAdmissionTotal.WithLabelValues(decision, reason, model, provider).Inc()
	}
}

func observeAdmissionLatency(model string, ms float64) {
	if prom.AIGatewayAdmissionDecisionLatency != nil {
		prom.AIGatewayAdmissionDecisionLatency.WithLabelValues(model).Observe(ms)
	}
}

// scriptResultToInt64 converts a Redis Lua script result into an int64.
func scriptResultToInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected script result type %T", value)
	}
}

// capacityKeyNames groups the per-upstream Redis key suffixes derived inside
// the Lua scripts from the ":conc" key prefix.
//   - ":rpm"         ZSET, one entry per acquire (score = acquire ms); ages
//     out of the sliding window via GC, never decremented on release.
//   - ":tpm:sum"     STRING, running sliding-window total (reservations +
//     live usage contributions). Refreshed EXPIRE on every write and on renew.
//   - ":tpm:usage"   ZSET, one entry per committed usage (member encodes its
//     token delta, score = commit ms); age-GC'd out of the window by the
//     admit/observe scripts, which subtract dropped tokens from the sum.
func capacityTPMSumKey(modelID string, upstreamID int64) string {
	return fmt.Sprintf("%s:{m:%s}:u:%d:tpm:sum", capacityKeyPrefix, modelID, upstreamID)
}

func capacityTPMUsageKey(modelID string, upstreamID int64) string {
	return fmt.Sprintf("%s:{m:%s}:u:%d:tpm:usage", capacityKeyPrefix, modelID, upstreamID)
}
