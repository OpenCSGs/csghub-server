package router

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	commontypes "opencsg.com/csghub-server/common/types"
)

const (
	RoutingStrategySingle      = "single"
	RoutingStrategyRoundRobin  = "round_robin"
	RoutingStrategySessionHash = "session_hash"

	defaultHashReplicas = 64
)

type SessionRouter interface {
	PickUpstream(modelKey string, sessionKey string, upstreams []commontypes.UpstreamConfig, policy commontypes.RoutingPolicy) (commontypes.UpstreamConfig, error)
}

type ringNode struct {
	hash     uint32
	upstream commontypes.UpstreamConfig
}

type sessionRouterImpl struct {
	rrCounters sync.Map // map[string]*atomic.Uint64
}

func NewSessionRouter() SessionRouter {
	return &sessionRouterImpl{}
}

func (r *sessionRouterImpl) PickUpstream(modelKey string, sessionKey string, enabledUpstreams []commontypes.UpstreamConfig, policy commontypes.RoutingPolicy) (commontypes.UpstreamConfig, error) {
	if len(enabledUpstreams) == 0 {
		return commontypes.UpstreamConfig{}, fmt.Errorf("no enabled upstream")
	}

	// Channel priority policy: candidates arrive priority-eligible and sorted
	// (EligibleUpstreamsForPriority). Restrict every strategy to the highest
	// priority tier present so non-low-priority callers route to the
	// high-priority channel whenever one is available; lower tiers stay in
	// the caller's fallback list for failed attempts. Session affinity is
	// untouched — it decides within the tier's candidates.
	candidates := TopPriorityTier(enabledUpstreams)

	strategy := strings.TrimSpace(policy.Strategy)
	if strategy == "" {
		if len(candidates) == 1 {
			strategy = RoutingStrategySingle
		} else if sessionKey != "" {
			strategy = RoutingStrategySessionHash
		} else {
			strategy = RoutingStrategySingle
		}
	}

	switch strategy {
	case RoutingStrategySingle:
		return candidates[0], nil
	case RoutingStrategyRoundRobin:
		return pickByRoundRobin(r.rrCounter(buildUpstreamsKey(candidates)), candidates), nil
	case RoutingStrategySessionHash:
		if sessionKey == "" {
			return candidates[0], nil
		}
		return r.pickByConsistentHash(modelKey, sessionKey, candidates, policy.HashReplicas), nil
	default:
		return candidates[0], nil
	}
}

// TopPriorityTier narrows the candidates to the highest priority value
// present in the list. Order-independent: it keeps every candidate sharing
// the maximum priority, preserving the incoming order within the tier.
func TopPriorityTier(upstreams []commontypes.UpstreamConfig) []commontypes.UpstreamConfig {
	if len(upstreams) <= 1 {
		return upstreams
	}
	top := upstreams[0].Priority
	for _, us := range upstreams[1:] {
		if us.Priority > top {
			top = us.Priority
		}
	}
	tier := make([]commontypes.UpstreamConfig, 0, len(upstreams))
	for _, us := range upstreams {
		if us.Priority == top {
			tier = append(tier, us)
		}
	}
	return tier
}

func normalizeEnabledUpstreams(upstreams []commontypes.UpstreamConfig) []commontypes.UpstreamConfig {
	filtered := make([]commontypes.UpstreamConfig, 0, len(upstreams))
	for _, us := range upstreams {
		if strings.TrimSpace(us.URL) == "" || strings.TrimSpace(us.ModelName) == "" || !us.Enabled {
			continue
		}
		if us.Weight <= 0 {
			us.Weight = 1
		}
		filtered = append(filtered, us)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].URL < filtered[j].URL
	})
	return filtered
}

// NormalizeEnabledUpstreams filters malformed/disabled upstreams and applies default weight.
func NormalizeEnabledUpstreams(upstreams []commontypes.UpstreamConfig) []commontypes.UpstreamConfig {
	return normalizeEnabledUpstreams(upstreams)
}

// EligibleUpstreamsForPriority enforces the caller's upstream channel
// priority: candidates whose priority exceeds the caller's are dropped
// (a low-priority user must never reach a higher-priority upstream), and the
// survivors are stable-sorted by descending priority so higher-priority
// channels lead the list handed to the session router (which keeps its
// affinity semantics — order only breaks ties for single/round-robin
// strategies and fallback attempt ordering). Ties keep their incoming
// (URL-normalized) order, so the result is deterministic.
func EligibleUpstreamsForPriority(upstreams []commontypes.UpstreamConfig, userPriority int) []commontypes.UpstreamConfig {
	eligible := make([]commontypes.UpstreamConfig, 0, len(upstreams))
	for _, us := range upstreams {
		if us.Priority <= userPriority {
			eligible = append(eligible, us)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		return eligible[i].Priority > eligible[j].Priority
	})
	return eligible
}

func pickByRoundRobin(counter *atomic.Uint64, upstreams []commontypes.UpstreamConfig) commontypes.UpstreamConfig {
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	sequence := counter.Add(1)
	return upstreams[(sequence-1)%uint64(len(upstreams))]
}

func (r *sessionRouterImpl) pickByConsistentHash(modelKey string, sessionKey string, upstreams []commontypes.UpstreamConfig, replicas int) commontypes.UpstreamConfig {
	if len(upstreams) == 1 {
		return upstreams[0]
	}
	if replicas <= 0 {
		replicas = defaultHashReplicas
	}

	nodes := buildRing(upstreams, replicas)
	routeKey := fmt.Sprintf("%s:%s", modelKey, sessionKey)
	targetHash := crc32.ChecksumIEEE([]byte(routeKey))
	index := sort.Search(len(nodes), func(i int) bool {
		return nodes[i].hash >= targetHash
	})
	if index == len(nodes) {
		index = 0
	}
	return nodes[index].upstream
}

func buildUpstreamsKey(upstreams []commontypes.UpstreamConfig) string {
	upstreamURLs := make([]string, 0, len(upstreams))
	for _, us := range upstreams {
		upstreamURLs = append(upstreamURLs, us.URL)
	}
	return strings.Join(upstreamURLs, "|")
}

func (r *sessionRouterImpl) rrCounter(key string) *atomic.Uint64 {
	if existing, ok := r.rrCounters.Load(key); ok {
		return existing.(*atomic.Uint64)
	}
	counter := &atomic.Uint64{}
	actual, _ := r.rrCounters.LoadOrStore(key, counter)
	return actual.(*atomic.Uint64)
}

func buildRing(upstreams []commontypes.UpstreamConfig, replicas int) []ringNode {
	nodes := make([]ringNode, 0, len(upstreams)*replicas)
	for i := range upstreams {
		for j := range replicas {
			virtualNodeKey := fmt.Sprintf("%s#%d", upstreams[i].URL, j)
			nodes = append(nodes, ringNode{
				hash:     crc32.ChecksumIEEE([]byte(virtualNodeKey)),
				upstream: upstreams[i],
			})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].hash < nodes[j].hash
	})
	return nodes
}
