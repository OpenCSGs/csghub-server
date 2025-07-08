package errorx

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CustomErr_Error(t *testing.T) {
	t.Run(
		"Test_Error_With_Original_Error",
		func(t *testing.T) {
			// Create a custom error with a wrapped error
			originalErr := fmt.Errorf("original error")
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
				err:    originalErr,
			}

			// Check the error message
			expectedMsg := fmt.Sprintf("%s-%d: %s", errSysPrefix, internalServerError, originalErr.Error())
			assert.Equal(t, expectedMsg, customErr.Error())
		},
	)
	t.Run(
		"Test_Error_Without_Original_Error",
		func(t *testing.T) {
			// Create a custom error without a wrapped error
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Check the error message
			expectedMsg := fmt.Sprintf("%s-%d", errSysPrefix, internalServerError)
			assert.Equal(t, expectedMsg, customErr.Error())
		},
	)
}

func Test_CustomErr_Code(t *testing.T) {
	t.Run(
		"Test_Code_With_Original_Error",
		func(t *testing.T) {
			// Create a custom error with a wrapped error
			originalErr := fmt.Errorf("original error")
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
				err:    originalErr,
			}

			// Check the code
			expectedCode := fmt.Sprintf("%s-%d", errSysPrefix, internalServerError)
			assert.Equal(t, expectedCode, customErr.Code())
		},
	)
	t.Run(
		"Test_Code_Without_Original_Error",
		func(t *testing.T) {
			// Create a custom error without a wrapped error
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Check the code
			expectedCode := fmt.Sprintf("%s-%d", errSysPrefix, internalServerError)
			assert.Equal(t, expectedCode, customErr.Code())
		},
	)
}

func Test_CustomErr_Is(t *testing.T) {
	t.Run(
		"Test_Is_With_Same_Prefix_And_Code",
		func(t *testing.T) {
			// Create a custom error
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Create another custom error with the same prefix and code
			sameErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Check if they are equal
			assert.True(t, customErr.Is(sameErr))
		},
	)
	t.Run(
		"Test_Is_With_Different_Prefix",
		func(t *testing.T) {
			// Create a custom error
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Create another custom error with a different prefix
			differentErr := CustomError{
				prefix: "DIFF-PREFIX",
				code:   internalServerError,
			}

			// Check if they are equal
			assert.False(t, customErr.Is(differentErr))
		},
	)
}

func Test_CustomErr_Unwrap(t *testing.T) {
	t.Run(
		"Test_Unwrap_With_Original_Error",
		func(t *testing.T) {
			// Create a custom error with a wrapped error
			originalErr := fmt.Errorf("original error")
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
				err:    originalErr,
			}

			// Unwrap the custom error
			unwrappedErr := UnwrapError(customErr)

			// Check if the unwrapped error is the original error
			assert.Equal(t, originalErr, unwrappedErr)
		},
	)
	t.Run(
		"Test_Unwrap_Without_Original_Error",
		func(t *testing.T) {
			// Create a custom error without a wrapped error
			customErr := CustomError{
				prefix: errSysPrefix,
				code:   internalServerError,
			}

			// Unwrap the custom error
			unwrappedErr := UnwrapError(customErr)

			// Check if the unwrapped error is nil
			assert.Nil(t, unwrappedErr)
		},
	)
}

func Test_Err_SYS_Core(t *testing.T) {
	t.Run(
		"Test_ErrDatabaseNoRows",
		func(t *testing.T) {
			getErr := fmt.Errorf("error msg: %w", ErrDatabaseNoRows)
			unwrapErr := UnwrapAllError(getErr)
			assert.Contains(t, unwrapErr, ErrDatabaseNoRows)

			getCustomErr, ok := GetFirstCustomError(getErr)
			assert.Equal(t, true, ok)
			assert.Equal(t, true, errors.Is(getCustomErr, ErrDatabaseNoRows))
			assert.Equal(t, true, IsValidErrorCode(getCustomErr.Error()))
		},
	)
	t.Run(
		"Test_ErrDatabaseFailure",
		func(t *testing.T) {
			getErr := fmt.Errorf("error msg: %w", ErrDatabaseFailure)
			unwrapErr := UnwrapAllError(getErr)
			assert.Contains(t, unwrapErr, ErrDatabaseFailure)

			getCustomErr, ok := GetFirstCustomError(getErr)
			assert.Equal(t, true, ok)
			assert.Equal(t, true, errors.Is(getCustomErr, ErrDatabaseFailure))
			assert.Equal(t, true, IsValidErrorCode(getCustomErr.Error()))
		},
	)
}

func Test_Err_SYS_HandleDB(t *testing.T) {
	t.Run(
		"Test_ErrDatabaseNoRows",
		func(t *testing.T) {
			getErr := HandleDBError(sql.ErrNoRows, map[string]interface{}{"tt": "tt"})
			assert.Equal(t, true, errors.Is(getErr, ErrDatabaseNoRows))
			assert.Equal(t, true, errors.Is(getErr, sql.ErrNoRows))

			unwrapErr := UnwrapAllError(getErr)
			assert.Contains(t, unwrapErr, CustomError{
				prefix:  errSysPrefix,
				code:    int(databaseNoRows),
				err:     sql.ErrNoRows,
				context: map[string]interface{}{"tt": "tt"},
			})

			getCustomErr := GetCustomErrors(getErr)
			assert.Equal(t, 1, len(getCustomErr))
			assert.Equal(t, true, IsValidErrorCode(getCustomErr[0].(CustomError).Code()))
			assert.Equal(t, getCustomErr[0], CustomError{
				prefix:  errSysPrefix,
				code:    int(databaseNoRows),
				err:     sql.ErrNoRows,
				context: map[string]interface{}{"tt": "tt"},
			})

			getFirstCustom, ok := GetFirstCustomError(getErr)
			assert.Equal(t, true, ok)
			assert.Equal(t, getFirstCustom, CustomError{
				prefix:  errSysPrefix,
				code:    int(databaseNoRows),
				err:     sql.ErrNoRows,
				context: map[string]interface{}{"tt": "tt"},
			})
		},
	)
	t.Run(
		"Test_ErrDatabaseFailure",
		func(t *testing.T) {
			getErr := HandleDBError(sql.ErrConnDone, nil)
			unwrapErr := UnwrapAllError(getErr)
			assert.Contains(t, unwrapErr, CustomError{
				prefix: errSysPrefix,
				code:   int(databaseFailure),
				err:    sql.ErrConnDone,
			})

			getCustomErr := GetCustomErrors(getErr)
			assert.Equal(t, 1, len(getCustomErr))
			assert.Equal(t, true, errors.Is(getCustomErr[0], ErrDatabaseFailure))
			assert.Equal(t, true, errors.Is(getErr, sql.ErrConnDone))
		},
	)
}
