package errorx

import "fmt"

const errRiskPrefix = "RISK-ERR"

const (
	contactBlocked = iota
)

var (
	// contact risk rules rejected the request
	//
	// Description: The contact (email domain or phone number) is not allowed by the risk rules.
	//
	// Description_ZH: 联系方式（邮箱域名或手机号）不在风控规则允许范围内。
	//
	// en-US: Contact is not allowed by risk rules
	//
	// zh-CN: 联系方式不在风控允许范围内
	//
	// zh-HK: 聯繫方式不在風控允許範圍內
	ErrContactBlocked error = CustomError{prefix: errRiskPrefix, code: contactBlocked}
)

// ContactBlocked creates a risk rejection carrying the risk engine's message
// (which states the specific scene, e.g. profile update or verification code
// requests) and the reason code for tracing.
func ContactBlocked(reasonCode, message string) error {
	return CustomError{
		prefix:  errRiskPrefix,
		code:    contactBlocked,
		err:     fmt.Errorf("%s", message),
		context: map[string]interface{}{"reason_code": reasonCode},
	}
}
