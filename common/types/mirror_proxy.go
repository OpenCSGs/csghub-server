package types

type GetSyncQuotaStatementReq struct {
	Token    string `json:"token"`
	RepoPath string `json:"repo_path"`
	RepoType string `json:"repo_type"`
}
