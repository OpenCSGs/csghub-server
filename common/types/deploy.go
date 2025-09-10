package types

type DeployReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
	RepoType    RepositoryType `json:"repo_type"`
	DeployType  int            `json:"deploy_type"`
	DeployTypes []int          `json:"deploy_types"`
	Status      []int          `json:"status"`
	Query       string         `json:"query"`
}

type ServiceEvent struct {
	ServiceName string `json:"service_name"` // service name
	Status      int    `json:"status"`       // event status
	Endpoint    string `json:"endpoint"`     // service endpoint
	Message     string `json:"message"`      // event message
	Reason      string `json:"reason"`       // event reason
	TaskID      int64  `json:"task_id"`      // task id
}

type StatRunningDeploy struct {
	DeployNum int `json:"deploy_num"`
	CPUNum    int `json:"cpu_num"`
	GPUNum    int `json:"gpu_num"`
	NpuNum    int `json:"npu_num"`
	GcuNum    int `json:"gcu_num"`
	MluNum    int `json:"mlu_num"`
	DcuNum    int `json:"dcu_num"`
	GPGpuNum  int `json:"gpgpu_num"`
}
