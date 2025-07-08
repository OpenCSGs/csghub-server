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
func ParseError(errorCode string, defaultErr CustomError, ctx context) CustomError {
	errDefault := CustomError{
		prefix:  defaultErr.prefix,
		code:    defaultErr.code,
		context: ctx,
	}

	matches := errorCodeRegex.FindStringSubmatch(errorCode)
	if len(matches) != 3 {
		return errDefault
	}

	prefix := matches[1]
	codeNum, err := strconv.Atoi(matches[2])
	if err != nil {
		return errDefault
	}

	return CustomError{
		prefix:  prefix,
		code:    codeNum,
		context: ctx,
	}
}

// CustomError used as standard error struct
// CustomError.code for i18n
// eg. ErrUnauthorized.GetErrCode() returns "unauthorized"
// CustomError implements the error interface
type CustomError struct {
	prefix  string
	code    int
	err     error
	context context
}

func NewCustomError(prefix string, code int, err error, context context) CustomError {
	return CustomError{
		prefix:  prefix,
		code:    code,
		err:     err,
		context: context,
	}
}

func (err CustomError) Is(other error) bool {
	if other == nil {
		return false
	}
	if customErr, ok := other.(CustomError); ok {
		return err.prefix == customErr.prefix && err.code == customErr.code
	}
	return false
}

func (err CustomError) Error() string {
	if err.err == nil {
		return fmt.Sprintf("%s-%d", err.prefix, err.code)
	} else {
		return fmt.Sprintf("%s-%d: %s", err.prefix, err.code, err.err.Error())
	}
}

func (err CustomError) Code() string {
	return fmt.Sprintf("%s-%d", err.prefix, err.code)
}

func (err CustomError) Context() context {
	return err.context
}

func (err CustomError) Detail() string {
	errorMsg := fmt.Sprintf("%s-%d", err.prefix, err.code)
	if len(err.context) > 0 {
		var auxParts []string
		for key, value := range err.context {
			auxParts = append(auxParts, fmt.Sprintf("%s:%v", key, value))
		}
		errorMsg += " [" + strings.Join(auxParts, ", ") + "]"
	}

	return errorMsg
}

// used for errors.Is to check error type
func (err CustomError) Unwrap() error {
	return err.err
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
