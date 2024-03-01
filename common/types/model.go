package types

import (
	"time"
)

type RepoBranchCommit struct {
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
	RepoType   RepositoryType
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

type DeleteRepoReq struct {
	Username  string         `json:"username" example:"creator_user_name"`
	Namespace string         `json:"namespace" example:"user_or_org_name"`
	Name      string         `json:"name" example:"model_name_1"`
	RepoType  RepositoryType `json:"-"`
}

type Model struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Nickname      string     `json:"nickname"`
	Description   string     `json:"description"`
	Likes         int64      `json:"likes"`
	Downloads     int64      `json:"downloads"`
	Path          string     `json:"path"`
	RepositoryID  int64      `json:"repository_id"`
	Private       bool       `json:"private"`
	User          User       `json:"user"`
	Tags          []RepoTag  `json:"tags"`
	Repository    Repository `json:"repository"`
	DefaultBranch string     `json:"default_branch"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	// widget UI style: generation,chat
	WidgetType ModelWidgetType `json:"widget_type" example:"generation"`
	// url to interact with the model
	Status string `json:"status" example:"RUNNING"`
}

type ModelWidgetType string

const (
	ModelWidgetTypeGeneration ModelWidgetType = "generation"
	ModelWidgetTypeChat       ModelWidgetType = "chat"
)
