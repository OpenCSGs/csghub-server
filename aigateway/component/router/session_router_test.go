package router

import (
	"testing"

	"github.com/stretchr/testify/require"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestSessionRouter_PickUpstream_SessionHashStable(t *testing.T) {
	router := NewSessionRouter()
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
	}

	first, err := router.PickUpstream("model-a", "session-1", upstreams, commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 64,
	})
	require.NoError(t, err)

	second, err := router.PickUpstream("model-a", "session-1", upstreams, commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 64,
	})
	require.NoError(t, err)

	require.Equal(t, first.URL, second.URL)
}

func TestSessionRouter_PickUpstream_FallbackToFirstEnabled(t *testing.T) {
	router := NewSessionRouter()
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://node-b.example/v1/chat/completions", ModelName: "modelA", Enabled: false},
		{URL: "https://node-a.example/v1/chat/completions", ModelName: "modelB", Enabled: true},
	}

	enabled := NormalizeEnabledUpstreams(upstreams)
	selected, err := router.PickUpstream("model-a", "", enabled, commontypes.RoutingPolicy{})
	require.NoError(t, err)
	require.Equal(t, "https://node-a.example/v1/chat/completions", selected.URL)
}

func TestSessionRouter_PickUpstream_RoundRobin(t *testing.T) {
	router := NewSessionRouter()
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
	}
	policy := commontypes.RoutingPolicy{Strategy: RoutingStrategyRoundRobin}

	first, err := router.PickUpstream("model-a", "", upstreams, policy)
	require.NoError(t, err)
	second, err := router.PickUpstream("model-a", "", upstreams, policy)
	require.NoError(t, err)

	require.NotEqual(t, first.URL, second.URL)
}

func TestSessionRouter_PickUpstream_SessionHashDistributesAcrossUpstreams(t *testing.T) {
	router := NewSessionRouter()
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
	}
	policy := commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 128,
	}

	hit := map[string]struct{}{}
	for i := 0; i < 20; i++ {
		selected, err := router.PickUpstream("model-a", "session-"+string(rune('a'+i)), upstreams, policy)
		require.NoError(t, err)
		hit[selected.URL] = struct{}{}
	}
	require.GreaterOrEqual(t, len(hit), 2)
}

func TestSessionRouter_PickUpstream_SessionHash_ReflectsUpstreamConfigChange(t *testing.T) {
	router := NewSessionRouter()
	upstreamsV1 := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true, ModelName: "gpt-4o", Provider: "openai", AuthHeader: "Bearer sk-old"},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true, ModelName: "gpt-4o-mini", Provider: "openai", AuthHeader: "Bearer sk-old"},
	}
	policy := commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 64,
	}

	selectedV1, err := router.PickUpstream("model-a", "session-1", upstreamsV1, policy)
	require.NoError(t, err)
	require.Equal(t, "openai", selectedV1.Provider)
	require.Equal(t, "Bearer sk-old", selectedV1.AuthHeader)

	upstreamsV2 := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true, ModelName: "gpt-4o-2024-08-06", Provider: "azure", AuthHeader: "Bearer sk-new"},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true, ModelName: "gpt-4o-mini-2024-07-18", Provider: "azure", AuthHeader: "Bearer sk-new"},
	}

	selectedV2, err := router.PickUpstream("model-a", "session-1", upstreamsV2, policy)
	require.NoError(t, err)

	require.Equal(t, "azure", selectedV2.Provider)
	require.Equal(t, "Bearer sk-new", selectedV2.AuthHeader)
	require.Equal(t, selectedV1.URL, selectedV2.URL)
}

func TestBuildRing_ValueCopy(t *testing.T) {
	upstreams := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true, ModelName: "v1-model"},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true, ModelName: "v1-model"},
	}

	nodes := buildRing(upstreams, 4)
	require.NotEmpty(t, nodes)

	upstreams[0].ModelName = "mutated-after-build"

	for _, n := range nodes {
		if n.upstream.URL == "https://node-a.example/v1/chat/completions" {
			require.Equal(t, "v1-model", n.upstream.ModelName, "ringNode must hold a value copy, not a pointer to the original slice")
		}
	}
}
