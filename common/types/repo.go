package types

import (
	"path"
	"strings"
	"time"
)

var (
	REPOCARD_FILENAME = "README.md"
	HUGGINGFACE_HOST  = "huggingface.co"
)

// KnownImageFileExts lists file extensions recognised as image files.
// Keep this as the single source of truth — do not duplicate elsewhere.
var KnownImageFileExts = []string{".png", ".jpg", ".jpeg", ".gif", ".tif", ".tiff", ".svg", ".bmp", ".webp"}

// IsImageFile returns true if the file path has a known image extension.
func IsImageFile(filePath string) bool {
	ext := path.Ext(filePath)
	if len(ext) == 0 {
		return false
	}
	for _, imageExt := range KnownImageFileExts {
		if strings.EqualFold(ext, imageExt) {
			return true
		}
	}
	return false
}

type (
	RepositoryType       string
	RepositorySource     string
	RepositorySyncStatus string
	PipelineTask         string
	InferenceEngine      string
)

type SensitiveCheckStatus int

// String returns a string representation of the sensitive check status.
//
// It returns one of "Fail", "Pending", "Pass", "Skip", "Exception", or "Unknown".
func (s SensitiveCheckStatus) String() string {
	switch s {
	case SensitiveCheckFail:
		return "Fail"
	case SensitiveCheckPending:
		return "Pending"
	case SensitiveCheckPass:
		return "Pass"
	case SensitiveCheckSkip:
		return "Skip"
	case SensitiveCheckException:
		return "Exception"
	default:
		return "Unknown"
	}
}

const (
	ResTypeKey    string = "hub-res-type"
	ResNameKey    string = "hub-res-name"
	ResDeployID   string = "hub-deploy-id"
	ResDeployUser string = "hub-deploy-user"

	RepoTypeSuffix string = "s_"

	ModelRepo     RepositoryType = "model"
	DatasetRepo   RepositoryType = "dataset"
	SpaceRepo     RepositoryType = "space"
	CodeRepo      RepositoryType = "code"
	PromptRepo    RepositoryType = "prompt"
	MCPServerRepo RepositoryType = "mcpserver"
	TemplateRepo  RepositoryType = "template"
	SkillRepo     RepositoryType = "skill"
	UnknownRepo   RepositoryType = ""

	OpenCSGSource     RepositorySource = "opencsg"
	LocalSource       RepositorySource = "local"
	HuggingfaceSource RepositorySource = "huggingface"

	SyncStatusPending    RepositorySyncStatus = "pending"
	SyncStatusInProgress RepositorySyncStatus = "inprogress"
	SyncStatusFailed     RepositorySyncStatus = "failed"
	SyncStatusCompleted  RepositorySyncStatus = "completed"
	SyncStatusCanceled   RepositorySyncStatus = "canceled"

	SensitiveCheckFail      SensitiveCheckStatus = -1 // sensitive content detected
	SensitiveCheckPending   SensitiveCheckStatus = 0  // default
	SensitiveCheckPass      SensitiveCheckStatus = 1  // pass
	SensitiveCheckSkip      SensitiveCheckStatus = 2  // skip
	SensitiveCheckException SensitiveCheckStatus = 3  // error happen

	EndpointPublic  int = 1 // public - anyone can access
	EndpointPrivate int = 2 // private - access with read permission

	MainBranch string = "main"

	InitCommitMessage     = "initial commit"
	ReadmeFileName        = "README.md"
	GitAttributesFileName = ".gitattributes"
	Gitignore             = ".gitignore"

	EntryFileAppFile    = "app.py"
	EntryFileNginx      = "nginx.conf"
	EntryFileDockerfile = "Dockerfile"

	TextGeneration     PipelineTask = "text-generation"
	Text2Image         PipelineTask = "text-to-image"
	Image2Image        PipelineTask = "image-to-image"
	Text2Video         PipelineTask = "text-to-video"
	Image2Video        PipelineTask = "image-to-video"
	ImageText2Text     PipelineTask = "image-text-to-text"
	FeatureExtraction  PipelineTask = "feature-extraction"
	SentenceSimilarity PipelineTask = "sentence-similarity"
	TextRanking        PipelineTask = "text-ranking"
	TaskAutoDetection  PipelineTask = "task-auto-detection"
	VideoText2Text     PipelineTask = "video-text-to-text"
	TextToSpeech       PipelineTask = "text-to-speech"
	TextToAudio        PipelineTask = "text-to-audio"
	// AutomaticSpeechRecognition is the Hugging Face pipeline task name. Keep
	// AutoSpeechRecognition as a legacy tag alias for existing seeded data.
	AutomaticSpeechRecognition  PipelineTask    = "automatic-speech-recognition"
	AutoSpeechRecognition       PipelineTask    = "auto-speech-recognition"
	OpticalCharacterRecognition PipelineTask    = "optical-character-recognition"
	LlamaCpp                    InferenceEngine = "llama.cpp"
	TEI                         InferenceEngine = "tei"
	Ktransformers               InferenceEngine = "ktransformers"

	MaxFileTreeSize int = 500
)

var (
	Sorts   = []string{"trending", "recently_update", "most_download", "most_favorite", "most_star"}
	Sources = []string{"opencsg", "huggingface", "local"}
)

type RepoRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Ref       string `json:"ref"`
	Path      string `json:"path"`
	Page      int    `json:"page"`
	Per       int    `json:"per"`
}

type RepoFullCheckResult struct {
	Skipped    bool   `json:"skipped"`
	WorkflowID string `json:"workflow_id"`
}

type ValidateYamlReq struct {
	Content  string         `json:"content"`
	RepoType RepositoryType `json:"repo_type"`
}

type Branch struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  RepoBranchCommit `json:"commit"`
}

type Tag struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  DatasetTagCommit `json:"commit"`
}

type CreateBranchReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	BranchName  string         `json:"branch_name"`
	CommitID    string         `json:"commit_id"`
	RepoType    RepositoryType `json:"-"`
	CurrentUser string         `json:"-"`
}

type DeleteBranchReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	BranchName  string         `json:"branch_name"`
	RepoType    RepositoryType `json:"-"`
	CurrentUser string         `json:"-"`
}

type Repository struct {
	HTTPCloneURL string `json:"http_clone_url"`
	SSHCloneURL  string `json:"ssh_clone_url"`
}

type Metadata struct {
	ModelParams       float32           `json:"model_params"`
	TensorType        string            `json:"tensor_type"`
	Architecture      string            `json:"architecture"`
	MiniGPUMemoryGB   float32           `json:"mini_gpu_memory_gb"`
	MiniGPUFinetuneGB float32           `json:"mini_gpu_finetune_gb"`
	ModelType         string            `json:"model_type"`
	ClassName         string            `json:"class_name"`
	Quantizations     []Quantization    `json:"quantizations,omitempty"`
	PDRecommendation  *PDRecommendation `json:"pd_recommendation,omitempty"`
}

type RepoPageOpts struct {
	PageOpts
	PageCount int `json:"page_count"`
	Total     int `json:"total"`
}

type Instance struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type InstanceInfo struct {
	Instances  []Instance `json:"instances"`
	Message    string     `json:"message"`
	Reason     string     `json:"reason"`
	ReadyCount int        `json:"ready_count"`
	IsCreating bool       `json:"is_creating"`
}

// repo object(cover model/space/code/dataset) for deployer
type DeployRequest struct {
	DeployID            int64      `json:"deploy_id,omitempty"`
	DeployName          string     `json:"deploy_name,omitempty"`
	SpaceID             int64      `json:"space_id,omitempty"`
	Path                string     `json:"model_id,omitempty"` // csghub ask for model_id = namespace/name
	Namespace           string     `json:"namespace,omitempty"`
	Name                string     `json:"name,omitempty"`
	Status              string     `json:"status"`
	GitPath             string     `json:"git_path,omitempty"`
	GitBranch           string     `json:"git_branch,omitempty"`
	Sdk                 string     `json:"sdk,omitempty"`
	SdkVersion          string     `json:"sdk_version,omitempty"`
	Env                 string     `json:"env,omitempty"`
	Secret              string     `json:"secret,omitempty"`
	Template            string     `json:"template,omitempty"`
	Hardware            string     `json:"hardware,omitempty"`
	ImageID             string     `json:"image_id,omitempty"`
	UserID              int64      `json:"user_id,omitempty"`
	ModelID             int64      `json:"repo_model_id,omitempty"` // for URM code logic
	RepoID              int64      `json:"repository_id,omitempty"`
	RuntimeFramework    string     `json:"runtime_framework,omitempty"`
	ContainerPort       int        `json:"container_port,omitempty"`
	Annotation          string     `json:"annotation,omitempty"`
	MinReplica          int        `json:"min_replica,omitempty"`
	MaxReplica          int        `json:"max_replica,omitempty"`
	SvcName             string     `json:"svc_name,omitempty"`
	Endpoint            string     `json:"endpoint,omitempty"`
	CreatedAt           time.Time  `json:"created_at,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at,omitempty"`
	ClusterID           string     `json:"cluster_id,omitempty"`
	SecureLevel         int        `json:"secure_level,omitempty"`
	ActualReplica       int        `json:"actual_replica,omitempty"`
	DesiredReplica      int        `json:"desired_replica,omitempty"`
	Instances           []Instance `json:"instances,omitempty"`
	InstanceName        string     `json:"instance_name,omitempty"`
	Private             bool       `json:"private"`
	Type                int        `json:"type,omitempty"`
	ProxyEndpoint       string     `json:"proxy_endpoint,omitempty"`
	UserUUID            string     `json:"user_uuid,omitempty"`
	SKU                 string     `json:"sku,omitempty"`
	OrderDetailID       int64      `json:"order_detail_id,omitempty"`
	PayMode             PayMode    `json:"pay_mode,omitempty"`
	Provider            string     `json:"provider,omitempty"`
	ResourceType        string     `json:"resource_type,omitempty"`
	RepoTag             string     `json:"repo_tag,omitempty"`
	Task                string     `json:"task,omitempty"`
	EngineArgs          string     `json:"engine_args,omitempty"`
	Variables           string     `json:"variables,omitempty"`
	Entrypoint          string     `json:"entrypoint,omitempty"`
	Reason              string     `json:"reason,omitempty"`
	Message             string     `json:"message,omitempty"`
	SupportFunctionCall bool       `json:"support_function_call,omitempty"`
	OwnerNamespace      string     `json:"owner_namespace,omitempty"`

	Since    string `json:"since,omitempty"`
	CommitID string `json:"commit_id,omitempty"`
	Instance string `json:"instance,omitempty"`

	DeployExtend

	Sandbox struct {
		Timeout    int64  `json:"timeout,omitempty"`
		TemplateID string `json:"templateID,omitempty"`
	}
}

type RuntimeFrameworkReq struct {
	FrameName     string `json:"frame_name"`
	FrameVersion  string `json:"frame_version"`
	FrameImage    string `json:"frame_image"`
	Enabled       int64  `json:"enabled"`
	ContainerPort int    `json:"container_port"`
	Type          int    `json:"type"`
	EngineArgs    string `json:"engine_args"`
	CurrentUser   string `json:"-"`
	ComputeType   string `json:"compute_type"`
	DriverVersion string `json:"driver_version"`
}

type RuntimeFramework struct {
	ID            int64  `json:"id"`
	FrameName     string `json:"frame_name"`
	FrameVersion  string `json:"frame_version"`
	FrameImage    string `json:"frame_image"`
	Enabled       int64  `json:"enabled"`
	ContainerPort int    `json:"container_port"`
	Type          int    `json:"type"`
	EngineArgs    string `json:"engine_args"`
	ComputeType   string `json:"compute_type"`
	DriverVersion string `json:"driver_version"`
	Description   string `json:"description"`
}

type RuntimeFrameworkV2 struct {
	FrameName    string             `json:"frame_name"`
	ComputeTypes []string           `json:"compute_types"`
	Versions     []RuntimeFramework `json:"versions"`
}

type RuntimeFrameworkModels struct {
	Models   []string     `json:"models"`
	ID       int64        `json:"id"`
	ScanType int          `json:"scan_type"`
	Task     PipelineTask `json:"task"`
}

type TreeReq struct {
	RepoId   int64
	Relation ModelRelation
}

type RepoFilter struct {
	Tags                []TagReq
	Sort                string
	Search              string
	Source              string
	Owner               string
	Username            string
	Tree                *TreeReq
	ListServerless      bool
	SpaceSDK            string
	XnetMigrationStatus *XnetMigrationTaskStatus
	Status              string
	DatasetType         string
	UserPurchased       bool
	ModelParamsMin      *float64
	ModelParamsMax      *float64
	RepoSizeMin         *int64
	RepoSizeMax         *int64
}

type BatchGetFilter struct {
	RepoType             RepositoryType        `json:"repo_type"`
	SensitiveCheckStatus *SensitiveCheckStatus `json:"sensitive_check_status"`
}

type TagReq struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Group    string `json:"group"`
}

type RuntimeArchitecture struct {
	Architectures []string `json:"architectures"`
}

type ScanReq struct {
	FrameID   int64
	FrameType int
	ArchMap   map[string]string
	Models    []string
	Task      PipelineTask
}

type OperationType string

const (
	OperationCreate OperationType = "create"
	OperationDelete OperationType = "delete"
)

type RepoNotificationReq struct {
	RepoType  RepositoryType
	RepoPath  string
	Operation OperationType
	UserUUID  string
}

type ChangePathReq struct {
	RepoType    RepositoryType
	Namespace   string
	Name        string
	NewPath     string `json:"new_path"`
	CurrentUser string
}

type TransferRepoReq struct {
	RepoType     RepositoryType `json:"-"`
	Namespace    string         `json:"-"`
	Name         string         `json:"-"`
	NewNamespace string         `json:"new_namespace"`
	CurrentUser  string         `json:"-"`
}

var validRepositoryTypes = map[RepositoryType]struct{}{
	ModelRepo:     {},
	DatasetRepo:   {},
	SpaceRepo:     {},
	CodeRepo:      {},
	PromptRepo:    {},
	MCPServerRepo: {},
	TemplateRepo:  {},
}

func (rt RepositoryType) IsValid() bool {
	_, exists := validRepositoryTypes[rt]
	return exists
}

type ReadLogRequest struct {
	DeployID     string            `json:"deploy_id"`
	TaskID       string            `json:"task_id"`
	TimeLoc      *time.Location    `json:"time_loc"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	Labels       map[string]string `json:"labels"`
	InstanceName string            `json:"instance_name"`
}

type CreateForkReq struct {
	SourceRepoType   RepositoryType `json:"source_repo_type"`
	SourceNamespace  string         `json:"source_namespace"`
	SourceName       string         `json:"source_name"`
	TargetNamespace  string         `json:"target_namespace"`
	TargetName       string         `json:"target_name"`
	CurrentUser      string         `json:"current_user"`
	SourceRepoID     int64          `json:"source_repo_id"`
	TargetRepoID     int64          `json:"target_repo_id"`
	RelatedDatasetID int64          `json:"related_dataset_id"`
	TaskID           int64          `json:"task_id"`
}

type CheckResourceAndAccountReq struct {
	UserName      string `json:"user_name"`
	ClusterID     string `json:"cluster_id"`
	OrderDetailID int64  `json:"order_detail_id"`
	CurrentUser   string `json:"current_user"`
}

type DownloadCodeZipReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Revision  string `json:"revision"`
}

type BatchRepoExtraReq struct {
	RepoIDs []int64 `json:"repo_ids" binding:"required"`
}

type RepoExtraItem struct {
	RepoID         int64 `json:"repo_id"`
	Size           int64 `json:"size"`
	LastCommitSize int64 `json:"last_commit_size"`
}

type RepoSizeResponse struct {
	TotalSize      int64 `json:"total_size"`
	LastCommitSize int64 `json:"last_commit_size"`
}
