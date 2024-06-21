package types

type ACCT_QUOTA_REQ struct {
	RepoCountLimit int64 `json:"repo_count_limit"`
	SpeedLimit     int64 `json:"speed_limit"`
	TrafficLimit   int64 `json:"traffic_limit"`
}

type ACCT_QUOTA_STATEMENT_REQ struct {
	RepoPath string `json:"repo_path"`
	RepoType string `json:"repo_type"`
}
