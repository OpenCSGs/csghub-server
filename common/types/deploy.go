package types

import "time"

const (
	// EngineArgEnableToolCalling is the engine argument for enabling tool calling
	EngineArgEnableToolCalling = "--enable-tool-calling"
)

type DeployReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
	RepoType    RepositoryType `json:"repo_type"`
	DeployType  int            `json:"deploy_type"`
	DeployTypes []int          `json:"deploy_types"`
	Status      []int          `json:"status"`
	Query       string         `json:"query"`
	StartTime   *time.Time     `json:"start_time,omitempty"`
	EndTime     *time.Time     `json:"end_time,omitempty"`
}

type ServiceEvent struct {
	ServiceName string `json:"service_name"` // service name
	Status      int    `json:"status"`       // event status
	Endpoint    string `json:"endpoint"`     // service endpoint
	Message     string `json:"message"`      // event message
	Reason      string `json:"reason"`       // event reason
	TaskID      int64  `json:"task_id"`      // task id
	ClusterNode string `json:"cluster_node"` // cluster node name
	QueueName   string `json:"queue_name"`   // queue name
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

type ClusterDeployReq struct {
	ClusterID    string `json:"cluster_id"`
	ClusterNode  string `json:"cluster_node"`
	Status       int    `json:"status"`
	ResourceID   int    `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	Search       string `json:"search"`
	Per          int    `json:"per"`
	Page         int    `json:"page"`
}
