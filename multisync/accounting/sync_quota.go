package accounting

import (
	"bytes"
	"encoding/json"
	"net/http"
)

type GetSyncQuotaReq struct {
	AccessToken string `json:"access_token"`
}

type SyncQuota struct {
	RepoCountLimit int64  `json:"repo_count_limit"`
	TrafficLimit   int64  `json:"traffic_limit"`
	AccessToken    string `json:"-"`
	RepoCountUsed  int64  `json:"repo_count_used"`
	SpeedLimit     int64  `json:"speed_limit"`
	TrafficUsed    int64  `json:"traffic_used"`
}

type SyncQuotaRes struct {
	Message string    `json:"msg"`
	Data    SyncQuota `json:"data"`
}

type CreateSyncQuotaReq = SyncQuota

type UpdateSyncQuotaReq = SyncQuota

func (c *AccountingClient) CreateOrUpdateSyncQuota(opt *CreateSyncQuotaReq) (*Response, error) {
	header := http.Header{"content-type": []string{"application/json"}}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	if opt.AccessToken != "" {
		header.Add("Authorization", "Bearer "+opt.AccessToken)
	}
	_, resp, err := c.getResponse("POST", "/accounting/multisync/quotas", header, bytes.NewReader(body))
	return resp, err
}

func (c *AccountingClient) GetSyncQuota(opt *GetSyncQuotaReq) (*SyncQuota, *Response, error) {
	s := new(SyncQuotaRes)
	header := http.Header{}
	if opt.AccessToken != "" {
		header.Add("Authorization", "Bearer "+opt.AccessToken)
	}
	resp, err := c.getParsedResponse("GET", "/accounting/multisync/quota", header, nil, s)
	return &s.Data, resp, err
}
