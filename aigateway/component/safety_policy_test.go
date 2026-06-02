package component

import (
	"context"
	"errors"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mockcomp "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/aigateway/component"
	mockdatabase "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestSensitivePolicyImpl_CheckChatSensitive(t *testing.T) {
	ctx := context.Background()
	messages := []openai.ChatCompletionMessageParamUnion{}

	t.Run("model is nil returns false", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		needCheck, result, err := policy.CheckChatSensitive(ctx, nil, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("NeedSensitiveCheck is false returns false", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{}

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("moderation is nil returns false", func(t *testing.T) {
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(nil, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("model is nil and moderation is nil", func(t *testing.T) {
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(nil, mockWhitelist).(*sensitivePolicyImpl)

		needCheck, result, err := policy.CheckChatSensitive(ctx, nil, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("whitelist hit skips moderation", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return(
			[]database.RepositoryFileCheckRule{{RuleType: database.RuleTypeNamespace, Pattern: "qwen"}}, nil,
		).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("whitelist with provider hit skips moderation", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen", "huggingface"}, "Qwen/Qwen3Guard").Return(
			[]database.RepositoryFileCheckRule{{RuleType: database.RuleTypeNamespace, Pattern: "huggingface"}}, nil,
		).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "huggingface")
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("whitelist query error returns error", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return(nil, errors.New("db error")).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.ErrorContains(t, err, "failed to query white list rules: db error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("whitelist miss calls moderation and returns not sensitive", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		expectedResult := &rpc.CheckResult{IsSensitive: false}

		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(mock.Anything, mock.Anything, mock.Anything).Return([]database.RepositoryFileCheckRule{}, nil).Once()
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("whitelist miss calls moderation with stream flag", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		expectedResult := &rpc.CheckResult{IsSensitive: false}

		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(mock.Anything, mock.Anything, mock.Anything).Return([]database.RepositoryFileCheckRule{}, nil).Once()
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:Qwen/Qwen3Guard", true).Return(expectedResult, nil).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", true, "")
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("moderation API error returns error", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}

		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(mock.Anything, mock.Anything, mock.Anything).Return([]database.RepositoryFileCheckRule{}, nil).Once()
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:Qwen/Qwen3Guard", false).Return(nil, errors.New("mod api error")).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.ErrorContains(t, err, "failed to call moderation error:mod api error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("moderation returns sensitive content", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		expectedResult := &rpc.CheckResult{IsSensitive: true, Reason: "toxic"}

		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(mock.Anything, mock.Anything, mock.Anything).Return([]database.RepositoryFileCheckRule{}, nil).Once()
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("whitelist is nil skips whitelist query", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		policy := NewSensitivePolicy(mockModeration, nil).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		expectedResult := &rpc.CheckResult{IsSensitive: false}
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "")
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("provider deduplicates with modelID namespace", func(t *testing.T) {
		mockModeration := mockcomp.NewMockModeration(t)
		mockWhitelist := mockdatabase.NewMockRepositoryFileCheckRuleStore(t)
		policy := NewSensitivePolicy(mockModeration, mockWhitelist).(*sensitivePolicyImpl)

		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "openai/gpt-4",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		expectedResult := &rpc.CheckResult{IsSensitive: false}

		// provider "openai" is same as namespace from model ID "openai/gpt-4", should be deduplicated
		mockWhitelist.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"openai"}, "openai/gpt-4").Return(
			[]database.RepositoryFileCheckRule{}, nil,
		).Once()
		mockModeration.EXPECT().CheckChatPrompts(ctx, messages, "uuid:openai/gpt-4", false).Return(expectedResult, nil).Once()

		needCheck, result, err := policy.CheckChatSensitive(ctx, model, messages, "uuid", false, "openai")
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})
}
