package sensitive_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/alibabacloud-go/green-20220302/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/green"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockgreen "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestSensitiveChecker_SplitTasks(t *testing.T) {
	c := new(sensitive.AliyunGreenChecker)
	largeText := strings.Repeat("a", 50000)
	tasks := c.SplitTasks(largeText)
	taskCount := math.Round(float64(len(largeText)) / float64(sensitive.LargeTextSize))
	fmt.Println(taskCount, len(tasks))
	if len(tasks) != int(taskCount) {
		t.Logf("task count mismatch, expected: %d, got: %d", int(taskCount), len(tasks))
		t.FailNow()
	}
}

func TestSensitiveChecker_PassImageURLCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)

	imageURL := "https://example.com/image.jpg"

	t.Run("API call failed", func(t *testing.T) {
		// Mock the ImageModeration method to return an error
		g2c.EXPECT().ImageModeration(mock.Anything).Return(nil, errors.New("API call failed")).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that an error is returned
		require.NotNil(t, err)
		require.Nil(t, result)
	})

	t.Run("API returns non-200 status code", func(t *testing.T) {
		// Mock the ImageModeration method to return a non-200 status code
		g2c.EXPECT().ImageModeration(mock.Anything).Return(&client.ImageModerationResponse{
			StatusCode: tea.Int32(500),
			Body: &client.ImageModerationResponseBody{
				Code: tea.Int32(500),
				Msg:  tea.String("Internal server error"),
			},
		}, nil).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that an error is returned
		require.NotNil(t, err)
		require.Nil(t, result)
	})

	t.Run("Image passes check - empty result", func(t *testing.T) {
		// Mock the ImageModeration method to return an empty result list
		g2c.EXPECT().ImageModeration(mock.Anything).Return(&client.ImageModerationResponse{
			StatusCode: tea.Int32(200),
			Body: &client.ImageModerationResponseBody{
				Code:      tea.Int32(200),
				RequestId: tea.String("request_id_1"),
				Data: &client.ImageModerationResponseBodyData{
					Result: []*client.ImageModerationResponseBodyDataResult{},
				},
			},
		}, nil).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that the image passes the check
		require.Nil(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsSensitive)
	})

	t.Run("Image passes check - nonLabel result", func(t *testing.T) {
		// Mock the ImageModeration method to return a nonLabel result
		g2c.EXPECT().ImageModeration(mock.Anything).Return(&client.ImageModerationResponse{
			StatusCode: tea.Int32(200),
			Body: &client.ImageModerationResponseBody{
				Code:      tea.Int32(200),
				RequestId: tea.String("request_id_2"),
				Data: &client.ImageModerationResponseBodyData{
					Result: []*client.ImageModerationResponseBodyDataResult{
						{
							Label:      tea.String("nonLabel"),
							Confidence: tea.Float32(0.1),
						},
					},
				},
			},
		}, nil).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that the image passes the check
		require.Nil(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsSensitive)
	})

	t.Run("Sensitive image detected", func(t *testing.T) {
		// Mock the ImageModeration method to return a sensitive result
		g2c.EXPECT().ImageModeration(mock.Anything).Return(&client.ImageModerationResponse{
			StatusCode: tea.Int32(200),
			Body: &client.ImageModerationResponseBody{
				Code:      tea.Int32(200),
				RequestId: tea.String("request_id_3"),
				Data: &client.ImageModerationResponseBodyData{
					Result: []*client.ImageModerationResponseBodyDataResult{
						{
							Label:      tea.String("porn"),
							Confidence: tea.Float32(0.95),
						},
					},
				},
			},
		}, nil).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that the image is detected as sensitive
		require.Nil(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsSensitive)
		require.Contains(t, result.Reason, "label:porn")
		require.Contains(t, result.Reason, "confidence:0.950000")
		require.Contains(t, result.Reason, "requestId:request_id_3")
	})

	t.Run("Multiple results with some sensitive", func(t *testing.T) {
		// Mock the ImageModeration method to return multiple results including a sensitive one
		g2c.EXPECT().ImageModeration(mock.Anything).Return(&client.ImageModerationResponse{
			StatusCode: tea.Int32(200),
			Body: &client.ImageModerationResponseBody{
				Code:      tea.Int32(200),
				RequestId: tea.String("request_id_4"),
				Data: &client.ImageModerationResponseBodyData{
					Result: []*client.ImageModerationResponseBodyDataResult{
						{
							Label:      tea.String("nonLabel"),
							Confidence: tea.Float32(0.1),
						},
						{
							Label:      tea.String("politics"),
							Confidence: tea.Float32(0.85),
						},
					},
				},
			},
		}, nil).Once()

		result, err := checker.PassImageURLCheck(context.Background(), types.ScenarioImageBaseLineCheck, imageURL)

		// Verify that the sensitive image is detected
		require.Nil(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsSensitive)
		require.Contains(t, result.Reason, "label:politics")
		require.Contains(t, result.Reason, "confidence:0.850000")
		require.Contains(t, result.Reason, "requestId:request_id_4")
	})
}

func TestSensitiveChecker_PassLargeTextCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	checker := sensitive.NewAliyunChecker(gc, nil)

	t.Run("text too long", func(t *testing.T) {
		_, err := checker.PassLargeTextCheck(
			context.Background(), strings.Repeat("a", 150*sensitive.LargeTextSize),
		)
		require.NotNil(t, err)
	})

	cases := []struct {
		name        string
		label       string
		rate        float32
		suggestion  string
		isSensitive bool
		wantReason  string
	}{
		{"non politics low rate", "ad", 0.1, "", false, ""},
		{"non politics hight rate", "ad", 0.9, "", false, ""},
		{"politics low rate", "politics", 0.1, "", false, ""},
		{"politics hight rate", "politics", 0.9, "block", true, "label:politics,taskId:task_id_1,requestId:request_id_1"},
		{"politic content hight rate", "political_content", 0.9, "block", true, "label:political_content,taskId:task_id_1,requestId:request_id_1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := strings.Repeat("a", sensitive.LargeTextSize+10)
			tasks := checker.SplitTasks(text)
			content, _ := json.Marshal(
				map[string]interface{}{
					"scenes": [...]string{"antispam"},
					"tasks":  tasks,
				},
			)

			textScanRequest := green.CreateTextScanRequest()
			textScanRequest.SetContent(content)

			gc.EXPECT().TextScan(textScanRequest).Return(&sensitive.TextScanResponse{
				Data: []sensitive.TextScanResponseDataItem{
					{
						Results: []sensitive.TextScanResponseDataItemResult{
							{Label: c.label, Rate: c.rate, Suggestion: c.suggestion},
						},
						TaskId: "task_id_1",
					},
				},
				RequestID: "request_id_1",
			}, nil).Once()
			result, err := checker.PassLargeTextCheck(context.Background(), strings.Repeat("a", sensitive.LargeTextSize+10))
			require.Nil(t, err)
			require.Equal(t, c.isSensitive, result.IsSensitive)
			require.Equal(t, c.wantReason, result.Reason)
		})
	}

}

func TestSensitiveChecker_PassTextCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)

	t.Run("large text", func(t *testing.T) {
		gc.EXPECT().TextScan(mock.Anything).Return(&sensitive.TextScanResponse{
			Data: []sensitive.TextScanResponseDataItem{
				{Results: []sensitive.TextScanResponseDataItemResult{
					{Label: "foo", Rate: 0.7, Suggestion: "pass"},
				}},
			}}, nil).Once()
		_, err := checker.PassLargeTextCheck(context.Background(), strings.Repeat("a", sensitive.LargeTextSize+10))
		require.Nil(t, err)
	})

	cases := []struct {
		labels      string
		isSensitive bool
		wantReason  string
	}{
		{"", false, ""},
		{"ad", false, ""},
		{"flood", false, ""},
		{"ad,flood", false, ""},
		{"ad,flood,politics", true, "label:politics,reason:bar,requestId:z"},
		{"politics", true, "label:politics,reason:bar,requestId:z"},
		{"political_content", true, "label:political_content,reason:bar,requestId:z"},
	}

	for _, c := range cases {
		t.Run(c.labels, func(t *testing.T) {
			task := map[string]string{"content": "foo"}
			params, err := json.Marshal(task)
			require.Nil(t, err)

			req := &client.TextModerationRequest{
				Service:           tea.String("foo"),
				ServiceParameters: tea.String(string(params)),
			}

			g2c.EXPECT().TextModeration(req).Return(&client.TextModerationResponse{
				StatusCode: tea.Int32(200),
				Body: &client.TextModerationResponseBody{
					Code:      tea.Int32(200),
					RequestId: tea.String("z"),
					Data: &client.TextModerationResponseBodyData{
						Labels: tea.String(c.labels),
						Reason: tea.String("bar"),
					},
				},
			}, nil).Once()
			result, err := checker.PassTextCheck(context.Background(), "foo", "foo")
			require.Nil(t, err)
			require.Equal(t, c.isSensitive, result.IsSensitive)
			require.Equal(t, c.wantReason, result.Reason)
		})
	}
}

func TestSensitiveChecker_PassLLMCheck(t *testing.T) {
	gc := mockgreen.NewMockGreenClient(t)
	g2c := mockgreen.NewMockGreen2022Client(t)
	checker := sensitive.NewAliyunChecker(gc, g2c)

	cases := []struct {
		labels      string
		isSensitive bool
		riskLevel   string
		wantReason  string
	}{
		{"", false, "none", ""},
		{"ad", false, "low", ""},
		{"flood", false, "low", ""},
		{"ad,flood", false, "low", ""},
		{"political_content", true, "high", "label:political_content,reason:risk_words,requestId:z"},
	}

	id := "123"
	riskWords := "risk_words"
	options := &util.RuntimeOptions{
		ReadTimeout:    tea.Int(500),
		ConnectTimeout: tea.Int(500),
	}
	for _, c := range cases {
		t.Run(c.labels, func(t *testing.T) {
			task := map[string]string{"content": "foo", "sessionId": id}
			params, err := json.Marshal(task)
			require.Nil(t, err)

			req := &client.TextModerationPlusRequest{
				Service:           tea.String("foo"),
				ServiceParameters: tea.String(string(params)),
			}

			g2c.EXPECT().TextModerationPlusWithOptions(req, options).Return(&client.TextModerationPlusResponse{
				StatusCode: tea.Int32(200),
				Body: &client.TextModerationPlusResponseBody{
					Code:      tea.Int32(200),
					RequestId: tea.String("z"),
					Data: &client.TextModerationPlusResponseBodyData{
						Result: []*client.TextModerationPlusResponseBodyDataResult{
							{
								Label:     &c.labels,
								RiskWords: &riskWords,
							},
						},
						RiskLevel: &c.riskLevel,
					},
				},
			}, nil).Once()
			result, err := checker.PassLLMCheck(context.Background(), "foo", "foo", id, "")
			require.Nil(t, err)
			require.Equal(t, c.isSensitive, result.IsSensitive)
			require.Equal(t, c.wantReason, result.Reason)
		})
	}

	for _, c := range cases {
		t.Run(c.labels, func(t *testing.T) {
			task := map[string]string{"content": "foo", "accountId": id}
			params, err := json.Marshal(task)
			require.Nil(t, err)

			req := &client.TextModerationPlusRequest{
				Service:           tea.String("foo"),
				ServiceParameters: tea.String(string(params)),
			}

			g2c.EXPECT().TextModerationPlusWithOptions(req, options).Return(&client.TextModerationPlusResponse{
				StatusCode: tea.Int32(200),
				Body: &client.TextModerationPlusResponseBody{
					Code:      tea.Int32(200),
					RequestId: tea.String("z"),
					Data: &client.TextModerationPlusResponseBodyData{
						Result: []*client.TextModerationPlusResponseBodyDataResult{
							{
								Label:     &c.labels,
								RiskWords: &riskWords,
							},
						},
						RiskLevel: &c.riskLevel,
					},
				},
			}, nil).Once()
			result, err := checker.PassLLMCheck(context.Background(), "foo", "foo", "", id)
			require.Nil(t, err)
			require.Equal(t, c.isSensitive, result.IsSensitive)
			require.Equal(t, c.wantReason, result.Reason)
		})
	}
}
