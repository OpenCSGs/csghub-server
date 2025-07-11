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
)

var (
	// --- Req-ERR-xxx: Request related errors ---
	ErrBadRequest error = CustomError{prefix: errReqPrefix, code: errBadRequest}

	// Request parameter format error
	ErrReqBodyFormat error = CustomError{prefix: errReqPrefix, code: errReqBodyFormat}
	// Request body is empty
	ErrReqBodyEmpty error = CustomError{prefix: errReqPrefix, code: errReqBodyEmpty}
	// Request body too large
	ErrReqBodyTooLarge error = CustomError{prefix: errReqPrefix, code: errReqBodyTooLarge}

	// Duplicate request parameter
	ErrReqParamDuplicate error = CustomError{prefix: errReqPrefix, code: errReqParamDuplicate}
	// Invalid request parameter
	ErrReqParamInvalid error = CustomError{prefix: errReqPrefix, code: errReqParamInvalid}

	// Unsupported content type
	ErrReqContentTypeUnsupported = CustomError{prefix: errReqPrefix, code: errReqContentTypeUnsupported}
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
