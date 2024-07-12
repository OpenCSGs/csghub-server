package accounting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type AccountingClient struct {
	remote    *url.URL
	client    *http.Client
	authToken string
}

func NewAccountingClient(config *config.Config) (*AccountingClient, error) {
	remoteURL := fmt.Sprintf("%s:%d", config.Accounting.Host, config.Accounting.Port)
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	return &AccountingClient{
		remote:    parsedURL,
		client:    http.DefaultClient,
		authToken: config.APIToken,
	}, nil
}

func (ac *AccountingClient) ListMeteringsByUserIDAndTime(req types.ACCT_STATEMENTS_REQ) (interface{}, error) {
	subUrlPath := fmt.Sprintf("/metering/%s/statements?current_user=%s&scene=%d&instance_name=%s&start_time=%s&end_time=%s&per=%d&page=%d", req.UserUUID, req.CurrentUser, req.Scene, req.InstanceName, url.QueryEscape(req.StartTime), url.QueryEscape(req.EndTime), req.Per, req.Page)
	return ac.handleResponse(ac.doRequest(http.MethodGet, subUrlPath, nil))
}

// Helper method to execute the actual HTTP request and read the response.
func (ac *AccountingClient) doRequest(method, subPath string, data interface{}) (*http.Response, error) {
	urlPath := fmt.Sprintf("%s%s%s", ac.remote, "/api/v1/accounting", subPath)
	// slog.Info("call", slog.Any("urlPath", urlPath))
	var buf io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, urlPath, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ac.authToken)

	resp, err := ac.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData interface{}
		err := json.NewDecoder(resp.Body).Decode(&errData)
		if err != nil {
			return nil, fmt.Errorf("unexpected http status code: %d, %w", resp.StatusCode, err)
		} else {
			return nil, fmt.Errorf("unexpected http status and error: %d, %v", resp.StatusCode, errData)
		}
	}

	return resp, nil
}

func (ac *AccountingClient) handleResponse(response *http.Response, err error) (interface{}, error) {
	if err != nil {
		return nil, err
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	var res struct {
		Msg  string      `json:"msg"`
		Data interface{} `json:"data"`
	}
	err = json.NewDecoder(response.Body).Decode(&res)
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}
