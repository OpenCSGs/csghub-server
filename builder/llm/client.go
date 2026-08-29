package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"

	"opencsg.com/csghub-server/common/types"
)

type LLMSvcClient interface {
	WithLLMConfig(llmConfig *database.LLMConfig) LLMSvcClient
	Chat(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (string, error)
	ChatStream(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (<-chan string, error)
	Tokenize(ctx context.Context, endpoint, host string, req interface{}) ([]byte, error)
	EmbeddingTokenize(ctx context.Context, endpoint, host string, req interface{}) ([]byte, error)
}

type Client struct {
	client    rpc.HttpDoer
	llmConfig *database.LLMConfig
}

func NewClient() *Client {
	return &Client{
		client: rpc.NewHttpClient("").WithRetry(2),
	}
}

// WithLLMConfig returns a new Client that derives endpoint, headers, and model name
// from the given LLMConfig's best available upstream. PopulateDerivedFields is called
// internally to select the best upstream. The original client is not modified, making
// this method safe for concurrent use.
func (c *Client) WithLLMConfig(llmConfig *database.LLMConfig) LLMSvcClient {
	if llmConfig != nil {
		llmConfig.PopulateDerivedFields()
	}
	return &Client{
		client:    c.client,
		llmConfig: llmConfig,
	}
}

// resolveLLMConfig overrides endpoint, host, headers, and model from the stored
// llmConfig (if set). Since WithLLMConfig creates a new Client per call, there is
// no need to clear llmConfig after use.
func (c *Client) resolveLLMConfig(endpoint, host string, headers map[string]string, data types.LLMReqBody) (string, string, map[string]string, types.LLMReqBody) {
	if c.llmConfig == nil {
		return endpoint, host, headers, data
	}
	cfg := c.llmConfig

	resolvedHeaders := headers
	if len(cfg.AuthHeader) > 0 {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(cfg.AuthHeader), &parsed); err == nil {
			resolvedHeaders = parsed
		} else {
			slog.Error("failed to parse llm config auth header json, request will proceed without resolved headers",
				slog.Any("error", err), slog.String("model_name", cfg.ModelName))
		}
	}
	if cfg.ModelName != "" {
		data.Model = cfg.ModelName
	}
	return cfg.ApiEndpoint, host, resolvedHeaders, data
}

func (c *Client) ChatStream(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (<-chan string, error) {
	endpoint, host, headers, data = c.resolveLLMConfig(endpoint, host, headers, data)
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, strings.TrimSpace(endpoint), host, headers, data)
	if err != nil {
		return nil, fmt.Errorf("do llm stream request, error: %w", err)
	}

	return c.readToChannel(rc), nil
}

func (c *Client) doRequest(ctx context.Context, method, url, host string, headers map[string]string, data interface{}) (io.ReadCloser, error) {
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "keep-alive")
	if len(host) > 0 {
		req.Host = host
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected http status code:%d", resp.StatusCode)
	}

	return resp.Body, nil
}

func (c *Client) readToChannel(rc io.ReadCloser) <-chan string {
	output := make(chan string, 2)
	br := bufio.NewReader(rc)

	go func() {
		for {
			line, _, err := br.ReadLine()
			if err != nil {
				slog.Warn("remote reader aborted", slog.Any("error", err))
				rc.Close()
				close(output)
				break
			}
			if len(line) > 0 {
				output <- string(line)
			}
		}
	}()

	return output
}

func (c *Client) Chat(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (string, error) {
	endpoint, host, headers, data = c.resolveLLMConfig(endpoint, host, headers, data)
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, host, headers, data)
	if err != nil {
		return "", fmt.Errorf("do llm request, error: %w", err)
	}

	bodyBytes, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read llm response body, error: %w", err)
	}
	defer rc.Close()

	bodyStr := string(bodyBytes)
	slog.Debug("Response body", slog.String("body", bodyStr))

	var chatCompletion types.ChatCompletion

	err = json.Unmarshal(bodyBytes, &chatCompletion)
	if err != nil {
		return "", fmt.Errorf("decode llm response, error: %w", err)
	}

	if len(chatCompletion.Choices) == 0 || len(chatCompletion.Choices[0].Message.Content) == 0 {
		return "", fmt.Errorf("summary of conversation is invalid")
	}

	return chatCompletion.Choices[0].Message.Content, nil
}

func (c *Client) Tokenize(ctx context.Context, endpoint, host string, req interface{}) ([]byte, error) {
	endpoint, host, headers, _ := c.resolveLLMConfig(endpoint, host, nil, types.LLMReqBody{})
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, host, headers, req)
	if err != nil {
		return nil, fmt.Errorf("do llm request, error: %w", err)
	}
	bodyBytes, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read llm response body, error: %w", err)
	}
	defer rc.Close()
	return bodyBytes, nil
}

func (c *Client) EmbeddingTokenize(ctx context.Context, endpoint, host string, req interface{}) ([]byte, error) {
	const path = "/tokenize"
	endpoint, host, headers, _ := c.resolveLLMConfig(endpoint, host, nil, types.LLMReqBody{})
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint+path, host, headers, req)
	if err != nil {
		return nil, fmt.Errorf("do llm request, error: %w", err)
	}
	bodyBytes, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read llm response body, error: %w", err)
	}
	defer rc.Close()
	return bodyBytes, nil
}
