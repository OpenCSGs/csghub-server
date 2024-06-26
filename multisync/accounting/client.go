package accounting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"opencsg.com/csghub-server/common/config"
)

type AccountingClient struct {
	baseURL    string
	httpClient *http.Client
	mutex      sync.RWMutex
	ctx        context.Context
}

type Response struct {
	*http.Response
}

func NewAccountingClient(config *config.Config) (*AccountingClient, error) {
	if config.Accounting.Host == "" {
		return nil, fmt.Errorf("accounting host should be configured")
	}

	if config.Accounting.Port == 0 {
		return nil, fmt.Errorf("accounting port should be configured")
	}
	if config.APIToken == "" {
		return nil, fmt.Errorf("api token should be configured")
	}

	return &AccountingClient{
		baseURL: fmt.Sprintf("%s:%d", config.Accounting.Host, config.Accounting.Port),
		httpClient: &http.Client{
			Timeout: time.Second * 5,
		},
		ctx: context.Background(),
	}, nil
}

func (c *AccountingClient) getParsedResponse(method, path string, header http.Header, body io.Reader, obj interface{}) (*Response, error) {
	data, resp, err := c.getResponse(method, path, header, body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(data, obj)
}

func (c *AccountingClient) getResponse(method, path string, header http.Header, body io.Reader) ([]byte, *Response, error) {
	resp, err := c.doRequest(method, path, header, body)
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()

	// check for errors
	data, err := statusCodeToErr(resp)
	if err != nil {
		return data, resp, err
	}

	// success (2XX), read body
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}

	return data, resp, nil
}

// Converts a response for a HTTP status code indicating an error condition
// (non-2XX) to a well-known error value and response body. For non-problematic
// (2XX) status codes nil will be returned. Note that on a non-2XX response, the
// response body stream will have been read and, hence, is closed on return.
func statusCodeToErr(resp *Response) (body []byte, err error) {
	// no error
	if resp.StatusCode/100 == 2 {
		return nil, nil
	}

	//
	// error: body will be read for details
	//
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("body read on HTTP error %d: %v", resp.StatusCode, err)
	}

	// Try to unmarshal and get an error message
	errMap := make(map[string]interface{})
	if err = json.Unmarshal(data, &errMap); err != nil {
		// when the JSON can't be parsed, data was probably empty or a
		// plain string, so we try to return a helpful error anyway
		path := resp.Request.URL.Path
		method := resp.Request.Method
		header := resp.Request.Header
		return data, fmt.Errorf("unknown API Error: %d\nRequest: '%s' with '%s' method '%s' header and '%s' body", resp.StatusCode, path, method, header, string(data))
	}

	if msg, ok := errMap["message"]; ok {
		return data, fmt.Errorf("%v", msg)
	}

	// If no error message, at least give status and data
	return data, fmt.Errorf("%s: %s", resp.Status, string(data))
}

func (c *AccountingClient) doRequest(method, path string, header http.Header, body io.Reader) (*Response, error) {
	c.mutex.RLock()
	req, err := http.NewRequestWithContext(c.ctx, method, c.baseURL+"/api/v1"+path, body)
	if err != nil {
		c.mutex.RUnlock()
		return nil, err
	}

	client := c.httpClient
	c.mutex.RUnlock()

	for k, v := range header {
		req.Header[k] = v
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return &Response{resp}, nil
}
