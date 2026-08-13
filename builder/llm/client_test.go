package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type mockHttpDoer struct {
	mock.Mock
}

func (m *mockHttpDoer) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	resp := args.Get(0)
	if resp == nil {
		return nil, args.Error(1)
	}
	return resp.(*http.Response), args.Error(1)
}

func TestNewClient(t *testing.T) {
	client := NewClient()
	require.NotNil(t, client)
	require.NotNil(t, client.client)
}

func TestClient_ChatStream(t *testing.T) {
	t.Run("successful stream", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		body := `{"id":"1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"hello"}}]}`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body + "\n" + body)),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		ch, err := c.ChatStream(context.Background(), "http://example.com/chat", "example.com", nil, types.LLMReqBody{
			Model: "test-model",
			Messages: []types.LLMMessage{
				{Role: "user", Content: "hello"},
			},
		})
		require.NoError(t, err)

		var results []string
		for msg := range ch {
			results = append(results, msg)
		}
		require.Len(t, results, 2)
		assert.Equal(t, body, results[0])
		assert.Equal(t, body, results[1])
	})

	t.Run("http request error", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		mockDoer.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()

		_, err := c.ChatStream(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "do llm stream request")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(""))}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.ChatStream(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected http status code")
	})

	t.Run("verify host header is set via req.Host", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("data: hello\n")),
		}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Host == "my-custom-host.com"
		})).Return(resp, nil).Once()

		_, err := c.ChatStream(context.Background(), "http://example.com/chat", "my-custom-host.com", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.NoError(t, err)
	})
}

func TestClient_Chat(t *testing.T) {
	t.Run("successful chat", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		chatResp := types.ChatCompletion{
			ID: "chatcmpl-123",
			Choices: []types.Choice{
				{
					Index: 0,
					Message: types.Message{
						Role:    "assistant",
						Content: "Hello! How can I help you?",
					},
				},
			},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		result, err := c.Chat(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
			Messages: []types.LLMMessage{
				{Role: "user", Content: "hi"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Hello! How can I help you?", result)
	})

	t.Run("http request error", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		mockDoer.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()

		_, err := c.Chat(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "do llm request")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(""))}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.Chat(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected http status code")
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`invalid json`)),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.Chat(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode llm response")
	})

	t.Run("empty choices response", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		chatResp := types.ChatCompletion{
			ID:      "chatcmpl-123",
			Choices: []types.Choice{},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.Chat(context.Background(), "http://example.com/chat", "", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "summary of conversation is invalid")
	})

	t.Run("verify host header is set via req.Host", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		chatResp := types.ChatCompletion{
			ID: "chatcmpl-123",
			Choices: []types.Choice{
				{Index: 0, Message: types.Message{Role: "assistant", Content: "ok"}},
			},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Host == "my-host"
		})).Return(resp, nil).Once()

		result, err := c.Chat(context.Background(), "http://example.com/chat", "my-host", nil, types.LLMReqBody{
			Model: "test-model",
		})
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
	})
}

func TestClient_Tokenize(t *testing.T) {
	t.Run("successful tokenize", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		expected := `{"count":5,"max_model_len":4096,"tokens":[1,2,3,4,5]}`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expected)),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		result, err := c.Tokenize(context.Background(), "http://example.com/tokenize", "", nil)
		require.NoError(t, err)
		assert.JSONEq(t, expected, string(result))
	})

	t.Run("http request error", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		mockDoer.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()

		_, err := c.Tokenize(context.Background(), "http://example.com/tokenize", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "do llm request")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusServiceUnavailable, Body: io.NopCloser(strings.NewReader(""))}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.Tokenize(context.Background(), "http://example.com/tokenize", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected http status code")
	})

	t.Run("verify host header is set via req.Host", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
		}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Host == "tokenize-host"
		})).Return(resp, nil).Once()

		_, err := c.Tokenize(context.Background(), "http://example.com/tokenize", "tokenize-host", nil)
		require.NoError(t, err)
	})
}

func TestClient_EmbeddingTokenize(t *testing.T) {
	t.Run("successful embedding tokenize", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		expected := `[[{"id":0,"text":"hello","special":false}]]`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(expected)),
		}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return strings.HasSuffix(req.URL.String(), "/tokenize")
		})).Return(resp, nil).Once()

		result, err := c.EmbeddingTokenize(context.Background(), "http://example.com/embed", "", nil)
		require.NoError(t, err)
		assert.JSONEq(t, expected, string(result))
	})

	t.Run("http request error", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		mockDoer.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()

		_, err := c.EmbeddingTokenize(context.Background(), "http://example.com/embed", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "do llm request")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader(""))}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.EmbeddingTokenize(context.Background(), "http://example.com/embed", "", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected http status code")
	})

	t.Run("verify /tokenize path appended", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[]`)),
		}
		var capturedReq *http.Request
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			capturedReq = req
			return true
		})).Return(resp, nil).Once()

		_, err := c.EmbeddingTokenize(context.Background(), "http://example.com/embed", "", nil)
		require.NoError(t, err)
		assert.Equal(t, "http://example.com/embed/tokenize", capturedReq.URL.String())
	})

	t.Run("verify host header is set via req.Host", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`[]`)),
		}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Host == "embed-host"
		})).Return(resp, nil).Once()

		_, err := c.EmbeddingTokenize(context.Background(), "http://example.com/embed", "embed-host", nil)
		require.NoError(t, err)
	})
}

func TestClient_doRequest(t *testing.T) {
	t.Run("set content-type and connection headers", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}
		var capturedReq *http.Request
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			capturedReq = req
			return true
		})).Return(resp, nil).Once()

		_, err := c.doRequest(context.Background(), http.MethodPost, "http://example.com/test", "", map[string]string{"key": "val"}, nil)
		require.NoError(t, err)
		assert.Equal(t, "application/json", capturedReq.Header.Get("Content-Type"))
		assert.Equal(t, "keep-alive", capturedReq.Header.Get("Connection"))
		assert.Equal(t, "val", capturedReq.Header.Get("key"))
	})

	t.Run("host is empty -> req.Host defaults to URL host", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Host == "example.com"
		})).Return(resp, nil).Once()

		_, err := c.doRequest(context.Background(), http.MethodGet, "http://example.com/test", "", nil, nil)
		require.NoError(t, err)
	})

	t.Run("nil data should produce nil body", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Body == nil || req.Body == http.NoBody
		})).Return(resp, nil).Once()

		_, err := c.doRequest(context.Background(), http.MethodGet, "http://example.com/test", "", nil, nil)
		require.NoError(t, err)
	})

	t.Run("json marshal error", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		_, err := c.doRequest(context.Background(), http.MethodPost, "http://example.com/test", "", nil, make(chan int))
		require.Error(t, err)
	})
}

func TestClient_readToChannel(t *testing.T) {
	t.Run("read multiple lines", func(t *testing.T) {
		c := &Client{}
		rc := io.NopCloser(strings.NewReader("line1\nline2\nline3\n"))
		ch := c.readToChannel(rc)

		var lines []string
		for msg := range ch {
			lines = append(lines, msg)
		}
		assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
	})

	t.Run("skip empty lines", func(t *testing.T) {
		c := &Client{}
		rc := io.NopCloser(strings.NewReader("line1\n\nline2\n\n"))
		ch := c.readToChannel(rc)

		var lines []string
		for msg := range ch {
			lines = append(lines, msg)
		}
		assert.Equal(t, []string{"line1", "line2"}, lines)
	})

	t.Run("empty input closes channel immediately", func(t *testing.T) {
		c := &Client{}
		rc := io.NopCloser(strings.NewReader(""))
		ch := c.readToChannel(rc)

		var lines []string
		for msg := range ch {
			lines = append(lines, msg)
		}
		assert.Empty(t, lines)
	})
}

func TestClient_WithLLMConfig(t *testing.T) {
	t.Run("chat uses llmConfig endpoint headers and model", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		llmConfig := &database.LLMConfig{
			ModelName:   "upstream-model",
			ApiEndpoint: "http://upstream.example.com/chat",
			AuthHeader:  `{"Authorization":"Bearer token123"}`,
			Upstreams: []database.Upstream{
				{
					URL:        "http://upstream.example.com/chat",
					Enabled:    true,
					ModelName:  "upstream-model",
					AuthHeader: `{"Authorization":"Bearer token123"}`,
					Provider:   "test-provider",
				},
			},
		}

		chatResp := types.ChatCompletion{
			ID: "chatcmpl-456",
			Choices: []types.Choice{
				{
					Index: 0,
					Message: types.Message{
						Role:    "assistant",
						Content: "Response from upstream",
					},
				},
			},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}

		var capturedReq *http.Request
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			capturedReq = req
			return true
		})).Return(resp, nil).Once()

		result, err := c.WithLLMConfig(llmConfig).Chat(context.Background(), "http://should-be-overridden", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{
				{Role: "user", Content: "hello"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Response from upstream", result)

		// Verify the request used the upstream endpoint
		assert.Equal(t, "http://upstream.example.com/chat", capturedReq.URL.String())
		// Verify the request used the upstream auth header
		assert.Equal(t, "Bearer token123", capturedReq.Header.Get("Authorization"))

		// Verify the original client was not modified (WithLLMConfig creates a new client)
		assert.Nil(t, c.llmConfig)
	})

	t.Run("chat without llmConfig uses explicit params", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		chatResp := types.ChatCompletion{
			ID: "chatcmpl-789",
			Choices: []types.Choice{
				{
					Index: 0,
					Message: types.Message{
						Role:    "assistant",
						Content: "Normal response",
					},
				},
			},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}

		var capturedReq *http.Request
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			capturedReq = req
			return true
		})).Return(resp, nil).Once()

		headers := map[string]string{"X-Custom": "val"}
		result, err := c.Chat(context.Background(), "http://explicit.example.com", "", headers, types.LLMReqBody{
			Model: "explicit-model",
			Messages: []types.LLMMessage{
				{Role: "user", Content: "hi"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "Normal response", result)
		assert.Equal(t, "http://explicit.example.com", capturedReq.URL.String())
		assert.Equal(t, "val", capturedReq.Header.Get("X-Custom"))
	})

	t.Run("WithLLMConfig does not mutate original client", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		llmConfig := &database.LLMConfig{
			ModelName:   "upstream-model",
			ApiEndpoint: "http://upstream.example.com/chat",
			AuthHeader:  `{"Authorization":"Bearer token"}`,
			Upstreams: []database.Upstream{
				{
					URL:        "http://upstream.example.com/chat",
					Enabled:    true,
					ModelName:  "upstream-model",
					AuthHeader: `{"Authorization":"Bearer token"}`,
				},
			},
		}

		// WithLLMConfig should return a new client, not modify the original
		configured := c.WithLLMConfig(llmConfig)
		assert.Nil(t, c.llmConfig, "original client should not have llmConfig set")

		// First call with WithLLMConfig
		chatResp1 := types.ChatCompletion{
			Choices: []types.Choice{
				{Index: 0, Message: types.Message{Role: "assistant", Content: "first"}},
			},
		}
		data1, _ := json.Marshal(chatResp1)
		resp1 := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data1))),
		}
		mockDoer.On("Do", mock.Anything).Return(resp1, nil).Once()

		_, err := configured.Chat(context.Background(), "", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "first"}},
		})
		require.NoError(t, err)

		// Second call without WithLLMConfig on the original client should use explicit params
		chatResp2 := types.ChatCompletion{
			Choices: []types.Choice{
				{Index: 0, Message: types.Message{Role: "assistant", Content: "second"}},
			},
		}
		data2, _ := json.Marshal(chatResp2)
		resp2 := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data2))),
		}

		var capturedReq *http.Request
		mockDoer.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			capturedReq = req
			return true
		})).Return(resp2, nil).Once()

		_, err = c.Chat(context.Background(), "http://explicit.example.com", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "second"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "http://explicit.example.com", capturedReq.URL.String())
	})

	t.Run("with nil llmConfig is no-op", func(t *testing.T) {
		mockDoer := new(mockHttpDoer)
		c := &Client{client: mockDoer}

		chatResp := types.ChatCompletion{
			Choices: []types.Choice{
				{Index: 0, Message: types.Message{Role: "assistant", Content: "ok"}},
			},
		}
		data, _ := json.Marshal(chatResp)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
		}
		mockDoer.On("Do", mock.Anything).Return(resp, nil).Once()

		_, err := c.WithLLMConfig(nil).Chat(context.Background(), "http://explicit.example.com", "", nil, types.LLMReqBody{
			Messages: []types.LLMMessage{{Role: "user", Content: "hi"}},
		})
		require.NoError(t, err)
	})
}
