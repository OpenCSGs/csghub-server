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

	"opencsg.com/csghub-server/common/types"
)

type LLMSvcClient interface {
	Tokenize(ctx context.Context, endpoint, host string, req interface{}) ([]byte, error)
}

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	return &Client{
		client: http.DefaultClient,
	}
}

func (c *Client) ChatStream(ctx context.Context, endpoint, host string, headers map[string]string, data types.LLMReqBody) (<-chan string, error) {
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, host, headers, data)
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
		req.Header.Set("Host", host)
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
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, host, nil, req)
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
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint+path, host, nil, req)
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
