package types

type CreateSSHKeyRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Content  string `json:"content"`
}

func (c *CreateSSHKeyRequest) SensName() string {
	return c.Name
}

func (c *CreateSSHKeyRequest) SensNickName() string {
	return ""
}

func (c *CreateSSHKeyRequest) SensDescription() string {
	return ""
}

func (c *CreateSSHKeyRequest) SensHomepage() string {
	return ""
}
