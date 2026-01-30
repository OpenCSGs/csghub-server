package types

import "time"

var _ SensitiveRequestV2 = (*FinetuneReq)(nil)

func (c *FinetuneReq) GetSensitiveFields() []SensitiveField {
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

type FinetuneReq struct {
	Username         string   `json:"-"`
	Token            string   `json:"-"`
	Hardware         HardWare `json:"-"`
	UserUUID         string   `json:"-"`
	ClusterID        string   `json:"-"`
	Image            string   `json:"-"`
	RepoType         string   `json:"-"`
	TaskType         TaskType `json:"-"`
	DownloadEndpoint string   `json:"-"`
	ResourceName     string   `json:"-"`

	TaskName           string  `json:"task_name"`
	TaskDesc           string  `json:"task_desc"`
	RuntimeFrameworkId int64   `json:"runtime_framework_id" binding:"required"`
	ResourceId         int64   `json:"resource_id" binding:"required"`
	ModelId            string  `json:"model_id" binding:"required"`
	DatasetId          string  `json:"dataset_id" binding:"required"`
	Epochs             int     `json:"epochs"`
	ShareMode          bool    `json:"share_mode"`
	LearningRate       float64 `json:"learning_rate"`
	CustomeArgs        string  `json:"custom_args"`
	Agent              string  `json:"agent,omitempty"`
	Nodes              []Node  `json:"-"`
}

type FinetineGetReq struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	ClusterID string `json:"cluster_id"`
	TaskID    string `json:"task_id"`
	Namespace string `json:"namespace"`
}

type FinetuneRes struct {
	ID           int64     `json:"id"`
	RepoIds      []string  `json:"repo_ids"`
	RepoType     string    `json:"repo_type,omitempty"`
	Username     string    `json:"username"`
	TaskName     string    `json:"task_name"`
	TaskId       string    `json:"task_id"`
	TaskType     TaskType  `json:"task_type"`
	TaskDesc     string    `json:"task_desc"`
	ResourceId   int64     `json:"resource_id,omitempty"`
	ResourceName string    `json:"resource_name,omitempty"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	Image        string    `bun:",notnull" json:"image"`
	SubmitTime   time.Time `json:"submit_time"`
	StartTime    time.Time `json:"start_time,omitempty"`
	EndTime      time.Time `json:"end_time,omitempty"`
	Datasets     []string  `json:"datasets"`
	ResultURL    string    `json:"result_url"`
}

type FinetuneLogReq struct {
	CurrentUser string
	Since       string
	ID          int64
	PodName     string
	SubmitTime  time.Time
}
