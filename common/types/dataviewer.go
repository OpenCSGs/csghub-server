package types

const (
	WorkflowPending = iota
	WorkflowRunning = 1
	WorkflowDone    = 2
	WorkflowFailed  = 3
)

const (
	WorkflowMsgPending = "Pending"
	WorkflowMsgRunning = "The dataset viewer should be available soon."
	WorkflowMsgDone    = "Done"
)

type UpdateViewerReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Branch      string         `json:"branch"`
	CurrentUser string         `json:"current_user"`
	RepoType    RepositoryType `json:"repo_type"`
	RepoID      int64          `json:"repo_id"`
}

type WorkFlowInfo struct {
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	Branch        string         `json:"branch"`
	RepoType      RepositoryType `json:"repo_type"`
	WorkFlowID    string         `json:"workflow_id"`
	WorkFlowRunID string         `json:"workflow_run_id"`
}
