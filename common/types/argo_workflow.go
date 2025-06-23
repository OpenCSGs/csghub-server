package types

import (
	"time"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
)

var WorkFlowFinished = map[v1alpha1.WorkflowPhase]struct{}{
	v1alpha1.WorkflowSucceeded: {},
	v1alpha1.WorkflowFailed:    {},
	v1alpha1.WorkflowError:     {},
}

var _ SensitiveRequestV2 = (*EvaluationReq)(nil)

func (c *EvaluationReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "task_name",
			Value: func() string {
				return c.TaskName
			},
			Scenario: "nickname_detection",
		},
		{
			Name: "task_desc",
			Value: func() string {
				return c.TaskDesc
			},
			Scenario: "comment_detection",
		},
	}
}

type TaskType string

const (
	TaskTypeEvaluation  TaskType = "evaluation"
	TaskTypeTraining    TaskType = "training"
	TaskTypeComparison  TaskType = "comparison"
	TaskTypeLeaderBoard TaskType = "leaderboard"
)

type EvaluationReq struct {
	Username           string   `json:"-"`
	TaskName           string   `json:"task_name"`
	TaskDesc           string   `json:"task_desc"`
	RuntimeFrameworkId int64    `json:"runtime_framework_id"` // ArgoWorkFlow framework
	Datasets           []string `json:"datasets,omitempty"`
	ResourceId         int64    `json:"resource_id"`
	ModelId            string   `json:"model_id"`
	ModelIds           []string `json:"model_ids,omitempty"` // for comparison
	ShareMode          bool     `json:"share_mode"`
	CustomDataSets     []string `json:"custom_datasets,omitempty"` // custom datasets
	Token              string   `json:"-"`
	Hardware           HardWare `json:"-"`
	UserUUID           string   `json:"-"`
	ClusterID          string   `json:"-"`
	Image              string   `json:"-"`
	RepoType           string   `json:"-"`
	TaskType           TaskType `json:"-"`
	DownloadEndpoint   string   `json:"-"`
	ResourceName       string   `json:"-"`
	Revisions          []string `json:"-"`
	DatasetRevisions   []string `json:"-"`
	UseCustomDataset   bool     `json:"-"`
}

type CustomData struct {
	TaskName string `json:"task_name"`
	DataSet  string `json:"dataset_name"`
}

type ArgoFlowTemplate struct {
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Command    []string          `json:"command"`
	Args       []string          `json:"args"`
	HardWare   HardWare          `json:"hardware,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Annotation map[string]string `json:"annotation,omitempty"`
}

type ArgoWorkFlowReq struct {
	ClusterID    string             `json:"cluster_id"`
	RepoType     string             `json:"repo_type"`
	Templates    []ArgoFlowTemplate `json:"templates,omitempty"`
	Entrypoint   string             `json:"entrypoint"`
	Username     string             `json:"username"`
	TaskName     string             `json:"task_name"`
	TaskId       string             `json:"task_id"`
	TaskType     TaskType           `json:"task_type"`
	RepoIds      []string           `json:"repo_ids"`
	TaskDesc     string             `json:"task_desc"`
	Image        string             `json:"image"` // ArgoWorkFlow framework
	Datasets     []string           `json:"datasets,omitempty"`
	ResourceId   int64              `json:"resource_id"`
	ResourceName string             `json:"resource_name"`
	UserUUID     string             `json:"user_uuid"`
	ShareMode    bool               `json:"share_mode"`
}

type ArgoWorkFlowListRes struct {
	List  []ArgoWorkFlowRes `json:"list"`
	Total int               `json:"total"`
}

type ArgoWorkFlowRes struct {
	ID          int64                  `json:"id"`
	RepoIds     []string               `json:"repo_ids"`
	RepoType    string                 `json:"repo_type,omitempty"`
	Username    string                 `json:"username"`
	TaskName    string                 `json:"task_name"`
	TaskId      string                 `json:"task_id"`
	TaskType    TaskType               `json:"task_type"`
	TaskDesc    string                 `json:"task_desc"`
	Datasets    []string               `json:"datasets,omitempty"`
	ResourceId  int64                  `json:"resource_id,omitempty"`
	Status      v1alpha1.WorkflowPhase `json:"status"`
	Reason      string                 `json:"reason,omitempty"`
	Image       string                 `bun:",notnull" json:"image"`
	SubmitTime  time.Time              `json:"submit_time"`
	StartTime   time.Time              `json:"start_time,omitempty"`
	EndTime     time.Time              `json:"end_time,omitempty"`
	ResultURL   string                 `json:"result_url"`
	DownloadURL string                 `json:"download_url"`
	FailuresURL string                 `json:"failures_url"`
}

type RepoTags struct {
	RepoId string    `json:"repo_id"`
	Tags   []RepoTag `json:"tags"`
}

type EvaluationRes struct {
	ID          int64      `json:"id"`
	RepoIds     []string   `json:"repo_ids"`
	RepoType    string     `json:"repo_type,omitempty"`
	Username    string     `json:"username"`
	TaskName    string     `json:"task_name"`
	TaskId      string     `json:"task_id"`
	TaskType    TaskType   `json:"task_type"`
	TaskDesc    string     `json:"task_desc"`
	Datasets    []RepoTags `json:"datasets,omitempty"`
	ResourceId  int64      `json:"resource_id,omitempty"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason,omitempty"`
	Image       string     `bun:",notnull" json:"image"`
	SubmitTime  time.Time  `json:"submit_time"`
	StartTime   time.Time  `json:"start_time,omitempty"`
	EndTime     time.Time  `json:"end_time,omitempty"`
	ResultURL   string     `json:"result_url"`
	DownloadURL string     `json:"download_url"`
	FailuresURL string     `json:"failures_url"`
}

type (
	EvaluationDelReq   = ArgoWorkFlowDeleteReq
	EvaluationGetReq   = ArgoWorkFlowDeleteReq
	ArgoWorkFlowGetReq = ArgoWorkFlowDeleteReq
)

type ArgoWorkFlowDeleteReq struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}
