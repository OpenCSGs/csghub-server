package types

import "time"

type CreateMirrorReq struct {
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	Interval        string         `json:"interval"`
	SourceUrl       string         `json:"source_url" binding:"required"`
	MirrorSourceID  int64          `json:"mirror_source_id" binding:"required"`
	Username        string         `json:"username"`
	AccessToken     string         `json:"password"`
	PushUrl         string         `json:"push_url"`
	PushUsername    string         `json:"push_username"`
	PushAccessToken string         `json:"push_access_token"`
	SourceRepoPath  string         `json:"source_repo_path"`
	LocalRepoPath   string         `json:"local_repo_path"`
	CurrentUser     string         `json:"current_user"`
	RepoType        RepositoryType `json:"repo_type"`
	SyncLfs         bool           `json:"sync_lfs"`
}

type CreateMirrorParams struct {
	SourceUrl      string `json:"source_url"`
	MirrorSourceID int64  `json:"mirror_source_id"`
	Username       string `json:"username"`
	AccessToken    string `json:"password"`
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
	MirrorSourceID int64 `json:"mirror_source_id" binding:"required"`

	// repo basic info
	RepoType RepositoryType `json:"repo_type" binding:"required"`

	DefaultBranch string `json:"branch"`

	// mirror source info
	SourceGitCloneUrl string `json:"source_url" binding:"required"`
	Description       string `json:"description"`
	License           string `json:"license"`
	SyncLfs           bool   `json:"sync_lfs"`
	CurrentUser       string `json:"current_user"`
}

type MirrorRepo struct {
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
	MirrorWaiting    MirrorTaskStatus = "waiting"
	MirrorRunning    MirrorTaskStatus = "running"
	MirrorRepoSynced MirrorTaskStatus = "repo_synced"
	MirrorFinished   MirrorTaskStatus = "finished"
	MirrorFailed     MirrorTaskStatus = "failed"
	MirrorIncomplete MirrorTaskStatus = "incomplete"
)

type Mapping string

const (
	AutoMapping   Mapping = "auto"
	HFMapping     Mapping = "hf"
	CSGHubMapping Mapping = "csghub"
)

type MirrorPriority int

const (
	HighMirrorPriority   MirrorPriority = 3
	MediumMirrorPriority MirrorPriority = 2
	LowMirrorPriority    MirrorPriority = 1
)

type Mirror struct {
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
}

type MirrorSource struct {
	SourceName string `json:"source_name"`
}
