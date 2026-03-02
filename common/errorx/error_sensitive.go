package errorx

const errSensitivePrefix = "SENSITIVE-ERR"

const (
	codeSensitiveInfoNotAllowedErr = iota
)

var (
	// Description: The sensitive information is not allowed.
	//
	// Description_ZH: 敏感信息不允许被使用
	//
	// en-US: The sensitive information is not allowed.
	//
	// zh-CN: 敏感信息不允许被使用
	//
	// zh-HK: 敏感資訊不允許被使用
	ErrSensitiveInfoNotAllowed error = CustomError{prefix: errSensitivePrefix, code: codeSensitiveInfoNotAllowedErr}
)
