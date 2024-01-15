package types

// swagger:model
type CreateUserRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type UpdateUserRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
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

type UserModelsReq = UserDatasetsReq
type DeleteUserTokenRequest = CreateUserTokenRequest

type PageOpts struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}
