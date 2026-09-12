package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

// TestRepositoryAuthorizationStore_CRUD verifies direct repository grants and their idempotent mutations.
func TestRepositoryAuthorizationStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.Background()
	store := database.NewRepositoryAuthorizationStoreWithDB(db)
	const repositoryID int64 = 900001

	user := &database.User{Username: "repo-auth-user", UUID: "repo-auth-user"}
	require.NoError(t, database.NewUserStoreWithDB(db).Create(ctx, user, &database.Namespace{Path: "repo-auth-user"}))
	organization := &database.Organization{Nickname: "repo-auth-org", Name: "repo-auth-org", UUID: uuid.New()}
	require.NoError(t, database.NewOrgStoreWithDB(db).Create(ctx, organization, &database.Namespace{Path: "repo-auth-org"}))

	organizationGrant := &database.RepositoryAuthorization{
		RepositoryID: repositoryID,
		SubjectType:  types.RepoAuthSubjectOrganization,
		SubjectID:    organization.ID,
		SubjectUUID:  "org-original-uuid",
		Role:         types.UserWrite,
		CreateUserID: 400001,
	}
	require.NoError(t, store.Create(ctx, organizationGrant))
	require.NotZero(t, organizationGrant.ID)

	found, err := store.Find(ctx, repositoryID, organization.ID, types.RepoAuthSubjectOrganization)
	require.NoError(t, err)
	require.Equal(t, "org-original-uuid", found.SubjectUUID)
	require.Equal(t, types.UserWrite, found.Role)

	// Creating the same repository-subject pair updates the existing grant instead of inserting a duplicate.
	updatedOrganizationGrant := &database.RepositoryAuthorization{
		RepositoryID: repositoryID,
		SubjectType:  types.RepoAuthSubjectOrganization,
		SubjectID:    organization.ID,
		SubjectUUID:  "org-updated-uuid",
		Role:         types.UserRead,
		CreateUserID: 400002,
	}
	require.NoError(t, store.Create(ctx, updatedOrganizationGrant))

	found, err = store.Find(ctx, repositoryID, organization.ID, types.RepoAuthSubjectOrganization)
	require.NoError(t, err)
	require.Equal(t, organizationGrant.ID, found.ID)
	require.Equal(t, "org-updated-uuid", found.SubjectUUID)
	require.Equal(t, types.UserRead, found.Role)
	require.Equal(t, int64(400001), found.CreateUserID)

	userGrant := &database.RepositoryAuthorization{
		RepositoryID: repositoryID,
		SubjectType:  types.RepoAuthSubjectUser,
		SubjectID:    user.ID,
		SubjectUUID:  "user-uuid",
		Role:         types.UserRead,
		CreateUserID: 400002,
	}
	require.NoError(t, store.Create(ctx, userGrant))

	items, count, err := store.List(ctx, repositoryID, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Len(t, items, 1)
	require.Equal(t, user.ID, items[0].SubjectID)

	items, count, err = store.List(ctx, repositoryID, 0, 0)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Len(t, items, 2)

	allItems, err := store.ListByRepository(ctx, repositoryID)
	require.NoError(t, err)
	require.Len(t, allItems, 2)

	require.NoError(t, store.UpdateRole(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser, types.UserWrite))
	found, err = store.Find(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser)
	require.NoError(t, err)
	require.Equal(t, types.UserWrite, found.Role)

	userItems, err := store.ListBySubject(ctx, types.RepoAuthSubjectUser, user.ID)
	require.NoError(t, err)
	require.Len(t, userItems, 1)
	require.Equal(t, repositoryID, userItems[0].RepositoryID)

	require.NoError(t, store.Delete(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser))
	require.NoError(t, store.Delete(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser))
	_, err = store.Find(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser)
	require.ErrorIs(t, err, errorx.ErrDatabaseNoRows)
	require.ErrorIs(t, store.UpdateRole(ctx, repositoryID, user.ID, types.RepoAuthSubjectUser, types.UserRead), errorx.ErrRepoAuthorizationNotFound)

	require.NoError(t, store.DeleteBySubject(ctx, types.RepoAuthSubjectOrganization, organization.ID))
	require.NoError(t, store.DeleteBySubject(ctx, types.RepoAuthSubjectOrganization, organization.ID))
	organizationItems, err := store.ListBySubject(ctx, types.RepoAuthSubjectOrganization, organization.ID)
	require.NoError(t, err)
	require.Empty(t, organizationItems)

	require.NoError(t, store.DeleteByRepository(ctx, repositoryID))
	require.NoError(t, store.DeleteByRepository(ctx, repositoryID))
	items, count, err = store.List(ctx, repositoryID, 50, 1)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Empty(t, items)
}

// TestRepositoryAuthorizationStore_RejectsUnsupportedRole verifies the store role allowlist.
func TestRepositoryAuthorizationStore_RejectsUnsupportedRole(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	store := database.NewRepositoryAuthorizationStoreWithDB(db)
	authorization := &database.RepositoryAuthorization{
		RepositoryID: 900002,
		SubjectType:  types.RepoAuthSubjectUser,
		SubjectID:    300002,
		SubjectUUID:  "user-uuid",
		Role:         types.UserAdmin,
		CreateUserID: 400001,
	}

	require.ErrorIs(t, store.Create(context.Background(), authorization), errorx.ErrReqParamInvalid)
	require.ErrorIs(t, store.UpdateRole(context.Background(), authorization.RepositoryID, authorization.SubjectID, authorization.SubjectType, types.UserAdmin), errorx.ErrReqParamInvalid)
}

// TestRepositoryAuthorizationStore_RejectsUnsupportedSubjectType verifies both store validation and the database constraint.
func TestRepositoryAuthorizationStore_RejectsUnsupportedSubjectType(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()

	ctx := context.Background()
	store := database.NewRepositoryAuthorizationStoreWithDB(db)
	invalidSubjectType := types.RepoAuthSubjectType("invalid")
	authorization := &database.RepositoryAuthorization{
		RepositoryID: 900003,
		SubjectType:  invalidSubjectType,
		SubjectID:    300003,
		SubjectUUID:  "invalid-subject-uuid",
		Role:         types.UserRead,
		CreateUserID: 400001,
	}

	require.ErrorIs(t, store.Create(ctx, authorization), errorx.ErrReqParamInvalid)
	_, err := store.Find(ctx, authorization.RepositoryID, authorization.SubjectID, invalidSubjectType)
	require.ErrorIs(t, err, errorx.ErrReqParamInvalid)
	require.ErrorIs(t, store.UpdateRole(ctx, authorization.RepositoryID, authorization.SubjectID, invalidSubjectType, types.UserWrite), errorx.ErrReqParamInvalid)
	require.ErrorIs(t, store.Delete(ctx, authorization.RepositoryID, authorization.SubjectID, invalidSubjectType), errorx.ErrReqParamInvalid)
	require.ErrorIs(t, store.DeleteBySubject(ctx, invalidSubjectType, authorization.SubjectID), errorx.ErrReqParamInvalid)
	_, err = store.ListBySubject(ctx, invalidSubjectType, authorization.SubjectID)
	require.ErrorIs(t, err, errorx.ErrReqParamInvalid)

	_, err = db.Operator.Core.NewInsert().Model(authorization).Exec(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository_authorizations_subject_type_check")
}
