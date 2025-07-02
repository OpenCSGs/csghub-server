package errorx

import (
	"fmt"
)

const errAuthPrefix = "AUTH-ERR"

type errAuth struct {
	code errAuthCode
}

func (err errAuth) Error() string {
	return fmt.Sprintf("%d", err.code)
}

func (err errAuth) Code() string {
	return errAuthPrefix + "-" + fmt.Sprintf("%d", err.code)
}

type errAuthCode int

const (
	unauthorized errAuthCode = iota
	userNotFound
	forbidden
	noEmail
)

var (
	// --- Auth-ERR-xxxx: User and Permission related errors ---
	// not allowed for anoymous user (need to login first)
	ErrUnauthorized = errAuth{code: unauthorized}
	ErrUserNotFound = errAuth{code: userNotFound}
	// not enough permission for current user
	ErrForbidden = errAuth{code: forbidden}
	ErrNoEmail   = errAuth{code: noEmail} // please set your email firs
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
