package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/aigateway/types"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
)

func TestOpenAIHandler_checkSensitive(t *testing.T) {
	ctx := context.Background()
	chatReq := &ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessageParamUnion{},
	}
	userUUID := "test-uuid"

	t.Run("NeedSensitiveCheck is false", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		model := &types.Model{
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: false,
			},
		}
		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("OfficialName namespace in whitelist", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:           "another/model",
				OfficialName: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(true, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("OfficialName namespace check fails", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID:           "another/model",
				OfficialName: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, errors.New("db error")).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to check namespace in white list: db error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Namespace in whitelist", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(true, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Namespace check fails", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, errors.New("db error")).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to check namespace in white list: db error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Check moderation API succeeds", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, nil).Once()
		tester.mocks.whitelistRule.EXPECT().MatchRegex(ctx, database.RuleTypeModelName, "Qwen/Qwen3Guard").Return(false, nil).Once()

		expectedResult := &rpc.CheckResult{IsSensitive: true}
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(ctx, chatReq.Messages, "test-uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Check model name regex match succeeds", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, nil).Once()
		tester.mocks.whitelistRule.EXPECT().MatchRegex(ctx, database.RuleTypeModelName, "Qwen/Qwen3Guard").Return(true, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Check model name regex match fails", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, nil).Once()
		tester.mocks.whitelistRule.EXPECT().MatchRegex(ctx, database.RuleTypeModelName, "Qwen/Qwen3Guard").Return(false, errors.New("db error")).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to match model name regex: db error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Check moderation API fails", func(t *testing.T) {
		tester, _, _ := setupTest(t)
		tester.mocks.whitelistRule.ExpectedCalls = nil
		tester.mocks.moderationComp.ExpectedCalls = nil
		model := &types.Model{
			BaseModel: types.BaseModel{
				ID: "Qwen/Qwen3Guard",
			},
			ExternalModelInfo: types.ExternalModelInfo{
				NeedSensitiveCheck: true,
				Provider:           "",
			},
		}
		tester.mocks.whitelistRule.EXPECT().Exists(ctx, database.RuleTypeNamespace, "Qwen").Return(false, nil).Once()
		tester.mocks.whitelistRule.EXPECT().MatchRegex(ctx, database.RuleTypeModelName, "Qwen/Qwen3Guard").Return(false, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(ctx, chatReq.Messages, "test-uuid:Qwen/Qwen3Guard", false).Return(nil, errors.New("mod api error")).Once()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to call moderation error:mod api error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})
}
