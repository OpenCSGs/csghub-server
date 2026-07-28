package types

import (
	"time"
)

type CreateMirrorReq struct {
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	SourceUrl       string         `json:"source_url" binding:"required"`
	MirrorSourceID  int64          `json:"mirror_source_id"`
	Username        string         `json:"username"`
	AccessToken     string         `json:"access_token"`
	PushUrl         string         `json:"push_url"`
	PushUsername    string         `json:"push_username"`
	PushAccessToken string         `json:"push_access_token"`
	SourceRepoPath  string         `json:"source_repo_path"`
	LocalRepoPath   string         `json:"local_repo_path"`
	CurrentUser     string         `json:"current_user"`
	RepoType        RepositoryType `json:"repo_type"`
	SyncLfs         bool           `json:"sync_lfs"`
	// Urgent routes the next synchronization through the urgent mirror queues.
	Urgent bool `json:"urgent,omitempty"`
}

// MirrorFromSaasReq identifies an existing repository and the user requesting a SaaS mirror sync.
type MirrorFromSaasReq struct {
	Namespace   string
	Name        string
	RepoType    RepositoryType
	CurrentUser string
}

// SyncMirrorParams contains optional controls for a manual mirror synchronization.
type SyncMirrorParams struct {
	// Urgent routes the synchronization through the urgent mirror queues.
	Urgent bool `json:"urgent,omitempty"`
}

// SyncMirrorReq identifies a mirror repository and how its replacement task should be scheduled.
type SyncMirrorReq struct {
	RepoType    RepositoryType
	Namespace   string
	Name        string
	CurrentUser string
	Urgent      bool
}

// MirrorFromSaasResponse identifies the mirror task accepted for asynchronous execution.
type MirrorFromSaasResponse struct {
	RepositoryID int64            `json:"repository_id"`
	MirrorID     int64            `json:"mirror_id"`
	TaskID       int64            `json:"task_id"`
	Status       MirrorTaskStatus `json:"status"`
}

// MirrorFromSaasStatusReq identifies a repository and an optional task observed by the caller.
type MirrorFromSaasStatusReq struct {
	Namespace       string
	Name            string
	RepoType        RepositoryType
	CurrentUser     string
	RequestedTaskID int64
}

// MirrorSyncPhase identifies which stage of a mirror task is active.
type MirrorSyncPhase string

const (
	// MirrorSyncPhaseRepo is the repository clone and ref update stage.
	MirrorSyncPhaseRepo MirrorSyncPhase = "repo"
	// MirrorSyncPhaseLFS is the Git LFS object transfer stage.
	MirrorSyncPhaseLFS MirrorSyncPhase = "lfs"
	// MirrorSyncPhaseDone indicates that no stage will execute again.
	MirrorSyncPhaseDone MirrorSyncPhase = "done"
)

// MirrorSyncFailureReason is a safe public classification of a terminal task failure.
type MirrorSyncFailureReason string

const (
	// MirrorSyncFailureRepoSyncFailed indicates a terminal repository sync failure.
	MirrorSyncFailureRepoSyncFailed MirrorSyncFailureReason = "REPO_SYNC_FAILED"
	// MirrorSyncFailureLFSSyncFailed indicates a terminal Git LFS sync failure.
	MirrorSyncFailureLFSSyncFailed MirrorSyncFailureReason = "LFS_SYNC_FAILED"
	// MirrorSyncFailureRepoRetryExhausted indicates the workhub repository sync job exhausted its retries.
	MirrorSyncFailureRepoRetryExhausted MirrorSyncFailureReason = "REPO_RETRY_EXHAUSTED"
	// MirrorSyncFailureLFSRetryExhausted indicates the workhub Git LFS sync job exhausted its retries.
	MirrorSyncFailureLFSRetryExhausted MirrorSyncFailureReason = "LFS_RETRY_EXHAUSTED"
	// MirrorSyncFailureLFSIncomplete indicates some Git LFS objects were not synchronized.
	MirrorSyncFailureLFSIncomplete MirrorSyncFailureReason = "LFS_INCOMPLETE"
	// MirrorSyncFailureLFSTooLarge indicates Git LFS synchronization exceeded its size limit.
	MirrorSyncFailureLFSTooLarge MirrorSyncFailureReason = "LFS_TOO_LARGE"
	// MirrorSyncFailureCanceled indicates the task was cancelled.
	MirrorSyncFailureCanceled MirrorSyncFailureReason = "SYNC_CANCELLED"
)

// MirrorSyncStatusResponse contains public task state suitable for repository readers.
type MirrorSyncStatusResponse struct {
	RepositoryID  int64                   `json:"repository_id"`
	MirrorID      int64                   `json:"mirror_id"`
	TaskID        int64                   `json:"task_id"`
	Status        MirrorTaskStatus        `json:"status"`
	Phase         MirrorSyncPhase         `json:"phase"`
	RepoReady     bool                    `json:"repo_ready"`
	Terminal      bool                    `json:"terminal"`
	Retrying      bool                    `json:"retrying"`
	Superseded    bool                    `json:"superseded"`
	Progress      int                     `json:"progress"`
	FailureReason MirrorSyncFailureReason `json:"failure_reason"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

// MirrorSyncOverallStatus describes the effective lifecycle of a complete mirror sync.
type MirrorSyncOverallStatus string

const (
	// MirrorSyncOverallNoTask means the mirror has no current synchronization task.
	MirrorSyncOverallNoTask MirrorSyncOverallStatus = "no_task"
	// MirrorSyncOverallWaiting means the current stage has not started its first attempt.
	MirrorSyncOverallWaiting MirrorSyncOverallStatus = "waiting"
	// MirrorSyncOverallRunning includes active execution and retry waits.
	MirrorSyncOverallRunning MirrorSyncOverallStatus = "running"
	// MirrorSyncOverallFinished means no stage will execute again.
	MirrorSyncOverallFinished MirrorSyncOverallStatus = "finished"
)

// MirrorSyncStageState describes one Repo or LFS stage lifecycle.
type MirrorSyncStageState string

const (
	// MirrorSyncStageNotStarted means the stage has not started its first attempt.
	MirrorSyncStageNotStarted MirrorSyncStageState = "not_started"
	// MirrorSyncStageRunning includes active execution and retry waits.
	MirrorSyncStageRunning MirrorSyncStageState = "running"
	// MirrorSyncStageFinished means the stage will not execute again.
	MirrorSyncStageFinished MirrorSyncStageState = "finished"
)

// MirrorSyncResult describes why a mirror sync or stage finished.
type MirrorSyncResult string

const (
	// MirrorSyncResultSuccess means synchronization completed successfully.
	MirrorSyncResultSuccess MirrorSyncResult = "success"
	// MirrorSyncResultFailed means synchronization failed permanently.
	MirrorSyncResultFailed MirrorSyncResult = "failed"
	// MirrorSyncResultCancelled means synchronization was cancelled.
	MirrorSyncResultCancelled MirrorSyncResult = "cancelled"
	// MirrorSyncResultIncomplete means some LFS objects were not synchronized.
	MirrorSyncResultIncomplete MirrorSyncResult = "incomplete"
	// MirrorSyncResultTooLarge means the repository exceeded an LFS size limit.
	MirrorSyncResultTooLarge MirrorSyncResult = "too_large"
	// MirrorSyncResultStateInvalid means the persisted mirror and task relationship is inconsistent.
	MirrorSyncResultStateInvalid MirrorSyncResult = "state_invalid"
)

// MirrorSyncListReq contains the initial list query fields. It can be extended without changing status calculation.
type MirrorSyncListReq struct {
	Page   int                     `json:"page"`
	Per    int                     `json:"per"`
	Search string                  `json:"search"`
	Status MirrorSyncOverallStatus `json:"status"`
}

// MirrorSyncStageSummary contains the stable list fields for one synchronization stage.
type MirrorSyncStageSummary struct {
	State  MirrorSyncStageState `json:"state"`
	Result MirrorSyncResult     `json:"result"`
}

// MirrorSyncSummary contains the public fields returned for one mirror synchronization list row.
type MirrorSyncSummary struct {
	MirrorID     int64  `json:"mirror_id"`
	RepositoryID int64  `json:"repository_id"`
	TaskID       int64  `json:"task_id"`
	SourceURL    string `json:"source_url"`
	Username     string `json:"username"`
	AccessToken  string `json:"access_token"`
	RepoPath     string `json:"repo_path"`
	// Priority is the scheduling priority persisted on the current mirror task.
	Priority MirrorPriority `json:"priority"`
	// IsUrgent reports whether the current mirror task uses the urgent queues.
	IsUrgent bool `json:"is_urgent"`
	Progress int  `json:"progress"`
	// RetryCount is the current stage retry count persisted on the mirror task.
	RetryCount int `json:"retry_count"`
	// MaxRetryCount is the configured maximum retry count and excludes the initial execution.
	MaxRetryCount int                     `json:"max_retry_count"`
	Status        MirrorSyncOverallStatus `json:"status"`
	Result        MirrorSyncResult        `json:"result"`
	Retrying      bool                    `json:"retrying"`
	RepoStage     MirrorSyncStageSummary  `json:"repo_stage"`
	LFSStage      MirrorSyncStageSummary  `json:"lfs_stage"`
}

// MirrorSyncListResponse contains one page of mirror synchronization summaries.
type MirrorSyncListResponse struct {
	Items []MirrorSyncSummary `json:"items"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Per   int                 `json:"per"`
}

type CreateMirrorParams struct {
	SourceUrl      string `json:"source_url"`
	MirrorSourceID int64  `json:"mirror_source_id"`
	Username       string `json:"username"`
	AccessToken    string `json:"access_token"`
	// Urgent routes the initial synchronization through the urgent mirror queues.
	Urgent bool `json:"urgent,omitempty"`
}

type GetMirrorReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
}

type UpdateMirrorReq = CreateMirrorReq

type UpdateMirrorParams = CreateMirrorParams

type DeleteMirrorReq = GetMirrorReq

type CreateMirrorSourceReq struct {
	SourceName  string `json:"source_name" binding:"required"`
	InfoAPiUrl  string `json:"info_api_url"`
	CurrentUser string `json:"current_user"`
}

type UpdateMirrorSourceReq struct {
	ID          int64  `json:"id"`
	SourceName  string `json:"source_name" binding:"required"`
	InfoAPiUrl  string `json:"info_api_url"`
	CurrentUser string `json:"current_user"`
}

type CreateMirrorRepoReq struct {
	SourceNamespace string `json:"source_namespace" binding:"required"`
	SourceName      string `json:"source_name" binding:"required"`
	// source id for HF,github etc
	MirrorSourceID int64 `json:"mirror_source_id"`

	// repo basic info
	RepoType RepositoryType `json:"repo_type" binding:"required"`
	Private  *bool          `json:"private,omitempty"`
	// CreateTargetRepo controls target repository creation. Nil selects automatically,
	// true requires a new target, and false requires an existing target.
	CreateTargetRepo *bool `json:"create_target_repo,omitempty"`

	DefaultBranch string `json:"branch"`

	// mirror source info
	SourceGitCloneUrl string `json:"source_url" binding:"required"`
	// Username is the upstream Git HTTP username.
	Username string `json:"username"`
	// AccessToken is the upstream Git HTTP access token.
	AccessToken string `json:"access_token"`
	Description string `json:"description"`
	License     string `json:"license"`
	CurrentUser string `json:"current_user"`

	//MCP only
	MCPServerAttributes MCPServerAttributes `json:"mcp_server_attributes"`

	// fork repo, local namespace/name
	ForkNamespace string `json:"fork_namespace"`
	ForkName      string `json:"fork_name"`
	// Priority controls scheduling order within the selected mirror queue.
	Priority MirrorPriority `json:"priority,omitempty"`
	// Urgent routes the initial synchronization through the urgent mirror queues.
	Urgent bool `json:"urgent,omitempty"`
}

type ResolveNamespaceReq struct {
	SourceNamespace string         `json:"source_namespace" form:"source_namespace" binding:"required"`
	SourceName      string         `json:"source_name" form:"source_name" binding:"required"`
	RepoType        RepositoryType `json:"repo_type" form:"repo_type" binding:"required"`
}

type ResolveNamespaceResp struct {
	TargetNamespace string `json:"target_namespace"`
	TargetName      string `json:"target_name"`
	Exists          bool   `json:"exists"`
}

type MCPServerAttributes struct {
	StarCount     int       `json:"star_count"`
	Tools         []MCPTool `json:"tools"`
	Configuration MCPSchema `json:"configuration"`
	AvatarURL     string    `json:"avatar_url"`
}

type BatchCreateMirrorReq struct {
	Mirrors []MirrorReq `json:"mirrors"`
}

type MirrorReq struct {
	SourceURL string `json:"source_url" binding:"required"`
	// Username is the upstream Git HTTP username.
	Username string `json:"username"`
	// AccessToken is the upstream Git HTTP access token.
	AccessToken   string         `json:"access_token"`
	SourceID      int64          `json:"mirror_source_id" binding:"required"`
	RepoType      RepositoryType `json:"repo_type" binding:"required"`
	DefaultBranch string         `json:"branch" binding:"required"`
	Priority      int8           `json:"priority"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type MirrorRepo struct {
	ID         int64                `json:"id"`
	TaskID     int64                `json:"task_id"`
	Path       string               `json:"path"`
	SyncStatus RepositorySyncStatus `json:"sync_status"`
	Progress   int8                 `json:"progress"`
	RepoType   RepositoryType       `json:"repo_type"`
}

type MirrorResp struct {
	Progress    int8
	LastMessage string
	TaskStatus  MirrorTaskStatus
}

type MirrorTaskStatus string

const (
	MirrorQueued           MirrorTaskStatus = "queued"
	MirrorRepoSyncStart    MirrorTaskStatus = "running"
	MirrorRepoSyncFailed   MirrorTaskStatus = "repo_failed"
	MirrorRepoSyncFinished MirrorTaskStatus = "repo_synced"
	MirrorRepoSyncFatal    MirrorTaskStatus = "repo_fatal"
	MirrorLfsSyncStart     MirrorTaskStatus = "lfs_start"
	MirrorLfsSyncFailed    MirrorTaskStatus = "failed"
	MirrorLfsSyncFinished  MirrorTaskStatus = "finished"
	MirrorLfsSyncFatal     MirrorTaskStatus = "fatal"
	MirrorLfsIncomplete    MirrorTaskStatus = "incomplete"
	MirrorRepoTooLarge     MirrorTaskStatus = "too_large"
	MirrorCanceled         MirrorTaskStatus = "cancelled"
)

type Mapping string

const (
	AutoMapping       Mapping = "auto"
	HFMapping         Mapping = "hf"
	CSGHubMapping     Mapping = "csghub"
	ModelScopeMapping Mapping = "ms"
)

type MirrorPriority int

const (
	ASAPMirrorPriority   MirrorPriority = 1
	HighMirrorPriority   MirrorPriority = 2
	MediumMirrorPriority MirrorPriority = 3
	LowMirrorPriority    MirrorPriority = 4

	//ASAPMirrorPriority   MirrorPriority = 12
	//P11MirrorPriority    MirrorPriority = 11
	//HighMirrorPriority   MirrorPriority = 10
	//P9MirrorPriority     MirrorPriority = 9
	//P8MirrorPriority     MirrorPriority = 8
	//P7MirrorPriority     MirrorPriority = 7
	//MediumMirrorPriority MirrorPriority = 6
	//P5MirrorPriority     MirrorPriority = 5
	//P4MirrorPriority     MirrorPriority = 4
	//P3MirrorPriority     MirrorPriority = 3
	//P2MirrorPriority     MirrorPriority = 2
	//LowMirrorPriority    MirrorPriority = 1
)

type Mirror struct {
	ID           int64        `json:"id"`
	SourceUrl    string       `json:"source_url"`
	MirrorSource MirrorSource `json:"mirror_source"`
	//source user name
	Username string `json:"username"`
	// source access token
	AccessToken     string           `json:"access_token"`
	PushUrl         string           `json:"push_url"`
	PushUsername    string           `json:"push_username"`
	PushAccessToken string           `json:"push_access_token"`
	Repository      *Repository      `json:"repository"`
	LastUpdatedAt   time.Time        `json:"last_updated_at"`
	SourceRepoPath  string           `json:"source_repo_path"`
	LocalRepoPath   string           `json:"local_repo_path"`
	LastMessage     string           `json:"last_message"`
	Status          MirrorTaskStatus `json:"status"`
	Progress        int8             `json:"progress"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type MirrorSource struct {
	SourceName string `json:"source_name"`
}

type MirrorStatusCount struct {
	Status MirrorTaskStatus
	Count  int
}

// MirrorListResp groups mirror tasks by execution state.
type MirrorListResp struct {
	// RunningTasks are tasks currently executing repo or LFS sync.
	RunningTasks []MirrorTask `json:"running_tasks"`
	// WaitingTasks are queued or retryable tasks waiting for worker execution.
	WaitingTasks []MirrorTask `json:"waiting_tasks"`
}

type MirrorTask struct {
	MirrorID  int64  `json:"mirror_id"`
	TaskID    int64  `json:"task_id"`
	SourceUrl string `json:"source_url"`
	Priority  int    `json:"priority"`
	RepoPath  string `json:"repo_path"`
}

type SyncInfo struct {
	RemoteURL string         `json:"remote_url"`
	LocalURL  string         `json:"local_url"`
	Status    string         `json:"status"`
	RepoType  RepositoryType `json:"repo_type"`
	Path      string         `json:"path"`
	Size      int64          `json:"size"`
}

type CreateMirrorNamespaceMappingReq struct {
	SourceNamespace string `json:"source_namespace"`
	TargetNamespace string `json:"target_namespace"`
	Enabled         *bool  `json:"enabled"`
}

type UpdateMirrorNamespaceMappingReq struct {
	SourceNamespace *string `json:"source_namespace"`
	TargetNamespace *string `json:"target_namespace"`
	Enabled         *bool   `json:"enabled"`
	ID              int64   `json:"id"`
}

type MirrorFilter struct {
	Search string            `json:"search"`
	Status *MirrorTaskStatus `json:"status"`
}
