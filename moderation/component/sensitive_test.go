package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	mock_sensitive "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/moderation/checker"
)

func TestSensitiveComponentImpl_PassTextCheck(t *testing.T) {
	mockSensitive := mock_sensitive.NewMockSensitiveChecker(t)
	component := SensitiveComponentImpl{
		checker: mockSensitive,
	}
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	checker.InitWithContentChecker(cfg, mockSensitive)
	mockSensitive.EXPECT().PassTextCheck(mock.Anything, types.ScenarioNicknameDetection, "你好").
		Return(&sensitive.CheckResult{
			IsSensitive: false,
		}, nil)
	result, err := component.PassTextCheck(context.Background(),
		types.ScenarioNicknameDetection, "你好")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}

func TestSensitiveComponentImpl_PassImageURLCheck(t *testing.T) {
	mockSensitive := mock_sensitive.NewMockSensitiveChecker(t)
	component := SensitiveComponentImpl{
		checker: mockSensitive,
	}
	mockSensitive.EXPECT().PassImageURLCheck(mock.Anything, types.ScenarioNicknameDetection, "你好").
		Return(&sensitive.CheckResult{
			IsSensitive: false,
		}, nil)
	result, err := component.PassImageURLCheck(context.Background(),
		types.ScenarioNicknameDetection, "你好")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}

func TestSensitiveComponentImpl_PassLLMQueryCheck(t *testing.T) {
	mockSeneitive := mock_sensitive.NewMockSensitiveChecker(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	component := SensitiveComponentImpl{
		checker: mockSeneitive,
		cfg:     cfg,
	}
	mockSeneitive.EXPECT().PassLLMCheck(mock.Anything, &types.LLMCheckRequest{
		Scenario:  types.ScenarioNicknameDetection,
		Text:      "你好",
		AccountId: "123",
		MaxTokens: 0,
		RawJSON:   "",
		ModelName: "test-model",
		Role:      "user",
	}).
		Return(&sensitive.CheckResult{
			IsSensitive: false,
		}, nil)
	result, err := component.PassLLMQueryCheck(context.Background(), &types.LLMCheckRequest{
		Scenario:  types.ScenarioNicknameDetection,
		Text:      "你好",
		AccountId: "123",
		MaxTokens: 0,
		RawJSON:   "",
		Role:      "user",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}

func TestSensitiveComponentImpl_PassStreamCheck(t *testing.T) {
	mockSeneitive := mock_sensitive.NewMockSensitiveChecker(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	component := SensitiveComponentImpl{
		checker: mockSeneitive,
		cfg:     cfg,
	}
	mockSeneitive.EXPECT().PassLLMCheck(mock.Anything, &types.LLMCheckRequest{
		Scenario:  types.ScenarioNicknameDetection,
		Text:      "你好",
		SessionId: "123",
		MaxTokens: 0,
		RawJSON:   "",
		ModelName: "test-stream-model",
		Role:      "assistant",
	}).
		Return(&sensitive.CheckResult{
			IsSensitive: false,
		}, nil)
	result, err := component.PassStreamCheck(context.Background(), &types.LLMCheckRequest{
		Scenario:  types.ScenarioNicknameDetection,
		Text:      "你好",
		SessionId: "123",
		MaxTokens: 0,
		RawJSON:   "",
		Role:      "assistant",
	})
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}
