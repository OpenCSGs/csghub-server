package types

type DeployReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
	RepoType   RepositoryType `json:"repo_type"`
	DeployType int            `json:"deploy_type"`
}

type ServiceEvent struct {
	ServiceName string `json:"service_name"` // service name
	Status      int    `json:"status"`       // event status
	Message     string `json:"message"`      // event message
	Reason      string `json:"reason"`       // event reason
}
