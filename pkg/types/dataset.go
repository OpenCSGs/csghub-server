package types

import "git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"

type Dataset database.Repository

type DatasetDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	DownloadCount int    `json:"download_count"`
	LastUpdatedAt string `json:"last_updated_at"`
}

type DatasetTag struct {
	Name    string           `json:"name"`
	Message string           `json:"message"`
	Commit  DatasetTagCommit `json:"commit"`
}

type DatasetTagCommit struct {
	ID string `json:"id"`
}

type DatasetBranch struct {
	Name    string              `json:"name"`
	Message string              `json:"message"`
	Commit  DatasetBranchCommit `json:"commit"`
}

type DatasetBranchCommit struct {
	ID string `json:"id"`
}
