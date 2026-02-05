package sensitive_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	green20220302 "github.com/alibabacloud-go/green-20220302/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	mockgreen "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestChainImpl_AliYun_PassTextCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "test text"
	task := map[string]string{"content": text}
	serviceParameters, _ := json.Marshal(task)
	textModerationRequest := &green20220302.TextModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	reason := "normal"
	labels := "normal"
	g2c.EXPECT().TextModeration(textModerationRequest).Return(&green20220302.TextModerationResponse{
		StatusCode: &statusCode,
		Body: &green20220302.TextModerationResponseBody{
			Code: &statusCode,
			Data: &green20220302.TextModerationResponseBodyData{
				Reason: &reason,
				Labels: &labels,
			},
		},
	}, nil)

	result, err := chain.PassTextCheck(ctx, scenario, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassTextCheck_Sensitive(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "sensitive text"
	task := map[string]string{"content": text}
	serviceParameters, _ := json.Marshal(task)
	textModerationRequest := &green20220302.TextModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	reason := "sensitive content"
	labels := "politics"
	requestId := "test-request-id"
	g2c.EXPECT().TextModeration(textModerationRequest).Return(&green20220302.TextModerationResponse{
		StatusCode: &statusCode,
		Body: &green20220302.TextModerationResponseBody{
			Code: &statusCode,
			Data: &green20220302.TextModerationResponseBodyData{
				Reason: &reason,
				Labels: &labels,
			},
			RequestId: &requestId,
		},
	}, nil)

	result, err := chain.PassTextCheck(ctx, scenario, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	if result.Reason != fmt.Sprintf("label:%s,reason:%s,requestId:%s", labels, reason, requestId) {
		t.Fatalf("expected reason %s, got %s", reason, result.Reason)
	}
}

func TestChainImpl_AliYun_PassImageCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	ossBucketName := "test-bucket"
	ossObjectName := "test-image.jpg"
	g2c.EXPECT().GetRegionId().Return("test-region-id")
	task := map[string]interface{}{
		"ossRegionId":   "test-region-id", // This will be set by the checker
		"ossBucketName": ossBucketName,
		"ossObjectName": ossObjectName,
	}
	serviceParameters, _ := json.Marshal(task)
	imageModerationRequest := &green20220302.ImageModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	g2c.EXPECT().ImageModeration(imageModerationRequest).Return(&green20220302.ImageModerationResponse{
		StatusCode: &statusCode,
		Body: &green20220302.ImageModerationResponseBody{
			Code: &statusCode,
			Data: &green20220302.ImageModerationResponseBodyData{
				Result: []*green20220302.ImageModerationResponseBodyDataResult{
					{
						Label: tea.String("nonLabel"),
					},
				},
			},
		},
	}, nil)

	result, err := chain.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassImageCheck_Sensitive(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	ossBucketName := "test-bucket"
	ossObjectName := "test-image.jpg"
	g2c.EXPECT().GetRegionId().Return("test-region-id")
	task := map[string]interface{}{
		"ossRegionId":   "test-region-id", // This will be set by the checker
		"ossBucketName": ossBucketName,
		"ossObjectName": ossObjectName,
	}
	serviceParameters, _ := json.Marshal(task)
	imageModerationRequest := &green20220302.ImageModerationRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	requestId := "test-request-id"
	g2c.EXPECT().ImageModeration(imageModerationRequest).Return(&green20220302.ImageModerationResponse{
		StatusCode: &statusCode,
		Body: &green20220302.ImageModerationResponseBody{
			Code: &statusCode,
			Data: &green20220302.ImageModerationResponseBodyData{
				Result: []*green20220302.ImageModerationResponseBodyDataResult{
					{
						Label:      tea.String("politics"),
						Confidence: tea.Float32(90.5),
					},
				},
			},
			RequestId: &requestId,
		},
	}, nil)

	result, err := chain.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	if result.Reason != "politics" {
		t.Fatalf("expected reason %s, got %s", "politics", result.Reason)
	}
}

func TestChainImpl_AliYun_PassImageURLCheck(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	imageURL := "https://example.com/normal-image.jpg"

	checker.EXPECT().PassImageURLCheck(ctx, scenario, imageURL).Return(&sensitive.CheckResult{
		IsSensitive: false,
	}, nil)

	result, err := chain.PassImageURLCheck(ctx, scenario, imageURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassImageURLCheck_Sensitive(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	imageURL := "https://example.com/sensitive-image.jpg"
	labels := "politics"
	confidence := 95.0
	requestId := "test-request-id"

	checker.EXPECT().PassImageURLCheck(ctx, scenario, imageURL).Return(&sensitive.CheckResult{
		IsSensitive: true,
		Reason:      fmt.Sprintf("label:%s,confidence:%f,requestId:%s", labels, confidence, requestId),
	}, nil)
	result, err := chain.PassImageURLCheck(ctx, scenario, imageURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	expectedReason := fmt.Sprintf("label:%s,confidence:%f,requestId:%s", labels, confidence, requestId)
	if result.Reason != expectedReason {
		t.Fatalf("expected reason %s, got %s", expectedReason, result.Reason)
	}
}

func TestChainImpl_AliYun_PassLLMCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "test llm text"
	sessionId := "test-session-id"
	task := map[string]string{
		"content":   text,
		"sessionId": sessionId,
	}
	serviceParameters, _ := json.Marshal(task)
	textModerationPlusRequest := &green20220302.TextModerationPlusRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	riskLevel := "low"
	options := &util.RuntimeOptions{
		ReadTimeout:    tea.Int(500),
		ConnectTimeout: tea.Int(500),
	}
	g2c.EXPECT().TextModerationPlusWithOptions(textModerationPlusRequest, options).Return(&green20220302.TextModerationPlusResponse{
		StatusCode: &statusCode,
		Body: &green20220302.TextModerationPlusResponseBody{
			Code: &statusCode,
			Data: &green20220302.TextModerationPlusResponseBodyData{
				RiskLevel: &riskLevel,
			},
		},
	}, nil)

	result, err := chain.PassLLMCheck(ctx, scenario, text, sessionId, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassLLMCheck_Sensitive(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "sensitive llm text"
	sessionId := "test-session-id"
	task := map[string]string{
		"content":   text,
		"sessionId": sessionId,
	}
	serviceParameters, _ := json.Marshal(task)
	textModerationPlusRequest := &green20220302.TextModerationPlusRequest{
		Service:           tea.String(string(scenario)),
		ServiceParameters: tea.String(string(serviceParameters)),
	}
	var statusCode int32 = 200
	requestId := "test-request-id"
	riskLevel := "high"
	labels := "political_content"
	riskWords := "risk words"
	options := &util.RuntimeOptions{
		ReadTimeout:    tea.Int(500),
		ConnectTimeout: tea.Int(500),
	}
	g2c.EXPECT().TextModerationPlusWithOptions(textModerationPlusRequest, options).Return(&green20220302.TextModerationPlusResponse{
		StatusCode: &statusCode,
		Body: &green20220302.TextModerationPlusResponseBody{
			Code:      &statusCode,
			RequestId: &requestId,
			Data: &green20220302.TextModerationPlusResponseBodyData{
				Result: []*green20220302.TextModerationPlusResponseBodyDataResult{
					{
						Label:     &labels,
						RiskWords: &riskWords,
					},
				},
				RiskLevel: &riskLevel,
			},
		},
	}, nil)

	result, err := chain.PassLLMCheck(ctx, scenario, text, sessionId, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.NotNil(t, result)
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	expectedReason := fmt.Sprintf("label:%s,reason:%s,requestId:%s", labels, riskWords, requestId)
	if result.Reason != expectedReason {
		t.Fatalf("expected reason %s, got %s", expectedReason, result.Reason)
	}
}
