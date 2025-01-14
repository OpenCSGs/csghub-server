package component

import (
	"errors"
	"fmt"
)

var (
	// not allowed for anoymous user (need to login first)
	ErrUnauthorized = errors.New("unauthorized")
	ErrNotFound     = errors.New("not found")
	// not enough permission for current user
	ErrForbidden        = errors.New("forbidden")
	ErrUserNotFound     = errors.New("user not found, please login first")
	ErrAlreadyExists    = errors.New("the record already exists")
	ErrPermissionDenied = errors.New("permission denied")
)

// ErrForbiddenMsg returns a new ErrForbidden with extra message
func ErrForbiddenMsg(msg string) error {
	return fmt.Errorf("%s, %w", msg, ErrForbidden)
}
