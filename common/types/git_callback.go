package types

type GiteaCallbackPushReq struct {
	Ref        string                          `json:"ref"`
	Commits    []GiteaCallbackPushReq_Commit   `json:"commits"`
	Repository GiteaCallbackPushReq_Repository `json:"repository"`
	HeadCommit GiteaCallbackPushReq_HeadCommit `json:"head_commit"`
}

type GiteaCallbackPushReq_Commit struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Modified []string `json:"modified"`
}

type GiteaCallbackPushReq_Repository struct {
	FullName string `json:"full_name"`
}

type GiteaCallbackPushReq_HeadCommit struct {
	Timestamp string `json:"timestamp"`
}
