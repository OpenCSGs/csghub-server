package types

type CreateMirrorReq struct {
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	Interval        string         `json:"interval"`
	SourceUrl       string         `json:"source_url" binding:"required"`
	MirrorSourceID  int64          `json:"mirror_source_id" binding:"required"`
	Username        string         `json:"-"`
	AccessToken     string         `json:"-"`
	PushUrl         string         `json:"push_url" binding:"required"`
	PushUsername    string         `json:"push_username" binding:"required"`
	PushAccessToken string         `json:"push_access_token" binding:"required"`
	SourceRepoPath  string         `json:"source_repo_path" binding:"required"`
	LocalRepoPath   string         `json:"local_repo_path" binding:"required"`
	CurrentUser     string         `json:"current_user"`
	RepoType        RepositoryType `json:"repo_type"`
}

type GetMirrorReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	CurrentUser string         `json:"current_user"`
}

type UpdateMirrorReq = CreateMirrorReq

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
}
