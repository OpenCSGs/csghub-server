package types

import "time"

type CreateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username   string `json:"username"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	CasdoorUID string `json:"casdoor_uid"`
}

type UpdateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username   string `json:"username"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	CasdoorUID string `json:"casdoor_uid"`
}

type UpdateUserResp struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserTokenRequest struct {
	Username    string                 `json:"username"`
	Name        string                 `json:"name"`
	ExpiredAt   time.Time              `json:"expired_at"`
	Application AccessTokenApplication `json:"application"`
}

type UserDatasetsReq struct {
	Owner       string `json:"owner"`
	CurrentUser string `json:"current_user"`
	PageOpts
}

type (
	UserModelsReq          = UserDatasetsReq
	UserCodesReq           = UserDatasetsReq
	UserSpacesReq          = UserDatasetsReq
	DeleteUserTokenRequest = CreateUserTokenRequest
)

type PageOpts struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

type User struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}

type UserLikesRequest struct {
	Username    string `json:"username"`
	Repo_id     int64  `json:"repo_id"`
	CurrentUser string `json:"current_user"`
}

type WhoamiResponse struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Auth  Auth   `json:"auth"`
}

type AccessToken struct {
	DisplayName string `json:"displayName,omitempty"`
	Role        string `json:"role,omitempty"`
}

type Auth struct {
	AccessToken `json:"accessToken,omitempty"`
	Type        string `json:"type,omitempty"`
}

type UserRepoReq struct {
	CurrentUser string `json:"current_user"`
	PageOpts
}

type AccessTokenApplication string

const (
	AccessTokenApplicationGit    AccessTokenApplication = "git"
	AccessTokenApplicationMirror AccessTokenApplication = "mirror"
)
