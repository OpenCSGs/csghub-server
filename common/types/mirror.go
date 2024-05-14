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
	RepoType        RepositoryType `json:"repo_type"`
}

type GetMirrorReq struct {
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	RepoType  RepositoryType `json:"repo_type"`
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
