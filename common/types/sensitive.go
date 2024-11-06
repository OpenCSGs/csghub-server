package types

type SensitiveRequestV2 interface {
	GetSensitiveFields() []SensitiveField
}

type SensitiveField struct {
	Name  string
	Value func() string
	// like nickname, chat, comment, etc. See sensitive.Scenario for more details.
	Scenario string
}
