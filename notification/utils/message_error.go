package utils

import "github.com/pkg/errors"

type ErrSendMsg struct {
	Err error
}

func NewErrSendMsg(err error, msg string) error {
	return &ErrSendMsg{Err: errors.Wrap(err, msg)}
}

func (e *ErrSendMsg) Error() string {
	return e.Err.Error()
}

func IsErrSendMsg(err error) bool {
	_, ok := err.(*ErrSendMsg)
	return ok
}
