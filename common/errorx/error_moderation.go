package errorx

const errModerationPrefix = "MOD-ERR"

const (
	codeNameRequire = iota
	codeWordRequire
)

var (

	// Description: The request parameter does not match the server requirements, and the server cannot process the request.
	//
	// Description_ZH: 请求参数不匹配, 服务器无法处理该请求。
	//
	// en-US: The group name cannot be empty.
	//
	// zh-CN: 组名 不能为空
	//
	// zh-HK: 組名 不能為空
	ErrSensitiveRequerName error = CustomError{prefix: errModerationPrefix, code: codeNameRequire}

	// Description: The request parameter does not match the server requirements, and the server cannot process the request.
	//
	// Description_ZH: 请求参数不匹配, 服务器无法处理该请求。
	//
	// en-US: The word cannot be empty.
	//
	// zh-CN: 内容 不能为空
	//
	// zh-HK: 內容 不能為空
	ErrSensitiveRequerWord error = CustomError{prefix: errModerationPrefix, code: codeWordRequire}
)
