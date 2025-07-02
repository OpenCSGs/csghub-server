package errorx

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var errorCodeRegex = regexp.MustCompile(`^([A-Z]+-ERR)-(\d+)$`)

func IsValidErrorCode(code string) bool {
	return errorCodeRegex.MatchString(code)
}

// ParseErrorString parses the corresponding error object from error code string
// Supports format: "PREFIX-ERR-NUMBER" (e.g.: "AUTH-ERR-1", "SYS-ERR-2")
// Returns corresponding CustomError instance, or returns generic error if parsing fails
func ParseErrorCode(errorCode string) CustomError {
	errUnknown := CustomError{
		Prefix: errUnknownPrefix,
		Code:   0,
	}

	matches := errorCodeRegex.FindStringSubmatch(errorCode)
	if len(matches) != 3 {
		return errUnknown
	}

	prefix := matches[1]
	codeNum, err := strconv.Atoi(matches[2])
	if err != nil {
		return errUnknown
	}

	return CustomError{
		Prefix: prefix,
		Code:   codeNum,
	}
}

type CoreError interface {
	Error() string
	Code() string
	CustomError() CustomError
}

// CustomError used as standard error struct
// CustomError.code for i18n
// eg. ErrUnauthorized.GetErrCode() returns "unauthorized"
// CustomError implements the error interface
type CustomError struct {
	Prefix  string  `json:"prefix"`
	Code    int     `json:"code"`
	Context context `json:"context,omitempty"`
}

func (err CustomError) Error() string {
	return err.Prefix + "-" + fmt.Sprintf("%d", err.Code)
}

func (err CustomError) Detail() string {
	errorMsg := errSysPrefix + "-" + fmt.Sprintf("%d", err.Code)
	if len(err.Context) > 0 {
		var auxParts []string
		for key, value := range err.Context {
			auxParts = append(auxParts, fmt.Sprintf("%s:%v", key, value))
		}
		errorMsg += " [" + strings.Join(auxParts, ", ") + "]"
	}

	return errorMsg
}

// used for errors.Is to check error type
func (err CustomError) Unwrap() error {
	switch err.Prefix {
	case errSysPrefix:
		return errSysMap[errSysCode(err.Code)]
	case errReqPrefix:
		return errReqMap[errReqCode(err.Code)]
	default:
		return ErrUnknown
	}
}

var (
	ErrNotFound = errors.New("not found")
	// not enough permission for current user
	ErrAlreadyExists         = errors.New("the record already exists")
	ErrContentLengthTooLarge = errors.New("content length too large")
)

/*
var (
	// --- ASSET-ERR-xxx: Resource management (Model, Dataset, Code) ---
	ErrNotFound      = CustomError{code: "Asset-ERR-001", msg: "not found"}
	ErrAlreadyExists = CustomError{code: "Asset-ERR-002", msg: "the record already exists"}

	// --- TASK-ERR-xxx: Model Inference / Fine-tuning / Evaluation Tasks ---
	ErrTaskNotFound        = CustomError{code: "Task-ERR-001", msg: "task not found"}
	ErrTaskCreateFailed    = CustomError{code: "Task-ERR-002", msg: "failed to create task"}
	ErrTaskExecutionFailed = CustomError{code: "Task-ERR-003", msg: "task execution failed"}
	ErrTaskCancelled       = CustomError{code: "Task-ERR-004", msg: "task was cancelled"}
	ErrTaskTimeout         = CustomError{code: "Task-ERR-005", msg: "task timeout"}
	ErrTaskAlreadyExists   = CustomError{code: "Task-ERR-006", msg: "task already exists"}

	// --- DATA-ERR-xxx: Data related / Preview, Processing, etc. ---
	ErrDataNotFound         = CustomError{code: "Data-ERR-001", msg: "data not found"}
	ErrDataProcessingFailed = CustomError{code: "Data-ERR-002", msg: "data processing failed"}
	ErrDataInvalidFormat    = CustomError{code: "Data-ERR-003", msg: "invalid data format"}

	// --- Req-ERR-xxx: Request related errors ---
	ErrContentLengthTooLarge = CustomError{code: "Req-ERR-001", msg: "content length too large"}
	ErrBadRequest            = CustomError{code: "Req-ERR-002", msg: "bad request"}
)*/

// ErrForbiddenMsg returns a new ErrForbidden with extra message
func ErrForbiddenMsg(msg string) error {
	return fmt.Errorf("%s, %w", msg, ErrForbidden)
}

type HTTPError struct {
	StatusCode int
	Message    any
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("StatusCode: %d, Message: %v", e.StatusCode, e.Message)
}
