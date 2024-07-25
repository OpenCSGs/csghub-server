package types

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
	SourceName string `json:"source_name" binding:"required"`
	InfoAPiUrl string `json:"info_api_url"`
}

type UpdateMirrorSourceReq struct {
	ID         int64  `json:"id"`
	SourceName string `json:"source_name" binding:"required"`
	InfoAPiUrl string `json:"info_api_url"`
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
