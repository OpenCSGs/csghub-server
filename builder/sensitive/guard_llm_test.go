package sensitive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestOpenAILLMChecker_PassTextCheck(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var reqBody types.LLMReqBody
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		// Mock sensitive response
		if strings.Contains(reqBody.Messages[0].Content, "bad word") {
			w.WriteHeader(http.StatusOK)
			resp := map[string]interface{}{
				"id": "test-id",
				"choices": []map[string]interface{}{
					{
						"message": map[string]interface{}{
							"content": `{"risk_level": "Unsafe", "category_labels": "politics"}`,
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Mock normal response
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"id": "test-id",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"risk_level": "Safe"}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Enable = true
	cfg.SensitiveCheck.LLM.Endpoint = server.URL
	cfg.SensitiveCheck.LLM.APIKey = "test-key"
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	cfg.SensitiveCheck.LLM.TimeoutMS = 1000

	checker := NewOpenAILLMChecker(cfg)

	ctx := context.Background()
	res, err := checker.PassTextCheck(ctx, types.ScenarioCommentDetection, "hello world")
	require.NoError(t, err)
	require.False(t, res.IsSensitive)

	res, err = checker.PassTextCheck(ctx, types.ScenarioCommentDetection, "bad word")
	require.NoError(t, err)
	require.True(t, res.IsSensitive)
	require.Equal(t, `{"risk_level": "Unsafe", "category_labels": "politics"}`, res.Reason)
}

func TestOpenAILLMChecker_RetryOn429(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		// Success on 3rd try
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"id": "test-id",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"risk_level": "Safe"}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Enable = true
	cfg.SensitiveCheck.LLM.Endpoint = server.URL
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	cfg.SensitiveCheck.LLM.TimeoutMS = 1000

	checker := NewOpenAILLMChecker(cfg)

	start := time.Now()
	res, err := checker.PassTextCheck(context.Background(), types.ScenarioCommentDetection, "test")
	duration := time.Since(start)

	require.NoError(t, err)
	require.False(t, res.IsSensitive)
	require.Equal(t, 3, requests)                     // Should retry 2 times, total 3 requests
	require.True(t, duration >= 200*time.Millisecond) // 100ms + 200ms sleep
}

func TestOpenAILLMChecker_ChunkedTextCheck(t *testing.T) {
	var receivedTexts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody types.LLMReqBody
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		receivedTexts = append(receivedTexts, reqBody.Messages[0].Content)

		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{{
				"message": map[string]interface{}{
					"content": `{"risk_level": "Safe"}`,
				},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Endpoint = server.URL
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	cfg.SensitiveCheck.LLM.TimeoutMS = 1000
	cfg.SensitiveCheck.StreamContextCache.MaxChars = 10 // chunk size 10

	checker := NewOpenAILLMChecker(cfg)

	// Text length 25 should be split into 3 chunks: 10, 10, 5
	text := "1234567890123456789012345"
	res, err := checker.PassTextCheck(context.Background(), types.ScenarioCommentDetection, text)
	require.NoError(t, err)
	require.False(t, res.IsSensitive)

	require.Equal(t, 3, len(receivedTexts))
	require.Equal(t, "1234567890", receivedTexts[0])
	require.Equal(t, "1234567890", receivedTexts[1])
	require.Equal(t, "12345", receivedTexts[2])
}

func TestParseLLMResponse(t *testing.T) {
	parser := NewChainParser(SafetyRegex)

	tests := []struct {
		name     string
		content  string
		expected *CheckResult
	}{
		{
			name:     "QwenGuard Safe",
			content:  "Safety: Safe\nCategories: None",
			expected: &CheckResult{IsSensitive: false},
		},
		{
			name:     "QwenGuard Unsafe with categories",
			content:  "Safety: Unsafe\nCategories: Violent\nCategories: PII",
			expected: &CheckResult{IsSensitive: true, Reason: "Safety: Unsafe\nCategories: Violent\nCategories: PII"},
		},
		{
			name:     "valid json sensitive",
			content:  `{"risk_level": "Unsafe", "category_labels": "porn"}`,
			expected: &CheckResult{IsSensitive: true, Reason: `{"risk_level": "Unsafe", "category_labels": "porn"}`},
		},
		{
			name:     "valid json non-sensitive",
			content:  `{"risk_level": "Safe"}`,
			expected: &CheckResult{IsSensitive: false},
		},
		{
			name:     "markdown json block sensitive",
			content:  "```json\n{\"risk_level\": \"Unsafe\", \"category_labels\": \"politics\"}\n```",
			expected: &CheckResult{IsSensitive: true, Reason: `{"risk_level": "Unsafe", "category_labels": "politics"}`},
		},
		{
			name:     "fallback text sensitive 1",
			content:  "The content violates rules. risk_level: Unsafe",
			expected: &CheckResult{IsSensitive: false},
		},
		{
			name:     "fallback text non-sensitive",
			content:  "The content is safe. risk_level=Safe",
			expected: &CheckResult{IsSensitive: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := parser.Parse(tt.content)
			require.Equal(t, tt.expected.IsSensitive, res.IsSensitive)
			require.Equal(t, tt.expected.Reason, res.Reason)
		})
	}
}
