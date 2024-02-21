package types

import "time"

type CreateSpaceReq struct {
	CreateRepoReq
	// Creator   string `json:"username" example:"creator_user_name"`
	// Namespace string `json:"namespace" example:"user_or_org_name"`
	// Name      string `json:"name" example:"space_name_1"`
	// License   string `json:"license" example:"MIT"`
	Sdk string `json:"sdk" example:"gradio"`
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
	Sdk string `json:"sdk" example:"gradio"`
	// the serving endpoint url
	Endpoint string `json:"endpoint" example:"https://localhost/spaces/myname/myspace"`
	// deploying, running, failed
	RunningStatus string    `json:"running_status"`
	CoverImg      string    `json:"cover_img"`
	CreatedAt     time.Time `json:"created_at"`
	Likes         int64     `json:"like_count"`
	Private       bool      `json:"private"`
}
