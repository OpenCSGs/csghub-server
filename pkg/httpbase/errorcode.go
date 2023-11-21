package httpbase

const (
	// CodeGeneralError normal or unclassified error
	CodeGeneralError = 1
	// CodeBadBinding binding failed
	CodeBadBinding              = 600001
	CodeDynamicalFormInputError = 600002
)

const (
	msgKeyGeneralError            = "internalError"
	msgKeyBindingError            = "bindingError"
	msgKeyDynamicalFormInputError = "dynamicalFormInputError"
)

// Error standard API error
type Error struct {
	// http status code
	statusCode int
	// business layer code
	code    int
	message string
	msgKey  string
}

// NewError create a new API error
func NewError(statusCode, code int, message, msgKey string) *Error {
	return &Error{
		statusCode: statusCode,
		code:       code,
		message:    message,
		msgKey:     msgKey,
	}
}

// Code returns error code
func (e *Error) Code() int {
	return e.code
}

// StatusCode returns http status code
func (e *Error) StatusCode() int {
	return e.statusCode
}

// MsgKey returns the i18n msgKey
func (e *Error) MsgKey() string {
	return e.msgKey
}

// Error implements error interface
func (e *Error) Error() string {
	return e.message
}
