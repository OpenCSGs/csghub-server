package errorx

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			assert.Contains(t, unwrapErr, ErrDatabaseNoRows)
			assert.Contains(t, unwrapErr, CustomError{
				Prefix:  errSysPrefix,
				Code:    int(databaseNoRows),
				Context: map[string]interface{}{"tt": "tt"},
			})

			getCustomErr := GetCustomErrors(getErr)
			assert.Equal(t, 2, len(getCustomErr))
			assert.Equal(t, true, IsValidErrorCode(getCustomErr[0].Error()))
			assert.Equal(t, getCustomErr[0], CustomError{
				Prefix:  errSysPrefix,
				Code:    int(databaseNoRows),
				Context: map[string]interface{}{"tt": "tt"},
			})

			getFirstCustom, ok := GetFirstCustomError(getErr)
			assert.Equal(t, true, ok)
			assert.Equal(t, getFirstCustom, CustomError{
				Prefix:  errSysPrefix,
				Code:    int(databaseNoRows),
				Context: map[string]interface{}{"tt": "tt"},
			})
		},
	)
	t.Run(
		"Test_ErrDatabaseFailure",
		func(t *testing.T) {
			getErr := HandleDBError(sql.ErrConnDone, nil)
			unwrapErr := UnwrapAllError(getErr)
			assert.Contains(t, unwrapErr, ErrDatabaseFailure)

			getCustomErr := GetCustomErrors(getErr)
			assert.Equal(t, 2, len(getCustomErr))
			assert.Equal(t, true, errors.Is(getCustomErr[0], ErrDatabaseFailure))
			assert.Equal(t, true, errors.Is(getErr, sql.ErrConnDone))
		},
	)
}
