package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// WithLLMConfig returns a new Client that calls the given LLMConfig's best
// available upstream through its declared API format: openai chat via the
// manual HTTP path, openai responses and anthropic messages via the
// corresponding official SDK. The config is only read, never mutated: the
// upstream selection is self-contained and mirrors the preference order of
// LLMConfig.PopulateDerivedFields, so one *LLMConfig can back concurrent
// clients and no upstream's model name can leak into another upstream's
// call. The original client is not modified, making this method safe for
// concurrent use.
func (c *Client) WithLLMConfig(llmConfig *database.LLMConfig) LLMSvcClient {
	return &Client{
		client:    c.client,
		llmConfig: llmConfig,
	}
}

// resolveLLMConfig overrides endpoint, host, headers, and model from the stored
// llmConfig (if set). It only serves the legacy path for configs without
// upstreams; configs with upstreams go through resolveUpstreamCall instead.
// Since WithLLMConfig creates a new Client per call, there is
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
	if c.llmConfig != nil && len(c.llmConfig.Upstreams) > 0 {
		return c.chatStreamWithUpstream(ctx, data)
	}
	endpoint, host, headers, data = c.resolveLLMConfig(endpoint, host, headers, data)
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, strings.TrimSpace(endpoint), host, headers, data)
	if err != nil {
		return nil, fmt.Errorf("do llm stream request, error: %w", err)
	}

	return c.readToChannel(ctx, rc), nil
}

// chatStreamWithUpstream resolves the selected upstream's native protocol and
// dispatches the stream call: declared protocols (chat/responses/messages) go
// through the corresponding official SDK, while an upstream without any
// protocol declaration keeps the legacy manual HTTP passthrough.
func (c *Client) chatStreamWithUpstream(ctx context.Context, data types.LLMReqBody) (<-chan string, error) {
	call, err := resolveUpstreamCall(c.llmConfig)
	if err != nil {
		return nil, err
	}
	if call.model != "" {
		data.Model = call.model
	}
	slog.Debug("chat stream with llm", slog.Any("protocol", call.protocol), slog.Any("endpoint", call.endpoint), slog.Any("data", data))
	switch call.protocol {
	case types.ApiFormatResponses:
		return c.chatStreamViaResponses(ctx, call, data)
	case types.ApiFormatMessages:
		return c.chatStreamViaMessages(ctx, call, data)
	case types.ApiFormatChat:
		return c.chatStreamViaChatCompletions(ctx, call, data)
	case "":
		// Undeclared upstream: legacy manual passthrough of the openai
		// chat wire format to the URL as-is.
		rc, err := c.doRequest(ctx, http.MethodPost, call.endpoint, call.host, call.headers, data)
		if err != nil {
			return nil, fmt.Errorf("do llm stream request, error: %w", err)
		}
		return c.readToChannel(ctx, rc), nil
	default:
		// Unreachable today: buildUpstreamCall only yields the empty
		// passthrough value, chat, responses, or messages. Guard against a
		// new callable protocol being added without a dispatch case —
		// sending a chat wire format to a multimodal or jev upstream would
		// only fail confusingly upstream.
		slog.Error("unsupported api format for internal llm stream call",
			slog.String("protocol", string(call.protocol)),
			slog.String("endpoint", call.endpoint))
		return nil, fmt.Errorf("unsupported api format %q for internal llm stream calls", call.protocol)
	}
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

func (c *Client) readToChannel(ctx context.Context, rc io.ReadCloser) <-chan string {
	output := make(chan string, 2)
	br := bufio.NewReader(rc)

	go func() {
		defer close(output)
		defer rc.Close()
		for {
			line, _, err := br.ReadLine()
			if err != nil {
				// A clean EOF ends the stream; any other read failure is
				// surfaced to the consumer as an openai error line so a
				// mid-stream abort is not mistaken for a clean end.
				if !errors.Is(err, io.EOF) {
					slog.Warn("remote reader aborted", slog.Any("error", err))
					emitStreamError(ctx, output, err)
				}
				return
			}
			if len(line) == 0 {
				continue
			}
			// Abort on ctx cancellation so a consumer that stops reading
			// cannot leak this goroutine.
			select {
			case output <- string(line):
			case <-ctx.Done():
				return
			}
		}
	}()

	return output
}

func (c *Client) Chat(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (string, error) {
	if c.llmConfig != nil && len(c.llmConfig.Upstreams) > 0 {
		return c.chatWithUpstream(ctx, data)
	}
	endpoint, host, headers, data = c.resolveLLMConfig(endpoint, host, headers, data)
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, host, headers, data)
	if err != nil {
		return "", fmt.Errorf("do llm request, error: %w", err)
	}

	return c.decodeChatCompletion(rc)
}

// chatWithUpstream resolves the selected upstream's native protocol and
// dispatches the call: declared protocols (chat/responses/messages) go
// through the corresponding official SDK, while an upstream without any
// protocol declaration keeps the legacy manual HTTP passthrough.
func (c *Client) chatWithUpstream(ctx context.Context, data types.LLMReqBody) (string, error) {
	call, err := resolveUpstreamCall(c.llmConfig)
	if err != nil {
		return "", err
	}
	if call.model != "" {
		data.Model = call.model
	}
	slog.Debug("chat with llm", slog.Any("protocol", call.protocol), slog.Any("endpoint", call.endpoint), slog.Any("data", data))
	switch call.protocol {
	case types.ApiFormatResponses:
		return c.chatViaResponses(ctx, call, data)
	case types.ApiFormatMessages:
		return c.chatViaMessages(ctx, call, data)
	case types.ApiFormatChat:
		return c.chatViaChatCompletions(ctx, call, data)
	case "":
		// Undeclared upstream: legacy manual passthrough of the openai
		// chat wire format to the URL as-is.
		rc, err := c.doRequest(ctx, http.MethodPost, call.endpoint, call.host, call.headers, data)
		if err != nil {
			return "", fmt.Errorf("do llm request, error: %w", err)
		}
		return c.decodeChatCompletion(rc)
	default:
		// Unreachable today: buildUpstreamCall only yields the empty
		// passthrough value, chat, responses, or messages. Guard against a
		// new callable protocol being added without a dispatch case —
		// sending a chat wire format to a multimodal or jev upstream would
		// only fail confusingly upstream.
		slog.Error("unsupported api format for internal llm call",
			slog.String("protocol", string(call.protocol)),
			slog.String("endpoint", call.endpoint))
		return "", fmt.Errorf("unsupported api format %q for internal llm calls", call.protocol)
	}
}

// decodeChatCompletion parses an openai chat completion response body and
// returns the message content.
func (c *Client) decodeChatCompletion(rc io.ReadCloser) (string, error) {
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
		return "", errInvalidSummary
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
