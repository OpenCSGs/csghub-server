package component

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
	PickEndpoint(modelKey string, sessionKey string, endpoints []commontypes.UpstreamConfig, policy commontypes.RoutingPolicy) (commontypes.UpstreamConfig, error)
}

type ringNode struct {
	hash     uint32
	endpoint *commontypes.UpstreamConfig
}

type sessionRouterImpl struct {
	ringCache  sync.Map // map[string][]ringNode
	rrCounters sync.Map // map[string]*atomic.Uint64
}

func NewSessionRouter() SessionRouter {
	return &sessionRouterImpl{}
}

func (r *sessionRouterImpl) PickEndpoint(modelKey string, sessionKey string, endpoints []commontypes.UpstreamConfig, policy commontypes.RoutingPolicy) (commontypes.UpstreamConfig, error) {
	enabledEndpoints := normalizeEnabledEndpoints(endpoints)
	if len(enabledEndpoints) == 0 {
		return commontypes.UpstreamConfig{}, fmt.Errorf("no enabled endpoint")
	}

	strategy := strings.TrimSpace(policy.Strategy)
	if strategy == "" {
		if len(enabledEndpoints) == 1 {
			strategy = RoutingStrategySingle
		} else if sessionKey != "" {
			strategy = RoutingStrategySessionHash
		} else {
			strategy = RoutingStrategySingle
		}
	}

	switch strategy {
	case RoutingStrategySingle:
		return enabledEndpoints[0], nil
	case RoutingStrategyRoundRobin:
		return pickByRoundRobin(r.rrCounter(buildEndpointsKey(enabledEndpoints)), enabledEndpoints), nil
	case RoutingStrategySessionHash:
		if sessionKey == "" {
			return enabledEndpoints[0], nil
		}
		return r.pickByConsistentHash(modelKey, sessionKey, enabledEndpoints, policy.HashReplicas), nil
	default:
		return enabledEndpoints[0], nil
	}
}

func normalizeEnabledEndpoints(endpoints []commontypes.UpstreamConfig) []commontypes.UpstreamConfig {
	filtered := make([]commontypes.UpstreamConfig, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if strings.TrimSpace(endpoint.URL) == "" || !endpoint.Enabled {
			continue
		}
		if endpoint.Weight <= 0 {
			endpoint.Weight = 1
		}
		filtered = append(filtered, endpoint)
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].URL < filtered[j].URL
	})
	return filtered
}

func pickByRoundRobin(counter *atomic.Uint64, endpoints []commontypes.UpstreamConfig) commontypes.UpstreamConfig {
	if len(endpoints) == 1 {
		return endpoints[0]
	}
	sequence := counter.Add(1)
	return endpoints[(sequence-1)%uint64(len(endpoints))]
}

func (r *sessionRouterImpl) pickByConsistentHash(modelKey string, sessionKey string, endpoints []commontypes.UpstreamConfig, replicas int) commontypes.UpstreamConfig {
	if len(endpoints) == 1 {
		return endpoints[0]
	}
	if replicas <= 0 {
		replicas = defaultHashReplicas
	}

	nodes := r.ring(buildRingCacheKey(endpoints, replicas), endpoints, replicas)
	routeKey := fmt.Sprintf("%s:%s", modelKey, sessionKey)
	targetHash := crc32.ChecksumIEEE([]byte(routeKey))
	index := sort.Search(len(nodes), func(i int) bool {
		return nodes[i].hash >= targetHash
	})
	if index == len(nodes) {
		index = 0
	}
	return *nodes[index].endpoint
}

func buildEndpointsKey(endpoints []commontypes.UpstreamConfig) string {
	endpointURLs := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		endpointURLs = append(endpointURLs, endpoint.URL)
	}
	return strings.Join(endpointURLs, "|")
}

func buildRingCacheKey(endpoints []commontypes.UpstreamConfig, replicas int) string {
	return fmt.Sprintf("%s#r%d", buildEndpointsKey(endpoints), replicas)
}

func (r *sessionRouterImpl) rrCounter(key string) *atomic.Uint64 {
	if existing, ok := r.rrCounters.Load(key); ok {
		return existing.(*atomic.Uint64)
	}
	counter := &atomic.Uint64{}
	actual, _ := r.rrCounters.LoadOrStore(key, counter)
	return actual.(*atomic.Uint64)
}

func (r *sessionRouterImpl) ring(cacheKey string, endpoints []commontypes.UpstreamConfig, replicas int) []ringNode {
	if cached, ok := r.ringCache.Load(cacheKey); ok {
		return cached.([]ringNode)
	}
	nodes := make([]ringNode, 0, len(endpoints)*replicas)
	for i := range endpoints {
		endpoint := &endpoints[i]
		for i := range replicas {
			virtualNodeKey := fmt.Sprintf("%s#%d", endpoint.URL, i)
			nodes = append(nodes, ringNode{
				hash:     crc32.ChecksumIEEE([]byte(virtualNodeKey)),
				endpoint: endpoint,
			})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].hash < nodes[j].hash
	})
	actual, _ := r.ringCache.LoadOrStore(cacheKey, nodes)
	return actual.([]ringNode)
}
