package types

import (
	"time"
)

type ModelDetail struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Introduction  string `json:"introduction"`
	License       string `json:"license"`
	Private       bool   `json:"private"`
	Downloads     int    `json:"downloads"`
	LastUpdatedAt string `json:"last_updated_at"`
	HTTPCloneURL  string `json:"http_clone_url"`
	SSHCloneURL   string `json:"ssh_clone_url"`
	Size          int    `json:"size"`
	DefaultBranch string `json:"default_branch"`
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
	CreateRepoReq
}

type UpdateModelReq struct {
	CreateRepoReq
}

type UpdateDownloadsReq struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	ReqDate    string `json:"date"`
	Date       time.Time
	CloneCount int64 `json:"download_count"`
}

type ModelPredictReq struct {
	Namespace   string `json:"-"`
	Name        string `json:"-"`
	Input       string `json:"input"`
	Version     string `json:"version"`
	CurrentUser string `json:"current_user"`
}

type ModelPredictResp struct {
	Content string `json:"content"`
	// TODO:add metrics like tokens, latency etc
}

type CreateRepoReq struct {
	Username      string         `json:"username" example:"creator_user_name"`
	Namespace     string         `json:"namespace" example:"user_or_org_name"`
	Name          string         `json:"name" example:"model_name_1"`
	Nickname      string         `json:"nickname" example:"model display name"`
	Description   string         `json:"description"`
	Private       bool           `json:"private"`
	Labels        string         `json:"labels" example:""`
	License       string         `json:"license" example:"MIT"`
	Readme        string         `json:"readme"`
	DefaultBranch string         `json:"default_branch" example:"main"`
	RepoType      RepositoryType `json:"-"`
}
