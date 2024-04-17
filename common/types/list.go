package types

import "time"

type ListByPathReq struct {
	Paths []string `json:"paths"`
}

type ModelResp struct {
	Path        string    `json:"path"`
	UpdatedAt   time.Time `json:"updated_at"`
	Downloads   int64     `json:"downloads"`
	Private     bool      `json:"private"`
	Name        string    `json:"name"`
	Nickname    string    `json:"nickname"`
	Description string    `json:"description"`
	Tags        []RepoTag `json:"tags,omitempty"`
}

type (
	DatasetResp = ModelResp
	SpaceResp   = ModelResp
)
