package types

type RemoveMemberRequest struct {
	Role string `json:"role" binding:"required"`
}
