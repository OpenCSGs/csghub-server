package component

import "errors"

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("forbidden")
	ErrUserNotFound     = errors.New("user not found, please login first")
	ErrAlreadyExists    = errors.New("the record already exists")
	ErrPermissionDenied = errors.New("permission denied")
)
