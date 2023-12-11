package types

type CreateMemberReq struct {
	Username    string `json:"username"`
	CurrentUser string `json:"current_user"`
}

type DeleteMemberReq struct {
	Username    string `json:"username"`
	CurrentUser string `json:"current_user"`
}
