package errorx

// UnwrapError recursively.
//
// If more than one error wrapped, use UnwrapAllError
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

func UnwrapAllError(err error) []error {
	if err == nil {
		return nil
	}

	var result []error
	result = append(result, err)

	if unwrapper, ok := err.(interface{ Unwrap() []error }); ok {
		for _, subErr := range unwrapper.Unwrap() {
			result = append(result, UnwrapAllError(subErr)...)
		}
		return result
	}

	if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
		if subErr := unwrapper.Unwrap(); subErr != nil {
			result = append(result, UnwrapAllError(subErr)...)
		}
	}

	return result
}

func GetCustomErrors(err error) []error {
	errors := UnwrapAllError(err)
	var customErrors []error
	for i := len(errors) - 1; i >= 0; i-- {
		if customError, ok := errors[i].(CustomError); ok {
			customErrors = append(customErrors, customError)
		}
	}
	return customErrors
}

func GetFirstCustomError(err error) (error, bool) {
	errors := UnwrapAllError(err)
	for i := len(errors) - 1; i >= 0; i-- {
		if customError, ok := errors[i].(CustomError); ok {
			return customError, true
		}
	}
	return err, false
}
