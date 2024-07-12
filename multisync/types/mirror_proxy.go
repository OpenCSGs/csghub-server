package types

type GetSyncQuotaStatementReq struct {
	RepoPath    string `json:"repo_path"`
	RepoType    string `json:"repo_type"`
	AccessToken string `json:"access_token"`
}
