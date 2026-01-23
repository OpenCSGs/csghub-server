package types

import (
	"time"
)

const LFSPrefix = "version https://git-lfs.github.com/spec/v1"

type RepoBranchCommit struct {
	ID string `json:"id"`
}

type CreateModelReq struct {
	BaseModel       string `json:"base_model"`
	ReportURL       string `json:"report_url"`
	MediumRiskCount int    `json:"medium_risk_count"`
	HighRiskCount   int    `json:"high_risk_count"`
	CreateRepoReq
}

type UpdateModelReq struct {
	BaseModel       *string `json:"base_model"`
	ReportURL       string  `json:"report_url"`
	MediumRiskCount int     `json:"medium_risk_count"`
	HighRiskCount   int     `json:"high_risk_count"`
	UpdateRepoReq
}

type UpdateRepoReq struct {
	Username  string `json:"-"`
	Namespace string `json:"-"`
	Name      string `json:"-"`
	// The type of the repository
	RepoType RepositoryType `json:"-"`
	// The new display name of the repository
	Nickname *string `json:"nickname" example:"model display name"`
	// The new description for the repository
	Description *string `json:"description"`
	// The new visibility of the repository
	Private     *bool  `json:"private" example:"false"`
	Admin       string `json:"-"`
	XnetEnabled *bool  `json:"xnet_enabled"`
}

// make sure UpdateModelReq implements SensitiveRequest interface
var _ SensitiveRequestV2 = (*UpdateRepoReq)(nil)

func (c *UpdateRepoReq) GetSensitiveFields() []SensitiveField {
	var fields []SensitiveField
	if c.Nickname != nil {
		fields = append(fields, SensitiveField{
			Name: "nickname",
			Value: func() string {
				return *c.Nickname
			},
			Scenario: "nickname_detection",
		})
	}
	if c.Description != nil {
		fields = append(fields, SensitiveField{
			Name: "description",
			Value: func() string {
				return *c.Description
			},
			Scenario: "comment_detection",
		})
	}
	return fields
}

type UpdateDownloadsReq struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	ReqDate    string `json:"date"`
	Date       time.Time
	CloneCount int64 `json:"download_count"`
	RepoType   RepositoryType
}

type ModelPredictReq struct {
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	Input       string `json:"input"`
	Version     string `json:"version"`
	CurrentUser string `json:"current_user"`
}

type ModelPredictResp struct {
	Content string `json:"content"`
	// TODO:add metrics like tokens, latency etc
}

type CreateRepoReq struct {
	Username string `json:"-"`
	// The namespace of the repository, which can be a user or an organization
	Namespace string `json:"namespace" example:"user_or_org_name"`
	// The name of the repository
	Name string `json:"name" example:"model_name_1"`
	// The display name of the repository
	Nickname string `json:"nickname" example:"model display name"`
	// A description for the repository
	Description string `json:"description" binding:"lt=1000"`
	// Whether the repository is private
	Private bool `json:"private"`
	// The license for the repository
	License string `json:"license" example:"MIT" binding:"lt=100"`
	Readme  string `json:"-"`
	// The default branch of the repository
	DefaultBranch string `json:"default_branch" example:"main"`
	// The type of the repository
	RepoType RepositoryType `json:"type"`
	// Admin user
	Admin string `json:"admin"`
	// Tool count
	ToolCount int `json:"tool_count"`
	// Star count
	StarCount int `json:"star_count"`
	// Files to commit
	CommitFiles []CommitFile `json:"commit_files"`
}

// make sure CreateRepoReq implements SensitiveRequest
var _ SensitiveRequestV2 = (*CreateRepoReq)(nil)

func (c *CreateRepoReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "description",
			Value: func() string {
				return c.Description
			},
			Scenario: "comment_detection",
		},
		{
			Name: "name",
			Value: func() string {
				return c.Name
			},
			Scenario: "nickname_detection",
		},
		{
			Name: "nickname",
			Value: func() string {
				return c.Nickname
			},
			Scenario: "nickname_detection",
		},
	}
}

type DeleteRepoReq struct {
	Username  string         `json:"username" example:"creator_user_name"`
	Namespace string         `json:"namespace" example:"user_or_org_name"`
	Name      string         `json:"name" example:"model_name_1"`
	RepoType  RepositoryType `json:"-"`
}

type Relations struct {
	Models   []*Model     `json:"models,omitempty"`
	Datasets []*Dataset   `json:"datasets,omitempty"`
	Codes    []*Code      `json:"codes,omitempty"`
	Spaces   []*Space     `json:"spaces,omitempty"`
	Prompts  []*PromptRes `json:"prompts,omitempty"`
}

type Model struct {
	ID            int64      `json:"id,omitempty"`
	Name          string     `json:"name"`
	Nickname      string     `json:"nickname"`
	Description   string     `json:"description"`
	Likes         int64      `json:"likes"`
	Downloads     int64      `json:"downloads"`
	Path          string     `json:"path"`
	RepositoryID  int64      `json:"repository_id"`
	Private       bool       `json:"private"`
	User          *User      `json:"user,omitempty"`
	Tags          []RepoTag  `json:"tags,omitempty"`
	Readme        string     `json:"readme"`
	Repository    Repository `json:"repository"`
	DefaultBranch string     `json:"default_branch"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// widget UI style: generation,chat
	WidgetType ModelWidgetType `json:"widget_type" example:"generation"`
	// url to interact with the model
	Status                  string               `json:"status" example:"RUNNING"`
	UserLikes               bool                 `json:"user_likes"`
	Source                  RepositorySource     `json:"source"`
	SyncStatus              RepositorySyncStatus `json:"sync_status"`
	EnableInference         bool                 `json:"enable_inference"`
	EnableFinetune          bool                 `json:"enable_finetune"`
	EnableEvaluation        bool                 `json:"enable_evaluation"`
	DisableInferenceReason  string               `json:"disable_inference_reason" i18n:"model.disable_inference_reason"`
	DisableFinetuneReason   string               `json:"disable_finetune_reason" i18n:"model.disable_finetune_reason"`
	DisableEvaluationReason string               `json:"disable_evaluation_reason" i18n:"model.disable_evaluation_reason"`
	BaseModel               string               `json:"base_model"`
	License                 string               `json:"license"`
	CanWrite                bool                 `json:"can_write"`
	CanManage               bool                 `json:"can_manage"`
	Namespace               *Namespace           `json:"namespace"`
	RecomOpWeight           int                  `json:"recom_op_weight,omitempty"`
	Scores                  []WeightScore        `json:"scores"`
	SensitiveCheckStatus    string               `json:"sensitive_check_status"`
	MirrorLastUpdatedAt     time.Time            `json:"mirror_last_updated_at"`
	Metadata                Metadata             `json:"metadata"`
	ReportURL               string               `json:"report_url"`
	MediumRiskCount         int                  `json:"medium_risk_count"`
	HighRiskCount           int                  `json:"high_risk_count"`
	URL                     string               `json:"url"`
	MirrorTaskStatus        MirrorTaskStatus     `json:"mirror_task_status"`
	MultiSource
}

type SDKModelInfo struct {
	ID     string `json:"id"`
	Author string `json:"author,omitempty"`
	// last commit sha
	Sha              string                 `json:"sha,omitempty"`
	CreatedAt        time.Time              `json:"created_at,omitempty"`
	LastModified     time.Time              `json:"last_modified,omitempty"`
	Private          bool                   `json:"private"`
	Disabled         bool                   `json:"disabled,omitempty"`
	Gated            interface{}            `json:"gated,omitempty"` // "auto", "manual", or false
	Downloads        int                    `json:"downloads"`
	Likes            int                    `json:"likes"`
	LibraryName      string                 `json:"library_name,omitempty"`
	Tags             []string               `json:"tags"`
	PipelineTag      string                 `json:"pipeline_tag,omitempty"`
	MaskToken        string                 `json:"mask_token,omitempty"`
	WidgetData       interface{}            `json:"widget_data,omitempty"`       // Type Any
	ModelIndex       map[string]interface{} `json:"model_index,omitempty"`       // Dict
	Config           map[string]interface{} `json:"config,omitempty"`            // Dict
	TransformersInfo interface{}            `json:"transformers_info,omitempty"` // TransformersInfo
	CardData         interface{}            `json:"card_data,omitempty"`         // ModelCardData
	Siblings         []SDKFile              `json:"siblings"`
	Spaces           []string               `json:"spaces,omitempty"`
	SafeTensors      interface{}            `json:"safetensors,omitempty"` // SafeTensorsInfo
}

type ModelWidgetType string

const (
	ModelWidgetTypeGeneration ModelWidgetType = "generation"
	ModelWidgetTypeChat       ModelWidgetType = "chat"
)

type ModelType string

const (
	GGUF        ModelType = "gguf"
	Safetensors ModelType = "safetensors"
	Unknown     ModelType = "unknown"
)

const (
	GGUFEntryPoint = "GGUF_ENTRY_POINT"
)

type ModelRunReq struct {
	DeployName         string `json:"deploy_name"`
	ClusterID          string `json:"cluster_id"`
	Env                string `json:"env"`
	ResourceID         int64  `json:"resource_id"`
	RuntimeFrameworkID int64  `json:"runtime_framework_id"`
	MinReplica         int    `json:"min_replica"`
	MaxReplica         int    `json:"max_replica"`
	Revision           string `json:"revision"`
	SecureLevel        int    `json:"secure_level"`
	OrderDetailID      int64  `json:"order_detail_id"`
	Entrypoint         string `json:"entrypoint"` // model file name for gguf model
	EngineArgs         string `json:"engine_args"`
	Agent              string `json:"agent"`
}

var _ SensitiveRequestV2 = (*ModelRunReq)(nil)

func (c *ModelRunReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "deploy_name",
			Value: func() string {
				return c.DeployName
			},
			Scenario: "nickname_detection",
		},
	}
}

type InstanceRunReq struct {
	DeployName         string `json:"deploy_name"`
	ClusterID          string `json:"cluster_id"`
	ResourceID         int64  `json:"resource_id"`
	RuntimeFrameworkID int64  `json:"runtime_framework_id"`
	Revision           string `json:"revision"`
	OrderDetailID      int64  `json:"order_detail_id"`
	EngineArgs         string `json:"engine_args"`
}

var _ SensitiveRequestV2 = (*InstanceRunReq)(nil)

func (c *InstanceRunReq) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "deploy_name",
			Value: func() string {
				return c.DeployName
			},
			Scenario: "nickname_detection",
		},
	}
}

type ModelUpdateRequest struct {
	MinReplica int               `json:"min_replica"` // min replica of instance/pod
	MaxReplica int               `json:"max_replica"` // max replica of instance/pod
	Hardware   HardWare          `json:"hardware"`    // resource requirements
	ImageID    string            `json:"image_id" binding:"required"`
	Env        map[string]string `json:"env"` // runtime env variables
	ClusterID  string            `json:"cluster_id"`
	SvcName    string            `json:"svc_name"`
}

type ModelUpdateResponse struct {
	DeployID int64  `json:"deploy_id"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
}

type ModelStatusEventData struct {
	Status  string     `json:"status"`
	Details []Instance `json:"details"`
	Message string     `json:"message"`
	Reason  string     `json:"reason"`
}

const (
	SpaceType      = iota // space
	InferenceType  = 1    // inference endpoint
	FinetuneType   = 2    // finetune
	ServerlessType = 3    // serverless
	EvaluationType = 4    // evaluation
	NotebookType   = 5    // notebook
	JobType        = 6    // job
	UnknownType    = -1   // unknown case
)

type DeployActReq struct {
	RepoType     RepositoryType `json:"repo_type"`
	Namespace    string         `json:"namespace"`
	Name         string         `json:"name"`
	CurrentUser  string         `json:"current_user"`
	DeployID     int64          `json:"deploy_id"`
	DeployType   int            `json:"deploy_type"`
	InstanceName string         `json:"instance_name"`
	Since        string         `json:"since,omitempty"`
	CommitID     string         `json:"commit_id"`
}

type DeployUpdateReq struct {
	DeployName         *string `json:"deploy_name"`
	ClusterID          *string `json:"cluster_id"`
	Env                *string `json:"env"`
	ResourceID         *int64  `json:"resource_id"`
	RuntimeFrameworkID *int64  `json:"runtime_framework_id"`
	MinReplica         *int    `json:"min_replica" validate:"min=0"`
	MaxReplica         *int    `json:"max_replica" validate:"min=1,gtefield=MinReplica"`
	Revision           *string `json:"revision"`
	SecureLevel        *int    `json:"secure_level"`
	Entrypoint         *string `json:"entrypoint"`
	Variables          *string `json:"variables"`
	EngineArgs         *string `json:"engine_args"`
}

type RelationModels struct {
	Models      []string `json:"models"`
	Namespace   string   `json:"-"`
	Name        string   `json:"-"`
	CurrentUser string   `json:"-"`
}

type RelationDatasets struct {
	Datasets    []string `json:"datasets"`
	Namespace   string   `json:"-"`
	Name        string   `json:"-"`
	CurrentUser string   `json:"-"`
}

type RelationModel struct {
	Model       string `json:"model"`
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	CurrentUser string `json:"-"`
}

type RelationDataset struct {
	Dataset     string `json:"dataset"`
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	CurrentUser string `json:"-"`
}

type ModelInfo struct {
	TotalParams int64 `json:"total_params"`
	// 1b, 7b, 13b
	ParamsBillions float32 `json:"params_billions"`
	//fp16, fp32, int8
	TensorType      string  `json:"tensor_type"`
	MiniGPUMemoryGB float32 `json:"mini_gpu_memory_gb"`
	//the gpu memory for finetuning on a mini GPU
	MiniGPUFinetuneGB float32 `json:"mini_gpu_finetune_gb"`
	ModelWeightsGB    float32 `json:"model_weights_gb"`
	//qwen2
	ModelType string `json:"model_type"`
	//Qwen2ForCausalLM
	Architecture      string         `json:"architecture"`
	ClassName         string         `json:"class_name"`
	Quantizations     []Quantization `json:"quantizations"`
	ContextSize       int            `json:"-"`
	BatchSize         int            `json:"-"`
	HiddenSize        int            `json:"-"`
	NumHiddenLayers   int            `json:"-"`
	BytesPerParam     int            `json:"-"`
	NumAttentionHeads int            `json:"-"`
}
type Quantization struct {
	VERSION         string  `json:"version"`
	TYPE            string  `json:"type"`
	MiniGPUMemoryGB float32 `json:"mini_gpu_memory_gb"`
}

type ModelConfig struct {
	Architectures     []string `json:"architectures"`
	ModelType         string   `json:"model_type"`
	NumHiddenLayers   int      `json:"num_hidden_layers"`
	HiddenSize        int      `json:"hidden_size"`
	NumAttentionHeads int      `json:"num_attention_heads"`
	TorchDtype        string   `json:"torch_dtype"`
}
type EngineConfig struct {
	EngineName      string            `json:"engine_name"`
	ContainerPort   int               `json:"container_port"`
	MinVersion      string            `json:"min_version"`
	ModelFormat     string            `json:"model_format"`
	EngineImages    []Image           `json:"engine_images"`
	SupportedArchs  []string          `json:"supported_archs"`
	SupportedModels []string          `json:"supported_models"`
	EngineArgs      []EngineArg       `json:"engine_args"`
	Enabled         int64             `json:"enabled"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Description     string            `json:"description"`
	ToolCallParsers map[string]string `json:"tool_call_parsers,omitempty"`
}

type ComputeType string

const (
	GPU_TYPE     ComputeType = "gpu"
	CPU_TYPE     ComputeType = "cpu"
	Enflame_Type ComputeType = "enflame"
	MLU_TYPE     ComputeType = "mlu"
)

// Image represents an engine image with a specific computing power type
type Image struct {
	ComputeType   ComputeType `json:"compute_type"`
	Image         string      `json:"image"`
	DriverVersion string      `json:"driver_version"`
	EngineVersion string      `json:"engine_version"`
	ExtraArchs    []string    `json:"extra_archs"`
	ExtraModels   []string    `json:"extra_models"`
}

type CreateInferenceVersionReq struct {
	DeployId int64  `json:"-"`
	CommitID string `json:"commit_id"`

	TrafficPercent int `json:"traffic_percent"`
}

type ListInferenceVersionsResp struct {
	Commit         string    `json:"commit"`
	CreateTime     time.Time `json:"create_time"`
	IsReady        bool      `json:"is_ready"`
	TrafficPercent int64     `json:"traffic_percent"`
	RevisionName   string    `json:"revision_name"`
	Message        string    `json:"message"`
	Reason         string    `json:"reason"`
}

type UpdateInferenceVersionTrafficReq struct {
	CommitID       string `json:"commit_id" binding:"required"`
	TrafficPercent int64  `json:"traffic_percent"`
}
