package types

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

type DeleteUserTokenRequest = CreateUserTokenRequest
