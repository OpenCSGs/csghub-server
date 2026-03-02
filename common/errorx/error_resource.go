package errorx

const errResourcePrefix = "RESOURCE-ERR"

const (
	codeResourceNotFoundErr = iota
	codeResourceUnavailableErr
)

var (
	// Description: The resource was not found.
	//
	// Description_ZH: 无法找到指定的资源
	//
	// en-US: The resource was not found.
	//
	// zh-CN: 无法找到指定的资源
	//
	// zh-HK: 無法找到指定的資源
	ErrResourceNotFound error = CustomError{prefix: errResourcePrefix, code: codeResourceNotFoundErr}

	// Description: The resource is temporarily unavailable.
	//
	// Description_ZH: 资源暂不可用
	//
	// en-US: The resource is temporarily unavailable.
	//
	// zh-CN: 资源暂不可用
	//
	// zh-HK: 資源暫不可用
	ErrResourceUnavailable error = CustomError{prefix: errResourcePrefix, code: codeResourceUnavailableErr}
)
