//go:build !ee && !saas

package component

import "opencsg.com/csghub-server/builder/store/database"

// newExtendOpenaiForTest creates an extendOpenai struct with DB-backed stores
// for E2E tests. In CE variant, extendOpenai is empty.
func newExtendOpenaiForTest(db *database.DB) extendOpenai {
	return extendOpenai{}
}
