package component

import "errors"

var (
	ErrUnauthorized = errors.New("permission denied")
	ErrNotFound     = errors.New("not found")
)
