package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestSessionRouter_PickEndpoint_SessionHashStable(t *testing.T) {
	router := NewSessionRouter()
	endpoints := []commontypes.UpstreamConfig{
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
	}

	first, err := router.PickEndpoint("model-a", "session-1", endpoints, commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 64,
	})
	require.NoError(t, err)

	second, err := router.PickEndpoint("model-a", "session-1", endpoints, commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 64,
	})
	require.NoError(t, err)

	require.Equal(t, first.URL, second.URL)
}

func TestSessionRouter_PickEndpoint_FallbackToFirstEnabled(t *testing.T) {
	router := NewSessionRouter()
	endpoints := []commontypes.UpstreamConfig{
		{URL: "https://node-b.example/v1/chat/completions", Enabled: false},
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
	}

	selected, err := router.PickEndpoint("model-a", "", endpoints, commontypes.RoutingPolicy{})
	require.NoError(t, err)
	require.Equal(t, "https://node-a.example/v1/chat/completions", selected.URL)
}

func TestSessionRouter_PickEndpoint_RoundRobin(t *testing.T) {
	router := NewSessionRouter()
	endpoints := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
	}
	policy := commontypes.RoutingPolicy{Strategy: RoutingStrategyRoundRobin}

	first, err := router.PickEndpoint("model-a", "", endpoints, policy)
	require.NoError(t, err)
	second, err := router.PickEndpoint("model-a", "", endpoints, policy)
	require.NoError(t, err)

	require.NotEqual(t, first.URL, second.URL)
}

func TestSessionRouter_PickEndpoint_SessionHashDistributesAcrossEndpoints(t *testing.T) {
	router := NewSessionRouter()
	endpoints := []commontypes.UpstreamConfig{
		{URL: "https://node-a.example/v1/chat/completions", Enabled: true},
		{URL: "https://node-b.example/v1/chat/completions", Enabled: true},
	}
	policy := commontypes.RoutingPolicy{
		Strategy:     RoutingStrategySessionHash,
		HashReplicas: 128,
	}

	hit := map[string]struct{}{}
	for i := 0; i < 20; i++ {
		selected, err := router.PickEndpoint("model-a", "session-"+string(rune('a'+i)), endpoints, policy)
		require.NoError(t, err)
		hit[selected.URL] = struct{}{}
	}
	require.GreaterOrEqual(t, len(hit), 2)
}
