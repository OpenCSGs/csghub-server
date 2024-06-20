package accounting

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type GetSyncQuotaReq struct {
	UserID int64 `json:"user_id"`
}

type SyncQuota struct {
	UserID         int64 `json:"user_id"`
	RepoCountLimit int64 `json:"repo_count_limit"`
	TrafficLimit   int64 `json:"traffic_limit"`
}

type CreateSyncQuotaReq = SyncQuota

type UpdateSyncQuotaReq = SyncQuota

func (c *AccountingClient) CreateSyncQuota(opt *CreateSyncQuotaReq) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("POST", "/accounting/multisync/quotas", jsonHeader, bytes.NewReader(body))
	return resp, err
}

func (c *AccountingClient) UpdateSyncQuota(opt *CreateSyncQuotaReq) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/accounting/multisync/%d/quotas", opt.UserID), jsonHeader, bytes.NewReader(body))
	return resp, err
}

func (c *AccountingClient) GetSyncQuota(opt *GetSyncQuotaReq) (*SyncQuota, *Response, error) {
	s := new(SyncQuota)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/accounting/multisync/%d/quota", opt.UserID), nil, nil, s)
	return s, resp, err
}
