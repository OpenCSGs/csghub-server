package errorx

const errReqPrefix = "REQ-ERR"

const (
	errBadRequest = iota

	errReqBodyFormat
	errReqBodyEmpty
	errReqBodyTooLarge

	errReqParamMissing
	errReqParamDuplicate
	errReqParamInvalid
	errReqParamOutOfRange
	errReqParamTypeError

	errReqContentTypeUnsupported
	errRateLimitExceeded
	errLimitedIPLocation
	errCaptchaIncorrect
)

var (
	// --- Req-ERR-xxx: Request related errors ---
	// general bad request error, server cannot process the request
	//
	// Description: The server could not understand the request due to malformed syntax or invalid request message framing.
	//
	// Description_ZH: 由于语法格式错误或无效的请求消息，服务器无法理解该请求。
	//
	// en-US: Bad request
	//
	// zh-CN: 错误的请求
	//
	// zh-HK: 錯誤的請求
	ErrBadRequest error = CustomError{prefix: errReqPrefix, code: errBadRequest}
	// request body format is incorrect, e.g. invalid JSON
	//
	// Description: The format of the request body is invalid or cannot be parsed. For example, the provided JSON is malformed.
	//
	// Description_ZH: 请求正文的格式无效或无法解析。例如，提供的JSON格式不正确。
	//
	// en-US: Invalid request body format
	//
	// zh-CN: 请求体格式错误
	//
	// zh-HK: 請求體格式錯誤
	ErrReqBodyFormat error = CustomError{prefix: errReqPrefix, code: errReqBodyFormat}
	// request body is empty but it is required
	//
	// Description: The request body is empty, but this endpoint requires a non-empty body to proceed.
	//
	// Description_ZH: 请求正文为空，但此接口需要非空的正文才能继续操作。
	//
	// en-US: Request body cannot be empty
	//
	// zh-CN: 请求体不能为空
	//
	// zh-HK: 請求體不能為空
	ErrReqBodyEmpty error = CustomError{prefix: errReqPrefix, code: errReqBodyEmpty}
	// request body is too large and exceeds a server-defined limit
	//
	// Description: The size of the request body exceeds the server's configured limit for this endpoint.
	//
	// Description_ZH: 请求正文的大小超过了服务器为此接口配置的限制。
	//
	// en-US: Request body too large
	//
	// zh-CN: 请求体过大
	//
	// zh-HK: 請求體過大
	ErrReqBodyTooLarge error = CustomError{prefix: errReqPrefix, code: errReqBodyTooLarge}
	// a duplicate request parameter was found
	//
	// Description: A parameter was provided more than once in the request, which is not allowed for this endpoint.
	//
	// Description_ZH: 请求中多次提供了同一个参数，而此接口不允许这样做。
	//
	// en-US: Duplicate request parameter
	//
	// zh-CN: 重复的请求参数
	//
	// zh-HK: 重複的請求參數
	ErrReqParamDuplicate error = CustomError{prefix: errReqPrefix, code: errReqParamDuplicate}
	// a request parameter is invalid (e.g. wrong type, out of range)
	//
	// Description: A request parameter is invalid. It may be of the wrong type, outside the allowed range, or a value that is not permissible.
	//
	// Description_ZH: 请求参数无效。它可能是错误的类型、超出允许范围或是不允许的值。
	//
	// en-US: Invalid request parameter
	//
	// zh-CN: 无效的请求参数
	//
	// zh-HK: 無效的請求參數
	ErrReqParamInvalid error = CustomError{prefix: errReqPrefix, code: errReqParamInvalid}
	// the 'Content-Type' of the request is not supported
	//
	// Description: The 'Content-Type' of the request is not supported by this endpoint. Please check the API documentation for allowed content types.
	//
	// Description_ZH: 此接口不支持请求的'Content-Type'。请查阅API文档以了解允许的内容类型。
	//
	// en-US: Unsupported content type
	//
	// zh-CN: 不支持的内容类型
	//
	// zh-HK: 不支持的內容類型
	ErrReqContentTypeUnsupported = CustomError{prefix: errReqPrefix, code: errReqContentTypeUnsupported}

	// request rate limit exceeded, captcha is required
	//
	// Description: The user has sent too many requests in a given amount of time. Further requests will be blocked until the rate limit resets or a valid captcha is provided.
	//
	// Description_ZH: 用户在给定的时间内发送了太多的请求。在速率限制重置或提供有效的验证码之前，将阻止进一步的请求。
	//
	// en-US: Too many requests, captcha is required
	//
	// zh-CN: 请求过于频繁，需要验证码
	//
	// zh-HK: 請求過於頻繁，需要驗證碼
	ErrRateLimitExceeded = CustomError{prefix: errReqPrefix, code: errRateLimitExceeded}

	// requests from this IP location are restricted, captcha is required
	//
	// Description: Requests originating from this IP location are restricted. To proceed, please complete a captcha verification.
	//
	// Description_ZH: 来自此IP位置的请求受到限制。要继续操作，请完成验证码验证。
	//
	// en-US: Requests from this IP location are restricted, captcha is required
	//
	// zh-CN: 来自该IP位置的请求受限，需要验证码
	//
	// zh-HK: 來自該IP位置的請求受限，需要驗證碼
	ErrLimitedIPLocation = CustomError{prefix: errReqPrefix, code: errLimitedIPLocation}

	// captcha verification failed
	//
	// Description: The provided captcha verification failed. Please try again with a valid captcha.
	//
	// Description_ZH: 提供的验证码验证失败。请使用有效的验证码重试。
	//
	// en-US: Captcha verification failed
	//
	// zh-CN: 验证码验证失败
	//
	// zh-HK: 驗證碼驗證失敗
	ErrCaptchaIncorrect = CustomError{prefix: errReqPrefix, code: errCaptchaIncorrect}
)

func BadRequest(originErr error, ext context) error {
	return CustomError{
		prefix:  errReqPrefix,
		code:    errBadRequest,
		err:     originErr,
		context: ext,
	}
}

func ReqBodyFormat(err error, ext context) error {
	return CustomError{
		prefix:  errReqPrefix,
		code:    errReqBodyFormat,
		err:     err,
		context: ext,
	}
}

func ReqParamInvalid(err error, ext context) error {
	return CustomError{
		prefix:  errReqPrefix,
		code:    errReqParamInvalid,
		err:     err,
		context: ext,
	}
}
