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
	Tokenize(ctx context.Context, headers map[string]string, endpoint, framework, model, content string) (int64, error)
}

type Client struct {
	client *http.Client
}

func NewClient() *Client {
	return &Client{
		client: http.DefaultClient,
	}
}

func (c *Client) ChatStream(ctx context.Context, endpoint string, headers map[string]string, data types.LLMReqBody) (<-chan string, error) {
	slog.Debug("chat with llm", slog.Any("endpoint", endpoint), slog.Any("data", data))
	rc, err := c.doRequest(ctx, http.MethodPost, endpoint, headers, data)
	if err != nil {
		return nil, fmt.Errorf("do llm stream request, error: %w", err)
	}

	return c.readToChannel(rc), nil
}

func (c *Client) doRequest(ctx context.Context, method, url string, headers map[string]string, data interface{}) (io.ReadCloser, error) {
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

func (c *Client) Tokenize(ctx context.Context, headers map[string]string, endpoint, framework, model, content string) (int64, error) {
	const path = "/tokenize"
	switch framework {
	case "VLLM":
		req := &VllmGPUTokenizeReq{
			Model:  model,
			Prompt: content,
		}
		var resp VllmGPUTokenizeResponse

		rc, err := c.doRequest(ctx, http.MethodPost, endpoint+path, headers, req)
		if err != nil {
			return 0, fmt.Errorf("do llm request, error: %w", err)
		}
		bodyBytes, err := io.ReadAll(rc)
		if err != nil {
			return 0, fmt.Errorf("read llm response body, error: %w", err)
		}
		defer rc.Close()
		bodyStr := string(bodyBytes)
		slog.Debug("Response body", slog.String("body", bodyStr))
		err = json.Unmarshal(bodyBytes, &resp)
		if err != nil {
			return 0, err
		}
		return resp.Count, nil
	case "llama.cpp":
		req := &LlamacppTokenizeReq{
			Content: content,
		}
		var resp LlamacppTokenizeResponse

		rc, err := c.doRequest(ctx, http.MethodPost, endpoint+path, headers, req)
		if err != nil {
			return 0, fmt.Errorf("do llm request, error: %w", err)
		}
		bodyBytes, err := io.ReadAll(rc)
		if err != nil {
			return 0, fmt.Errorf("read llm response body, error: %w", err)
		}
		defer rc.Close()
		bodyStr := string(bodyBytes)
		slog.Debug("Response body", slog.String("body", bodyStr))
		err = json.Unmarshal(bodyBytes, &resp)
		if err != nil {
			return 0, err
		}
		return int64(len(resp.Tokens)), nil
	case "TGI":
		req := &TGITokenizeReq{
			Inputs: content,
		}
		var resp []TGITokenizeResponse

		rc, err := c.doRequest(ctx, http.MethodPost, endpoint+path, headers, req)
		if err != nil {
			return 0, fmt.Errorf("do llm request, error: %w", err)
		}
		bodyBytes, err := io.ReadAll(rc)
		if err != nil {
			return 0, fmt.Errorf("read llm response body, error: %w", err)
		}
		defer rc.Close()
		bodyStr := string(bodyBytes)
		slog.Debug("Response body", slog.String("body", bodyStr))
		err = json.Unmarshal(bodyBytes, &resp)
		if err != nil {
			return 0, err
		}
		return int64(len(resp)), nil
	case "SGLang":
		slog.Error("call SGLang framework, no tokenize")
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown runtime_framework:%s", framework)
	}
}
