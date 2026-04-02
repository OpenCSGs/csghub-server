package errorx

const errSandboxPrefix = "SANDBOX-ERR"

// Error code enumeration
const (
	codeSandboxNameEmptyErr       = iota // name is empty
	codeSandboxNameTooLongErr            // name exceeds maximum length
	codeSandboxNameUppercaseErr          // contains uppercase letters
	codeSandboxNameInvalidCharErr        // contains invalid characters or format error
)

var (
	// Description: Sandbox name is empty.
	//
	// Description_ZH: 沙箱名称不能为空
	//
	// en-US: Sandbox name is empty.
	//
	// zh-CN: 沙箱名称不能为空
	//
	// zh-HK: 沙箱名稱不能為空
	ErrSandboxNameEmpty error = CustomError{prefix: errSandboxPrefix, code: codeSandboxNameEmptyErr}

	// Description: Sandbox name exceeds maximum length (253 characters).
	//
	// Description_ZH: 沙箱名称长度超过限制（最大253个字符）
	//
	// en-US: Sandbox name exceeds maximum length (253 characters).
	//
	// zh-CN: 沙箱名称长度超过限制（最大253个字符）
	//
	// zh-HK: 沙箱名稱長度超過限制（最大253個字符）
	ErrSandboxNameTooLong error = CustomError{prefix: errSandboxPrefix, code: codeSandboxNameTooLongErr}

	// Description: Sandbox name contains uppercase letters (only lowercase allowed).
	//
	// Description_ZH: 沙箱名称包含大写字母（仅允许小写）
	//
	// en-US: Sandbox name contains uppercase letters (only lowercase allowed).
	//
	// zh-CN: 沙箱名称包含大写字母（仅允许小写）
	//
	// zh-HK: 沙箱名稱包含大寫字母（僅允許小寫）
	ErrSandboxNameUppercase error = CustomError{prefix: errSandboxPrefix, code: codeSandboxNameUppercaseErr}

	// Description: Sandbox name has invalid characters or format (only a-z, 0-9, -, . allowed; cannot start/end with -/. or have consecutive -/. ).
	//
	// Description_ZH: 沙箱名称包含非法字符或格式错误（仅允许小写字母、数字、连字符、点；不能以连字符/点开头/结尾，不能有连续的连字符/点）
	//
	// en-US: Sandbox name has invalid characters or format (only a-z, 0-9, -, . allowed; cannot start/end with -/. or have consecutive -/. ).
	//
	// zh-CN: 沙箱名称包含非法字符或格式错误（仅允许小写字母、数字、连字符、点；不能以连字符/点开头/结尾，不能有连续的连字符/点）
	//
	// zh-HK: 沙箱名稱包含非法字符或格式錯誤（僅允許小寫字母、數字、連字符、點；不能以連字符/點開頭/結尾，不能有連續的連字符/點）
	ErrSandboxNameInvalidChar error = CustomError{prefix: errSandboxPrefix, code: codeSandboxNameInvalidCharErr}
)
