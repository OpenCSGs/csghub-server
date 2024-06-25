package accounting

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type SyncQuotaStatement struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	RepoPath  string    `json:"repo_path"`
	RepoType  string    `json:"repo_type"`
	CreatedAt time.Time `json:"created_at"`
}

type SyncQuotaStatementRes struct {
	Message string             `json:"msg"`
	Data    SyncQuotaStatement `json:"data"`
}

type GetSyncQuotaStatementsReq struct {
	UserID      int64  `json:"user_id"`
	RepoPath    string `json:"repo_path"`
	RepoType    string `json:"repo_type"`
	AccessToken string `json:"access_token"`
}

type CreateSyncQuotaStatementReq = GetSyncQuotaStatementsReq

func (c *AccountingClient) CreateSyncQuotaStatement(opt *CreateSyncQuotaStatementReq) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	if opt.AccessToken != "" {
		jsonHeader.Add("Authorization", "Bearer "+opt.AccessToken)
	}
	_, resp, err := c.getResponse("POST", "/accounting/multisync/downloads", jsonHeader, bytes.NewReader(body))
	return resp, err
}

func (c *AccountingClient) GetSyncQuotaStatement(opt *GetSyncQuotaStatementsReq) (*SyncQuotaStatement, *Response, error) {
	s := new(SyncQuotaStatementRes)
	header := http.Header{}
	if opt.AccessToken != "" {
		header.Add("Authorization", "Bearer "+opt.AccessToken)
	}
	resp, err := c.getParsedResponse("GET", "/accounting/multisync/download", header, nil, s)
	return &s.Data, resp, err
}