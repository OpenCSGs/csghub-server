package types

import "git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"

type Model database.Repository

type ModelDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	DownloadCount int    `json:"download_count"`
	LastUpdatedAt string `json:"last_updated_at"`
}

type ModelTag struct {
	Name    string         `json:"name"`
	Message string         `json:"message"`
	Commit  ModelTagCommit `json:"commit"`
}

type ModelTagCommit struct {
	ID string `json:"id"`
}

type ModelBranch struct {
	Name    string            `json:"name"`
	Message string            `json:"message"`
	Commit  ModelBranchCommit `json:"commit"`
}

type ModelBranchCommit struct {
	ID string `json:"id"`
}

type CreateModelReq struct {
	Username      string `json:"username"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch"`
}

type UpdateModelReq struct {
	Username      string `json:"username"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Labels        string `json:"labels"`
	License       string `json:"license"`
	Readme        string `json:"readme"`
	DefaultBranch string `json:"default_branch"`
}
