package types

type CreateSSHKeyRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Content  string `json:"content"`
}
