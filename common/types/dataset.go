package types

import (
	"time"
)

type DatasetTagCommit struct {
	ID string `json:"id"`
}

type CreateDatasetReq struct {
	CreateRepoReq
}

type UpdateDatasetReq struct {
	CreateRepoReq
}

type Dataset struct {
	ID            int64      `json:"id,omitempty"`
	Name          string     `json:"name"`
	Nickname      string     `json:"nickname"`
	Description   string     `json:"description"`
	Likes         int64      `json:"likes"`
	Downloads     int64      `json:"downloads"`
	Path          string     `json:"path"`
	RepositoryID  int64      `json:"repository_id"`
	Repository    Repository `json:"repository"`
	Private       bool       `json:"private"`
	User          User       `json:"user"`
	Tags          []RepoTag  `json:"tags"`
	DefaultBranch string     `json:"default_branch"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
