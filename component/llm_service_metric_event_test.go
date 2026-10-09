package component

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestLLMServiceComponent_ListMetricEvents(t *testing.T) {
	ctx := context.TODO()
	metricEventStore := mockdatabase.NewMockAIGatewayMetricEventStore(t)
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	mc := &llmServiceComponentImpl{
		upstreamStore:    upstreamStore,
		metricEventStore: metricEventStore,
	}

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	dbEvents := []database.AIGatewayMetricEvent{
		{
			RequestID: "req-1", StartTime: start, CreatedAt: start,
			Model: "kimi-k3", UpstreamID: 7, StatusCode: 200,
			PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110,
		},
		{
			RequestID: "req-2", StartTime: end, CreatedAt: end,
			Model: "qwen-72b", UpstreamID: 8, StatusCode: 500,
			ErrorType: "upstream_error", ErrorMessage: "boom",
		},
	}
	expectedQuery := types.AIGatewayMetricEventQuery{StartTime: start, EndTime: end, Per: 20, Page: 1}
	metricEventStore.EXPECT().List(ctx, expectedQuery).Return(dbEvents, 2, nil)
	upstreamStore.EXPECT().MapUpstreamIDsToLLMConfigIDs(ctx, []int64{7, 8}).
		Return(map[int64]int64{7: 42}, nil)

	logs, total, err := mc.ListMetricEvents(ctx, expectedQuery)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, logs, 2)
	require.Equal(t, "req-1", logs[0].RequestID)
	require.Equal(t, int64(42), logs[0].LlmConfigID)
	// Upstream 8 has no matching upstream row: llm_config_id stays 0.
	require.Equal(t, int64(0), logs[1].LlmConfigID)
	require.Equal(t, "boom", logs[1].ErrorMessage)
}

func TestLLMServiceComponent_ListMetricEvents_LlmConfigFilter(t *testing.T) {
	ctx := context.TODO()
	metricEventStore := mockdatabase.NewMockAIGatewayMetricEventStore(t)
	upstreamStore := mockdatabase.NewMockUpstreamStore(t)
	mc := &llmServiceComponentImpl{
		upstreamStore:    upstreamStore,
		metricEventStore: metricEventStore,
	}

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	upstreams := []*database.Upstream{{ID: 7, LLMConfigID: 42}, {ID: 9, LLMConfigID: 42}}
	upstreamStore.EXPECT().ListByLLMConfigID(ctx, int64(42)).Return(upstreams, nil)
	expectedQuery := types.AIGatewayMetricEventQuery{
		StartTime:   start,
		EndTime:     end,
		LlmConfigID: 42,
		UpstreamIDs: []int64{7, 9},
		Per:         20,
		Page:        1,
	}
	metricEventStore.EXPECT().List(ctx, expectedQuery).
		Return([]database.AIGatewayMetricEvent{}, 0, nil)
	upstreamStore.EXPECT().MapUpstreamIDsToLLMConfigIDs(ctx, mock.Anything).
		Return(map[int64]int64{}, nil)

	logs, total, err := mc.ListMetricEvents(ctx, types.AIGatewayMetricEventQuery{
		StartTime: start, EndTime: end, LlmConfigID: 42, Per: 20, Page: 1,
	})
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, logs)
}
