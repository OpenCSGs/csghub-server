package errorx

import (
	"database/sql"
	"errors"
	"strings"
)

// used to check error type

const errSysPrefix = "SYS-ERR"

const (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	internalServerError = iota
	remoteServiceFail
	// When select in DB, encounter connection failure or other error
	databaseFailure
	// Replace sql.ErrNoRows
	databaseNoRows
	databaseDuplicateKey

	lfsNotFound

	lastOrgAdmin
)

var (
	// --- SYS-ERR-xxx: System / Service exceptions ---
	// Used when marshal error, type convert error
	ErrInternalServerError error = CustomError{prefix: errSysPrefix, code: internalServerError}
	// Used in httpClient, then need to convert it to specific error, such as ErrUserServiceFailure
	ErrRemoteServiceFail = CustomError{prefix: errSysPrefix, code: remoteServiceFail}
	// Used to instead of sql.ErrConnDone and other unhandled error
	ErrDatabaseFailure = CustomError{prefix: errSysPrefix, code: databaseFailure}
	// Used to instead of sql.ErrNoRows
	//
	// Convert it to specific error in component or handler
	ErrDatabaseNoRows       = CustomError{prefix: errSysPrefix, code: databaseNoRows}
	ErrDatabaseDuplicateKey = CustomError{prefix: errSysPrefix, code: databaseDuplicateKey}

	ErrLFSNotFound = CustomError{prefix: errSysPrefix, code: lfsNotFound}

	ErrLastOrgAdmin = CustomError{prefix: errSysPrefix, code: lastOrgAdmin}
)

// Used in DB to convert db error to custom error
//
// Add new error in future
func HandleDBError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		err:     err,
	}
	if errors.Is(err, sql.ErrNoRows) {
		customErr.code = int(databaseNoRows)
		return customErr
	} else if strings.Contains(err.Error(), "duplicate key value") {
		customErr.code = int(databaseDuplicateKey)
		return customErr
	} else {
		customErr.code = int(databaseFailure)
		return customErr
	}
}

func InternalServerError(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		code:    internalServerError,
		err:     err,
		context: ctx,
	}
	return customErr
}

// Used to convert service error to custom error
func RemoteSvcFail(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		code:    int(remoteServiceFail),
		err:     err,
	}
	return customErr
}

func LFSNotFound(err error, ctx context) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errSysPrefix,
		context: ctx,
		code:    int(lfsNotFound),
		err:     err,
	}
	return customErr
}

func LastOrgAdmin(err error, ctx context) error {
	if err == nil {
		return nil
	}
	return CustomError{
		prefix:  errSysPrefix,
		err:     err,
		code:    lastOrgAdmin,
		context: ctx,
	}
}
