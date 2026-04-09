package sensitive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func TestOpenAILLMChecker_PassImageAndURLCheck(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Endpoint = "http://localhost"
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	checker := NewOpenAILLMChecker(cfg)

	ctx := context.Background()
	res, err := checker.PassImageCheck(ctx, types.ScenarioCommentDetection, "bucket", "obj")
	require.NoError(t, err)
	require.False(t, res.IsSensitive)

	res, err = checker.PassImageURLCheck(ctx, types.ScenarioCommentDetection, "http://example.com/img.png")
	require.NoError(t, err)
	require.False(t, res.IsSensitive)
}

func TestOpenAILLMChecker_PassLLMCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"id": "test-id",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `{"is_sensitive": false}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Endpoint = server.URL
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	cfg.SensitiveCheck.LLM.TimeoutMS = 1000

	checker := NewOpenAILLMChecker(cfg)

	ctx := context.Background()
	req := &types.LLMCheckRequest{
		Text:      "hello world",
		MaxTokens: 100,
	}
	res, err := checker.PassLLMCheck(ctx, req)
	require.NoError(t, err)
	require.False(t, res.IsSensitive)
}

func TestOpenAILLMChecker_doCheckEmptyText(t *testing.T) {
	cfg := &config.Config{}
	cfg.SensitiveCheck.LLM.Endpoint = "http://localhost"
	cfg.SensitiveCheck.LLM.GuardModel = "test-model"
	cfg.SensitiveCheck.LLM.GuardStreamModel = "test-stream-model"
	checker := NewOpenAILLMChecker(cfg)

	ctx := context.Background()
	req := &types.LLMCheckRequest{
		Text:      "",
		ModelName: "test-model",
	}
	res, err := checker.doCheck(ctx, req)
	require.NoError(t, err)
	require.False(t, res.IsSensitive)
}
