package errorx

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// used to check error type

const errSysPrefix = "SYS-ERR"

type errSysCode int

type errSys struct {
	code errSysCode
}

func (err errSys) Error() string {
	return fmt.Sprintf("%d", err.code)
}

func (err errSys) Code() string {
	return errSysPrefix + "-" + fmt.Sprintf("%d", err.code)
}

func (err errSys) CustomError() CustomError {
	return CustomError{
		Prefix: errSysPrefix,
		Code:   int(err.code),
	}
}

const (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	internalServerError errSysCode = iota
	remoteServiceFail
	internalServiceFailure
	// When select in DB, encounter connection failure or other error
	databaseFailure
	// Replace sql.ErrNoRows
	databaseNoRows
	databaseDuplicateKey
)

var (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	// Used when marshal error, type convert error
	ErrInternalServerError = errSys{code: internalServerError}
	ErrRemoteServiceFail   = errSys{code: remoteServiceFail}
	// Used in httpClient, then need to convert it to specific error, such as ErrUserServiceFailure
	ErrInternalServiceFailure = errSys{code: internalServiceFailure}
	// Used to instead of sql.ErrConnDone and other unhandled error
	ErrDatabaseFailure = errSys{code: databaseFailure}
	// Used to instead of sql.ErrNoRows
	//
	// Convert it to specific error in component or handler
	ErrDatabaseNoRows       = errSys{code: databaseNoRows}
	ErrDatabaseDuplicateKey = errSys{code: databaseDuplicateKey}
)

var errSysMap = map[errSysCode]errSys{
	internalServerError:    ErrInternalServerError,
	remoteServiceFail:      ErrRemoteServiceFail,
	internalServiceFailure: ErrInternalServiceFailure,
	databaseFailure:        ErrDatabaseFailure,
	databaseNoRows:         ErrDatabaseNoRows,
	databaseDuplicateKey:   ErrDatabaseDuplicateKey,
}

// Used in DB to convert db error to custom error
//
// Add new error in future
func HandleDBError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		Prefix:  errSysPrefix,
		Context: ctx,
	}
	if errors.Is(err, sql.ErrNoRows) {
		customErr.Code = int(databaseNoRows)
		return fmt.Errorf("%w, %w", err, customErr)
	} else if strings.Contains(err.Error(), "duplicate key value") {
		customErr.Code = int(databaseDuplicateKey)
		return fmt.Errorf("%w, %w", err, customErr)
	} else {
		customErr.Code = int(databaseFailure)
		return fmt.Errorf("%w, %w", err, customErr)
	}
}
