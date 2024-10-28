package userservice

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

type UserServiceClient struct {
	remote    *url.URL
	client    *http.Client
	authToken string
}

func NewUserServiceClient(config *config.Config) (*UserServiceClient, error) {
	remoteURL := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	parsedURL, err := url.Parse(remoteURL)
	if err != nil {
		return nil, err
	}
	return &UserServiceClient{
		remote:    parsedURL,
		client:    http.DefaultClient,
		authToken: config.APIToken,
	}, nil
}

func (ac *UserServiceClient) AddBalance(req types.UpdateBalanceRequest) (interface{}, error) {
	subUrlPath := fmt.Sprintf("/user/%s/balance?current_user=%s", req.VisitorName, req.CurrentUser)
	return ac.handleResponse(ac.doRequest(http.MethodPut, subUrlPath, req))
}

// Helper method to execute the actual HTTP request and read the response.
func (ac *UserServiceClient) doRequest(method, subPath string, data interface{}) (*http.Response, error) {
	urlPath := fmt.Sprintf("%s%s%s", ac.remote, "/api/v1", subPath)
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

func (ac *UserServiceClient) handleResponse(response *http.Response, err error) (interface{}, error) {
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
