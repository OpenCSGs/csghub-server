package types

type SensitiveRequest interface {
	SensName() string
	SensNickName() string
	SensDescription() string
	SensHomepage() string
}

var _ SensitiveRequest = (*UpdateRepoReq)(nil)
var _ SensitiveRequest = (*CreateRepoReq)(nil)
var _ SensitiveRequest = (*ModelRunReq)(nil)
var _ SensitiveRequest = (*InstanceRunReq)(nil)
var _ SensitiveRequest = (*CreateCollectionReq)(nil)
var _ SensitiveRequest = (*CreateSSHKeyRequest)(nil)
var _ SensitiveRequest = (*UpdateUserRequest)(nil)
var _ SensitiveRequest = (*CreateOrgReq)(nil)
var _ SensitiveRequest = (*EditOrgReq)(nil)
var _ SensitiveRequest = (*CreateUserTokenRequest)(nil)

type SensitiveRequestV2 interface {
	GetSensitiveFields() []SensitiveField
}

type SensitiveField struct {
	Name  string
	Value func() string
	// like nickname, chat, comment, etc. See sensitive.Scenario for more details.
	Scenario string
}
