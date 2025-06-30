package errorx

import "fmt"

const errReqPrefix = "REQ-ERR"

type errReqCode int

type errReq struct {
	code errReqCode
}

func (err errReq) Error() string {
	return fmt.Sprintf("%d", err.code)
}

func (err errReq) Code() string {
	return errReqPrefix + "-" + fmt.Sprintf("%d", err.code)
}

func (err errReq) CustomError() CustomError {
	return CustomError{
		Prefix: errReqPrefix,
		Code:   int(err.code),
	}
}

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
	ErrBadRequest = errReq{code: errBadRequest}

	// Request parameter format error
	ErrReqBodyFormat = errReq{code: errReqBodyFormat}
	// Request body is empty
	ErrReqBodyEmpty = errReq{code: errReqBodyEmpty}
	// Request body too large
	ErrReqBodyTooLarge = errReq{code: errReqBodyTooLarge}

	// Duplicate request parameter
	ErrReqParamDuplicate = errReq{code: errReqParamDuplicate}
	// Invalid request parameter
	ErrReqParamInvalid = errReq{code: errReqParamInvalid}

	// Unsupported content type
	ErrReqContentTypeUnsupported = errReq{code: errReqContentTypeUnsupported}
)

var errReqMap = map[errReqCode]errReq{
	errBadRequest:      ErrBadRequest,
	errReqBodyFormat:   ErrReqBodyFormat,
	errReqBodyEmpty:    ErrReqBodyEmpty,
	errReqBodyTooLarge: ErrReqBodyTooLarge,

	errReqParamDuplicate: ErrReqParamDuplicate,
	errReqParamInvalid:   ErrReqParamInvalid,

	errReqContentTypeUnsupported: ErrReqContentTypeUnsupported,
}

func HandleBadRequest(originErr error, coreErr errReq, ext context) error {
	customErr := coreErr.CustomError()
	customErr.Context = ext
	return fmt.Errorf("%w, %w", originErr, customErr)
}

func ReqBodyFormat(err error, ext context) error {
	customErr := ErrReqBodyFormat.CustomError()
	customErr.Context = ext
	return fmt.Errorf("%w, %w", err, customErr)
}

func ReqParamInvalid(err error, ext context) error {
	customErr := ErrReqParamInvalid.CustomError()
	customErr.Context = ext
	return fmt.Errorf("%w, %w", err, customErr)
}
