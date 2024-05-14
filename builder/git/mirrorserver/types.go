package mirrorserver

import (
	"opencsg.com/csghub-server/common/types"
)

type CreateMirrorRepoReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CloneUrl    string `json:"clone_url"`
	Username    string `json:"username"`
	AccessToken string `json:"access_token"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
	Interval    string `json:"interval"`
}

type CreatePushMirrorReq struct {
	Name        string               `json:"name"`
	PushUrl     string               `json:"push_url"`
	Username    string               `json:"username"`
	AccessToken string               `json:"access_token"`
	Interval    string               `json:"interval"`
	RepoType    types.RepositoryType `json:"repo_type"`
}

type MirrorSyncReq struct {
	Namespace string               `json:"namespace"`
	Name      string               `json:"name"`
	RepoType  types.RepositoryType `json:"repo_type"`
}
