package accounting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

type SyncQuotaStatement struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	RepoPath  string    `json:"repo_path"`
	RepoType  string    `json:"repo_type"`
	CreatedAt time.Time `json:"created_at"`
}

type GetSyncQuotaStatementsReq struct {
	UserID   int64  `json:"user_id"`
	RepoPath string `json:"repo_path"`
	RepoType string `json:"repo_type"`
}

type CreateSyncQuotaStatementReq = GetSyncQuotaStatementsReq

func (c *AccountingClient) CreateSyncQuotaStatement(opt *CreateSyncQuotaStatementReq) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("POST", "/accounting/multisync/quota/statements", jsonHeader, bytes.NewReader(body))
	return resp, err
}

func (c *AccountingClient) GetSyncQuotaStatement(opt *GetSyncQuotaStatementsReq) (*SyncQuotaStatement, *Response, error) {
	s := new(SyncQuotaStatement)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/accounting/multisync/%d/quota/statement", opt.UserID), nil, nil, s)
	return s, resp, err
}
