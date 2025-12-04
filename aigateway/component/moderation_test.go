package component

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mock_rpc "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/rpc"
)

func TestSplitContentIntoChunksByWindow_Table(t *testing.T) {
	longLen := slidingWindowSize*2 + 10
	longStr := strings.Repeat("a", longLen)

	// build expected chunks for longStr
	var expectedLong []string
	for i := 0; i < longLen; i += slidingWindowSize {
		end := i + slidingWindowSize
		if end > longLen {
			end = longLen
		}
		expectedLong = append(expectedLong, longStr[i:end])
	}

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "", want: []string{}},
		{name: "simple sentences", in: "Hello world. How are you? I'm fine!", want: []string{"Hello world", "How are you", "I'm fine"}},
		{name: "leading/trailing/extra punctuation", in: "  .hello.. world!  ", want: []string{"hello", "world"}},
		{name: "long single sentence", in: longStr, want: expectedLong},
		{name: "mixed with long sentence and short", in: "short. " + longStr + "! tail?", want: append(append([]string{"short"}, expectedLong...), "tail")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitContentIntoChunksByWindow(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected result for %q:\ngot:  %#v\nwant: %#v", tt.name, got, tt.want)
			}
		})
	}
}

func TestModerationImpl_CheckLLMPrompt_WithoutCache(t *testing.T) {
	ctx := context.Background()
	key := "test-key"

	t.Run("short and not sensitive", func(t *testing.T) {
		mockClient := mock_rpc.NewMockModerationSvcClient(t)
		moderation := NewModerationImplWithClient(mockClient, nil)
		content := "this is a short and safe text"

		mockClient.On("PassLLMPromptCheck", ctx, content, key).Return(&rpc.CheckResult{IsSensitive: false}, nil).Once()

		result, err := moderation.CheckLLMPrompt(ctx, content, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.IsSensitive)
		mockClient.AssertExpectations(t)
	})

	t.Run("short and sensitive", func(t *testing.T) {
		mockClient := mock_rpc.NewMockModerationSvcClient(t)
		moderation := NewModerationImplWithClient(mockClient, nil)
		content := "this is a short and sensitive text"

		mockClient.On("PassLLMPromptCheck", ctx, content, key).Return(&rpc.CheckResult{IsSensitive: true, Reason: "sensitive"}, nil).Once()

		result, err := moderation.CheckLLMPrompt(ctx, content, key)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.IsSensitive)
		mockClient.AssertExpectations(t)
	})
}
