package rpc

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestModerationSvcHttpClient_PassTextCheck(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/text", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Scenario string `json:"scenario"`
			Text     string `json:"text"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test_scenario", req.Scenario)
		assert.Equal(t, "test_text", req.Text)

		resp := httpbase.R{
			Data: CheckResult{
				IsSensitive: true,
				Reason:      "test_reason",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	res, err := client.PassTextCheck(context.Background(), "test_scenario", "test_text")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.IsSensitive)
	assert.Equal(t, "test_reason", res.Reason)
}

func TestModerationSvcHttpClient_PassImageCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/image", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Scenario      string `json:"scenario"`
			OssBucketName string `json:"oss_bucket_name"`
			OssObjectName string `json:"oss_object_name"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test_scenario", req.Scenario)
		assert.Equal(t, "test_bucket", req.OssBucketName)
		assert.Equal(t, "test_object", req.OssObjectName)

		resp := httpbase.R{
			Data: CheckResult{
				IsSensitive: false,
				Reason:      "",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	res, err := client.PassImageCheck(context.Background(), "test_scenario", "test_bucket", "test_object")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.IsSensitive)
	assert.Empty(t, res.Reason)
}

func TestModerationSvcHttpClient_PassLLMRespCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/llmresp", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Service           string `json:"Service"`
			ServiceParameters struct {
				Content   string `json:"content"`
				SessionId string `json:"sessionId"`
			} `json:"ServiceParameters"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, string(sensitive.ScenarioLLMResModeration), req.Service)
		assert.Equal(t, "test_text", req.ServiceParameters.Content)
		assert.Equal(t, "test_session", req.ServiceParameters.SessionId)

		resp := httpbase.R{
			Data: CheckResult{
				IsSensitive: true,
				Reason:      "sensitive content detected",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	res, err := client.PassLLMRespCheck(context.Background(), "test_text", "test_session")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.IsSensitive)
	assert.Equal(t, "sensitive content detected", res.Reason)
}

func TestModerationSvcHttpClient_PassLLMPromptCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/llmprompt", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Service           string `json:"Service"`
			ServiceParameters struct {
				Content   string `json:"content"`
				SessionId string `json:"sessionId"`
			} `json:"ServiceParameters"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "llm_query_moderation", req.Service)
		assert.Equal(t, "test_prompt", req.ServiceParameters.Content)
		assert.Equal(t, "test_account", req.ServiceParameters.SessionId)

		resp := httpbase.R{
			Data: CheckResult{
				IsSensitive: false,
				Reason:      "",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	res, err := client.PassLLMPromptCheck(context.Background(), "test_prompt", "test_account")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.IsSensitive)
	assert.Empty(t, res.Reason)
}
func TestModerationSvcHttpClient_SubmitRepoCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repo", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			RepoType  types.RepositoryType `json:"repo_type"`
			Namespace string               `json:"namespace"`
			Name      string               `json:"name"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, types.ModelRepo, req.RepoType)
		assert.Equal(t, "test_namespace", req.Namespace)
		assert.Equal(t, "test_name", req.Name)
		resp := httpbase.R{}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	err := client.SubmitRepoCheck(context.Background(), types.ModelRepo, "test_namespace", "test_name")
	assert.NoError(t, err)
}

func TestModerationSvcHttpClient_PassImageURLCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/image", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req struct {
			Scenario string `json:"scenario"`
			ImageURL string `json:"image_url"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "test_scenario", req.Scenario)
		assert.Equal(t, "test_image_url", req.ImageURL)

		resp := httpbase.R{
			Data: CheckResult{
				IsSensitive: false,
				Reason:      "",
			},
		}
		err = json.NewEncoder(w).Encode(resp)
		assert.NoError(t, err)
	}))
	hc := &HttpClient{
		endpoint: server.URL,
		hc:       server.Client(),
		logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	defer server.Close()

	client := &ModerationSvcHttpClient{
		hc: hc,
	}
	res, err := client.PassImageURLCheck(context.Background(), "test_scenario", "test_image_url")
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.IsSensitive)
	assert.Empty(t, res.Reason)
}
