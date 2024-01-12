package utils

// UnwrapError recursively.
func UnwrapError(err error) error {
	for err != nil {
		if wrappedErr, ok := err.(interface{ Unwrap() error }); ok {
			err = wrappedErr.Unwrap()
		} else {
			break
		}
	}

	return err
}
