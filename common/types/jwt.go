package types

type CreateJWTReq struct {
	CurrentUser   string   `json:"current_user"`
	Organizations []string `json:"organizations"`
}
