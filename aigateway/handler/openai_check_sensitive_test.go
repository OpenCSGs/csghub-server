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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen", "another"}, "another/model").Return([]database.RepositoryFileCheckRule{
			{RuleType: database.RuleTypeNamespace, Pattern: "qwen"},
		}, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
		tester.mocks.whitelistRule.AssertNotCalled(t, "Exists", mock.Anything, mock.Anything, mock.Anything)
		tester.mocks.whitelistRule.AssertNotCalled(t, "MatchRegex", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("load whitelist rules fails", func(t *testing.T) {
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen", "another"}, "another/model").Return(nil, errors.New("db error")).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to query white list rules: db error")
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{
			{RuleType: database.RuleTypeNamespace, Pattern: "qwen"},
		}, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("No whitelist match and continue moderation", func(t *testing.T) {
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{}, nil).Once()
		expectedResult := &rpc.CheckResult{IsSensitive: false}
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(ctx, chatReq.Messages, "test-uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{}, nil).Once()

		expectedResult := &rpc.CheckResult{IsSensitive: true}
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(ctx, chatReq.Messages, "test-uuid:Qwen/Qwen3Guard", false).Return(expectedResult, nil).Once()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.True(t, needCheck)
		assert.Equal(t, expectedResult, result)
	})

	t.Run("Check model name exact match succeeds", func(t *testing.T) {
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{
			{RuleType: database.RuleTypeModelName, Pattern: "qwen/qwen3guard"},
		}, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})

	t.Run("Any matched rule skips regardless of detail", func(t *testing.T) {
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{
			{RuleType: database.RuleTypeModelName, Pattern: "meta/meta1"},
		}, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.NoError(t, err)
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
		tester.mocks.whitelistRule.EXPECT().ListBySensitiveCheckTargets(ctx, []string{"qwen"}, "Qwen/Qwen3Guard").Return([]database.RepositoryFileCheckRule{}, nil).Once()
		tester.mocks.moderationComp.EXPECT().CheckChatPrompts(ctx, chatReq.Messages, "test-uuid:Qwen/Qwen3Guard", false).Return(nil, errors.New("mod api error")).Once()

		needCheck, result, err := tester.handler.checkSensitive(ctx, model, chatReq, userUUID, false)
		assert.ErrorContains(t, err, "failed to call moderation error:mod api error")
		assert.False(t, needCheck)
		assert.Nil(t, result)
	})
}

func Test_extractNamespaceTarget(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "normal path",
			path:     "Qwen/Qwen3Guard",
			expected: "qwen",
		},
		{
			name:     "multiple slash path",
			path:     "Qwen///Qwen3Guard",
			expected: "qwen",
		},
		{
			name:     "leading and multiple slash path",
			path:     "///Qwen////Qwen3Guard",
			expected: "qwen",
		},
		{
			name:     "single slash path",
			path:     "/",
			expected: "",
		},
		{
			name:     "special symbols namespace",
			path:     "ns-_.+@123/model-1",
			expected: "ns-_.+@123",
		},
		{
			name:     "no slash path",
			path:     "OnlyNamespace",
			expected: "onlynamespace",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := extractNamespaceTarget(testCase.path)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func Test_buildNamespaceTargets(t *testing.T) {
	t.Run("deduplicate namespaces from official name and model id", func(t *testing.T) {
		targets := buildNamespaceTargets("Qwen///Qwen3Guard", "///Qwen////model")
		assert.Equal(t, []string{"qwen"}, targets)
	})

	t.Run("support one slash and special symbols", func(t *testing.T) {
		targets := buildNamespaceTargets("/", "ns-_.+@123/model")
		assert.Equal(t, []string{"ns-_.+@123"}, targets)
	})
}
