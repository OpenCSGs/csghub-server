package types

type CreateSSHKeyRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Content  string `json:"content"`
}

var _ SensitiveRequestV2 = (*CreateSSHKeyRequest)(nil)

func (c *CreateSSHKeyRequest) GetSensitiveFields() []SensitiveField {
	return []SensitiveField{
		{
			Name: "name",
			Value: func() string {
				return c.Content
			},
			Scenario: "nickname_detection",
		},
	}
}
