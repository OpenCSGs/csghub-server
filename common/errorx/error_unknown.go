package errorx

import "fmt"

const errUnknownPrefix = "Unknown-ERR"

type errUnknown struct {
	code int
}

func (err errUnknown) Error() string {
	return fmt.Sprintf("%d", err.code)
}

func (err errUnknown) Code() string {
	return errUnknownPrefix + "-" + fmt.Sprintf("%d", err.code)
}

func (err errUnknown) CustomError() CustomError {
	return CustomError{
		Prefix: errUnknownPrefix,
		Code:   err.code,
	}
}

var ErrUnknown = errUnknown{code: 0}
