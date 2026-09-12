//go:build !ee && !saas

package component

import (
	"context"
	"testing"

	mockrebac "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rebac"
	"opencsg.com/csghub-server/builder/store/database"
)

// setupSoftDeleteRepositoryAuthorizationCleanup does not configure EE/SaaS-only cleanup expectations for CE tests.
func setupSoftDeleteRepositoryAuthorizationCleanup(*testing.T, context.Context, *mockrebac.MockAuthorizer, database.User) database.RepositoryAuthorizationStore {
	return nil
}

// setupDeleteRepositoryAuthorizationCleanup does not configure EE/SaaS-only cleanup expectations for CE tests.
func setupDeleteRepositoryAuthorizationCleanup(*testing.T, context.Context, *mockrebac.MockAuthorizer, database.User) database.RepositoryAuthorizationStore {
	return nil
}
