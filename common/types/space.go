package types

import "time"

type CreateSpaceReq struct {
	CreateRepoReq
	// Creator   string `json:"username" example:"creator_user_name"`
	// Namespace string `json:"namespace" example:"user_or_org_name"`
	// Name      string `json:"name" example:"space_name_1"`
	// License   string `json:"license" example:"MIT"`
	SdkID         int64  `json:"sdk_id" example:"1"`
	ResourceID    int64  `json:"resource_id" example:"1"`
	CoverImageUrl string `json:"cover_image_url"`
	// Private   bool   `json:"private"`
}

// Space is the domain object for spaces
type Space struct {
	Creator   string `json:"username" example:"creator_user_name"`
	Namespace string `json:"namespace" example:"user_or_org_name"`
	Name      string `json:"name" example:"space_name_1"`
	Path      string `json:"path" example:"user_or_org_name/space_name_1"`
	License   string `json:"license" example:"MIT"`
	// like gradio,steamlit etc
	Sdk           SpaceSdk      `json:"sdk"`
	Resource      SpaceResource `json:"resource"`
	CoverImageUrl string        `json:"cover_image_url"`
	// the serving endpoint url
	Endpoint string `json:"endpoint" example:"https://localhost/spaces/myname/myspace"`
	// deploying, running, failed
	RunningStatus string    `json:"running_status"`
	CreatedAt     time.Time `json:"created_at"`
	Likes         int64     `json:"like_count"`
	Private       bool      `json:"private"`
}
