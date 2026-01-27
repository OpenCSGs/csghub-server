package types

import "net/url"

type XnetTokenReq struct {
	Permission string         `json:"permission"`
	Namespace  string         `json:"namespace"`
	Name       string         `json:"name"`
	RepoType   RepositoryType `json:"repoType"`
	Branch     string         `json:"branch"`
	Username   string         `json:"username"`
	RepoID     string         `json:"repoID"`
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

type XetFileExistsReq struct {
	ObjectKey string `json:"objectKey"`
	RepoID    string `json:"repoID"`
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

type MigrationStatsResp struct {
	TotalOriginalSize      int64   `json:"total_original_size"`
	TotalXnetSize          int64   `json:"total_xnet_size"`
	TotalStatsSource       string  `json:"total_stats_source"`
	StorageEfficiencyRatio float64 `json:"storage_efficiency_ratio"`
	LfsStorageSize         int64   `json:"lfs_storage_size"`
	LfsObjectCount         int64   `json:"lfs_object_count"`
}
