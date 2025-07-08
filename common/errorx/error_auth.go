package errorx

import (
	"errors"
)

const errAuthPrefix = "AUTH-ERR"

const (
	unauthorized = iota
	userNotFound
	forbidden
	noEmail
	invalidJWT
	invalidAuthHeader
	notAdmin
	userNotMatch
	needUUID
	needAPIKey
)

var (
	// --- Auth-ERR-xxxx: User and Permission related errors ---
	// not allowed for anoymous user (need to login first)
	ErrUnauthorized error = CustomError{prefix: errAuthPrefix, code: unauthorized}
	ErrUserNotFound error = CustomError{prefix: errAuthPrefix, code: userNotFound}
	// not enough permission for current user
	ErrForbidden error = CustomError{prefix: errAuthPrefix, code: forbidden}
	ErrNoEmail   error = CustomError{prefix: errAuthPrefix, code: noEmail} // please set your email firs

	ErrInvalidJWT        error = CustomError{prefix: errAuthPrefix, code: invalidJWT}
	ErrInvalidAuthHeader error = CustomError{prefix: errAuthPrefix, code: invalidAuthHeader}
	ErrUserNotAdmin      error = CustomError{prefix: errAuthPrefix, code: notAdmin}
	ErrUserNotMatch      error = CustomError{prefix: errAuthPrefix, code: userNotMatch}
	ErrNeedUUID          error = CustomError{prefix: errAuthPrefix, code: needUUID}   // need uuid in request header or body to identify user account
	ErrNeedAPIKey        error = CustomError{prefix: errAuthPrefix, code: needAPIKey} // need api key in request header or body to identify user account
)

/*
func ErrUnauthorized() errAuth {
	return errUnauthorized
}

func ErrUserNotFound() errAuth {
	return errUserNotFound
}

func ErrPermissionDenied() errAuth {
	return errPermissionDenied
}

func ErrForbidden() errAuth {
	return errForbidden
}
*/

func InvalidJWT(err error, errCtx context) error {
	customErr := CustomError{
		prefix:  errAuthPrefix,
		code:    invalidJWT,
		err:     err,
		context: errCtx,
	}
	return customErr
}

func InvalidAuthHeader(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    invalidAuthHeader,
		err:     err,
		context: errCtx,
	}
}

func UserNotFound(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    userNotFound,
		err:     err,
		context: errCtx,
	}
}

func UserNotMatch(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    userNotMatch,
		err:     err,
		context: errCtx,
	}
}

func NeedUUID(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    needUUID,
		err:     err,
		context: errCtx,
	}
}

func NeedAPIKey(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    needAPIKey,
		err:     err,
		context: errCtx,
	}
}

func UserNotAdmin(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    notAdmin,
		err:     err,
		context: errCtx,
	}
}

func Forbidden(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    forbidden,
		err:     err,
		context: errCtx,
	}
}

func NoEmail(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    noEmail,
		err:     err,
		context: errCtx,
	}
}

func Unauthorized(err error, errCtx context) error {
	return CustomError{
		prefix:  errAuthPrefix,
		code:    unauthorized,
		err:     err,
		context: errCtx,
	}
}

// ErrForbiddenMsg returns a new ErrForbidden with extra message
func ErrForbiddenMsg(msg string) error {

	return CustomError{
		prefix:  errAuthPrefix,
		code:    forbidden,
		err:     errors.New(msg),
		context: nil,
	}
}
