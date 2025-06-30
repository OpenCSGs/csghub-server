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
	for _, err := range errors {
		if IsValidErrorCode(err.Error()) {
			customErrors = append(customErrors, err)
		} else if coreError, ok := err.(CoreError); ok {
			customErrors = append(customErrors, coreError.CustomError())
		}
	}
	return customErrors
}

func GetFirstCustomError(err error) (error, bool) {
	errors := UnwrapAllError(err)
	for _, err := range errors {
		if IsValidErrorCode(err.Error()) {
			return err, true
		} else if coreError, ok := err.(CoreError); ok {
			return coreError.CustomError(), true
		}
	}
	return err, false
}
