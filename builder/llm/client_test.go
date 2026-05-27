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
