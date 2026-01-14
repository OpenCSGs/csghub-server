package types

import "net/url"

type XnetTokenReq struct {
	Permission string         `json:"permission"`
	Namespace  string         `json:"namespace"`
	Name       string         `json:"name"`
	RepoType   RepositoryType `json:"repoType"`
	Branch     string         `json:"branch"`
	Username   string         `json:"username"`
}

type XnetTokenResp struct {
	AccessToken string `json:"accessToken"`
	CasURL      string `json:"casUrl"`
	ExprireTime int64  `json:"exp"`
}

type XetEnabled struct {
	ID      string `json:"id"`
	Enabled bool   `json:"xetEnabled"`
	HashID  string `json:"_id"`
}

type XnetDownloadURLResp struct {
	URL url.URL `json:"url"`
}

type XetFileExistsResp struct {
	Exists bool `json:"exists"`
}

type XnetMigrationTaskStatus string

const (
	XnetMigrationTaskStatusPending   XnetMigrationTaskStatus = "pending"
	XnetMigrationTaskStatusRunning   XnetMigrationTaskStatus = "running"
	XnetMigrationTaskStatusCompleted XnetMigrationTaskStatus = "completed"
	XnetMigrationTaskStatusFailed    XnetMigrationTaskStatus = "failed"
)
