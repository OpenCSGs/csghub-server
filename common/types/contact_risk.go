package types

// RiskPhonePrefix identifies a risky national-number prefix within a calling code.
type RiskPhonePrefix struct {
	PhoneArea string
	Prefix    string
}
