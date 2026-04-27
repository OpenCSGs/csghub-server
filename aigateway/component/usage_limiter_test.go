package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcache "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/cache"
	"opencsg.com/csghub-server/aigateway/token"
	"opencsg.com/csghub-server/aigateway/types"
	commontypes "opencsg.com/csghub-server/common/types"
)

func TestUsageLimiter_Check_Exceeded(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	limiter := &usageLimiterImpl{
		redisClient: redisClient,
		nowFn: func() time.Time {
			return time.Unix(1713330000, 0)
		},
	}
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "gpt-4o", OwnedBy: "openai"},
		Endpoint: "https://a.example.com/v1/chat/completions",
		Upstreams: []commontypes.UpstreamConfig{
			{
				URL: "https://a.example.com/v1/chat/completions",
				LimitPolicy: &commontypes.UsageLimitPolicy{
					Enabled:        true,
					WindowSeconds:  60,
					MaxTotalTokens: 100,
				},
			},
		},
	}

	redisClient.EXPECT().
		RunScript(mock.Anything, usageLimitCheckScript, mock.Anything, int64(100), int64(0), int64(0)).
		Return(int64(0), nil).
		Once()

	err := limiter.Check(context.Background(), "user-1", model, model.Endpoint)
	require.Error(t, err)
	require.True(t, IsUsageLimitExceeded(err))
}

func TestUsageLimiter_Commit_NormalizesCachedPromptCost(t *testing.T) {
	redisClient := mockcache.NewMockRedisClient(t)
	limiter := &usageLimiterImpl{
		redisClient: redisClient,
		nowFn: func() time.Time {
			return time.Unix(1713330000, 0)
		},
	}
	model := &types.Model{
		BaseModel: types.BaseModel{ID: "gpt-4o", OwnedBy: "openai"},
		Endpoint: "https://a.example.com/v1/chat/completions",
		Upstreams: []commontypes.UpstreamConfig{
			{
				URL: "https://a.example.com/v1/chat/completions",
				LimitPolicy: &commontypes.UsageLimitPolicy{
					Enabled:              true,
					WindowSeconds:        60,
					CachedTokenCostRatio: 0.5,
				},
			},
		},
	}
	usage := &token.Usage{
		PromptTokens:       100,
		CompletionTokens:   20,
		CachedPromptTokens: 40,
	}

	redisClient.EXPECT().
		RunScript(mock.Anything, usageLimitCommitScript, mock.Anything, int64(100), int64(80), int64(20), mock.Anything).
		Return(int64(1), nil).
		Once()

	err := limiter.Commit(context.Background(), "user-1", model, model.Endpoint, usage)
	require.NoError(t, err)
}

func TestNormalizeUsageForLimit(t *testing.T) {
	promptCost, completionCost, totalCost := normalizeUsageForLimit(&token.Usage{
		PromptTokens:              120,
		CompletionTokens:          30,
		CachedPromptTokens:        20,
		CacheCreationPromptTokens: 10,
	}, commontypes.UsageLimitPolicy{
		CachedTokenCostRatio: 0.5,
		CacheCreateCostRatio: 1,
	})

	require.Equal(t, int64(120), promptCost)
	require.Equal(t, int64(30), completionCost)
	require.Equal(t, int64(150), totalCost)
}

func TestGetUsageLimitPolicy_UsesEndpointPolicy(t *testing.T) {
	model := &types.Model{
		Endpoint: "https://a.example.com/v1/chat/completions",
		Upstreams: []commontypes.UpstreamConfig{
			{
				URL: "https://a.example.com/v1/chat/completions",
				LimitPolicy: &commontypes.UsageLimitPolicy{
					Enabled:        true,
					WindowSeconds:  120,
					MaxTotalTokens: 300,
				},
			},
		},
	}

	policy := getUsageLimitPolicy(model, model.Endpoint)
	require.NotNil(t, policy)
	require.Equal(t, int64(120), policy.WindowSeconds)
	require.Equal(t, int64(300), policy.MaxTotalTokens)
}

func TestBuildUsageLimitKey_IncludesEndpoint(t *testing.T) {
	model := &types.Model{
		BaseModel: types.BaseModel{
			ID:      "gpt-4o",
			OwnedBy: "openai",
		},
		Endpoint: "https://b.example.com/v1/chat/completions",
	}

	key := buildUsageLimitKey("user-1", model, model.Endpoint, 60, time.Unix(1713330000, 0))
	require.Contains(t, key, "https://b.example.com/v1/chat/completions")
}
