package types

import "time"

type CreateSpaceReq struct {
	CreateRepoReq
	// Creator   string `json:"username" example:"creator_user_name"`
	// Namespace string `json:"namespace" example:"user_or_org_name"`
	// Name      string `json:"name" example:"space_name_1"`
	// License   string `json:"license" example:"MIT"`
	Sdk           string `json:"sdk" example:"1"`
	SdkVersion    string `json:"sdk_version" example:"v0.1"`
	CoverImageUrl string `json:"cover_image_url"`
	Template      string `json:"template"`
	Env           string `json:"env"`
	Hardware      string `json:"hardware"`
	Secrets       string `json:"secrets"`
	// Private   bool   `json:"private"`
}

// Space is the domain object for spaces
type Space struct {
	ID            int64      `json:"id"`
	Creator       string     `json:"username" example:"creator_user_name"`
	Namespace     string     `json:"namespace" example:"user_or_org_name"`
	Name          string     `json:"name" example:"space_name_1"`
	Nickname      string     `json:"nickname" example:""`
	Description   string     `json:"description" example:""`
	Path          string     `json:"path" example:"user_or_org_name/space_name_1"`
	License       string     `json:"license" example:"MIT"`
	Tags          []RepoTag  `json:"tags"`
	User          User       `json:"user"`
	Repository    Repository `json:"repository"`
	DefaultBranch string     `json:"default_branch"`
	Likes         int64      `json:"like_count"`
	Private       bool       `json:"private"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// like gradio,steamlit etc
	Sdk           string `json:"sdk" example:"1"`
	SdkVersion    string `json:"sdk_version" example:"v0.1"`
	CoverImageUrl string `json:"cover_image_url"`
	Template      string `json:"template"`
	Env           string `json:"env"`
	Hardware      string `json:"hardware"`
	Secrets       string `json:"secrets"`
	// the serving endpoint url
	Endpoint string `json:"endpoint" example:"https://localhost/spaces/myname/myspace"`
	// deploying, running, failed
	Status string `json:"status"`
}

type UpdateSpaceReq struct {
	CreateRepoReq
	// Creator   string `json:"username" example:"creator_user_name"`
	// Namespace string `json:"namespace" example:"user_or_org_name"`
	// Name      string `json:"name" example:"space_name_1"`
	// License   string `json:"license" example:"MIT"`
	Sdk           string `json:"sdk" example:"1"`
	SdkVersion    string `json:"sdk_version" example:"v0.1"`
	CoverImageUrl string `json:"cover_image_url"`
	Template      string `json:"template"`
	Env           string `json:"env"`
	Hardware      string `json:"hardware"`
	Secrets       string `json:"secrets"`
	// Private   bool   `json:"private"`
}
