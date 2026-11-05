package database

import (
	"database/sql"
	"errors"
	"fmt"

	"opencsg.com/csghub-server/common/errorx"
)

// resourceLookupError marks only a missing primary resource as not found.
// Keep the cause so callers can still recognize existing database errors.
func resourceLookupError(err error, namespace, name string) error {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errorx.ErrDatabaseNoRows) {
		return errorx.RepoNotFound(err, errorx.Ctx().Set("path", fmt.Sprintf("%s/%s", namespace, name)))
	}
	return err
}
