package types

type CreateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email"`
}

type UpdateUserRequest struct {
	// Display name of the user
	Name string `json:"name"`
	// the login name
	Username string `json:"username"`
	Email    string `json:"email"`
}

type UpdateUserResp struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CreateUserTokenRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
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
