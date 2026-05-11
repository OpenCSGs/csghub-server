package types

import "time"

const AccessTokenEnvKey = "ACCESS_TOKEN"
const DFPVCNamePrefix = "df-"
const DFParamDagTaskIDKey = "bld_dag_task_id"
const DFParamDagTaskNameKey = "bld_dag_task_name"

const DFLabelTagKey = "workflow-scope"
const DFLabelTagValue = "csghub-dataflow"
const DFLabelDagTaskIDKey = "csghub_df_dag_task_id"
const DFLabelDagTaskNameKey = "csghub_df_dag_task_name"

const DFUniqueIDKey = "csghub_df_unique_id"
const DFArgoTaskIDKey = "csghub_df_argo_task_id"
const DFOpUserUUIDKey = "csghub_df_op_user_uuid"
const DFOpUserNameKey = "csghub_df_op_user_name"
const DFNSUUIDKey = "csghub_df_ns_uuid"
const DFClusterIDKey = "csghub_df_cluster_id"
const DFResourceIDKey = "csghub_df_res_id"
const DFResourceNameKey = "csghub_df_res_name"
const DFJobIDKey = "csghub_df_job_id"
const DFJobNameKey = "csghub_df_job_name"
const DFJobDescKey = "csghub_df_job_desc"
const DFImageKey = "csghub_df_image"
const DFStorageSizeKey = "csghub_df_storage_size"

const DFCancelled = "Canceled"

// DataflowArgoJobReq - Request body for creating dataflow job
type DataflowArgoJobReq struct {
	ID           int64  `json:"id"`
	ClusterID    string `json:"cluster_id"`
	ArgoTaskID   string `json:"argo_task_id"` // db task id
	ResourceName string `json:"resource_name"`
	OpUserUUID   string `json:"op_user_uuid"`
	Username     string `json:"username"`
	NSUUID       string `json:"ns_uuid"`

	RepoIds     []string         `json:"repo_ids"`                        // dataset repo ids from dataflow
	ResourceId  int64            `json:"resource_id" binding:"required"`  // from dataflow
	JobID       string           `json:"job_id" binding:"required"`       // job id from dataflow
	JobName     string           `json:"job_name" binding:"required"`     // job name from dataflow
	JobDesc     string           `json:"job_desc"`                        // job description from dataflow
	StorageSize string           `json:"storage_size" binding:"required"` // from dataflow
	Entrypoint  string           `json:"entrypoint" binding:"required"`   // from dataflow
	Template    ArgoFlowTemplate `json:"template" binding:"required"`     // from dataflow
	DagTasks    []ArgoDagTask    `json:"dag_tasks" binding:"required"`    // from dataflow

	Nodes       []Node     `json:"nodes"`
	Scheduler   *Scheduler `json:"scheduler,omitempty"`
	AccessToken string     `json:"access_token,omitempty"` // user's access token for pod env
	DeployExtend
}

// DataflowCreateReq is an alias for DataflowArgoJobReq (used in deployer)
type DataflowCreateReq = DataflowArgoJobReq

// DagTask - DAG task definition with dependencies
type ArgoDagTask struct {
	ID         string             `json:"id"  binding:"required"`
	Name       string             `json:"name" binding:"required"`
	Deps       []string           `json:"deps"`
	Template   string             `json:"template" binding:"required"`
	Parameters []ArgoDagTaskParam `json:"parameters" binding:"required"`
}

type ArgoDagTaskParam struct {
	Name  string `json:"name"  binding:"required"`
	Value string `json:"value" binding:"required"`
}

// DataflowJobStatusResp - Response for job status query
type DataflowArgoJobResp struct {
	ID         int64  `json:"id"`
	ArgoTaskID string `json:"argo_task_id"`
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	CreatedAt  int64  `json:"created_at"`
	DagTasks   string `json:"dag_tasks"`
	DeleteAt   int64  `json:"delete_at"`
}

// DataflowLogsResp - Response for job logs query
type DataflowArgoReq struct {
	ArgoTaskID string `json:"argo_task_id"`
	ClusterID  string `json:"cluster_id"`
}

// DataflowDeleteReq - Request body for deleting dataflow job
type DataflowDeleteReq struct {
	OpUserUUID string `json:"-"`
	Username   string `json:"-"`
	NSUUID     string `json:"ns_uuid"`
	ArgoTaskID string `json:"argo_task_id"`
}

type DataflowDagTask struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// DataflowLogReq - Request for querying dataflow job logs
type DataflowLogReq struct {
	CurrentUser string    // Current user for permission check
	Since       string    // Query parameter: "10mins", "30mins", "1hour", "6hours", "1day", "2days", "1week"
	TaskId      string    // ArgoTaskID from path (workflow name)
	DagTaskId   string    // Query parameter: specific DAG task id
	SubmitTime  time.Time // Job submit time - populated from workflow
}
